package geminicli

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// worktreePattern is a --worktree checkout inside a project, which a
// session's project path names.
var worktreePattern = regexp.MustCompile(`^.*[/\\]\.gemini[/\\]worktrees[/\\]([^/\\]+)\z`)

// numbered is one record of a session file and its line.
type numbered struct {
	line   int
	record jsonvalue.Value
}

// session emits the rows of one session file: the session starts, a
// scratch directory row for a worktree session, the rows of the messages
// in force after the replay, in replay order, then the summaries, rewinds
// and removals, then a row per problem.
func (p *Parser) session(artifact *input.Artifact, options parser.Options, match pathMatch,
	emit parser.Emit) {
	var problems parser.Problems
	state := newReplay()
	for _, each := range sessionRecords(artifact, &problems) {
		state.apply(each.line, each.record)
	}
	project := resolveProject(inBase(artifact, match), match["slug"],
		parser.Str(state.projectHash), &problems)
	for _, start := range state.starts {
		place := where{start.line, start.timestamp, start.sessionID, project}
		emit(p.systemRow(artifact, place, start.text))
	}
	worktree := worktreePattern.FindStringSubmatch(project)
	if worktree != nil && len(state.starts) > 0 {
		first := state.starts[0]
		emit(p.systemRow(artifact, where{first.line, first.timestamp, first.sessionID, project},
			"scratch directory: "+project+" (gemini --worktree "+worktree[1]+")"))
	}
	models := parser.Tracker{}
	answered := answeredCalls(&state.messages)
	for _, messageID := range state.messages.order {
		entry := state.messages.byID[messageID]
		sessionID := entry.sessionID
		if sessionID == "" {
			sessionID = state.sessionID
		}
		turn := messageTurn{owner: p, artifact: artifact, options: options, line: entry.line,
			sessionID: sessionID, project: project}
		parser.EmitAll(emit, turn.rows(entry.message, models, answered))
	}
	for _, each := range state.events {
		emit(p.systemRow(artifact, where{each.line, each.timestamp, each.sessionID, project}, each.text))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, state.sessionID, project))
}

// sessionRecords returns the records of a session file: the lines of a
// JSONL file, or the one record of a legacy whole-JSON file. Every record
// is read before the replay, so line problems come before the others.
func sessionRecords(artifact *input.Artifact, problems *parser.Problems) []numbered {
	if strings.HasSuffix(artifact.Rel, ".json") {
		return legacyRecords(artifact, problems)
	}
	var records []numbered
	for line, record := range parser.JSONLines(artifact.DiskPath, problems) {
		records = append(records, numbered{line, record})
	}
	return records
}

// legacyRecords returns the one record of a legacy whole-JSON session
// file, or none after adding why it is not one.
func legacyRecords(artifact *input.Artifact, problems *parser.Problems) []numbered {
	data, ok := parser.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return nil
	}
	record, err := jsonvalue.Decode(data)
	if err != nil {
		problems.Add(1, "unparseable session file: "+err.Error())
		return nil
	}
	if !record.IsObject() || !record.Get("sessionId").IsString() {
		problems.Add(1, "not a ConversationRecord")
		return nil
	}
	return []numbered{{1, record}}
}

// isFailed reports whether a tool call's status is one that gives a
// tool_result row without a result.
func isFailed(status jsonvalue.Value) bool {
	return isText(status, "error") || isText(status, "cancelled")
}

// answeredCalls returns the ids of the tool calls whose own result, or
// failed status, already gives a tool_result row, so that their response
// in a tool-response message gives none.
func answeredCalls(messages *messageLog) map[string]bool {
	answered := map[string]bool{}
	for _, messageID := range messages.order {
		message := messages.byID[messageID].message
		if !isText(message.Get("type"), "gemini") {
			continue
		}
		for _, call := range message.Get("toolCalls").Items() {
			if call.IsObject() && (!call.Get("result").IsNull() || isFailed(call.Get("status"))) {
				answered[parser.Str(call.Get("id"))] = true
			}
		}
	}
	return answered
}

// messageTurn is where the rows of one replayed message go: its line,
// session and project.
type messageTurn struct {
	owner              *Parser
	artifact           *input.Artifact
	options            parser.Options
	line               int
	sessionID, project string
	timestamp, model   string
}

// row returns a row of the message at when, else at the message's time.
func (t *messageTurn) row(turn, text, when string) timeline.Row {
	row := t.owner.BaseRow(t.artifact)
	row.SourceLine = t.line
	row.TimestampUTC = timeutil.FirstNonEmpty(when, t.timestamp)
	row.SessionID, row.ProjectPath = t.sessionID, t.project
	row.TurnType, row.Model, row.Text = turn, t.model, timeline.Compact(text)
	return row
}

