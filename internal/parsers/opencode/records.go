package opencode

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// recordRows builds the rows of one 2.x session_message record.
type recordRows struct {
	owner     *Parser
	artifact  *input.Artifact
	scoped    *scope
	line      int
	timestamp string
	model     string
	options   parserkit.Options
	emit      parserkit.Emit
	// content is how an assistant record of a repeated id shows the id's
	// items, and linked places its rows in the id's group (nil for any
	// other record).
	content itemRows
	linked  *link
}

// itemRows is how one assistant record claiming the session_part items of
// a message id that more than one record claims gives them: repeated is
// the id's group, nil for any other record, and items the items, for the
// first record of the id only.
type itemRows struct {
	repeated *repeat
	items    []jsonvalue.Value
}

// add emits a row of the record at when, else at the record's time, and
// returns it.
func (b *recordRows) add(turn, text, when string, tool ...string) timeline.Row {
	row := b.owner.row(b.artifact, b.scoped, "", b.line, cmp.Or(when, b.timestamp), b.model)
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	if len(tool) == 2 {
		row.ToolName, row.ToolUseID = tool[0], tool[1]
	}
	b.linked.place(&row, when == "")
	b.emit(row)
	return row
}

// system emits a system row with text unless text is empty.
func (b *recordRows) system(text string) {
	if text != "" {
		b.add(timeline.TurnSystem, text, "")
	}
}

// recordSummary returns the system row text of a record that is neither a
// message nor a switch of model or agent.
func recordSummary(kind string, data jsonvalue.Value) string {
	failure := ""
	if !data.Get("error").IsEmpty() {
		failure = errorText(data.Get("error"))
	}
	switch kind {
	case "compaction":
		return join("compaction", parserkit.Text(data.Get("reason")), parserkit.Text(data.Get("status")),
			parserkit.Text(data.Get("summary")), failure)
	case "location-switched":
		location := parserkit.ObjectOrEmpty(data.Get("location"))
		previous := parserkit.ObjectOrEmpty(parserkit.ObjectOrEmpty(data.Get("previous")).Get("location"))
		before := parserkit.Labelled("", previous.Get("directory"))
		if before != "" {
			before += " ->"
		}
		return join("location switched:", before, parserkit.Text(location.Get("directory")),
			parserkit.Labelled("workspace=", location.Get("workspaceID")),
			parserkit.Labelled("subpath=", data.Get("subpath")))
	case "skill":
		return join("skill "+parserkit.Text(data.Get("name"))+":", parserkit.Text(data.Get("text")))
	case "idle":
		return join("idle:", parserkit.Text(data.Get("outcome")), failure)
	}
	var kept []jsonvalue.Member
	for _, member := range data.Members() {
		if member.Key != "time" {
			kept = append(kept, member)
		}
	}
	return join(kind+":", parserkit.JSONText(jsonvalue.NewObject(kept...)))
}

// problemRows emits a "parser:" row for each problem, which is how a member
// of the wrong kind is reported.
func (b *recordRows) problemRows(problems parserkit.Problems) {
	for _, found := range problems {
		b.add(timeline.TurnSystem, parserkit.ProblemPrefix+found.Message, "")
	}
}

// tool emits a 2.x tool item's tool_use row, and once the call has
// finished its tool_result row and, for a refusal, its
// "permission decision:" row. A state.content of the wrong kind is counted
// in tally, so the record gives one problem row for every tool item.
func (b *recordRows) tool(item jsonvalue.Value, tally *parserkit.EntryTally) {
	state, times := parserkit.ObjectOrEmpty(item.Get("state")), parserkit.ObjectOrEmpty(item.Get("time"))
	name, callID := parserkit.Text(item.Get("name")), parserkit.Text(item.Get("id"))
	b.add(timeline.TurnToolUse, ToolSummary(state.Get("input")),
		timeutil.FromValue(times.Get("created")), name, callID)
	status := textOf(state.Get("status"))
	if status != "completed" && status != "error" {
		return
	}
	var text string
	if status == "error" {
		text = "[error] " + errorText(state.Get("error"))
	} else {
		var chunks []string
		for _, chunk := range tally.EntriesOfKind(state.Get("content"), "state.content", jsonvalue.Object) {
			if !chunk.Get("text").IsNull() {
				chunks = append(chunks, chunk.Get("text").Text())
			}
		}
		text = strings.Join(chunks, "\n")
	}
	ended := cmp.Or(timeutil.FromValue(times.Get("completed")),
		timeutil.FromValue(times.Get("ran")))
	b.add(timeline.TurnToolResult, text, ended, name, callID)
	if status != "error" {
		return
	}
	if decision := decisionText(state.Get("error")); decision != "" {
		b.add(timeline.TurnSystem, decision, ended, name, callID)
	}
}

