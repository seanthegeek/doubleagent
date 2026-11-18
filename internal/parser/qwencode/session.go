package qwencode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// sessionState is what a session file's records carry forward: the
// session id and project last named, the model and approval mode in force
// per session, and the line, time, session and cwd of the first record.
type sessionState struct {
	sessionID, project string
	models, modes      parser.Tracker
	first              *numbered
}

// record is one ChatRecord being read, with the parser and artifact its
// rows belong to.
type record struct {
	owner    *Parser
	artifact *input.Artifact
	options  parser.Options
	state    *sessionState
	line     int
	value    jsonvalue.Value
}

// row returns a row of the record: its line, time, cwd and git branch,
// under the session id in force.
func (r *record) row(turn, text string) timeline.Row {
	row := r.owner.BaseRow(r.artifact)
	row.SourceLine = r.line
	row.TimestampUTC = timeutil.FromValue(r.value.Get("timestamp"))
	row.SessionID = r.state.sessionID
	row.ProjectPath = parser.Text(r.value.Get("cwd"))
	row.GitBranch = parser.Text(r.value.Get("gitBranch"))
	row.TurnType, row.Text = turn, timeline.Compact(text)
	return row
}

// session emits the rows of one chat file, then its worktree sidecar's
// scratch directory row, then a row per problem.
func (p *Parser) session(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	state := &sessionState{models: parser.Tracker{}, modes: parser.Tracker{}}
	for line, value := range parser.JSONLines(artifact.DiskPath, &problems) {
		if state.first == nil {
			state.first = &numbered{line, value}
		}
		if named := value.Get("sessionId"); !named.IsEmpty() {
			state.sessionID = named.Text()
		}
		if cwd := value.Get("cwd"); !cwd.IsEmpty() {
			state.project = cwd.Text()
		}
		current := &record{owner: p, artifact: artifact, options: options, state: state, line: line,
			value: value}
		parser.EmitAll(emit, current.rows())
	}
	if state.first != nil {
		p.worktreeRows(artifact, state.first, &problems, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, state.sessionID, state.project))
}

// rows returns the rows of one record: the model and approval mode changes
// of its execution context, then those of its type.
func (r *record) rows() []timeline.Row {
	out := r.contextChanges()
	switch kind := r.value.Get("type"); {
	case isText(kind, "user"):
		out = append(out, r.userRows()...)
	case isText(kind, "assistant"):
		out = append(out, r.assistantRows()...)
	case isText(kind, "tool_result"):
		out = append(out, r.toolResultRows()...)
	case isText(kind, "system"):
		out = append(out, r.systemRows()...)
	}
	return out
}

// contextChanges returns the "model:" and "permission mode:" rows of a
// record's executionContext, for a model or mode that differs from the one
// in force for the session; a model row carries the model.
func (r *record) contextChanges() []timeline.Row {
	context := r.value.Get("executionContext")
	if !context.IsObject() {
		return nil
	}
	var out []timeline.Row
	model := parser.Text(context.Get("modelId"))
	if change := r.state.models.Change("model", r.state.sessionID, model); change != "" {
		if auth := context.Get("authType"); !auth.IsEmpty() {
			change += " auth=" + auth.Text()
		}
		row := r.row(timeline.TurnSystem, change)
		row.Model = model
		out = append(out, row)
	}
	mode := parser.Text(context.Get("approvalMode"))
	if change := r.state.modes.Change("permission mode", r.state.sessionID, mode); change != "" {
		out = append(out, r.row(timeline.TurnSystem, change))
	}
	return out
}

// userRows returns a user record's row (a system row prefixed with its
// origin when the user did not type it), then one row per group message
// its text quotes.
func (r *record) userRows() []timeline.Row {
	var texts []string
	for _, part := range messageParts(r.value) {
		if part.Get("text").IsString() && part.Get("thought").IsEmpty() {
			texts = append(texts, part.Get("text").StringValue())
		}
	}
	text := strings.Join(texts, "\n")
	if text == "" {
		text = parser.Text(r.value.Get("systemPayload").Get("displayText"))
	}
	var out []timeline.Row
	switch origin := injectedOrigin(r.value); {
	case origin != "" && text != "":
		out = append(out, r.row(timeline.TurnSystem, origin+": "+text))
	case origin != "":
		out = append(out, r.row(timeline.TurnSystem, origin))
	default:
		out = append(out, r.row(timeline.TurnUser, text))
	}
	for _, quoted := range quotedGroupMessages(text) {
		out = append(out, r.row(timeline.TurnSystem, quoted))
	}
	return out
}

