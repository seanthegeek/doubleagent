package agentzero

import (
	"cmp"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The keys and sections read from the plugin and connector files.
var (
	// telegramKeys are the data keys of a Telegram chat, in row order.
	telegramKeys = []string{"telegram_bot", "telegram_chat_id", "telegram_user_id",
		"telegram_username"}
	// policyScalars and policyLists are the policy keys of _tool_access.
	policyScalars = []string{"mode", "default", "mcp_default"}
	policyLists   = []string{"allowed", "blocked"}
	// orchestratorKeys are the approval bypass of each coding CLI the
	// Orchestrator drives; nothing else of the file is read.
	orchestratorKeys = [][2]string{{"codex", "bypass_sandbox"}, {"claude", "permission_mode"},
		{"claude", "allowed_tools"}, {"cursor", "force"}, {"grok", "always_approve"},
		{"hermes", "yolo"}, {"opencode", "auto"}}
	// lastContextKeys are the connector .env keys of the last chat used.
	lastContextKeys = []string{"AGENT_ZERO_LAST_CONTEXT_HOST", "AGENT_ZERO_LAST_CONTEXT_ID"}
)

// key returns a field as row text, "-" when it is absent or empty.
func key(record jsonvalue.Value, name string) string {
	if value := record.Get(name); parser.Present(value) {
		return value.Text()
	}
	return "-"
}

// projectOf returns the container path of an Agent Zero project, "" for
// none.
func projectOf(name jsonvalue.Value) string {
	if !name.IsEmpty() {
		return "/a0/usr/projects/" + name.Text()
	}
	return ""
}

// fileRow returns a system row of a file beside the chats.
func (p *Parser) fileRow(artifact *input.Artifact, text, timestamp, sessionID, projectPath string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.TimestampUTC = timestamp
	row.SessionID = sessionID
	row.ProjectPath = projectPath
	return row
}

// loadDocument returns the file as JSON and whether it decoded, or why it
// did not: "not JSON (<reason>)". A file that cannot be read adds its
// unreadable problem to problems.
func loadDocument(artifact *input.Artifact, problems *parser.Problems) (jsonvalue.Value, string) {
	content, ok := parser.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.NewNull(), ""
	}
	document, err := parser.DecodeJSONBytes(artifact, content)
	if err != nil {
		return jsonvalue.NewNull(), "not JSON (" + err.Error() + ")"
	}
	return document, ""
}

