package openclaw

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/pi"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// tool emits a tool call or result row with no model.
func (w *writer) tool(current *session, line int, timestamp, turnType, text, name, toolUseID string) {
	row := w.row(current, line, timestamp, turnType, text)
	row.ToolName, row.ToolUseID = name, toolUseID
	w.emit(row)
}

// bash emits the call and the result of a bash execution the user ran.
func (w *writer) bash(current *session, line int, timestamp string, record, message jsonvalue.Value) {
	toolUseID := parserkit.Text(record.Get("id"))
	w.tool(current, line, timestamp, timeline.TurnToolUse, parserkit.Text(message.Get("command")), "bash",
		toolUseID)
	status := "exit=" + message.Get("exitCode").Text()
	if !message.Get("cancelled").IsEmpty() {
		status += " cancelled"
	}
	if !message.Get("truncated").IsEmpty() {
		status += " truncated"
	}
	w.tool(current, line, timestamp, timeline.TurnToolResult,
		"["+status+"] "+parserkit.Text(message.Get("output")), "bash", toolUseID)
}

// block returns the row of one content block of an assistant message, and
// false for a block that gives none.
func (w *writer) block(current *session, line int, timestamp string, block jsonvalue.Value,
	model string) (timeline.Row, bool) {
	var row timeline.Row
	blockType := block.Get("type")
	switch {
	case parserkit.IsText(blockType, "text") && !block.Get("text").IsEmpty():
		row = w.row(current, line, timestamp, timeline.TurnAssistant, block.Get("text").Text())
	case parserkit.IsText(blockType, "thinking") && w.options.IncludeThinking:
		thinking := parserkit.Text(block.Get("thinking"))
		if thinking == "" && !block.Get("redacted").IsEmpty() {
			thinking = "[redacted thinking]"
		}
		if thinking == "" {
			return row, false
		}
		row = w.row(current, line, timestamp, timeline.TurnThinking, thinking)
	case parserkit.IsText(blockType, "toolCall"):
		row = w.row(current, line, timestamp, timeline.TurnToolUse, ToolArgsText(w.artifact, block.Get("arguments")))
		row.ToolName, row.ToolUseID = parserkit.Text(block.Get("name")), parserkit.Text(block.Get("id"))
	case parserkit.IsText(blockType, "image"):
		row = w.row(current, line, timestamp, timeline.TurnAssistant, "[image]")
	default:
		return row, false
	}
	row.Model = model
	return row, true
}

// assistant emits the rows of an assistant message: its `model:` row when
// the model is new, its blocks, and an error or abort. A content that is
// neither text nor an array of blocks is a problem, not silence.
func (w *writer) assistant(current *session, line int, timestamp string, message jsonvalue.Value) {
	requested := parserkit.Text(message.Get("model"))
	model := cmp.Or(parserkit.Text(message.Get("responseModel")), requested, current.model)
	if model != "" {
		current.model = model
	}
	named := !message.Get("responseModel").IsEmpty() || requested != ""
	lent := current.lent && !named
	current.lent = lent
	if named {
		text := pi.ModelText(current.seen, current.sessionID, model, parserkit.Text(message.Get("provider")), requested)
		if text != "" {
			w.system(current, line, timestamp, text, model)
		}
	}
	content := message.Get("content")
	if content.IsString() {
		content = jsonvalue.NewArray(jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	var problems parserkit.Problems
	if content.IsArray() {
		for _, block := range parserkit.EntriesOfKind(content, "content", jsonvalue.Object, line, &problems) {
			if row, ok := w.block(current, line, timestamp, block, model); ok {
				w.emitLent(current, row, lent)
			}
		}
	} else {
		parserkit.ArrayMember(content, "content", line, &problems)
	}
	for _, problem := range problems {
		w.bad(current, problem.Line, problem.Message, timestamp)
	}
	stop := message.Get("stopReason")
	errorMessage := message.Get("errorMessage")
	if parserkit.IsText(stop, "error") || parserkit.IsText(stop, "aborted") || !errorMessage.IsEmpty() {
		row := w.row(current, line, timestamp, timeline.TurnSystem, "assistant "+
			cmp.Or(parserkit.Text(stop), "error")+": "+parserkit.Text(errorMessage))
		row.Model = model
		w.emitLent(current, row, lent)
	}
}

// message emits the rows of a message entry by its role.
func (w *writer) message(current *session, line int, timestamp string, record, message jsonvalue.Value) {
	role := message.Get("role")
	content := parserkit.TextOf(message.Get("content"))
	switch {
	case parserkit.IsText(role, "user"):
		if !message.Get("runtimeContext").IsEmpty() || !message.Get("runtimeContextCarrier").IsEmpty() {
			w.system(current, line, timestamp, "runtime context: "+content, "")
		} else {
			w.emit(w.row(current, line, timestamp, timeline.TurnUser, content))
		}
	case parserkit.IsText(role, "assistant"):
		w.assistant(current, line, timestamp, message)
	case parserkit.IsText(role, "toolResult"):
		if !message.Get("isError").IsEmpty() {
			content = "[error] " + content
		}
		w.tool(current, line, timestamp, timeline.TurnToolResult, content,
			parserkit.Text(message.Get("toolName")), parserkit.Text(message.Get("toolCallId")))
	case parserkit.IsText(role, "bashExecution"):
		w.bash(current, line, timestamp, record, message)
	case parserkit.IsText(role, "branchSummary"):
		w.system(current, line, timestamp, "branch summary: "+parserkit.Text(message.Get("summary")), "")
	case parserkit.IsText(role, "compactionSummary"):
		w.system(current, line, timestamp, "compaction summary: "+parserkit.Text(message.Get("summary")), "")
	case parserkit.IsText(role, "custom"):
		w.system(current, line, timestamp, "custom "+parserkit.Text(message.Get("customType"))+": "+content, "")
	default:
		w.system(current, line, timestamp, cmp.Or(parserkit.Text(role), "message")+": "+
			cmp.Or(content, parserkit.JSONText(message)), "")
	}
}
