package goose

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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

// orObject returns value, or an empty object in place of an empty value,
// so a lookup on a missing level reads as empty.
func orObject(value jsonvalue.Value) jsonvalue.Value {
	if !value.IsEmpty() {
		return value
	}
	return jsonvalue.NewObject()
}

// textBlocks returns content as its blocks: a string is one text block.
func textBlocks(content jsonvalue.Value) jsonvalue.Value {
	if content.IsString() {
		return jsonvalue.NewArray(jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	return content
}

// blocks emits the rows of a message's content blocks from base. answered,
// when not nil (only for sessions.db), holds the ids of the tool requests
// the session's toolConfirmationResponse blocks answer: a toolRequest whose
// _meta holds goose.executable false and no answer is followed by its
// "permission decision: not executable" row.
func (r *blockReader) blocks(base timeline.Row, role string, content jsonvalue.Value,
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
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// block returns the row of one content block, or false for a thinking
// block the options leave out.
func (r *blockReader) block(base timeline.Row, role, textType string, visible bool,
	block jsonvalue.Value) (timeline.Row, bool) {
	row, text := base, ""
	kind := block.Get("type")
	switch kindText := kind.StringValue(); {
	case !kind.IsString():
		row.TurnType, text = timeline.TurnSystem, parser.JSONText(block)
	case kindText == "text":
		row.TurnType, text = textType, parser.Text(block.Get("text"))
		if textType == timeline.TurnSystem {
			text = role + ": " + text
		}
		if !visible {
			text = "[hidden] " + text
		}
	case kindText == "image" || kindText == "document":
		row.TurnType = textType
		text = "[" + kindText + "] " + parser.JoinNonEmpty(" | ", parser.Text(block.Get("name")),
			parser.Text(block.Get("mimeType")))
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
			text = parser.Text(block.Get("thinking"))
		}
	default:
		text = r.systemBlock(&row, kindText, block)
	}
	row.Text = timeline.Compact(text)
	return row, true
}

// toolRequest fills the row of a toolRequest block and returns its text,
// remembering the tool's name for the response.
func (r *blockReader) toolRequest(row *timeline.Row, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnToolUse
	row.ToolUseID = parser.Text(block.Get("id"))
	call := orObject(block.Get("toolCall"))
	text := ""
	if isText(call.Get("status"), "error") {
		text = "[error] " + parser.Text(call.Get("error"))
	} else {
		value := orObject(call.Get("value"))
		row.ToolName = parser.Text(value.Get("name"))
		text = parser.JSONText(orObject(value.Get("arguments")))
	}
	r.toolNames[row.ToolUseID] = row.ToolName
	return text
}

// toolResponse fills the row of a toolResponse block and returns its text.
func (r *blockReader) toolResponse(row *timeline.Row, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnToolResult
	row.ToolUseID = parser.Text(block.Get("id"))
	row.ToolName = r.toolNames[row.ToolUseID]
	text, isError := resultText(block.Get("toolResult"))
	if isError {
		text = "[error] " + text
	}
	return text
}

// systemBlock fills the row of a confirmation request, notification, error,
// action or unknown block, all system rows, and returns its text.
func (r *blockReader) systemBlock(row *timeline.Row, kind string, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnSystem
	switch kind {
	case "toolConfirmationRequest":
		row.ToolName = parser.Text(block.Get("toolName"))
		row.ToolUseID = parser.Text(block.Get("id"))
		return parser.JoinNonEmpty(" | ", "confirmation requested: "+row.ToolName, parser.Text(block.Get("prompt")),
			parser.JSONText(orObject(block.Get("arguments"))))
	case "systemNotification":
		notification := parser.Text(block.Get("notificationType"))
		if notification == "" {
			notification = "notification"
		}
		return notification + ": " + parser.Text(block.Get("msg"))
	case "error":
		return "error " + parser.Text(block.Get("kind")) + ": " + parser.Text(block.Get("message"))
	case "actionRequired":
		return r.actionText(row, block.Get("data"))
	}
	return parser.JSONText(block)
}

// actionText returns the text of an actionRequired block, setting the
// row's tool join: a tool confirmation is a "permission request:", its
// answer a "permission decision:", and an elicitation keeps the generic
// "action required:" row.
func (r *blockReader) actionText(row *timeline.Row, data jsonvalue.Value) string {
	switch kind := data.Get("actionType"); {
	case data.IsObject() && isText(kind, "toolConfirmation"):
		row.ToolName = parser.Text(data.Get("toolName"))
		row.ToolUseID = parser.Text(data.Get("id"))
		return parser.JoinNonEmpty(" | ", "permission request: "+row.ToolName, parser.Text(data.Get("prompt")),
			parser.JSONText(orObject(data.Get("arguments"))))
	case data.IsObject() && isText(kind, "toolConfirmationResponse"):
		row.ToolUseID = parser.Text(data.Get("id"))
		row.ToolName = r.toolNames[row.ToolUseID]
		permission := data.Get("permission")
		if permission.IsEmpty() {
			permission = jsonvalue.NewString("-")
		}
		return "permission decision: " + parser.SettingValue(permission) + " (toolConfirmationResponse)"
	}
	return "action required: " + parser.JSONText(data)
}

// resultText returns the text of a toolResponse's toolResult and whether
// it is an error.
func resultText(result jsonvalue.Value) (string, bool) {
	if !result.IsObject() {
		return parser.JSONText(result), false
	}
	if isText(result.Get("status"), "error") {
		return parser.Text(result.Get("error")), true
	}
	switch value := result.Get("value"); {
	case value.IsArray():
		// Older rows hold a bare content array.
		return parser.TextOf(value), false
	case value.IsObject():
		text := parser.TextOf(value.Get("content"))
		if structured, ok := value.Lookup("structuredContent"); text == "" && ok && !structured.IsNull() {
			text = parser.JSONText(structured)
		}
		return text, !value.Get("isError").IsEmpty()
	}
	return "", false
}
