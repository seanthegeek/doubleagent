package claudedesktop

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/claudecode"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// transcript emits the rows of a transcript in Claude Code's format. When
// the session record names a folder, every row but a scratch directory's
// takes it as project path. session, the session dir relative to the home,
// is "" for a staged transcript, which has no record.
func (p *Parser) transcript(artifact *input.Artifact, options parserkit.Options, session string,
	emit parserkit.Emit) {
	project := ""
	if session != "" {
		record, _ := p.sessionRecord(artifact.Home, session)
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
	var ignored parserkit.Problems
	for _, line := range parserkit.JSONLines(artifact.DiskPath, &ignored) {
		if parserkit.IsText(line.Get("type"), "system") && parserkit.IsText(line.Get("subtype"), "init") {
			if cwd := line.Get("cwd").Text(); cwd != "" {
				return cwd
			}
		}
	}
	return ""
}

// auditLine is the state of an audit log's parse and the line being read.
type auditLine struct {
	parser    *Parser
	artifact  *input.Artifact
	options   parserkit.Options
	emit      parserkit.Emit
	project   string
	models    parserkit.Tracker
	line      int
	timestamp string
	sessionID string
	// problems collects the audit log's problems.
	problems *parserkit.Problems
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
// session dir relative to the home. The project is the record's folder,
// else the CLI's working directory, else the record's cwd; the session is
// the record's CLI session id, else each line's own.
func (p *Parser) audit(artifact *input.Artifact, options parserkit.Options, session string,
	emit parserkit.Emit) {
	record, found := p.sessionRecord(artifact.Home, session)
	project := folder(record)
	if project == "" {
		project = initCwd(artifact)
	}
	if project == "" && found && !record.IsEmpty() {
		project = record.Get("cwd").Text()
	}
	cliID := ""
	if found && !record.IsEmpty() {
		cliID = record.Get("cliSessionId").Text()
	}
	var problems parserkit.Problems
	state := &auditLine{parser: p, artifact: artifact, options: options, emit: emit,
		project: project, models: parserkit.Tracker{}, problems: &problems}
	lastSessionID := cliID
	for number, line := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		state.line = number
		state.sessionID = cliID
		if state.sessionID == "" {
			state.sessionID = line.Get("session_id").Text()
		}
		if state.sessionID != "" {
			lastSessionID = state.sessionID
		}
		state.timestamp = cmp.Or(timeutil.FromValue(line.Get("timestamp")),
			timeutil.FromValue(line.Get("_audit_timestamp")))
		state.read(line)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, lastSessionID, project))
}

