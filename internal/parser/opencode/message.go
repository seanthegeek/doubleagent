package opencode

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// origin is where a 1.x message or part was read: its source file ("" for
// the artifact), its line, and the time to fall back on when the record
// carries none.
type origin struct {
	file string
	line int
	time string
}

// part is one decoded 1.x part with where it was read.
type part struct {
	data  jsonvalue.Value
	where origin
}

// partRows builds the rows of one message's parts in order.
type partRows struct {
	owner    *Parser
	artifact *input.Artifact
	scoped   *scope
	model    string
	options  parser.Options
	rows     []timeline.Row
}

// add appends a row of the part at, at timestamp, and returns its index.
func (b *partRows) add(where origin, timestamp, turn, text string) int {
	row := b.owner.row(b.artifact, b.scoped, where.file, where.line, timestamp, b.model)
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	b.rows = append(b.rows, row)
	return len(b.rows) - 1
}

// messageRows emits the rows of one 1.x message and its parts: the model
// and agent changes, the parts' rows with the user's text parts joined
// into one row where the first of them was, the message's error, then a
// `parser:` row per part that could not be decoded.
func (p *Parser) messageRows(artifact *input.Artifact, scoped *scope, message jsonvalue.Value,
	where origin, parts []part, options parser.Options, failures []failure, emit parser.Emit) {
	provider, modelID := messageRef(message)
	builder := &partRows{owner: p, artifact: artifact, scoped: scoped, model: modelName(provider, modelID),
		options: options}
	times := objectOf(message.Get("time"))
	timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(times.Get("created")), where.time)
	if scoped.projectPath == "" {
		scoped.projectPath = parser.Str(objectOf(message.Get("path")).Get("cwd"))
	}
	for _, text := range []string{modelRowText(scoped.models, scoped.sessionID, provider, modelID),
		agentRowText(scoped.modes, scoped.sessionID, message.Get("agent"))} {
		if text != "" {
			builder.add(where, timestamp, timeline.TurnSystem, text)
		}
	}
	builder.parts(parts, isText(message.Get("role"), "user"), timestamp)
	parser.EmitAll(emit, builder.rows)
	if failed := message.Get("error"); !failed.IsEmpty() {
		row := p.row(artifact, scoped, where.file, where.line, timeutil.FirstNonEmpty(
			timeutil.FromValue(times.Get("completed")), timestamp), builder.model)
		row.TurnType = timeline.TurnSystem
		row.Text = timeline.Compact("error: " + errorText(failed))
		emit(row)
	}
	parser.EmitAll(emit, p.failureRows(artifact, failures, scoped.sessionID, scoped.projectPath))
}

// parts adds the rows of the parts, a user message's text parts joined
// into one row placed where the first of them was.
func (b *partRows) parts(parts []part, user bool, messageTime string) {
	var texts []string
	first, place := origin{}, -1
	start := len(b.rows)
	for _, each := range parts {
		data := each.data
		timestamp := timeutil.FirstNonEmpty(
			timeutil.FromValue(objectOf(data.Get("time")).Get("start")), each.where.time, messageTime)
		text := data.Get("text")
		isPlainText := isText(data.Get("type"), "text") && data.Get("synthetic").IsEmpty() &&
			data.Get("ignored").IsEmpty()
		if isPlainText && user {
			if !text.IsEmpty() {
				texts = append(texts, parser.Str(text))
				if place < 0 {
					first, place = each.where, len(b.rows)-start
				}
			}
			continue
		}
		b.part(each.where, data, timestamp)
	}
	if len(texts) > 0 {
		row := b.owner.row(b.artifact, b.scoped, first.file, first.line, messageTime, b.model)
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(strings.Join(texts, "\n"))
		b.rows = slices.Insert(b.rows, start+place, row)
	}
}

