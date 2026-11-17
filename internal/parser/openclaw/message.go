package openclaw

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// message emits the rows of a message entry by its role.
func (w *writer) message(current *session, line int, timestamp string, record, message jsonvalue.Value) {
	role := message.Get("role")
	content := parser.TextOf(message.Get("content"))
	switch {
	case isText(role, "user"):
		if !message.Get("runtimeContext").IsEmpty() || !message.Get("runtimeContextCarrier").IsEmpty() {
			w.system(current, line, timestamp, "runtime context: "+content, "")
		} else {
			w.emit(w.row(current, line, timestamp, timeline.TurnUser, content))
		}
	case isText(role, "assistant"):
		w.assistant(current, line, timestamp, message)
	case isText(role, "toolResult"):
		if !message.Get("isError").IsEmpty() {
			content = "[error] " + content
		}
		w.tool(current, line, timestamp, timeline.TurnToolResult, content,
			parser.Text(message.Get("toolName")), parser.Text(message.Get("toolCallId")))
	case isText(role, "bashExecution"):
		w.bash(current, line, timestamp, record, message)
	case isText(role, "branchSummary"):
		w.system(current, line, timestamp, "branch summary: "+parser.Text(message.Get("summary")), "")
	case isText(role, "compactionSummary"):
		w.system(current, line, timestamp, "compaction summary: "+parser.Text(message.Get("summary")), "")
	case isText(role, "custom"):
		w.system(current, line, timestamp, "custom "+parser.Text(message.Get("customType"))+": "+content, "")
	default:
		w.system(current, line, timestamp, orText(parser.Text(role), "message")+": "+
			orText(content, parser.JSONText(message)), "")
	}
}

// tool emits a tool call or result row with no model.
func (w *writer) tool(current *session, line int, timestamp, turnType, text, name, toolUseID string) {
	row := w.row(current, line, timestamp, turnType, text)
	row.ToolName, row.ToolUseID = name, toolUseID
	w.emit(row)
}

// bash emits the call and the result of a bash execution the user ran.
func (w *writer) bash(current *session, line int, timestamp string, record, message jsonvalue.Value) {
	toolUseID := parser.Text(record.Get("id"))
	w.tool(current, line, timestamp, timeline.TurnToolUse, parser.Text(message.Get("command")), "bash",
		toolUseID)
	status := "exit=" + message.Get("exitCode").Text()
	if !message.Get("cancelled").IsEmpty() {
		status += " cancelled"
	}
	if !message.Get("truncated").IsEmpty() {
		status += " truncated"
	}
	w.tool(current, line, timestamp, timeline.TurnToolResult,
		"["+status+"] "+parser.Text(message.Get("output")), "bash", toolUseID)
}

// assistant emits the rows of an assistant message: its `model:` row when
// the model is new, its blocks, and an error or abort.
func (w *writer) assistant(current *session, line int, timestamp string, message jsonvalue.Value) {
	requested := parser.Text(message.Get("model"))
	model := cmp.Or(parser.Text(message.Get("responseModel")), requested, current.model)
	if model != "" {
		current.model = model
	}
	if !message.Get("responseModel").IsEmpty() || requested != "" {
		text := pi.ModelText(current.seen, current.sessionID, model, parser.Text(message.Get("provider")), requested)
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
	if content.IsArray() {
		for _, block := range content.Items() {
			w.block(current, line, timestamp, block, model)
		}
	}
	stop := message.Get("stopReason")
	errorMessage := message.Get("errorMessage")
	if isText(stop, "error") || isText(stop, "aborted") || !errorMessage.IsEmpty() {
		w.system(current, line, timestamp, "assistant "+orText(parser.Text(stop), "error")+": "+
			parser.Text(errorMessage), model)
	}
}

// block emits the row of one content block of an assistant message.
func (w *writer) block(current *session, line int, timestamp string, block jsonvalue.Value, model string) {
	if !block.IsObject() {
		return
	}
	var row timeline.Row
	blockType := block.Get("type")
	switch {
	case isText(blockType, "text") && !block.Get("text").IsEmpty():
		row = w.row(current, line, timestamp, timeline.TurnAssistant, block.Get("text").Text())
	case isText(blockType, "thinking") && w.options.IncludeThinking:
		thinking := parser.Text(block.Get("thinking"))
		if thinking == "" && !block.Get("redacted").IsEmpty() {
			thinking = "[redacted thinking]"
		}
		if thinking == "" {
			return
		}
		row = w.row(current, line, timestamp, timeline.TurnThinking, thinking)
	case isText(blockType, "toolCall"):
		row = w.row(current, line, timestamp, timeline.TurnToolUse, ToolArgsText(block.Get("arguments")))
		row.ToolName, row.ToolUseID = parser.Text(block.Get("name")), parser.Text(block.Get("id"))
	case isText(blockType, "image"):
		row = w.row(current, line, timestamp, timeline.TurnAssistant, "[image]")
	default:
		return
	}
	row.Model = model
	w.emit(row)
}
