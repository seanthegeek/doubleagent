package claudedesktop

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// text returns a value's str(), "" for null.
func text(value pyjson.Value) string {
	if value.IsNull() {
		return ""
	}
	return value.Str()
}

// boolText returns true or false as JSON spells them, any other value's
// str(), and "" for null.
func boolText(value pyjson.Value) string {
	if flag, ok := value.Bool(); ok {
		if flag {
			return "true"
		}
		return "false"
	}
	return text(value)
}

// orDash returns value, or "-" when it is empty.
func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// transcript emits the rows of a transcript in Claude Code's format. When
// the session record names a folder, every row but a scratch directory's
// takes it as project path. session, the session dir relative to the
// home, is "" for a staged transcript, which has no record.
func (p *Parser) transcript(artifact *input.Artifact, options parser.Options, session string,
	emit parser.Emit) {
	project := ""
	if session != "" {
		record, _ := findRecord(artifact.Home, session)
		project = folder(record)
	}
	p.code.Session(artifact, options, func(row timeline.Row) {
		if project != "" && !(row.TurnType == timeline.TurnSystem &&
			strings.HasPrefix(row.Text, timeline.ScratchPrefix)) {
			row.ProjectPath = project
		}
		emit(row)
	})
}

// initCwd returns the cwd of the audit log's first system init line that
// has one, the CLI's working directory, or "".
func initCwd(artifact *input.Artifact) string {
	var ignored parser.Problems
	for _, line := range parser.JSONLines(artifact.DiskPath, &ignored) {
		if isText(line.Get("type"), "system") && isText(line.Get("subtype"), "init") {
			if cwd := text(line.Get("cwd")); cwd != "" {
				return cwd
			}
		}
	}
	return ""
}

// isText reports whether value is the string want.
func isText(value pyjson.Value, want string) bool {
	return value.IsString() && value.Text() == want
}

// auditLine is the state of an audit log's parse and the line being read.
type auditLine struct {
	parser    *Parser
	artifact  *input.Artifact
	options   parser.Options
	emit      parser.Emit
	project   string
	models    parser.Tracker
	line      int
	timestamp string
	sessionID string
}

// row returns a row of the current line with the turn type and text given
// (text compacted).
func (a *auditLine) row(turnType, text string) timeline.Row {
	row := a.parser.BaseRow(a.artifact)
	row.SourceLine = a.line
	row.TimestampUTC = a.timestamp
	row.SessionID = a.sessionID
	row.ProjectPath = a.project
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// audit emits the rows of a session dir's audit.jsonl, session being the
// dir relative to the home. The project is the
// record's folder, else the CLI's working directory, else the record's cwd;
// the session is the record's CLI session id, else each line's own.
func (p *Parser) audit(artifact *input.Artifact, options parser.Options, session string,
	emit parser.Emit) {
	record, found := findRecord(artifact.Home, session)
	project := folder(record)
	if project == "" {
		project = initCwd(artifact)
	}
	if project == "" && found && record.Truthy() {
		project = text(record.Get("cwd"))
	}
	cliID := ""
	if found && record.Truthy() {
		cliID = text(record.Get("cliSessionId"))
	}
	state := &auditLine{parser: p, artifact: artifact, options: options, emit: emit,
		project: project, models: parser.Tracker{}}
	var problems parser.Problems
	lastSessionID := cliID
	for number, line := range parser.JSONLines(artifact.DiskPath, &problems) {
		state.line = number
		state.sessionID = cliID
		if state.sessionID == "" {
			state.sessionID = text(line.Get("session_id"))
		}
		if state.sessionID != "" {
			lastSessionID = state.sessionID
		}
		state.timestamp = timeutil.FirstNonEmpty(timeutil.FromValue(line.Get("timestamp")),
			timeutil.FromValue(line.Get("_audit_timestamp")))
		state.read(line)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, lastSessionID, project))
}

// read emits the rows of one audit line, by its type and subtype.
func (a *auditLine) read(line pyjson.Value) {
	lineType, subtype := line.Get("type"), line.Get("subtype")
	isSystem := isText(lineType, "system")
	switch {
	case isSystem && (isText(subtype, "permission_request") || isText(subtype, "permission_response") ||
		isText(subtype, "permission_auto_approved") || isText(subtype, "permission_auto_denied")):
		a.permission(line, subtype.Text())
	case isText(lineType, "result"):
		a.emit(a.row(timeline.TurnSystem, "result: "+text(subtype)+" turns="+
			text(line.Get("num_turns"))+" error="+boolText(line.Get("is_error"))))
	case isSystem && isText(subtype, "init"):
		// The session's model, even when a transcript exists.
		a.modelRow(text(line.Get("model")))
		var parts []string
		for _, key := range []string{"cwd", "model"} {
			if value := line.Get(key); value.Truthy() {
				parts = append(parts, key+"="+value.Str())
			}
		}
		message := "init"
		if len(parts) > 0 {
			message = "init: " + strings.Join(parts, " ")
		}
		a.emit(a.row(timeline.TurnSystem, message))
	case isSystem && isText(subtype, "model_refusal_fallback"):
		// The subtype is in the bundle, its fields are not recorded: they
		// are printed as they are.
		a.emit(a.row(timeline.TurnSystem, "model_refusal_fallback: "+
			parser.CompactJSON(withoutEnvelope(line))))
	case isText(lineType, "user"):
		a.user(line)
	case isText(lineType, "assistant"):
		a.modelRow(text(line.Get("message").Get("model")))
		a.assistant(line)
	}
}