// items emits the rows of an assistant record's content items. A content
// that is not an array adds a problem row.
func (b *recordRows) items(content jsonvalue.Value) {
	var problems parserkit.Problems
	items := parserkit.ListOfKind(content, "content", jsonvalue.Object, 0, &problems)
	b.problemRows(problems)
	var tally parserkit.EntryTally
	for _, item := range items {
		text := item.Get("text")
		switch textOf(item.Get("type")) {
		case "text":
			if !text.IsEmpty() {
				b.add(timeline.TurnAssistant, text.Text(), "")
			}
		case "reasoning":
			if b.options.IncludeThinking && !text.IsEmpty() {
				b.add(timeline.TurnThinking, text.Text(), "")
			}
		case "tool":
			b.tool(item, &tally)
		}
	}
	var wrong parserkit.Problems
	tally.Report(0, &wrong)
	b.problemRows(wrong)
}

// assistant emits an assistant record's model and agent changes, the rows
// of its content items and its error. A content that is not an array adds a
// problem row. A record of a repeated id gives the id's items, when it is
// the first, then its own content, as the rows the items replace.
func (b *recordRows) assistant(data, reference jsonvalue.Value) {
	scoped := b.scoped
	for _, text := range []string{modelRowText(scoped.models, scoped.sessionID, reference.Get("providerID"),
		parserkit.FirstNonEmpty(reference.Get("id"), reference.Get("modelID"))),
		agentRowText(scoped.modes, scoped.sessionID, data.Get("agent"))} {
		b.system(text)
	}
	if b.linked != nil {
		if b.content.items != nil {
			b.linked.members = true
			b.items(jsonvalue.NewArray(b.content.items...))
			b.linked.members = false
		}
		b.linked.replaced = true
	}
	b.items(data.Get("content"))
	if b.linked != nil {
		b.linked.replaced = false
	}
	if failure := data.Get("error"); !failure.IsEmpty() {
		b.add(timeline.TurnSystem, "error: "+errorText(failure), "")
	}
}

// isZero reports whether value is a number equal to zero.
func isZero(value jsonvalue.Value) bool {
	return jsonvalue.Equal(value, jsonvalue.NewInt(0))
}

// shell emits a shell record's tool_use row and, once it has output or
// has ended, its tool_result row. 1.x records callID and output as a
// string; 2.0 shellID, status (running, exited, timeout, killed), exit and
// output.output. A result that did not exit 0 starts "[<status>]" or
// "[exit <code>]".
func (b *recordRows) shell(data jsonvalue.Value) {
	callID := parserkit.Text(parserkit.FirstNonEmpty(data.Get("callID"), data.Get("shellID")))
	b.add(timeline.TurnToolUse, parserkit.Text(data.Get("command")), "", "shell", callID)
	output, status, code := data.Get("output"), data.Get("status"), data.Get("exit")
	if output.IsNull() && (status.IsNull() || parserkit.IsText(status, "running")) {
		return
	}
	if output.IsObject() {
		output = output.Get("output")
	}
	text := textOf(output)
	switch {
	case parserkit.IsText(status, "timeout") || parserkit.IsText(status, "killed"):
		text = join("["+status.StringValue()+"]", text)
	case !code.IsNull() && !isZero(code):
		text = join("[exit "+code.Text()+"]", text)
	}
	b.add(timeline.TurnToolResult, text,
		timeutil.FromValue(parserkit.ObjectOrEmpty(data.Get("time")).Get("completed")), "shell", callID)
}

// recordRows emits the rows of one session_message record of the given
// type, at line, created being the row's time_created; content says how
// an assistant record of a repeated id gives the id's items.
func (p *Parser) recordRows(artifact *input.Artifact, scoped *scope, kind string, data jsonvalue.Value,
	line int, created string, options parserkit.Options, content itemRows, emit parserkit.Emit) {
	reference := parserkit.ObjectOrEmpty(data.Get("model"))
	builder := &recordRows{owner: p, artifact: artifact, scoped: scoped, line: line, options: options,
		emit: emit, content: content,
		timestamp: cmp.Or(
			timeutil.FromValue(parserkit.ObjectOrEmpty(data.Get("time")).Get("created")), created),
		model: modelName(reference.Get("providerID"),
			parserkit.FirstNonEmpty(reference.Get("modelID"), reference.Get("id")))}
	if content.repeated != nil {
		start := p.row(artifact, scoped, "", line, builder.timestamp, builder.model)
		builder.linked = content.repeated.start(&start)
		emit(start)
	}
	switch kind {
	case "model-switched":
		builder.system(modelRowText(scoped.models, scoped.sessionID, reference.Get("providerID"),
			parserkit.FirstNonEmpty(reference.Get("id"), reference.Get("modelID"))))
	case "agent-switched":
		builder.system(agentRowText(scoped.modes, scoped.sessionID, data.Get("agent")))
	case "user":
		builder.add(timeline.TurnUser, parserkit.Text(data.Get("text")), "")
	case "synthetic", "system":
		builder.add(timeline.TurnSystem, join(kind+":", parserkit.Text(data.Get("text"))), "")
	case "shell":
		builder.shell(data)
	case "assistant":
		builder.assistant(data, reference)
	case "":
	default:
		builder.add(timeline.TurnSystem, recordSummary(kind, data), "")
	}
}