// assistantRows returns an assistant record's model change, then a row per
// part: a tool_use per functionCall, a thinking row per thought when asked
// for, an assistant row per non-empty text; each carries the model.
func (r *record) assistantRows() []timeline.Row {
	var out []timeline.Row
	model := parser.Text(r.value.Get("model"))
	if change := r.state.models.Change("model", r.state.sessionID, model); change != "" {
		row := r.row(timeline.TurnSystem, change)
		row.Model = model
		out = append(out, row)
	}
	for _, part := range messageParts(r.value) {
		var row timeline.Row
		text := part.Get("text")
		switch call := part.Get("functionCall"); {
		case call.IsObject():
			row = r.row(timeline.TurnToolUse, argsSummary(call.Get("args")))
			row.ToolName, row.ToolUseID = parser.Text(call.Get("name")), parser.Text(call.Get("id"))
		case text.IsString() && !part.Get("thought").IsEmpty():
			if !r.options.IncludeThinking {
				continue
			}
			row = r.row(timeline.TurnThinking, text.StringValue())
		case text.IsString() && text.StringValue() != "":
			row = r.row(timeline.TurnAssistant, text.StringValue())
		default:
			continue
		}
		row.Model = model
		out = append(out, row)
	}
	return out
}

// toolResultRows returns a tool_result record's row, its text the result
// display, else the first response's text, prefixed with a status other
// than success; then a permission decision row for a call cancelled or
// awaiting approval.
func (r *record) toolResultRows() []timeline.Row {
	result := r.value.Get("toolCallResult")
	first := jsonvalue.NewObject()
	for _, part := range messageParts(r.value) {
		if response := part.Get("functionResponse"); response.IsObject() {
			first = response
			break
		}
	}
	display := result.Get("resultDisplay")
	var text string
	switch {
	case display.IsString() && display.StringValue() != "":
		text = display.StringValue()
	case !first.IsEmpty():
		text = responseText(first.Get("response"))
	case !display.IsEmpty():
		text = parser.JSONText(display)
	}
	status := parser.Text(result.Get("status"))
	if status != "" && status != "success" {
		tag := status
		if errorType := result.Get("errorType"); !errorType.IsEmpty() {
			tag += " " + errorType.Text()
		}
		text = "[" + tag + "] " + text
	}
	row := r.row(timeline.TurnToolResult, text)
	row.ToolName = parser.Text(first.Get("name"))
	row.ToolUseID = parser.Text(parser.FirstNonEmpty(result.Get("callId"), first.Get("id")))
	out := []timeline.Row{row}
	if status == "cancelled" || status == "awaiting_approval" {
		decision := r.row(timeline.TurnSystem, "permission decision: "+status)
		decision.ToolName, decision.ToolUseID = row.ToolName, row.ToolUseID
		out = append(out, decision)
	}
	return out
}

