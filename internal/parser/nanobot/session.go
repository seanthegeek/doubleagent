package nanobot

import (
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
	problems          *parser.Problems
	// carried cuts the workspace each metadata record's row repeats.
	carried *parser.Carried
	// repeated is the group of a key the sessions table repeats, nil for
	// any other session: the rows of each metadata record are a parent's
	// (parent, while inMetadata), the messages' rows members, taking the
	// time and model the parent gave them while fromParent says so.
	repeated        *timeline.Group
	parent          *timeline.Parent
	inMetadata      bool
	timeFromParent  bool
	modelFromParent bool
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
	s.join(&row)
	s.emit(row)
}

// join gives a row of a repeated key its place in the key's group.
func (s *session) join(row *timeline.Row) {
	switch {
	case s.repeated == nil:
	case s.inMetadata && s.parent == nil:
		s.parent = s.repeated.Parent(row)
	case s.inMetadata:
		s.parent.Row(row, timeline.Always)
	default:
		var takes timeline.Takes
		if s.timeFromParent {
			takes |= timeline.TakeTime
		}
		if s.modelFromParent {
			takes |= timeline.TakeModel
		}
		s.repeated.Member(row, takes)
	}
}

// sessionRows emits the rows of one session from its records, the metadata
// record first, then the messages, as a session JSONL holds them and as the
// database's rows are rebuilt. It returns the session id: the metadata's
// key when there is one, else sessionID. A member of the wrong kind adds a
// problem to problems. repeated is the group of a key the sessions table
// repeats, else nil.
func (p *Parser) sessionRows(artifact *input.Artifact, options parser.Options, sessionID,
	project string, records []numbered, repeated *timeline.Group, emit parser.Emit,
	problems *parser.Problems) string {
	state := &session{Parser: p, artifact: artifact, options: options, emit: emit,
		sessionID: sessionID, project: project, models: parser.Tracker{},
		switched: p.hasSwitch(records), problems: problems, carried: parser.NewCarried(artifact),
		repeated: repeated}
	for _, each := range records {
		record := each.record
		recordType, typed := record.Lookup("_type")
		switch {
		case parser.IsText(recordType, "metadata"):
			state.metadata(each.line, record)
		case typed && !recordType.IsNull() && !record.Has("role"):
			// provider_state and record types to come
		default:
			state.message(each.line, record)
		}
	}
	state.carried.Report(0, problems)
	return state.sessionID
}

// hasSwitch reports whether a session records a `/model` switch reply.
func (p *Parser) hasSwitch(records []numbered) bool {
	for _, each := range records {
		record := each.record
		if parser.IsText(record.Get("role"), "assistant") && !record.Get("_command").IsEmpty() &&
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
	s.inMetadata, s.parent, s.timeFromParent, s.modelFromParent = true, nil, true, !s.switched
	defer func() { s.inMetadata = false }()
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
		parts = append(parts, "workspace="+s.carried.Cut("workspace", s.project))
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
		s.row(line, timeline.TurnSystem, "permission mode: access_mode="+
			parser.Text(scope.Get("access_mode"))+" project="+
			parser.Text(scope.Get("project_path"))+" (from session metadata workspace_scope)", "", "")
	}
}

// message emits the rows of one chat message by its role.
func (s *session) message(line int, record jsonvalue.Value) {
	if own := timeutil.FromValue(record.Get("timestamp")); own != "" {
		s.timestamp, s.timeFromParent = own, false
	}
	role := record.Get("role")
	text := parser.TextOf(record.Get("content"))
	switch {
	case parser.IsText(role, "user") && !record.Get("_hidden_history").IsEmpty():
		s.row(line, timeline.TurnSystem, "summarised history: "+text, "", "")
	case parser.IsText(role, "user"):
		if !record.Get("_command").IsEmpty() {
			if match := s.text.modelCommand.FindStringSubmatch(text); match != nil {
				s.requested = match[1]
			}
		}
		s.row(line, timeline.TurnUser, text, "", "")
	case parser.IsText(role, "assistant"):
		s.assistant(line, record, text)
	case parser.IsText(role, "tool"):
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
// text, the `model:` row of a `/model` switch reply, and its tool calls; a
// tool_calls that is not an array adds a problem.
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
	for _, call := range parser.EntriesOfKind(record.Get("tool_calls"), "tool_calls", jsonvalue.Object, line,
		s.problems) {
		function := call.Get("function")
		s.row(line, timeline.TurnToolUse, openclaw.ToolArgsText(s.artifact, function.Get("arguments")),
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
	s.preset, s.model, s.requested, s.modelFromParent = preset, preset, "", false
	s.row(line, timeline.TurnSystem, out.String(), "", "")
}
