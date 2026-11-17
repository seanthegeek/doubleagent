package claudecode

import (
	"fmt"
	"path"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The settings and ~/.claude.json keys read. Of the Remote Control keys,
// the machine id is an identity value and is never printed, only noted.
const (
	retentionKey          = "cleanupPeriodDays"
	modelKey              = "model"
	githubStatusKey       = "githubWebConnectionStatusCache"
	remoteControlMachine  = "remoteControlMachineId"
	remoteControlSpawnKey = "remoteControlSpawnMode"
	cloudEnvironmentKey   = "hasRemoteEnvironment"
)

// history emits one user row per line of history.jsonl, its paste
// placeholders expanded, then a row per problem.
func (r *Reader) history(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	historyDir := parser.Beside(artifact)
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := r.BaseRow(artifact)
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.SessionID = parser.Str(record.Get("sessionId"))
		row.ProjectPath = parser.Str(record.Get("project"))
		row.TurnType = timeline.TurnUser
		if display := record.Get("display"); display.IsString() {
			row.Text = timeline.Compact(expandPastes(historyDir, display.Text(),
				record.Get("pastedContents"), &problems))
		} else {
			row.Text = timeline.CompactValue(display)
		}
		emit(row)
	}
	parser.EmitAll(emit, r.ErrorRows(artifact, problems, "", ""))
}

// fileRow returns a system row at line 1 and the file's modification time.
func (r *Reader) fileRow(artifact *input.Artifact) timeline.Row {
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	row.TimestampUTC = parser.FileMtime(artifact)
	return row
}

// JSONObject returns the artifact's content as a JSON object, or false with
// the system row that says why it is not one: the unreadable row for a file
// that cannot be read, else "parser: <what> is not a JSON object" at line 1
// and the file's modification time.
func (r *Reader) JSONObject(artifact *input.Artifact, what string) (pyjson.Value, timeline.Row, bool) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return pyjson.NewNull(), r.ErrorRows(artifact, problems, "", "")[0], false
	}
	document, err := pyjson.Decode(data)
	if err == nil && document.IsObject() {
		return document, timeline.Row{}, true
	}
	row := r.fileRow(artifact)
	row.Text = "parser: " + what + " is not a JSON object"
	return pyjson.NewNull(), row, false
}

// ccrTip emits the "session synced:" row of a session's ccr-tip.json: the
// last claude.ai event the local side of a Remote Control session handled.
func (r *Reader) ccrTip(artifact *input.Artifact, emit parser.Emit) {
	session := ""
	if match := ccrTipPattern.FindStringSubmatch(artifact.Rel); match != nil {
		session = match[1]
	}
	document, bad, ok := r.JSONObject(artifact, "ccr-tip.json")
	if !ok {
		bad.SessionID = session
		emit(bad)
		return
	}
	row := r.fileRow(artifact)
	row.SessionID = session
	row.Text = timeline.Compact(fmt.Sprintf("%s remote event %s at %s (from %s, file mtime)",
		timeline.SyncedPrefix, valueText(document, "eventId"), valueText(document, "updatedAt"),
		artifact.Rel))
	emit(row)
}

// bridgePointer emits the "remote control:" row of a project's
// bridge-pointer.json, the Remote Control environment last enabled for it.
// Its sessionId is the bridge's, not a transcript's, so the row has none.
func (r *Reader) bridgePointer(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := r.JSONObject(artifact, "bridge-pointer.json")
	if !ok {
		emit(bad)
		return
	}
	row := r.fileRow(artifact)
	row.Text = timeline.Compact(fmt.Sprintf(
		"remote control: environment %s session %s source=%s (from %s, file mtime)",
		valueText(document, "environmentId"), valueText(document, "sessionId"),
		valueText(document, "source"), artifact.Rel))
	emit(row)
}

// registry emits the row of a process registry file sessions/<pid>.json,
// and a "session synced:" row when the process ran a Remote Control bridge.
func (r *Reader) registry(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, r.ErrorRows(artifact, problems, "", ""))
		return
	}
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	record, err := pyjson.Decode(data)
	if err != nil || !record.IsObject() {
		row.Text = "parser: session registry file is not a JSON object"
		emit(row)
		return
	}
	pid := strings.TrimSuffix(path.Base(artifact.Rel), ".json")
	row.TimestampUTC = timeutil.FromValue(record.Get("startedAt"))
	row.SessionID = strOr(record.Get("sessionId"), "")
	row.ProjectPath = strOr(record.Get("cwd"), "")
	row.Text = fmt.Sprintf("claude-code process %s %s status=%s entrypoint=%s bridge=%s host_session=%s",
		strOr(record.Get("pid"), pid), strOr(record.Get("kind"), "-"),
		strOr(record.Get("status"), "-"), strOr(record.Get("entrypoint"), "-"),
		strOr(record.Get("bridgeSessionId"), "none"), strOr(record.Get("hostSessionId"), "none"))
	emit(row)
	if bridge := record.Get("bridgeSessionId"); bridge.IsString() && bridge.Text() != "" {
		sync := r.fileRow(artifact)
		sync.SessionID = row.SessionID
		sync.ProjectPath = row.ProjectPath
		sync.Text = fmt.Sprintf("%s remote control bridge %s url - (from %s, file mtime)",
			timeline.SyncedPrefix, bridge.Text(), artifact.Rel)
		emit(sync)
	}
}

