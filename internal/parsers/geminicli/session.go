package geminicli

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
// and removals, then a row per problem, among them each member of the
// wrong kind the replay or a message met.
func (p *Parser) session(artifact *input.Artifact, options parserkit.Options, match pathMatch,
	emit parserkit.Emit) {
	var problems parserkit.Problems
	state := newReplay(p.Work)
	for _, each := range sessionRecords(artifact, &problems) {
		state.apply(each.line, each.record)
	}
	problems = append(problems, state.problems...)
	project := p.resolveProject(artifact, inBase(artifact, match), match["slug"],
		parserkit.Text(state.projectHash), &problems)
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
	models := parserkit.Tracker{}
	order := state.messages.order()
	answered := answeredCalls(&state.messages, order)
	// tallies holds a count per line, since the messages of a legacy file
	// or of one $set record share their record's line.
	tallies := map[int]*parserkit.EntryTally{}
	var lines []int
	for _, messageID := range order {
		entry := state.messages.byID[messageID]
		sessionID := entry.sessionID
		if sessionID == "" {
			sessionID = state.sessionID
		}
		if tallies[entry.line] == nil {
			tallies[entry.line] = &parserkit.EntryTally{}
			lines = append(lines, entry.line)
		}
		turn := messageTurn{owner: p, artifact: artifact, options: options, line: entry.line,
			sessionID: sessionID, project: project, tally: tallies[entry.line]}
		parserkit.EmitAll(emit, turn.rows(entry.message, models, answered))
	}
	for _, line := range lines {
		tallies[line].Report(line, &problems)
	}
	for _, each := range state.events {
		emit(p.systemRow(artifact, where{each.line, each.timestamp, each.sessionID, project}, each.text))
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, state.sessionID, project))
}

// sessionRecords returns the records of a session file: the lines of a
// JSONL file, or the one record of a legacy whole-JSON file. Every record
// is read before the replay, so line problems come before the others.
func sessionRecords(artifact *input.Artifact, problems *parserkit.Problems) []numbered {
	if strings.HasSuffix(artifact.Rel, ".json") {
		return legacyRecords(artifact, problems)
	}
	var records []numbered
	for line, record := range parserkit.JSONLines(artifact.DiskPath, problems) {
		records = append(records, numbered{line, record})
	}
	return records
}

// legacyRecords returns the one record of a legacy whole-JSON session
// file, or none after adding why it is not one.
func legacyRecords(artifact *input.Artifact, problems *parserkit.Problems) []numbered {
	data, ok := parserkit.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return nil
	}
	record, err := parserkit.DecodeJSONBytes(artifact, data)
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
	return parserkit.IsText(status, "error") || parserkit.IsText(status, "cancelled")
}

// answeredCalls returns the ids of the tool calls, among the messages of
// order, whose own result, or failed status, already gives a tool_result
// row, so that their response in a tool-response message gives none.
func answeredCalls(messages *messageLog, order []string) map[string]bool {
	answered := map[string]bool{}
	for _, messageID := range order {
		message := messages.byID[messageID].message
		if !parserkit.IsText(message.Get("type"), "gemini") {
			continue
		}
		for _, call := range message.Get("toolCalls").Items() {
			if call.IsObject() && (!call.Get("result").IsNull() || isFailed(call.Get("status"))) {
				answered[parserkit.Text(call.Get("id"))] = true
			}
		}
	}
	return answered
}

// messageTurn is where the rows of one replayed message go: its line,
// session and project, and the count of its members of the wrong kind,
// shared by the messages of its line.
type messageTurn struct {
	owner              *Parser
	artifact           *input.Artifact
	options            parserkit.Options
	line               int
	sessionID, project string
	timestamp, model   string
	tally              *parserkit.EntryTally
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
// one system row. A model message's thoughts or toolCalls of the wrong
// kind is counted in the tally.
func (t *messageTurn) rows(message jsonvalue.Value, models parserkit.Tracker,
	answered map[string]bool) []timeline.Row {
	kind := message.Get("type")
	t.timestamp = timeutil.FromValue(message.Get("timestamp"))
	if parserkit.IsText(kind, "gemini") {
		t.model = parserkit.Text(message.Get("model"))
	}
	content := message.Get("content")
	switch {
	case parserkit.IsText(kind, "user"):
		return t.userRows(content, answered)
	case !parserkit.IsText(kind, "gemini"):
		label := parserkit.Text(kind)
		if label == "" {
			label = "message"
		}
		return []timeline.Row{t.row(timeline.TurnSystem, label+": "+partText(content, false), "")}
	}
	var out []timeline.Row
	if change := models.Change("model", t.sessionID, t.model); change != "" {
		out = append(out, t.row(timeline.TurnSystem, change, ""))
	}
	thoughts := t.tally.ListOfKind(message.Get("thoughts"), "thoughts", jsonvalue.Object)
	if t.options.IncludeThinking {
		out = append(out, t.thinkingRows(thoughts, content)...)
	}
	if text := partText(content, false); text != "" {
		out = append(out, t.row(timeline.TurnAssistant, text, ""))
	}
	for _, call := range t.toolCalls(message) {
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
		callID := parserkit.Text(each.value.Get("id"))
		if answered[callID] {
			continue
		}
		result := t.row(timeline.TurnToolResult, each.resultText(), "")
		result.ToolName, result.ToolUseID = parserkit.Text(each.value.Get("name")), callID
		out = append(out, result)
	}
	if len(runs) == 0 && len(responses) == 0 {
		out = append(out, t.row(timeline.TurnUser, "", ""))
	}
	return out
}

// thinkingRows returns a model message's thinking rows: one per thoughts
// entry, "<subject>: <description>", at its own time, then one for the
// thought parts of its content.
func (t *messageTurn) thinkingRows(thoughts []jsonvalue.Value, content jsonvalue.Value) []timeline.Row {
	var out []timeline.Row
	for _, entry := range thoughts {
		text := parserkit.Text(entry.Get("description"))
		if subject := parserkit.Text(entry.Get("subject")); subject != "" {
			text = subject + ": " + text
		}
		out = append(out, t.row(timeline.TurnThinking, text, timeutil.FromValue(entry.Get("timestamp"))))
	}
	if thought := partText(content, true); thought != "" {
		out = append(out, t.row(timeline.TurnThinking, thought, ""))
	}
	return out
}

// toolCalls returns a model message's tool calls: its toolCalls objects,
// then each functionCall part of its content whose id none of them has,
// as a call of that id, name and args. A toolCalls of the wrong kind is
// counted in the tally.
func (t *messageTurn) toolCalls(message jsonvalue.Value) []jsonvalue.Value {
	var calls []jsonvalue.Value
	seen := map[string]bool{}
	for _, call := range t.tally.EntriesOfKind(message.Get("toolCalls"), "toolCalls", jsonvalue.Object) {
		calls = append(calls, call)
		seen[parserkit.Text(call.Get("id"))] = true
	}
	for _, part := range parts(message.Get("content")) {
		call := part.Get("functionCall")
		if call.IsObject() && !seen[parserkit.Text(call.Get("id"))] {
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
	callID, name := parserkit.Text(call.Get("id")), parserkit.Text(call.Get("name"))
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
	if parserkit.IsText(status, "cancelled") {
		decision := t.row(timeline.TurnSystem, "permission decision: cancelled", when)
		decision.ToolName, decision.ToolUseID = name, callID
		out = append(out, decision)
	}
	return out
}
