package goose

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	options   parserkit.Options
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

// toolRequest fills the row of a toolRequest block and returns its text,
// remembering the tool's name for the response.
func (r *blockReader) toolRequest(row *timeline.Row, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnToolUse
	row.ToolUseID = parserkit.Text(block.Get("id"))
	call := orObject(block.Get("toolCall"))
	text := ""
	if parserkit.IsText(call.Get("status"), "error") {
		text = "[error] " + parserkit.Text(call.Get("error"))
	} else {
		value := orObject(call.Get("value"))
		row.ToolName = parserkit.Text(value.Get("name"))
		text = parserkit.JSONText(orObject(value.Get("arguments")))
	}
	r.toolNames[row.ToolUseID] = row.ToolName
	return text
}

// resultText returns the text of a toolResponse's toolResult and whether
// it is an error.
func resultText(result jsonvalue.Value) (string, bool) {
	if !result.IsObject() {
		return parserkit.JSONText(result), false
	}
	if parserkit.IsText(result.Get("status"), "error") {
		return parserkit.Text(result.Get("error")), true
	}
	switch value := result.Get("value"); {
	case value.IsArray():
		// Older rows hold a bare content array.
		return parserkit.TextOf(value), false
	case value.IsObject():
		text := parserkit.TextOf(value.Get("content"))
		if structured, ok := value.Lookup("structuredContent"); text == "" && ok && !structured.IsNull() {
			text = parserkit.JSONText(structured)
		}
		return text, !value.Get("isError").IsEmpty()
	}
	return "", false
}

// toolResponse fills the row of a toolResponse block and returns its text.
func (r *blockReader) toolResponse(row *timeline.Row, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnToolResult
	row.ToolUseID = parserkit.Text(block.Get("id"))
	row.ToolName = r.toolNames[row.ToolUseID]
	text, isError := resultText(block.Get("toolResult"))
	if isError {
		text = "[error] " + text
	}
	return text
}

// actionText returns the text of an actionRequired block, setting the
// row's tool join: a tool confirmation is a "permission request:", its
// answer a "permission decision:", and an elicitation keeps the generic
// "action required:" row.
func (r *blockReader) actionText(row *timeline.Row, data jsonvalue.Value) string {
	switch kind := data.Get("actionType"); {
	case data.IsObject() && parserkit.IsText(kind, "toolConfirmation"):
		row.ToolName = parserkit.Text(data.Get("toolName"))
		row.ToolUseID = parserkit.Text(data.Get("id"))
		return parserkit.JoinNonEmpty(" | ", "permission request: "+row.ToolName, parserkit.Text(data.Get("prompt")),
			parserkit.JSONText(orObject(data.Get("arguments"))))
	case data.IsObject() && parserkit.IsText(kind, "toolConfirmationResponse"):
		row.ToolUseID = parserkit.Text(data.Get("id"))
		row.ToolName = r.toolNames[row.ToolUseID]
		permission := data.Get("permission")
		if permission.IsEmpty() {
			permission = jsonvalue.NewString("-")
		}
		return "permission decision: " + parserkit.SettingValue(permission) + " (toolConfirmationResponse)"
	}
	return "action required: " + parserkit.JSONText(data)
}

// systemBlock fills the row of a confirmation request, notification, error,
// action or unknown block, all system rows, and returns its text.
func (r *blockReader) systemBlock(row *timeline.Row, kind string, block jsonvalue.Value) string {
	row.TurnType = timeline.TurnSystem
	switch kind {
	case "toolConfirmationRequest":
		row.ToolName = parserkit.Text(block.Get("toolName"))
		row.ToolUseID = parserkit.Text(block.Get("id"))
		return parserkit.JoinNonEmpty(" | ", "confirmation requested: "+row.ToolName, parserkit.Text(block.Get("prompt")),
			parserkit.JSONText(orObject(block.Get("arguments"))))
	case "systemNotification":
		notification := parserkit.Text(block.Get("notificationType"))
		if notification == "" {
			notification = "notification"
		}
		return notification + ": " + parserkit.Text(block.Get("msg"))
	case "error":
		return "error " + parserkit.Text(block.Get("kind")) + ": " + parserkit.Text(block.Get("message"))
	case "actionRequired":
		return r.actionText(row, block.Get("data"))
	}
	return parserkit.JSONText(block)
}

// block returns the row of one content block, or false for a thinking
// block the options leave out.
func (r *blockReader) block(base timeline.Row, role, textType string, visible bool,
	block jsonvalue.Value) (timeline.Row, bool) {
	row, text := base, ""
	kind := block.Get("type")
	switch kindText := kind.StringValue(); {
	case !kind.IsString():
		row.TurnType, text = timeline.TurnSystem, parserkit.JSONText(block)
	case kindText == "text":
		row.TurnType, text = textType, parserkit.Text(block.Get("text"))
		if textType == timeline.TurnSystem {
			text = role + ": " + text
		}
		if !visible {
			text = "[hidden] " + text
		}
	case kindText == "image" || kindText == "document":
		row.TurnType = textType
		text = "[" + kindText + "] " + parserkit.JoinNonEmpty(" | ", parserkit.Text(block.Get("name")),
			parserkit.Text(block.Get("mimeType")))
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
			text = parserkit.Text(block.Get("thinking"))
		}
	default:
		text = r.systemBlock(&row, kindText, block)
	}
	row.Text = timeline.Compact(text)
	return row, true
}

// blocks emits the rows of a message's content blocks from base. answered,
// when not nil (only for sessions.db), holds the ids of the tool requests
// the session's toolConfirmationResponse blocks answer: a toolRequest whose
// _meta holds goose.executable false and no answer is followed by its
// "permission decision: not executable" row. Blocks that are not objects
// add a problem to problems, at the base row's line, and so does a role
// longer than parserkit.CarriedLimit, which each text block's row repeats.
func (r *blockReader) blocks(base timeline.Row, role string, content jsonvalue.Value,
	visible bool, answered map[string]bool, problems *parserkit.Problems, emit parserkit.Emit) {
	content = textBlocks(content)
	if !content.IsArray() {
		return
	}
	textType := timeline.TurnSystem
	if role == timeline.TurnUser || role == timeline.TurnAssistant {
		textType = role
	}
	carried := parserkit.NewCarried(r.artifact)
	role = carried.Cut("role", role)
	defer carried.Report(base.SourceLine, problems)
	for _, block := range parserkit.EntriesOfKind(content, "content", jsonvalue.Object, base.SourceLine, problems) {
		row, keep := r.block(base, role, textType, visible, block)
		if !keep {
			continue
		}
		emit(row)
		if !parserkit.IsText(block.Get("type"), "toolRequest") || answered == nil {
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