// StandingRow returns the system row of a standing setting at the given
// time and project: "<label>: <key>=<value> (from <rel>, file mtime)".
func (r *Reader) StandingRow(artifact *input.Artifact, timestamp, project, key string,
	value pyjson.Value, label string) timeline.Row {
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	row.TimestampUTC = timestamp
	row.ProjectPath = project
	row.Text = timeline.Compact(fmt.Sprintf("%s: %s=%s (from %s, file mtime)", label, key,
		parser.SettingValue(value), artifact.Rel))
	return row
}

// PermissionSetting is the label of a permission's standing row.
const PermissionSetting = "permission setting"

// settings emits the permission, retention and model rows of a settings
// file: the permissions lists one row per entry, then the scalars.
func (r *Reader) settings(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := r.JSONObject(artifact, "settings file")
	if !ok {
		emit(bad)
		return
	}
	timestamp, project := parser.FileMtime(artifact), ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	standing := func(key string, value pyjson.Value, label string) {
		emit(r.StandingRow(artifact, timestamp, project, key, value, label))
	}
	if permissions := document.Get("permissions"); permissions.IsObject() {
		for _, key := range []string{"allow", "deny", "ask", "additionalDirectories"} {
			for _, entry := range permissions.Get(key).Items() {
				standing("permissions."+key, entry, PermissionSetting)
			}
		}
		for _, key := range []string{"defaultMode", "disableBypassPermissionsMode"} {
			if value := permissions.Get(key); !value.IsNull() {
				standing("permissions."+key, value, PermissionSetting)
			}
		}
	}
	if value := document.Get(retentionKey); !value.IsNull() {
		standing(retentionKey, value, "retention setting")
	}
	if value := document.Get(modelKey); value.IsString() && value.Text() != "" {
		standing(modelKey, value, "model setting")
	}
}

// globalConfig emits the rows of ~/.claude.json: each project's allowed
// tools, trust flags and Remote Control spawn mode, then the GitHub app
// connection, the Remote Control machine id and the cloud flag.
func (r *Reader) globalConfig(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := r.JSONObject(artifact, ".claude.json")
	if !ok {
		emit(bad)
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, member := range document.Get("projects").Members() {
		r.projectRows(artifact, timestamp, member.Key, member.Value, emit)
	}
	if github := document.Get(githubStatusKey); github.IsObject() {
		// accountUuid and orgUuid are identity values: never printed.
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = timeline.Compact(fmt.Sprintf("source host: github status=%s (from %s, file mtime)",
			valueText(github, "status"), artifact.Rel))
		emit(row)
	}
	if !isMissing(document.Get(remoteControlMachine)) {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = "remote control: machine id set (from " + artifact.Rel + ", file mtime)"
		emit(row)
	}
	if value := document.Get(cloudEnvironmentKey); !value.IsNull() {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = timeline.Compact(fmt.Sprintf("cloud: %s=%s (from %s, file mtime)",
			cloudEnvironmentKey, parser.SettingValue(value), artifact.Rel))
		emit(row)
	}
}

// projectRows emits the rows of one project of ~/.claude.json.
func (r *Reader) projectRows(artifact *input.Artifact, timestamp, projectPath string,
	project pyjson.Value, emit parser.Emit) {
	if !project.IsObject() {
		return
	}
	for _, entry := range project.Get("allowedTools").Items() {
		emit(r.StandingRow(artifact, timestamp, projectPath, "allowedTools", entry, PermissionSetting))
	}
	for _, key := range []string{"hasTrustDialogAccepted", "hasClaudeMdExternalIncludesApproved"} {
		if value := project.Get(key); !value.IsNull() {
			emit(r.StandingRow(artifact, timestamp, projectPath, key, value, PermissionSetting))
		}
	}
	if spawn := project.Get(remoteControlSpawnKey); !isMissing(spawn) {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.ProjectPath = projectPath
		row.Text = timeline.Compact(fmt.Sprintf(
			"remote control: spawn mode %s for %s (from %s, file mtime)",
			parser.SettingValue(spawn), projectPath, artifact.Rel))
		emit(row)
	}
}