// rows returns the rows of one replayed message: a user message's runs and
// its unanswered responses, a model message's model change, thinking,
// text, tool calls and results, or an info, error or warning message as
// one system row.
func (t *messageTurn) rows(message jsonvalue.Value, models parser.Tracker,
	answered map[string]bool) []timeline.Row {
	kind := message.Get("type")
	t.timestamp = timeutil.FromValue(message.Get("timestamp"))
	if isText(kind, "gemini") {
		t.model = parser.Str(message.Get("model"))
	}
	content := message.Get("content")
	switch {
	case isText(kind, "user"):
		return t.userRows(content, answered)
	case !isText(kind, "gemini"):
		label := parser.Str(kind)
		if label == "" {
			label = "message"
		}
		return []timeline.Row{t.row(timeline.TurnSystem, label+": "+partText(content, false), "")}
	}
	var out []timeline.Row
	if change := models.Change("model", t.sessionID, t.model); change != "" {
		out = append(out, t.row(timeline.TurnSystem, change, ""))
	}
	if t.options.IncludeThinking {
		out = append(out, t.thinkingRows(message)...)
	}
	if text := partText(content, false); text != "" {
		out = append(out, t.row(timeline.TurnAssistant, text, ""))
	}
	for _, call := range toolCalls(message) {
		out = append(out, t.callRows(call)...)
	}
	if len(out) == 0 {
		// An empty model turn, with no text and no tools, still marks a
		// model call.
		out = append(out, t.row(timeline.TurnAssistant, "", ""))
	}
	return out
}

// userRows returns a user message's runs as user and context rows, then a
// tool_result row with its media per response whose call is not answered;
// a message with neither gives an empty user row.
func (t *messageTurn) userRows(content jsonvalue.Value, answered map[string]bool) []timeline.Row {
	var out []timeline.Row
	runs, responses := contentRuns(content, true)
	row := func(turn, text string) timeline.Row { return t.row(turn, text, "") }
	for _, each := range runs {
		out = append(out, runRow(row, each, true))
	}
	for _, each := range responses {
		callID := parser.Str(each.value.Get("id"))
		if answered[callID] {
			continue
		}
		result := t.row(timeline.TurnToolResult, each.resultText(), "")
		result.ToolName, result.ToolUseID = parser.Str(each.value.Get("name")), callID
		out = append(out, result)
	}
	if len(runs) == 0 && len(responses) == 0 {
		out = append(out, t.row(timeline.TurnUser, "", ""))
	}
	return out
}

// thinkingRows returns a model message's thinking rows: one per thoughts
// entry, "<subject>: <description>", at its own time, then one for its
// thought parts.
func (t *messageTurn) thinkingRows(message jsonvalue.Value) []timeline.Row {
	var out []timeline.Row
	for _, entry := range message.Get("thoughts").Items() {
		if !entry.IsObject() {
			continue
		}
		text := parser.Str(entry.Get("description"))
		if subject := parser.Str(entry.Get("subject")); subject != "" {
			text = subject + ": " + text
		}
		out = append(out, t.row(timeline.TurnThinking, text, timeutil.FromValue(entry.Get("timestamp"))))
	}
	if thought := partText(message.Get("content"), true); thought != "" {
		out = append(out, t.row(timeline.TurnThinking, thought, ""))
	}
	return out
}

// toolCalls returns a model message's tool calls: its toolCalls objects,
// then each functionCall part of its content whose id none of them has,
// as a call of that id, name and args.
func toolCalls(message jsonvalue.Value) []jsonvalue.Value {
	var calls []jsonvalue.Value
	seen := map[string]bool{}
	for _, call := range message.Get("toolCalls").Items() {
		if call.IsObject() {
			calls = append(calls, call)
			seen[parser.Str(call.Get("id"))] = true
		}
	}
	for _, part := range parts(message.Get("content")) {
		call := part.Get("functionCall")
		if call.IsObject() && !seen[parser.Str(call.Get("id"))] {
			calls = append(calls, jsonvalue.NewObject(jsonvalue.Member{Key: "id", Value: call.Get("id")},
				jsonvalue.Member{Key: "name", Value: call.Get("name")},
				jsonvalue.Member{Key: "args", Value: call.Get("args")}))
		}
	}
	return calls
}

// callRows returns the rows of one tool call: its tool_use row, a
// tool_result row when it has a result or failed (the text prefixed with
// the failed status), and a permission decision row when it was cancelled.
func (t *messageTurn) callRows(call jsonvalue.Value) []timeline.Row {
	callID, name := parser.Str(call.Get("id")), parser.Str(call.Get("name"))
	when := timeutil.FromValue(call.Get("timestamp"))
	use := t.row(timeline.TurnToolUse, toolText(call), when)
	use.ToolName, use.ToolUseID = name, callID
	out := []timeline.Row{use}
	status := call.Get("status")
	if !call.Get("result").IsNull() || isFailed(status) {
		text := callResultText(call.Get("result"), callID)
		if isFailed(status) && text != "" {
			text = "[" + status.StringValue() + "] " + text
		} else if isFailed(status) {
			text = "[" + status.StringValue() + "]"
		}
		result := t.row(timeline.TurnToolResult, text, when)
		result.ToolName, result.ToolUseID = name, callID
		out = append(out, result)
	}
	if isText(status, "cancelled") {
		decision := t.row(timeline.TurnSystem, "permission decision: cancelled", when)
		decision.ToolName, decision.ToolUseID = name, callID
		out = append(out, decision)
	}
	return out
}