// read emits the rows of one audit line, by its type and subtype.
func (a *auditLine) read(line jsonvalue.Value) {
	lineType, subtype := line.Get("type"), line.Get("subtype")
	isSystem := parserkit.IsText(lineType, "system")
	switch {
	case isSystem && (parserkit.IsText(subtype, "permission_request") || parserkit.IsText(subtype, "permission_response") ||
		parserkit.IsText(subtype, "permission_auto_approved") || parserkit.IsText(subtype, "permission_auto_denied")):
		a.permission(line, subtype.StringValue())
	case parserkit.IsText(lineType, "result"):
		a.emit(a.row(timeline.TurnSystem, "result: "+subtype.Text()+" turns="+
			line.Get("num_turns").Text()+" error="+line.Get("is_error").Text()))
	case isSystem && parserkit.IsText(subtype, "init"):
		// The session's model, even when a transcript exists.
		a.modelRow(line.Get("model").Text())
		var parts []string
		for _, key := range []string{"cwd", "model"} {
			if value := line.Get(key); !value.IsEmpty() {
				parts = append(parts, key+"="+value.Text())
			}
		}
		message := "init"
		if len(parts) > 0 {
			message = "init: " + strings.Join(parts, " ")
		}
		a.emit(a.row(timeline.TurnSystem, message))
	case isSystem && parserkit.IsText(subtype, "model_refusal_fallback"):
		// The subtype is in the bundle, its fields are not recorded: they
		// are printed as they are.
		a.emit(a.row(timeline.TurnSystem, "model_refusal_fallback: "+
			parserkit.JSONText(withoutEnvelope(line))))
	case parserkit.IsText(lineType, "user"):
		a.user(line)
	case parserkit.IsText(lineType, "assistant"):
		a.modelRow(line.Get("message").Get("model").Text())
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
func withoutEnvelope(line jsonvalue.Value) jsonvalue.Value {
	var kept []jsonvalue.Member
	for _, member := range line.Members() {
		switch member.Key {
		case "type", "subtype", "uuid", "session_id", "timestamp", "_audit_timestamp",
			"_audit_hmac", "_audit_redactions":
			continue
		}
		kept = append(kept, member)
	}
	return jsonvalue.NewObject(kept...)
}

// permission emits the row of a permission request, a response, or an
// automatic approval or denial with what decided it.
func (a *auditLine) permission(line jsonvalue.Value, subtype string) {
	tool := line.Get("tool_name").Text()
	message := ""
	switch subtype {
	case "permission_request":
		summary := ""
		if toolInput := line.Get("tool_input"); !toolInput.IsNull() {
			summary = claudecode.ToolSummary(tool, toolInput)
		}
		message = "permission request: " + tool + " " + summary
	case "permission_response":
		message = "permission response: " + tool + " " + line.Get("decision").Text() + " granted=" +
			line.Get("granted").Text()
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
func autoReason(line jsonvalue.Value) string {
	why := line.Get("source")
	if why.IsEmpty() {
		why = line.Get("scheduled_task_id")
	}
	if !why.IsEmpty() {
		return why.Text()
	}
	if matched := line.Get("matched_ops"); !matched.IsNull() {
		return parserkit.JSONText(matched)
	}
	return ""
}

// blocks returns a message's content as blocks: a list as it is, anything
// else as one text block holding it.
func (a *auditLine) blocks(content jsonvalue.Value) []jsonvalue.Value {
	if content.IsArray() {
		return parserkit.ListOfKind(content, "message.content", jsonvalue.Object, a.line, a.problems)
	}
	return []jsonvalue.Value{jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
		jsonvalue.Member{Key: "text", Value: content})}
}

// user emits the rows of a user line: tool results, and texts as user rows
// when the person typed them in the app, else as context.
func (a *auditLine) user(line jsonvalue.Value) {
	synthetic := !line.Get("isSynthetic").IsEmpty()
	typed := parserkit.IsText(line.Get("client_platform"), "desktop_app") && !synthetic
	for _, block := range a.blocks(line.Get("message").Get("content")) {
		blockType := block.Get("type")
		switch {
		case parserkit.IsText(blockType, "tool_result"):
			result := claudecode.ResultText(block.Get("content"))
			if !block.Get("is_error").IsEmpty() {
				result = "[error] " + result
			}
			row := a.row(timeline.TurnToolResult, result)
			row.ToolUseID = block.Get("tool_use_id").Text()
			a.emit(row)
		case parserkit.IsText(blockType, "text"), parserkit.IsText(blockType, "image"):
			body := parserkit.TextOf(jsonvalue.NewArray(block))
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
func (a *auditLine) assistant(line jsonvalue.Value) {
	message := line.Get("message")
	model := message.Get("model").Text()
	for _, block := range a.blocks(message.Get("content")) {
		blockType := block.Get("type")
		var row timeline.Row
		switch {
		case parserkit.IsText(blockType, "tool_use"):
			name := block.Get("name").Text()
			row = a.row(timeline.TurnToolUse, claudecode.ToolSummary(name, block.Get("input")))
			row.ToolName = name
			row.ToolUseID = block.Get("id").Text()
		case parserkit.IsText(blockType, "text") && !block.Get("text").IsEmpty():
			row = a.row(timeline.TurnAssistant, block.Get("text").Text())
		case parserkit.IsText(blockType, "thinking") && a.options.IncludeThinking && !block.Get("thinking").IsEmpty():
			row = a.row(timeline.TurnThinking, block.Get("thinking").Text())
		case parserkit.IsText(blockType, "redacted_thinking") && a.options.IncludeThinking:
			row = a.row(timeline.TurnThinking, "[redacted]")
		default:
			continue
		}
		row.Model = model
		a.emit(row)
	}
}
