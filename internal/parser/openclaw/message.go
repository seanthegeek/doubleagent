package openclaw

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// message emits the rows of a message entry by its role.
func (w *writer) message(current *session, line int, timestamp string, record, message pyjson.Value) {
	role := message.Get("role")
	content := parser.TextOf(message.Get("content"))
	switch {
	case isText(role, "user"):
		if message.Get("runtimeContext").Truthy() || message.Get("runtimeContextCarrier").Truthy() {
			w.system(current, line, timestamp, "runtime context: "+content, "")
		} else {
			w.emit(w.row(current, line, timestamp, timeline.TurnUser, content))
		}
	case isText(role, "assistant"):
		w.assistant(current, line, timestamp, message)
	case isText(role, "toolResult"):
		if message.Get("isError").Truthy() {
			content = "[error] " + content
		}
		w.tool(current, line, timestamp, timeline.TurnToolResult, content,
			parser.Str(message.Get("toolName")), parser.Str(message.Get("toolCallId")))
	case isText(role, "bashExecution"):
		w.bash(current, line, timestamp, record, message)
	case isText(role, "branchSummary"):
		w.system(current, line, timestamp, "branch summary: "+parser.Str(message.Get("summary")), "")
	case isText(role, "compactionSummary"):
		w.system(current, line, timestamp, "compaction summary: "+parser.Str(message.Get("summary")), "")
	case isText(role, "custom"):
		w.system(current, line, timestamp, "custom "+parser.Str(message.Get("customType"))+": "+content, "")
	default:
		w.system(current, line, timestamp, orText(parser.Str(role), "message")+": "+
			orText(content, parser.CompactJSON(message)), "")
	}
}

// tool emits a tool call or result row with no model.
func (w *writer) tool(current *session, line int, timestamp, turnType, text, name, toolUseID string) {
	row := w.row(current, line, timestamp, turnType, text)
	row.ToolName, row.ToolUseID = name, toolUseID
	w.emit(row)
}

// bash emits the call and the result of a bash execution the user ran.
func (w *writer) bash(current *session, line int, timestamp string, record, message pyjson.Value) {
	toolUseID := parser.Str(record.Get("id"))
	w.tool(current, line, timestamp, timeline.TurnToolUse, parser.Str(message.Get("command")), "bash",
		toolUseID)
	status := "exit=" + message.Get("exitCode").Str()
	if message.Get("cancelled").Truthy() {
		status += " cancelled"
	}
	if message.Get("truncated").Truthy() {
		status += " truncated"
	}
	w.tool(current, line, timestamp, timeline.TurnToolResult,
		"["+status+"] "+parser.Str(message.Get("output")), "bash", toolUseID)
}

// assistant emits the rows of an assistant message: its `model:` row when
// the model is new, its blocks, and an error or abort.
func (w *writer) assistant(current *session, line int, timestamp string, message pyjson.Value) {
	requested := parser.Str(message.Get("model"))
	model := cmp.Or(parser.Str(message.Get("responseModel")), requested, current.model)
	if model != "" {
		current.model = model
	}
	if message.Get("responseModel").Truthy() || requested != "" {
		text := pi.ModelText(current.seen, current.sessionID, model, parser.Str(message.Get("provider")), requested)
		if text != "" {
			w.system(current, line, timestamp, text, model)
		}
	}
	content := message.Get("content")
	if content.IsString() {
		content = pyjson.NewArray(pyjson.NewObject(
			pyjson.Member{Key: "type", Value: pyjson.NewString("text")},
			pyjson.Member{Key: "text", Value: content}))
	}
	if content.IsArray() {
		for _, block := range content.Items() {
			w.block(current, line, timestamp, block, model)
		}
	}
	stop := message.Get("stopReason")
	errorMessage := message.Get("errorMessage")
	if isText(stop, "error") || isText(stop, "aborted") || errorMessage.Truthy() {
		w.system(current, line, timestamp, "assistant "+orText(parser.Str(stop), "error")+": "+
			parser.Str(errorMessage), model)
	}
}

// block emits the row of one content block of an assistant message.
func (w *writer) block(current *session, line int, timestamp string, block pyjson.Value, model string) {
	if !block.IsObject() {
		return
	}
	var row timeline.Row
	blockType := block.Get("type")
	switch {
	case isText(blockType, "text") && block.Get("text").Truthy():
		row = w.row(current, line, timestamp, timeline.TurnAssistant, block.Get("text").Str())
	case isText(blockType, "thinking") && w.options.IncludeThinking:
		thinking := parser.Str(block.Get("thinking"))
		if thinking == "" && block.Get("redacted").Truthy() {
			thinking = "[redacted thinking]"
		}
		if thinking == "" {
			return
		}
		row = w.row(current, line, timestamp, timeline.TurnThinking, thinking)
	case isText(blockType, "toolCall"):
		row = w.row(current, line, timestamp, timeline.TurnToolUse, ToolArgsText(block.Get("arguments")))
		row.ToolName, row.ToolUseID = parser.Str(block.Get("name")), parser.Str(block.Get("id"))
	case isText(blockType, "image"):
		row = w.row(current, line, timestamp, timeline.TurnAssistant, "[image]")
	default:
		return
	}
	row.Model = model
	w.emit(row)
}
