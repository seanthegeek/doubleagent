package goose

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// toolExecutableKey is the _meta key of a tool request the agent may not
// run.
const toolExecutableKey = "goose.executable"

// blockReader turns content blocks into rows, remembering each tool
// request's name for its response across one file.
type blockReader struct {
	parser    *Parser
	artifact  *input.Artifact
	options   parser.Options
	toolNames map[string]string
}

// orObject returns value when it is truthy, else an empty object: the
// `value or {}` of a value read as an object.
func orObject(value pyjson.Value) pyjson.Value {
	if value.Truthy() {
		return value
	}
	return pyjson.NewObject()
}

// textBlocks returns content as its blocks: a string is one text block.
func textBlocks(content pyjson.Value) pyjson.Value {
	if content.IsString() {
		return pyjson.NewArray(pyjson.NewObject(pyjson.Member{Key: "type", Value: pyjson.NewString("text")},
			pyjson.Member{Key: "text", Value: content}))
	}
	return content
}

// blocks emits the rows of a message's content blocks from base. answered,
// when not nil (only for sessions.db), holds the ids of the tool requests
// the session's toolConfirmationResponse blocks answer: a toolRequest whose
// _meta holds goose.executable false and no answer is followed by its
// "permission decision: not executable" row.
func (r *blockReader) blocks(base timeline.Row, role string, content pyjson.Value,
	visible bool, answered map[string]bool, emit parser.Emit) {
	content = textBlocks(content)
	if !content.IsArray() {
		return
	}
	textType := timeline.TurnSystem
	if role == timeline.TurnUser || role == timeline.TurnAssistant {
		textType = role
	}
	for _, block := range content.Items() {
		if !block.IsObject() {
			continue
		}
		row, keep := r.block(base, role, textType, visible, block)
		if !keep {
			continue
		}
		emit(row)
		if !isText(block.Get("type"), "toolRequest") || answered == nil {
			continue
		}
		verdict := block.Get("_meta")
		executable, isBool := verdict.Get(toolExecutableKey).Bool()
		if verdict.IsObject() && isBool && !executable && !answered[row.ToolUseID] {
			row.TurnType = timeline.TurnSystem
			row.Text = "permission decision: not executable (goose.executable=false)"
			emit(row)
		}
	}
}

// isText reports whether value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// block returns the row of one content block, or false for a thinking
// block the options leave out.
func (r *blockReader) block(base timeline.Row, role, textType string, visible bool,
	block pyjson.Value) (timeline.Row, bool) {
	row, text := base, ""
	kind := block.Get("type")
	switch kindText := kind.Text(); {
	case !kind.IsString():
		row.TurnType, text = timeline.TurnSystem, parser.CompactJSON(block)
	case kindText == "text":
		row.TurnType, text = textType, parser.Str(block.Get("text"))
		if textType == timeline.TurnSystem {
			text = role + ": " + text
		}
		if !visible {
			text = "[hidden] " + text
		}
	case kindText == "image" || kindText == "document":
		row.TurnType = textType
		text = "[" + kindText + "] " + joinParts(parser.Str(block.Get("name")),
			parser.Str(block.Get("mimeType")))
	case kindText == "toolRequest":
		text = r.toolRequest(&row, block)
	case kindText == "toolResponse":
		text = r.toolResponse(&row, block)
	case kindText == "thinking" || kindText == "redactedThinking":
		if !r.options.IncludeThinking {
			return row, false
		}
		row.TurnType, text = timeline.TurnThinking, "[redacted thinking]"
		if kindText == "thinking" {
			text = parser.Str(block.Get("thinking"))
		}
	default:
		text = r.systemBlock(&row, kindText, block)
	}
	row.Text = timeline.Compact(text)
	return row, true
}

