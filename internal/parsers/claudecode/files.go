package claudecode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The settings and ~/.claude.json keys read. The Remote Control machine id
// and the GitHub connection's account and organisation ids are identity
// values, not credentials, and are shown as written.
const (
	retentionKey          = "cleanupPeriodDays"
	modelKey              = "model"
	githubStatusKey       = "githubWebConnectionStatusCache"
	remoteControlMachine  = "remoteControlMachineId"
	remoteControlSpawnKey = "remoteControlSpawnMode"
	cloudEnvironmentKey   = "hasRemoteEnvironment"
)

// history emits one user row per line of history.jsonl, its paste
// placeholders standing for their texts (expandPastes), then a row per
// problem.
func (r *Reader) history(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	// A directory the kit refuses comes back empty, and Under refuses that.
	historyDir, _ := parserkit.Beside(artifact)
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		row := r.BaseRow(artifact)
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.SessionID = parserkit.Text(record.Get("sessionId"))
		row.ProjectPath = parserkit.Text(record.Get("project"))
		row.TurnType = timeline.TurnUser
		if display := record.Get("display"); display.IsString() {
			var out timeline.TextBuilder
			r.expandPastes(&out, artifact.Home, historyDir, display.StringValue(),
				record.Get("pastedContents"), &problems)
			out.Set(&row)
		} else {
			row.Text = timeline.CompactValue(display)
		}
		emit(row)
	}
	parserkit.EmitAll(emit, r.ErrorRows(artifact, problems, "", ""))
}

// fileRow returns a system row at line 1 and the file's modification time.
func (r *Reader) fileRow(artifact *input.Artifact) timeline.Row {
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	row.TimestampUTC = parserkit.FileMtime(artifact)
	return row
}

// JSONObject returns the artifact's content as a JSON object, or false with
// the system row that says why it is not one: the unreadable row for a file
// that cannot be read, else "parser: <what> is not a JSON object" at line 1
// and the file's modification time.
func (r *Reader) JSONObject(artifact *input.Artifact, what string) (jsonvalue.Value, timeline.Row, bool) {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return jsonvalue.NewNull(), r.ErrorRows(artifact, problems, "", "")[0], false
	}
	document, err := parserkit.DecodeJSONBytes(artifact, data)
	if err == nil && document.IsObject() {
		return document, timeline.Row{}, true
	}
	row := r.fileRow(artifact)
	row.Text = "parser: " + what + " is not a JSON object"
	return jsonvalue.NewNull(), row, false
}

// ccrTip emits the "session synced:" row of a session's ccr-tip.json: the
// last claude.ai event the local side of a Remote Control session handled.
func (r *Reader) ccrTip(artifact *input.Artifact, emit parserkit.Emit) {
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
	row.Text = timeline.Compact(timeline.SyncedPrefix + " remote event " + parserkit.SettingOrDash(document.Get("eventId")) +
		" at " + parserkit.SettingOrDash(document.Get("updatedAt")) + " (from " + artifact.Rel + ", file mtime)")
	emit(row)
}

// bridgePointer emits the "remote control:" row of a project's
// bridge-pointer.json, the Remote Control environment last enabled for it.
// Its sessionId is the bridge's, not a transcript's, so the row has none.
func (r *Reader) bridgePointer(artifact *input.Artifact, emit parserkit.Emit) {
	document, bad, ok := r.JSONObject(artifact, "bridge-pointer.json")
	if !ok {
		emit(bad)
		return
	}
	row := r.fileRow(artifact)
	row.Text = timeline.Compact("remote control: environment " + parserkit.SettingOrDash(document.Get("environmentId")) +
		" session " + parserkit.SettingOrDash(document.Get("sessionId")) + " source=" + parserkit.SettingOrDash(document.Get("source")) +
		" (from " + artifact.Rel + ", file mtime)")
	emit(row)
}

// registry emits the row of a process registry file sessions/<pid>.json,
// and a "session synced:" row when the process ran a Remote Control bridge.
func (r *Reader) registry(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, r.ErrorRows(artifact, problems, "", ""))
		return
	}
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	record, err := parserkit.DecodeJSONBytes(artifact, data)
	if err != nil || !record.IsObject() {
		row.Text = "parser: session registry file is not a JSON object"
		emit(row)
		return
	}
	pid := registryPattern.FindStringSubmatch(artifact.Rel)[1]
	row.TimestampUTC = timeutil.FromValue(record.Get("startedAt"))
	row.SessionID = record.Get("sessionId").Text()
	row.ProjectPath = record.Get("cwd").Text()
	row.Text = "claude-code process " + textOr(record.Get("pid"), pid) + " " + textOr(record.Get("kind"), "-") +
		" status=" + textOr(record.Get("status"), "-") + " entrypoint=" + textOr(record.Get("entrypoint"), "-") +
		" bridge=" + textOr(record.Get("bridgeSessionId"), "none") +
		" host_session=" + textOr(record.Get("hostSessionId"), "none")
	emit(row)
	if bridge := record.Get("bridgeSessionId"); bridge.IsString() && bridge.StringValue() != "" {
		sync := r.fileRow(artifact)
		sync.SessionID = row.SessionID
		sync.ProjectPath = row.ProjectPath
		sync.Text = timeline.SyncedPrefix + " remote control bridge " + bridge.StringValue() +
			" url - (from " + artifact.Rel + ", file mtime)"
		emit(sync)
	}
}

