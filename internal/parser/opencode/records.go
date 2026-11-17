package opencode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	options   parser.Options
	emit      parser.Emit
}

// add emits a row of the record at when, else at the record's time, and
// returns it.
func (b *recordRows) add(turn, text, when string, tool ...string) timeline.Row {
	row := b.owner.row(b.artifact, b.scoped, "", b.line, timeutil.FirstNonEmpty(when, b.timestamp), b.model)
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	if len(tool) == 2 {
		row.ToolName, row.ToolUseID = tool[0], tool[1]
	}
	b.emit(row)
	return row
}

// recordRows emits the rows of one session_message record of the given
// type, at line, created being the row's time_created.
func (p *Parser) recordRows(artifact *input.Artifact, scoped *scope, kind string, data jsonvalue.Value,
	line int, created string, options parser.Options, emit parser.Emit) {
	reference := objectOf(data.Get("model"))
	builder := &recordRows{owner: p, artifact: artifact, scoped: scoped, line: line, options: options,
		emit: emit,
		timestamp: timeutil.FirstNonEmpty(
			timeutil.FromValue(objectOf(data.Get("time")).Get("created")), created),
		model: modelName(reference.Get("providerID"),
			firstTruthy(reference.Get("modelID"), reference.Get("id")))}
	switch kind {
	case "model-switched":
		builder.system(modelRowText(scoped.models, scoped.sessionID, reference.Get("providerID"),
			firstTruthy(reference.Get("id"), reference.Get("modelID"))))
	case "agent-switched":
		builder.system(agentRowText(scoped.modes, scoped.sessionID, data.Get("agent")))
	case "user":
		builder.add(timeline.TurnUser, parser.Text(data.Get("text")), "")
	case "synthetic", "system":
		builder.add(timeline.TurnSystem, join(kind+":", parser.Text(data.Get("text"))), "")
	case "shell":
		builder.shell(data)
	case "assistant":
		builder.assistant(data, reference)
	case "":
	default:
		builder.add(timeline.TurnSystem, recordSummary(kind, data), "")
	}
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
		return join("compaction", parser.Text(data.Get("reason")), parser.Text(data.Get("status")),
			parser.Text(data.Get("summary")), failure)
	case "location-switched":
		location := objectOf(data.Get("location"))
		previous := objectOf(objectOf(data.Get("previous")).Get("location"))
		before := labelled("", previous.Get("directory"))
		if before != "" {
			before += " ->"
		}
		return join("location switched:", before, parser.Text(location.Get("directory")),
			labelled("workspace=", location.Get("workspaceID")),
			labelled("subpath=", data.Get("subpath")))
	case "skill":
		return join("skill "+parser.Text(data.Get("name"))+":", parser.Text(data.Get("text")))
	case "idle":
		return join("idle:", parser.Text(data.Get("outcome")), failure)
	}
	var kept []jsonvalue.Member
	for _, member := range data.Members() {
		if member.Key != "time" {
			kept = append(kept, member)
		}
	}
	return join(kind+":", parser.JSONText(jsonvalue.NewObject(kept...)))
}

// assistant emits an assistant record's model and agent changes, the rows
// of its content items and its error.
func (b *recordRows) assistant(data, reference jsonvalue.Value) {
	scoped := b.scoped
	for _, text := range []string{modelRowText(scoped.models, scoped.sessionID, reference.Get("providerID"),
		firstTruthy(reference.Get("id"), reference.Get("modelID"))),
		agentRowText(scoped.modes, scoped.sessionID, data.Get("agent"))} {
		b.system(text)
	}
	for _, item := range data.Get("content").Items() {
		item = objectOf(item)
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
			b.tool(item)
		}
	}
	if failure := data.Get("error"); !failure.IsEmpty() {
		b.add(timeline.TurnSystem, "error: "+errorText(failure), "")
	}
}

// tool emits a 2.x tool item's tool_use row, and once the call has
// finished its tool_result row and, for a refusal, its
// "permission decision:" row.
func (b *recordRows) tool(item jsonvalue.Value) {
	state, times := objectOf(item.Get("state")), objectOf(item.Get("time"))
	name, callID := parser.Text(item.Get("name")), parser.Text(item.Get("id"))
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
		for _, chunk := range state.Get("content").Items() {
			if chunk.IsObject() && !chunk.Get("text").IsNull() {
				chunks = append(chunks, chunk.Get("text").Text())
			}
		}
		text = strings.Join(chunks, "\n")
	}
	result := b.add(timeline.TurnToolResult, text, timeutil.FirstNonEmpty(
		timeutil.FromValue(times.Get("completed")), timeutil.FromValue(times.Get("ran"))),
		name, callID)
	if status != "error" {
		return
	}
	if decision := decisionText(state.Get("error")); decision != "" {
		b.add(timeline.TurnSystem, decision, result.TimestampUTC, name, callID)
	}
}

// shell emits a shell record's tool_use row and, once it has output or
// has ended, its tool_result row. 1.x records callID and output as a
// string; 2.0 shellID, status (running, exited, timeout, killed), exit and
// output.output. A result that did not exit 0 starts "[<status>]" or
// "[exit <code>]".
func (b *recordRows) shell(data jsonvalue.Value) {
	callID := parser.Text(firstTruthy(data.Get("callID"), data.Get("shellID")))
	b.add(timeline.TurnToolUse, parser.Text(data.Get("command")), "", "shell", callID)
	output, status, code := data.Get("output"), data.Get("status"), data.Get("exit")
	if output.IsNull() && (status.IsNull() || isText(status, "running")) {
		return
	}
	if output.IsObject() {
		output = output.Get("output")
	}
	text := textOf(output)
	switch {
	case isText(status, "timeout") || isText(status, "killed"):
		text = join("["+status.StringValue()+"]", text)
	case !code.IsNull() && !isZero(code):
		text = join("[exit "+code.Text()+"]", text)
	}
	b.add(timeline.TurnToolResult, text,
		timeutil.FromValue(objectOf(data.Get("time")).Get("completed")), "shell", callID)
}

// isZero reports whether Python counts value equal to 0: a number of value
// zero, or false.
func isZero(value jsonvalue.Value) bool {
	if flag, ok := value.Bool(); ok {
		return !flag
	}
	number, ok := value.Float()
	return ok && value.IsNumber() && number == 0
}