// toolRequest fills the row of a toolRequest block and returns its text,
// remembering the tool's name for the response.
func (r *blockReader) toolRequest(row *timeline.Row, block pyjson.Value) string {
	row.TurnType = timeline.TurnToolUse
	row.ToolUseID = parser.Str(block.Get("id"))
	call := orObject(block.Get("toolCall"))
	text := ""
	if isText(call.Get("status"), "error") {
		text = "[error] " + parser.Str(call.Get("error"))
	} else {
		value := orObject(call.Get("value"))
		row.ToolName = parser.Str(value.Get("name"))
		text = parser.CompactJSON(orObject(value.Get("arguments")))
	}
	r.toolNames[row.ToolUseID] = row.ToolName
	return text
}

// toolResponse fills the row of a toolResponse block and returns its text.
func (r *blockReader) toolResponse(row *timeline.Row, block pyjson.Value) string {
	row.TurnType = timeline.TurnToolResult
	row.ToolUseID = parser.Str(block.Get("id"))
	row.ToolName = r.toolNames[row.ToolUseID]
	text, isError := resultText(block.Get("toolResult"))
	if isError {
		text = "[error] " + text
	}
	return text
}

// systemBlock fills the row of a confirmation request, notification, error,
// action or unknown block, all system rows, and returns its text.
func (r *blockReader) systemBlock(row *timeline.Row, kind string, block pyjson.Value) string {
	row.TurnType = timeline.TurnSystem
	switch kind {
	case "toolConfirmationRequest":
		row.ToolName = parser.Str(block.Get("toolName"))
		row.ToolUseID = parser.Str(block.Get("id"))
		return joinParts("confirmation requested: "+row.ToolName, parser.Str(block.Get("prompt")),
			parser.CompactJSON(orObject(block.Get("arguments"))))
	case "systemNotification":
		notification := parser.Str(block.Get("notificationType"))
		if notification == "" {
			notification = "notification"
		}
		return notification + ": " + parser.Str(block.Get("msg"))
	case "error":
		return "error " + parser.Str(block.Get("kind")) + ": " + parser.Str(block.Get("message"))
	case "actionRequired":
		return r.actionText(row, block.Get("data"))
	}
	return parser.CompactJSON(block)
}

// actionText returns the text of an actionRequired block, setting the
// row's tool join: a tool confirmation is a "permission request:", its
// answer a "permission decision:", and an elicitation keeps the generic
// "action required:" row.
func (r *blockReader) actionText(row *timeline.Row, data pyjson.Value) string {
	switch kind := data.Get("actionType"); {
	case data.IsObject() && isText(kind, "toolConfirmation"):
		row.ToolName = parser.Str(data.Get("toolName"))
		row.ToolUseID = parser.Str(data.Get("id"))
		return joinParts("permission request: "+row.ToolName, parser.Str(data.Get("prompt")),
			parser.CompactJSON(orObject(data.Get("arguments"))))
	case data.IsObject() && isText(kind, "toolConfirmationResponse"):
		row.ToolUseID = parser.Str(data.Get("id"))
		row.ToolName = r.toolNames[row.ToolUseID]
		permission := data.Get("permission")
		if !permission.Truthy() {
			permission = pyjson.NewString("-")
		}
		return "permission decision: " + parser.SettingValue(permission) + " (toolConfirmationResponse)"
	}
	return "action required: " + parser.CompactJSON(data)
}

// resultText returns the text of a toolResponse's toolResult and whether
// it is an error.
func resultText(result pyjson.Value) (string, bool) {
	if !result.IsObject() {
		return parser.CompactJSON(result), false
	}
	if isText(result.Get("status"), "error") {
		return parser.Str(result.Get("error")), true
	}
	switch value := result.Get("value"); {
	case value.IsArray():
		// Older rows hold a bare content array.
		return parser.TextOf(value), false
	case value.IsObject():
		text := parser.TextOf(value.Get("content"))
		if structured, ok := value.Lookup("structuredContent"); text == "" && ok && !structured.IsNull() {
			text = parser.CompactJSON(structured)
		}
		return text, value.Get("isError").Truthy()
	}
	return "", false
}