// worktreeRows emits the "scratch directory:" row of a session's
// .worktree.json sidecar, which marks a session working in a worktree
// checkout. An absent sidecar is silent; one that cannot be read or is not
// a JSON object adds a problem. The row takes the session's first record's
// line, time and session id, and the worktree as its project.
func (p *Parser) worktreeRows(artifact *input.Artifact, first *numbered, problems *parser.Problems,
	emit parser.Emit) {
	name := strings.TrimSuffix(fileName(artifact), ".jsonl") + ".worktree.json"
	sidecar, found := parser.Beside(artifact, name)
	if !found || !parser.RegularFile(sidecar) {
		return
	}
	data, ok := parser.FileBytes(sidecar, problems)
	if !ok {
		return
	}
	document, err := jsonvalue.Decode(data)
	if err != nil || !document.IsObject() {
		problems.Add(0, name+" is not a JSON object")
		return
	}
	worktree := parser.Text(document.Get("worktreePath"))
	if worktree == "" {
		return
	}
	trimmed := strings.TrimRight(worktree, `/\`)
	slug := trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]
	row := p.SystemRow(artifact, timeline.ScratchPrefix+" "+worktree+" (enter_worktree "+slug+")")
	row.SourceLine = first.line
	row.TimestampUTC = timeutil.FromValue(first.record.Get("timestamp"))
	row.SessionID = parser.Text(first.record.Get("sessionId"))
	row.ProjectPath = worktree
	emit(row)
}

// systemRows returns a system record's row, its subtype and payload
// summary; a model or approval mode it sets is written as a change when it
// is one. A tool call's approval decision recorded in a ui_telemetry record
// adds a permission decision row.
func (r *record) systemRows() []timeline.Row {
	subtype := parser.Text(r.value.Get("subtype"))
	if subtype == "" {
		subtype = "system"
	}
	payload := r.value.Get("systemPayload")
	text, model := systemText(subtype, payload), ""
	switch {
	case subtype == "session_model" && payload.IsObject():
		if model = parser.Text(payload.Get("modelId")); model != "" {
			change := r.state.models.Change("model", r.state.sessionID, model)
			if change == "" {
				change = "model: " + model
			}
			text = change + " auth=" + parser.Text(payload.Get("authType"))
		}
	case subtype == "session_approval_mode" && payload.IsObject():
		if mode := parser.Text(payload.Get("mode")); mode != "" {
			text = r.state.modes.Change("permission mode", r.state.sessionID, mode)
			if text == "" {
				text = "permission mode: " + mode
			}
			parts := []string{text}
			for _, key := range []string{"prePlanMode", "planExecutionMode"} {
				if value := payload.Get(key); !value.IsEmpty() {
					parts = append(parts, key+"="+value.Text())
				}
			}
			text = strings.Join(parts, " ")
		}
	}
	row := r.row(timeline.TurnSystem, text)
	row.Model = model
	out := []timeline.Row{row}
	if subtype == "ui_telemetry" {
		out = append(out, r.telemetryDecision(payload.Get("uiEvent"))...)
	}
	return out
}

// telemetryDecision returns the permission decision row of a ui_telemetry
// record's tool call event, as the CLI's logToolCall records it under its
// own event name or the class's default one, or none.
func (r *record) telemetryDecision(event jsonvalue.Value) []timeline.Row {
	name := event.Get("event.name")
	if !event.IsObject() || !isText(name, "qwen-code.tool_call") && !isText(name, "tool_call") ||
		event.Get("decision").IsEmpty() {
		return nil
	}
	decision := r.row(timeline.TurnSystem, "permission decision: "+event.Get("decision").Text()+
		" (tool_call ui_telemetry)")
	decision.ToolName = parser.Text(event.Get("function_name"))
	decision.ToolUseID = parser.Text(event.Get("call_id"))
	return []timeline.Row{decision}
}

// entry is one object of a JSON array and its 1-based index.
type entry struct {
	index int
	value jsonvalue.Value
}

// logs emits one row per entry of a logs.json, a secondary source: a user
// entry as a user row, a model switch as a "model:" row carrying the new
// model, anything else as "<type>: <message>".
func (p *Parser) logs(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for _, each := range logEntries(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = each.index
		row.TimestampUTC = timeutil.FromValue(each.value.Get("timestamp"))
		row.SessionID = parser.Text(each.value.Get("sessionId"))
		kind, message := each.value.Get("type"), each.value.Get("message")
		var text string
		if isText(kind, "user") {
			row.TurnType, text = timeline.TurnUser, textOrJSON(message)
		} else {
			row.TurnType = timeline.TurnSystem
			event := message
			if message.IsString() {
				if decoded, err := jsonvalue.DecodeString(message.StringValue()); err == nil {
					event = decoded
				}
			}
			if isText(kind, "model_switch") && event.IsObject() {
				text = "model: " + event.Get("fromModel").Text() + " -> " + event.Get("toModel").Text() +
					" reason=" + event.Get("reason").Text()
				row.Model = parser.Text(event.Get("toModel"))
			} else {
				text = kind.Text() + ": " + textOrJSON(event)
			}
		}
		row.Text = timeline.Compact(text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// logEntries returns the object entries, with their 1-based index, of a
// logs.json array, decoded element by element so that a file cut
// mid-write keeps the entries before the cut. A decode failure adds
// "unparseable record <n> (<reason>)" with the text from the element to the
// end as raw, and ends the walk; a file whose text is not an array adds "not
// a JSON array" at 0, and one without its closing bracket "unterminated
// JSON array". Elements need no comma between them, and whitespace alone
// is no array.
func logEntries(path string, problems *parser.Problems) []entry {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return nil
	}
	text := textlines.ValidUTF8(data)
	offset := skipWhitespace(text, 0)
	if offset >= len(text) {
		return nil
	}
	if text[offset] != '[' {
		problems.Add(0, "not a JSON array")
		return nil
	}
	var out []entry
	index := 0
	for offset = skipWhitespace(text, offset+1); offset < len(text) && text[offset] != ']'; {
		index++
		value, next, err := jsonvalue.DecodePrefix(text, offset)
		if err != nil {
			problems.AddRaw(index, parser.UnparseableRecord(index, err), parser.TailText(text, offset))
			return out
		}
		if value.IsObject() {
			out = append(out, entry{index, value})
		}
		if offset = skipWhitespace(text, next); offset < len(text) && text[offset] == ',' {
			offset = skipWhitespace(text, offset+1)
		}
	}
	if offset >= len(text) {
		problems.Add(index+1, "unterminated JSON array")
	}
	return out
}

// skipWhitespace returns the first offset at or after offset that is not
// JSON whitespace.
func skipWhitespace(text string, offset int) int {
	for offset < len(text) && strings.IndexByte(" \t\r\n", text[offset]) >= 0 {
		offset++
	}
	return offset
}
