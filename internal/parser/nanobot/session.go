package nanobot

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/openclaw"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// numbered is one record of a session and its source line: a JSONL line
// number, or a rowid of the session database.
type numbered struct {
	line   int
	record jsonvalue.Value
}

// session is the state of one session while its rows are made: the
// session id, project, the model in force and the time of the last record
// that had one, and the presets of `/model` switches.
type session struct {
	*Parser
	artifact  *input.Artifact
	options   parser.Options
	emit      parser.Emit
	sessionID string
	project   string
	model     string
	timestamp string
	// switched is set when the session records a `/model` switch: the
	// metadata's preset is then the last switch's, not the session's first,
	// so the rows before the first switch name no model.
	switched bool
	// preset is the preset the last switch set; requested the preset name
	// of the last `/model <name>` command.
	preset, requested string
	models            parser.Tracker
}

// row emits a row of the session at the time of the last dated record,
// with the model in force.
func (s *session) row(line int, turnType, text, toolName, toolUseID string) {
	row := s.BaseRow(s.artifact)
	row.SourceLine = line
	row.SessionID = s.sessionID
	row.ProjectPath = s.project
	row.TimestampUTC = s.timestamp
	row.Model = s.model
	row.TurnType = turnType
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	s.emit(row)
}

// sessionRows emits the rows of one session from its records, the metadata
// record first, then the messages, as a session JSONL holds them and as the
// database's rows are rebuilt. It returns the session id: the metadata's
// key when there is one, else sessionID.
func (p *Parser) sessionRows(artifact *input.Artifact, options parser.Options, sessionID,
	project string, records []numbered, emit parser.Emit) string {
	state := &session{Parser: p, artifact: artifact, options: options, emit: emit,
		sessionID: sessionID, project: project, models: parser.Tracker{},
		switched: p.hasSwitch(records)}
	for _, each := range records {
		record := each.record
		recordType, typed := record.Lookup("_type")
		switch {
		case isText(recordType, "metadata"):
			state.metadata(each.line, record)
		case typed && !recordType.IsNull() && !record.Has("role"):
			// provider_state and record types to come
		default:
			state.message(each.line, record)
		}
	}
	return state.sessionID
}

// hasSwitch reports whether a session records a `/model` switch reply.
func (p *Parser) hasSwitch(records []numbered) bool {
	for _, each := range records {
		record := each.record
		if isText(record.Get("role"), "assistant") && !record.Get("_command").IsEmpty() &&
			p.text.switched.MatchString(parser.TextOf(record.Get("content"))) {
			return true
		}
	}
	return false
}

// metadata emits the rows of a session's metadata record: its start, relay
// rows, the preset's `model:` row and the workspace scope.
func (s *session) metadata(line int, record jsonvalue.Value) {
	if key := record.Get("key"); !key.IsEmpty() {
		s.sessionID = key.Text()
	}
	meta := record.Get("metadata")
	if !meta.IsObject() {
		meta = jsonvalue.NewObject()
	}
	s.model = ""
	if !s.switched {
		s.model = parser.Text(meta.Get("_nanobot_model_preset"))
	}
	s.timestamp = timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("created_at")),
		timeutil.FromValue(record.Get("updated_at")))
	parts := []string{"session start: key=" + s.sessionID}
	if last := meta.Get("last_channel"); !last.IsEmpty() {
		parts = append(parts, "last_channel="+last.Text())
	}
	if updated := record.Get("updated_at"); !updated.IsEmpty() {
		parts = append(parts, "updated_at="+updated.Text())
	}
	if s.project != "" {
		parts = append(parts, "workspace="+s.project)
	}
	s.row(line, timeline.TurnSystem, strings.Join(parts, " "), "", "")
	for _, text := range relayTexts(s.sessionID, meta.Get("last_channel")) {
		s.row(line, timeline.TurnSystem, text, "", "")
	}
	if text := s.models.Change("model", s.sessionID, s.model); text != "" {
		s.row(line, timeline.TurnSystem, text+" (preset)", "", "")
	}
	scope := meta.Get("workspace_scope")
	if scope.IsObject() && (!scope.Get("access_mode").IsEmpty() || !scope.Get("project_path").IsEmpty()) {
		s.row(line, timeline.TurnSystem, fmt.Sprintf("permission mode: access_mode=%s project=%s"+
			" (from session metadata workspace_scope)", parser.Text(scope.Get("access_mode")),
			parser.Text(scope.Get("project_path"))), "", "")
	}
}

// message emits the rows of one chat message by its role.
func (s *session) message(line int, record jsonvalue.Value) {
	s.timestamp = timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("timestamp")), s.timestamp)
	role := record.Get("role")
	text := parser.TextOf(record.Get("content"))
	switch {
	case isText(role, "user") && !record.Get("_hidden_history").IsEmpty():
		s.row(line, timeline.TurnSystem, "summarised history: "+text, "", "")
	case isText(role, "user"):
		if !record.Get("_command").IsEmpty() {
			if match := s.text.modelCommand.FindStringSubmatch(text); match != nil {
				s.requested = match[1]
			}
		}
		s.row(line, timeline.TurnUser, text, "", "")
	case isText(role, "assistant"):
		s.assistant(line, record, text)
	case isText(role, "tool"):
		s.row(line, timeline.TurnToolResult, text, parser.Text(record.Get("name")),
			parser.Text(record.Get("tool_call_id")))
	default:
		label := parser.Text(role)
		if label == "" {
			label = "message"
		}
		s.row(line, timeline.TurnSystem, label+": "+text, "", "")
	}
}

// assistant emits the rows of an assistant message: its reasoning, its
// text, the `model:` row of a `/model` switch reply, and its tool calls.
func (s *session) assistant(line int, record jsonvalue.Value, text string) {
	if reasoning := record.Get("reasoning_content"); s.options.IncludeThinking &&
		reasoning.IsString() && reasoning.StringValue() != "" {
		s.row(line, timeline.TurnThinking, reasoning.StringValue(), "", "")
	}
	if text != "" {
		s.row(line, timeline.TurnAssistant, text, "", "")
	}
	if !record.Get("_command").IsEmpty() {
		if match := s.text.switched.FindStringSubmatch(text); match != nil {
			s.switchRow(line, text, match[1])
		}
	}
	for _, call := range parser.Elements(record.Get("tool_calls")) {
		if !call.IsObject() {
			continue
		}
		function := call.Get("function")
		s.row(line, timeline.TurnToolUse, openclaw.ToolArgsText(function.Get("arguments")),
			parser.Text(function.Get("name")), parser.Text(call.Get("id")))
	}
}

// switchRow emits the `model:` row of a `/model` switch reply that set
// preset, naming the preset before it, the preset asked for and the model
// the reply resolved, and makes preset the session's model.
func (s *session) switchRow(line int, text, preset string) {
	var out strings.Builder
	out.WriteString("model: ")
	if s.preset != "" {
		out.WriteString(s.preset + " -> ")
	}
	out.WriteString(preset + " (preset)")
	if s.requested != "" {
		out.WriteString(" requested=" + s.requested)
	}
	if resolved := s.text.resolved.FindStringSubmatch(text); resolved != nil {
		out.WriteString(" resolved=" + resolved[1])
	}
	s.preset, s.model, s.requested = preset, preset, ""
	s.row(line, timeline.TurnSystem, out.String(), "", "")
}
