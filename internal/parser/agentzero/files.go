package agentzero

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pytext"
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
	// lastContextKeys are the connector .env keys read, and only these.
	lastContextKeys = []string{"AGENT_ZERO_LAST_CONTEXT_HOST", "AGENT_ZERO_LAST_CONTEXT_ID"}
)

// blank reports whether value is null or empty text.
func blank(value jsonvalue.Value) bool {
	return value.IsNull() || (value.IsString() && value.StringValue() == "")
}

// key returns a field as row text, "-" when it is absent or empty.
func key(record jsonvalue.Value, name string) string {
	if value := record.Get(name); !blank(value) {
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
	document, err := jsonvalue.Decode(content)
	if err != nil {
		return jsonvalue.NewNull(), "not JSON (" + err.Error() + ")"
	}
	return document, ""
}

// firstNonEmpty returns the first of the texts that is not empty.
func firstNonEmpty(texts ...string) string {
	for _, each := range texts {
		if each != "" {
			return each
		}
	}
	return ""
}

// orDefault returns a value as text, or fallback when Python counts it as
// false.
func orDefault(value jsonvalue.Value, fallback string) string {
	if !value.IsEmpty() {
		return value.Text()
	}
	return fallback
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
		emit(p.fileRow(artifact, "parser: scheduler tasks file "+firstNonEmpty(why, "has no tasks list"),
			"", "", ""))
		return
	}
	for i, task := range tasks.Items() {
		number := i + 1
		if !task.IsObject() {
			problems.AddRaw(number, "unparseable record "+strconv.Itoa(number)+" (not a JSON object)",
				parser.CompactJSON(task))
			continue
		}
		p.taskRows(artifact, number, task, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// taskRows emits the rows of one scheduler task.
func (p *Parser) taskRows(artifact *input.Artifact, number int, task jsonvalue.Value, emit parser.Emit) {
	sessionID := parser.Str(firstTruthyValue(task.Get("context_id"), task.Get("uuid")))
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
	if !result.IsString() || pytext.Strip(result.StringValue()) == "" {
		return
	}
	text, turn := result.StringValue(), timeline.TurnAssistant
	if isString(task.Get("state"), "error") && strings.HasPrefix(text, "ERROR: ") {
		text, turn = "scheduled task failed: "+text[len("ERROR: "):], timeline.TurnSystem
	}
	row = p.fileRow(artifact, text, timeutil.FromValue(task.Get("last_run")), sessionID, projectPath)
	row.TurnType, row.SourceLine = turn, number
	emit(row)
}

// firstTruthyValue returns the first of the values that is true in Python,
// else null.
func firstTruthyValue(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values {
		if !value.IsEmpty() {
			return value
		}
	}
	return jsonvalue.NewNull()
}

// isString reports whether value is the string want.
func isString(value jsonvalue.Value, want string) bool {
	return value.IsString() && value.StringValue() == want
}

// parseTelegram emits a "session deleted:" row per Telegram chat whose chat
// file is not in the input. A failure to look for a chat file stops the
// parse and is returned, as it is in Python.
func (p *Parser) parseTelegram(artifact *input.Artifact, emit parser.Emit) error {
	var problems parser.Problems
	document, why := loadDocument(artifact, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return nil
	}
	chats := document.Get("chats")
	if !chats.IsObject() {
		emit(p.fileRow(artifact, "parser: state.json "+firstNonEmpty(why, "has no chats map"), "", "", ""))
		return nil
	}
	if _, found := parser.Above(artifact, 2); !found {
		return nil
	}
	for _, member := range chats.Members() {
		contextID := parser.Str(member.Value)
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
// being two levels above the state file's directory. An id that is not a
// plain file name is a chat that is not there. A failure other than the
// path being absent is returned.
func chatPresent(artifact *input.Artifact, contextID string) (bool, error) {
	name, named := parser.Component(contextID)
	nested, found := parser.Above(artifact, 2, "chats", name, "chat.json")
	flat, _ := parser.Above(artifact, 2, "chats", name+".json")
	if !named || !found {
		return false, nil
	}
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
// plugin config: a _tool_access policy or the Orchestrator's settings.
func (p *Parser) parsePolicy(artifact *input.Artifact, match []string, emit parser.Emit) {
	var problems parser.Problems
	document, why := loadDocument(artifact, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.fileRow(artifact, "parser: "+artifact.Rel+" "+firstNonEmpty(why, "is not a JSON object"),
			"", "", ""))
		return
	}
	scope, project := p.scopeText(match), projectOf(jsonvalue.NewString(groupOf(p.policy, match, "project")))
	if groupOf(p.policy, match, "plugin") == "_tool_access" {
		for _, name := range policyScalars {
			if value := document.Get(name); !blank(value) {
				emit(p.settingRow(artifact, "tool_access."+name+"="+parser.SettingValue(value)+scope, project))
			}
		}
		for _, name := range policyLists {
			if entries := document.Get(name); entries.IsArray() {
				for _, entry := range entries.Items() {
					emit(p.settingRow(artifact, "tool_access."+name+"="+parser.SettingValue(entry)+scope, project))
				}
			}
		}
		return
	}
	for _, each := range orchestratorKeys {
		if section := document.Get(each[0]); section.IsObject() && !blank(section.Get(each[1])) {
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

// parseConnector emits the "session origin:" row of the A0 CLI connector's
// ~/.agent-zero/.env. The file is a secret: only the last chat's host and id
// are read, as the connector reads them, and no other line reaches a row.
func (p *Parser) parseConnector(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	values := map[string]string{}
	for _, line := range pythonLines(pytext.DecodeUTF8(content)) {
		name, value, found := strings.Cut(pytext.Strip(line), "=")
		name = pytext.Strip(name)
		if found && (name == lastContextKeys[0] || name == lastContextKeys[1]) {
			values[name] = strings.Trim(pytext.Strip(value), `"'`)
		}
	}
	host, contextID := values[lastContextKeys[0]], values[lastContextKeys[1]]
	if host != "" && contextID != "" {
		emit(p.fileRow(artifact, "session origin: remote agent-zero instance "+host+" context "+
			contextID+" (from .agent-zero/.env, file mtime)", parser.FileMtime(artifact), contextID, ""))
	}
}

// pythonLines splits text at the line boundaries of Python's str.splitlines:
// LF, CR, CRLF, vertical tab, form feed, U+001C to U+001E, U+0085, U+2028
// and U+2029.
func pythonLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); {
		char, width := pytext.NextCodePoint(text[i:])
		switch char {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			lines = append(lines, text[start:i])
			start = i + width
		case '\r':
			lines = append(lines, text[start:i])
			if strings.HasPrefix(text[i:], "\r\n") {
				width = 2
			}
			start = i + width
		}
		i += width
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