// StandingRow returns the system row of a standing setting at the given
// time and project: "<label>: <key>=<value> (from <rel>, file mtime)".
func (r *Reader) StandingRow(artifact *input.Artifact, timestamp, project, key string,
	value jsonvalue.Value, label string) timeline.Row {
	row := r.BaseRow(artifact)
	row.SourceLine = 1
	row.TurnType = timeline.TurnSystem
	row.TimestampUTC = timestamp
	row.ProjectPath = project
	row.Text = timeline.Compact(label + ": " + key + "=" + parserkit.SettingValue(value) +
		" (from " + artifact.Rel + ", file mtime)")
	return row
}

// PermissionSetting is the label of a permission's standing row.
const PermissionSetting = "permission setting"

// settings emits the permission, retention and model rows of a settings
// file: the permissions lists one row per entry, then the scalars, then
// the problem of a permissions or permissions list of the wrong kind.
func (r *Reader) settings(artifact *input.Artifact, emit parserkit.Emit) {
	document, bad, ok := r.JSONObject(artifact, "settings file")
	if !ok {
		emit(bad)
		return
	}
	timestamp, project := parserkit.FileMtime(artifact), ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	standing := func(key string, value jsonvalue.Value, label string) {
		emit(r.StandingRow(artifact, timestamp, project, key, value, label))
	}
	var problems parserkit.Problems
	permissions := document.Get("permissions")
	parserkit.ObjectMember(permissions, "permissions", 1, &problems)
	if permissions.IsObject() {
		for _, key := range []string{"allow", "deny", "ask", "additionalDirectories"} {
			for _, entry := range parserkit.ArrayMember(permissions.Get(key), "permissions."+key, 1, &problems) {
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
	if value := document.Get(modelKey); value.IsString() && value.StringValue() != "" {
		standing(modelKey, value, "model setting")
	}
	r.problemRows(artifact, problems, emit)
}

// problemRows emits a "parser:" row per problem of a JSON file, at line 1
// and the file's modification time, as the row of a file that is not a
// JSON object is.
func (r *Reader) problemRows(artifact *input.Artifact, problems parserkit.Problems, emit parserkit.Emit) {
	for _, problem := range problems {
		row := r.fileRow(artifact)
		row.Text = parserkit.ProblemPrefix + problem.Message
		emit(row)
	}
}

// globalConfig emits the rows of ~/.claude.json: each project's allowed
// tools, trust flags and Remote Control spawn mode, then the GitHub app
// connection, the Remote Control machine id and the cloud flag, then the
// problems of a projects, project or allowedTools of the wrong kind.
func (r *Reader) globalConfig(artifact *input.Artifact, emit parserkit.Emit) {
	document, bad, ok := r.JSONObject(artifact, ".claude.json")
	if !ok {
		emit(bad)
		return
	}
	timestamp := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	for _, member := range parserkit.ObjectMember(document.Get("projects"), "projects", 1, &problems) {
		r.projectRows(artifact, timestamp, member.Key, member.Value, &problems, emit)
	}
	if github := document.Get(githubStatusKey); github.IsObject() {
		text := "source host: github status=" + parserkit.SettingOrDash(github.Get("status"))
		for _, pair := range [][2]string{{"accountUuid", "account"}, {"orgUuid", "org"}} {
			if value := github.Get(pair[0]); parserkit.Present(value) {
				text += " " + pair[1] + "=" + parserkit.SettingValue(value)
			}
		}
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = timeline.Compact(text + " (from " + artifact.Rel + ", file mtime)")
		emit(row)
	}
	if machine := document.Get(remoteControlMachine); parserkit.Present(machine) {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = timeline.Compact("remote control: machine id " + parserkit.SettingValue(machine) + " (from " +
			artifact.Rel + ", file mtime)")
		emit(row)
	}
	if value := document.Get(cloudEnvironmentKey); !value.IsNull() {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.Text = timeline.Compact("cloud: " + cloudEnvironmentKey + "=" + parserkit.SettingValue(value) +
			" (from " + artifact.Rel + ", file mtime)")
		emit(row)
	}
	r.problemRows(artifact, problems, emit)
}

// projectRows emits the rows of one project of ~/.claude.json. A project
// that is not a JSON object, or its allowedTools that is not an array,
// adds a problem.
func (r *Reader) projectRows(artifact *input.Artifact, timestamp, projectPath string,
	project jsonvalue.Value, problems *parserkit.Problems, emit parserkit.Emit) {
	member := "projects." + projectPath
	if !project.IsObject() {
		parserkit.ObjectMember(project, member, 1, problems)
		return
	}
	for _, entry := range parserkit.ArrayMember(project.Get("allowedTools"), member+".allowedTools", 1, problems) {
		emit(r.StandingRow(artifact, timestamp, projectPath, "allowedTools", entry, PermissionSetting))
	}
	for _, key := range []string{"hasTrustDialogAccepted", "hasClaudeMdExternalIncludesApproved"} {
		if value := project.Get(key); !value.IsNull() {
			emit(r.StandingRow(artifact, timestamp, projectPath, key, value, PermissionSetting))
		}
	}
	if spawn := project.Get(remoteControlSpawnKey); parserkit.Present(spawn) {
		row := r.fileRow(artifact)
		row.TimestampUTC = timestamp
		row.ProjectPath = projectPath
		row.Text = timeline.Compact("remote control: spawn mode " + parserkit.SettingValue(spawn) + " for " +
			projectPath + " (from " + artifact.Rel + ", file mtime)")
		emit(row)
	}
}