// part adds the rows of one part other than a user's text.
func (b *partRows) part(where origin, data jsonvalue.Value, timestamp string) {
	kind, text := data.Get("type"), data.Get("text")
	switch textOf(kind) {
	case "text":
		switch {
		case !data.Get("synthetic").IsEmpty():
			b.add(where, timestamp, timeline.TurnSystem, join("synthetic:", parser.Str(text)))
		case !data.Get("ignored").IsEmpty():
			b.add(where, timestamp, timeline.TurnSystem, join("ignored:", parser.Str(text)))
		case !text.IsEmpty():
			b.add(where, timestamp, timeline.TurnAssistant, parser.Str(text))
		}
	case "reasoning":
		if b.options.IncludeThinking && !text.IsEmpty() {
			b.add(where, timestamp, timeline.TurnThinking, parser.Str(text))
		}
	case "tool":
		b.tool(where, data, timestamp)
	case "step-start", "snapshot":
	default:
		b.add(where, timestamp, timeline.TurnSystem, partSummary(kind, data))
	}
}

// partSummary returns the system row text of a part that is not text,
// reasoning or a tool call.
func partSummary(kind, data jsonvalue.Value) string {
	switch textOf(kind) {
	case "step-finish":
		tokens := objectOf(data.Get("tokens"))
		return join("step-finish", labelled("reason=", data.Get("reason")),
			present("cost=", data.Get("cost")), present("tokens_in=", tokens.Get("input")),
			present("tokens_out=", tokens.Get("output")))
	case "subtask":
		return join("subtask", labelled("agent=", data.Get("agent")),
			parser.Str(data.Get("description"))+":", parser.Str(data.Get("prompt")))
	case "compaction":
		auto := ""
		if !data.Get("auto").IsEmpty() {
			auto = "auto"
		}
		return join("compaction", auto)
	case "file":
		return join("file:", parser.Str(data.Get("filename")), parser.Str(data.Get("url")))
	case "agent":
		return join("agent:", parser.Str(data.Get("name")))
	case "retry":
		return join("retry", labelled("attempt=", data.Get("attempt")), errorText(data.Get("error")))
	case "patch":
		var files []string
		for _, file := range data.Get("files").Items() {
			files = append(files, file.Text())
		}
		return join("patch:", strings.Join(files, " "))
	}
	label := "part"
	if !kind.IsEmpty() {
		label = kind.Text()
	}
	return join(label+":", parser.CompactJSON(data))
}

// present returns label followed by the value's text unless it is null or
// missing.
func present(label string, value jsonvalue.Value) string {
	if value.IsNull() {
		return ""
	}
	return label + value.Text()
}

// tool adds a 1.x tool part's tool_use row, and once the call has finished
// its tool_result row and, for a refusal, its "permission decision:" row.
func (b *partRows) tool(where origin, data jsonvalue.Value, timestamp string) {
	state := objectOf(data.Get("state"))
	times := objectOf(state.Get("time"))
	name, callID := parser.Str(data.Get("tool")), parser.Str(data.Get("callID"))
	summary := ToolSummary(state.Get("input"))
	if summary == "" {
		summary = parser.Str(state.Get("title"))
	}
	started := timeutil.FromValue(times.Get("start"))
	b.named(b.add(where, timeutil.FirstNonEmpty(started, timestamp), timeline.TurnToolUse, summary),
		name, callID)
	status := textOf(state.Get("status"))
	if status != "completed" && status != "error" {
		return
	}
	text := ""
	switch output := state.Get("output"); {
	case status == "error":
		text = "[error] " + errorText(state.Get("error"))
	case output.IsString():
		text = output.StringValue()
	default:
		text = parser.CompactJSON(output)
	}
	ended := timeutil.FirstNonEmpty(timeutil.FromValue(times.Get("end")), started, timestamp)
	result := b.add(where, ended, timeline.TurnToolResult, text)
	b.named(result, name, callID)
	if status != "error" {
		return
	}
	if decision := decisionText(state.Get("error")); decision != "" {
		b.named(b.add(where, timeutil.FirstNonEmpty(b.rows[result].TimestampUTC, timestamp),
			timeline.TurnSystem, decision), name, callID)
	}
}

// named sets the tool name and call id of the row at index.
func (b *partRows) named(index int, name, callID string) {
	b.rows[index].ToolName = name
	b.rows[index].ToolUseID = callID
}