// parseTasks emits the rows of the scheduler's tasks.json: per task a
// "scheduled task:" row at created_at, the prompt as the chat logged it with
// no time of its own, and the last result at last_run.
func (p *Parser) parseTasks(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, why := loadDocument(artifact, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	tasks := document.Get("tasks")
	if !tasks.IsArray() {
		emit(p.fileRow(artifact, "parser: scheduler tasks file "+cmp.Or(why, "has no tasks list"),
			"", "", ""))
		return
	}
	for i, task := range tasks.Items() {
		number := i + 1
		if !task.IsObject() {
			problems.AddRaw(number, "unparseable record "+strconv.Itoa(number)+" (not a JSON object)",
				parser.JSONText(task))
			continue
		}
		p.taskRows(artifact, number, task, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// taskRows emits the rows of one scheduler task.
func (p *Parser) taskRows(artifact *input.Artifact, number int, task jsonvalue.Value, emit parser.Emit) {
	sessionID := parser.Text(parser.FirstNonEmpty(task.Get("context_id"), task.Get("uuid")))
	projectPath := projectOf(task.Get("project_name"))
	row := p.fileRow(artifact, "scheduled task: "+key(task, "name")+" type="+key(task, "type")+
		" state="+key(task, "state"), timeutil.FromValue(task.Get("created_at")), sessionID, projectPath)
	row.SourceLine = number
	emit(row)
	if prompt := task.Get("prompt"); !prompt.IsEmpty() {
		// The scheduler logs "## Task:\n<prompt>" as the chat's user message
		// when the task has no context.
		row = p.fileRow(artifact, "## Task:\n"+prompt.Text(), "", sessionID, projectPath)
		row.TurnType, row.SourceLine = timeline.TurnUser, number
		emit(row)
	}
	result := task.Get("last_result")
	if !result.IsString() || strings.TrimSpace(result.StringValue()) == "" {
		return
	}
	text, turn := result.StringValue(), timeline.TurnAssistant
	if parser.IsText(task.Get("state"), "error") && strings.HasPrefix(text, "ERROR: ") {
		text, turn = "scheduled task failed: "+text[len("ERROR: "):], timeline.TurnSystem
	}
	row = p.fileRow(artifact, text, timeutil.FromValue(task.Get("last_run")), sessionID, projectPath)
	row.TurnType, row.SourceLine = turn, number
	emit(row)
}

// parseTelegram emits a "session deleted:" row per Telegram chat whose chat
// file is not in the input. A failure to look for a chat file, other than
// the file being absent, stops the parse and is returned.
func (p *Parser) parseTelegram(artifact *input.Artifact, emit parser.Emit) error {
	var problems parser.Problems
	document, why := loadDocument(artifact, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return nil
	}
	chats := document.Get("chats")
	if !chats.IsObject() {
		emit(p.fileRow(artifact, "parser: state.json "+cmp.Or(why, "has no chats map"), "", "", ""))
		return nil
	}
	for _, member := range chats.Members() {
		contextID := parser.Text(member.Value)
		if contextID == "" {
			continue
		}
		present, err := chatPresent(artifact, contextID)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		bot, user, chat := member.Key, "-", "-"
		if parts := strings.Split(member.Key, ":"); len(parts) >= 3 {
			last := len(parts)
			bot, user, chat = strings.Join(parts[:last-2], ":"), parts[last-2], parts[last-1]
		}
		emit(p.fileRow(artifact, "session deleted: telegram bot "+bot+" chat "+chat+" user "+user+
			" ctxid "+contextID+" (from state.json, no chat.json in the input, file mtime)",
			parser.FileMtime(artifact), contextID, ""))
	}
	return nil
}

// chatPresent reports whether the chat of a context id is among the files:
// usr/chats/<id>/chat.json or usr/chats/<id>.json as a regular file, usr
// being two levels above the state file's directory, which the state file's
// path (plugins/_telegram_integration/state.json at the least) always has
// below its home. An id that is not a plain file name is a chat that is not
// there. A failure other than the path being absent is returned.
func chatPresent(artifact *input.Artifact, contextID string) (bool, error) {
	name, named := parser.Component(contextID)
	if !named {
		return false, nil
	}
	nested, _ := parser.Above(artifact, 2, "chats", name, "chat.json")
	flat, _ := parser.Above(artifact, 2, "chats", name+".json")
	for _, path := range []string{nested, flat} {
		info, err := osfile.Lstat(path)
		if err != nil {
			if absentLookup(err) {
				continue
			}
			return false, err
		}
		if info.Mode().IsRegular() {
			return true, nil
		}
	}
	return false, nil
}

// absentLookup reports whether a failed look at a path is as good as the
// path being absent.
func absentLookup(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) ||
		errors.Is(err, syscall.ELOOP)
}

// settingRow returns the "permission setting:" row of a plugin file.
func (p *Parser) settingRow(artifact *input.Artifact, text, project string) timeline.Row {
	return p.fileRow(artifact, "permission setting: "+text+" (from "+artifact.Rel+", file mtime)",
		parser.FileMtime(artifact), "", project)
}

// scopeText returns " profile=<p> project=<n>" for a scoped plugin file,
// each part when it is set.
func (p *Parser) scopeText(match []string) string {
	var text string
	for _, name := range []string{"profile", "project"} {
		if value := groupOf(p.policy, match, name); value != "" {
			text += " " + name + "=" + value
		}
	}
	return text
}

// parsePolicy emits a "permission setting:" row per key and list entry of a
// plugin config: a _tool_access policy or the Orchestrator's settings. A
// policy list that is not an array adds a problem.
func (p *Parser) parsePolicy(artifact *input.Artifact, match []string, emit parser.Emit) {
	var problems parser.Problems
	document, why := loadDocument(artifact, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.fileRow(artifact, "parser: "+artifact.Rel+" "+cmp.Or(why, "is not a JSON object"),
			"", "", ""))
		return
	}
	scope, project := p.scopeText(match), projectOf(jsonvalue.NewString(groupOf(p.policy, match, "project")))
	if groupOf(p.policy, match, "plugin") == "_tool_access" {
		for _, name := range policyScalars {
			if value := document.Get(name); parser.Present(value) {
				emit(p.settingRow(artifact, "tool_access."+name+"="+parser.SettingValue(value)+scope, project))
			}
		}
		for _, name := range policyLists {
			for _, entry := range parser.ArrayMember(document.Get(name), name, 0, &problems) {
				emit(p.settingRow(artifact, "tool_access."+name+"="+parser.SettingValue(entry)+scope, project))
			}
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for _, each := range orchestratorKeys {
		if section := document.Get(each[0]); section.IsObject() && parser.Present(section.Get(each[1])) {
			emit(p.settingRow(artifact, "orchestrator."+each[0]+"."+each[1]+"="+
				parser.SettingValue(section.Get(each[1])), project))
		}
	}
}

// parseToggle emits the "permission setting:" row of a plugin's .toggle-1 or
// .toggle-0 file: the empty file switches the Infection Check or the
// Orchestrator on (1) or off (0).
func (p *Parser) parseToggle(artifact *input.Artifact, match []string, emit parser.Emit) {
	state := "disabled"
	if groupOf(p.toggle, match, "state") == "1" {
		state = "enabled"
	}
	text := strings.TrimLeft(groupOf(p.toggle, match, "plugin"), "_") + "=" + state
	for _, name := range []string{"profile", "project"} {
		if value := groupOf(p.toggle, match, name); value != "" {
			text += " " + name + "=" + value
		}
	}
	emit(p.settingRow(artifact, text, projectOf(jsonvalue.NewString(groupOf(p.toggle, match, "project")))))
}

// restoreTokenKey is the connector .env key of the computer-use restore
// token, the one credential the connector writes there; its other keys are
// the instance URL, the last chat's host and id and on/off flags
// (research/agent-zero.md, "Host-side companions").
const restoreTokenKey = "AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN"

// isRestoreTokenName reports whether a .env key is the restore token's, bare
// or after the word export and any white space.
func isRestoreTokenName(name string) bool {
	words := strings.Fields(name)
	if len(words) == 2 && words[0] == "export" {
		words = words[1:]
	}
	return len(words) == 1 && words[0] == restoreTokenKey
}

// parseConnector emits the rows of the A0 CLI connector's ~/.agent-zero/.env:
// one "connector settings:" row of every key in file order, the restore
// token through parser.SecretText (with an export prefix too) and every
// other value as written, and the "session origin:" row of the last chat's
// host and id.
func (p *Parser) parseConnector(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	// A line ends at LF; the CR of a CRLF line and the whitespace around a
	// key or value are trimmed, then every single or double quote at either
	// end of a value, as the connector's own reader strips them; a line that
	// starts with # is a comment, and an export prefix stays part of the key,
	// as that reader keeps it (research/agent-zero.md, "Origin").
	values := map[string]string{}
	var settings []string
	for line := range strings.Lines(textlines.ValidUTF8(content)) {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		values[name] = value
		if isRestoreTokenName(name) {
			value = parser.SecretText(artifact, value)
		}
		settings = append(settings, name+"="+value)
	}
	if len(settings) > 0 {
		emit(p.fileRow(artifact, "connector settings: "+strings.Join(settings, " ")+
			" (from .agent-zero/.env, file mtime)", parser.FileMtime(artifact), "", ""))
	}
	host, contextID := values[lastContextKeys[0]], values[lastContextKeys[1]]
	if host != "" && contextID != "" {
		emit(p.fileRow(artifact, "session origin: remote agent-zero instance "+host+
			" context "+contextID+" (from .agent-zero/.env, file mtime)", parser.FileMtime(artifact), contextID, ""))
	}
}