// modelRow emits the "model:" row of the session's first or changed model.
func (a *auditLine) modelRow(model string) {
	if change := a.models.Change("model", a.sessionID, model); change != "" {
		row := a.row(timeline.TurnSystem, change)
		row.Model = model
		a.emit(row)
	}
}

// withoutEnvelope returns an audit line without its envelope keys.
func withoutEnvelope(line pyjson.Value) pyjson.Value {
	var kept []pyjson.Member
	for _, member := range line.Members() {
		switch member.Key {
		case "type", "subtype", "uuid", "session_id", "timestamp", "_audit_timestamp",
			"_audit_hmac", "_audit_redactions":
			continue
		}
		kept = append(kept, member)
	}
	return pyjson.NewObject(kept...)
}

// permission emits the row of a permission request, a response, or an
// automatic approval or denial with what decided it.
func (a *auditLine) permission(line pyjson.Value, subtype string) {
	tool := text(line.Get("tool_name"))
	message := ""
	switch subtype {
	case "permission_request":
		summary := ""
		if toolInput := line.Get("tool_input"); !toolInput.IsNull() {
			summary = claudecode.ToolSummary(tool, toolInput)
		}
		message = "permission request: " + tool + " " + summary
	case "permission_response":
		message = "permission response: " + tool + " " + text(line.Get("decision")) + " granted=" +
			boolText(line.Get("granted"))
	default:
		word := "denied"
		if subtype == "permission_auto_approved" {
			word = "approved"
		}
		message = "permission auto-" + word + ": " + tool
		if why := autoReason(line); why != "" {
			message += " (" + why + ")"
		}
	}
	row := a.row(timeline.TurnSystem, message)
	row.ToolName = tool
	a.emit(row)
}

// autoReason returns what decided an automatic approval or denial: its
// source, else the scheduled task, else the matched operations as JSON, or
// "".
func autoReason(line pyjson.Value) string {
	why := line.Get("source")
	if !why.Truthy() {
		why = line.Get("scheduled_task_id")
	}
	if why.Truthy() {
		return why.Str()
	}
	if matched := line.Get("matched_ops"); !matched.IsNull() {
		return parser.CompactJSON(matched)
	}
	return ""
}

// blocks returns a message's content as blocks: a list as it is, anything
// else as one text block holding it.
func blocks(content pyjson.Value) []pyjson.Value {
	if content.IsArray() {
		return content.Items()
	}
	return []pyjson.Value{pyjson.NewObject(pyjson.Member{Key: "type", Value: pyjson.NewString("text")},
		pyjson.Member{Key: "text", Value: content})}
}

// user emits the rows of a user line: tool results, and texts as user rows
// when the person typed them in the app, else as context.
func (a *auditLine) user(line pyjson.Value) {
	synthetic := line.Get("isSynthetic").Truthy()
	typed := isText(line.Get("client_platform"), "desktop_app") && !synthetic
	for _, block := range blocks(line.Get("message").Get("content")) {
		blockType := block.Get("type")
		switch {
		case !block.IsObject():
		case isText(blockType, "tool_result"):
			result := claudecode.ResultText(block.Get("content"))
			if block.Get("is_error").Truthy() {
				result = "[error] " + result
			}
			row := a.row(timeline.TurnToolResult, result)
			row.ToolUseID = text(block.Get("tool_use_id"))
			a.emit(row)
		case isText(blockType, "text"), isText(blockType, "image"):
			body := parser.TextOf(pyjson.NewArray(block))
			switch {
			case body == "":
			case typed:
				a.emit(a.row(timeline.TurnUser, body))
			case synthetic:
				a.emit(a.row(timeline.TurnSystem, "context: notification: "+body))
			default:
				a.emit(a.row(timeline.TurnSystem, "context: "+body))
			}
		}
	}
}

// assistant emits the rows of an assistant line's tool calls, texts and,
// when asked for, thinking.
func (a *auditLine) assistant(line pyjson.Value) {
	message := line.Get("message")
	model := text(message.Get("model"))
	for _, block := range blocks(message.Get("content")) {
		blockType := block.Get("type")
		var row timeline.Row
		switch {
		case !block.IsObject():
			continue
		case isText(blockType, "tool_use"):
			name := text(block.Get("name"))
			row = a.row(timeline.TurnToolUse, claudecode.ToolSummary(name, block.Get("input")))
			row.ToolName = name
			row.ToolUseID = text(block.Get("id"))
		case isText(blockType, "text") && block.Get("text").Truthy():
			row = a.row(timeline.TurnAssistant, text(block.Get("text")))
		case isText(blockType, "thinking") && a.options.IncludeThinking && block.Get("thinking").Truthy():
			row = a.row(timeline.TurnThinking, text(block.Get("thinking")))
		case isText(blockType, "redacted_thinking") && a.options.IncludeThinking:
			row = a.row(timeline.TurnThinking, "[redacted]")
		default:
			continue
		}
		row.Model = model
		a.emit(row)
	}
}
