package clinelegacy

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// ParseUI emits the rows of ui_messages.json, then a row per problem.
func (p *TaskParser) ParseUI(artifact *input.Artifact, context *TaskContext,
	options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	model, lastToolID := "", ""
	for position, message := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !message.IsObject() {
			continue
		}
		if info := message.Get("modelInfo"); info.IsObject() && !info.Get("modelId").IsEmpty() {
			model = info.Get("modelId").Text()
		}
		timestamp := message.Get("ts")
		rowModel := model
		if rowModel == "" {
			rowModel = context.ModelAt(timestamp)
		}
		kind := messageKind(message)
		reasoning := message.Get("reasoning")
		if options.IncludeThinking && reasoning.IsString() && reasoning.StringValue() != "" && kind != "reasoning" {
			row := p.Row(artifact, context, position, timeutil.FromValue(timestamp), rowModel)
			row.TurnType = timeline.TurnThinking
			row.Text = timeline.Compact(reasoning.StringValue())
			emit(row)
		}
		mapped, ok := MapUI(kind, message, position == 1)
		if !ok || (mapped.TurnType == timeline.TurnThinking && !options.IncludeThinking) {
			continue
		}
		row := p.Row(artifact, context, position, timeutil.FromValue(timestamp), rowModel)
		row.TurnType = mapped.TurnType
		switch mapped.TurnType {
		case timeline.TurnToolUse:
			lastToolID = toolID(timestamp, position)
			row.ToolUseID, row.ToolName = lastToolID, mapped.ToolName
		case timeline.TurnToolResult:
			row.ToolUseID, row.ToolName = lastToolID, mapped.ToolName
		}
		row.Text = timeline.Compact(partialText(mapped.Text, message))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.TaskID, context.ProjectPath))
}

// messageKind returns a message's say for a "say" message, else its ask,
// falling back to whichever of say, ask and type is set.
func messageKind(message jsonvalue.Value) string {
	kind := message.Get("ask")
	if isText(message.Get("type"), "say") {
		kind = message.Get("say")
	}
	return parser.Text(FirstTruthy(kind, message.Get("say"), message.Get("ask"), message.Get("type")))
}

// toolID returns the id a tool call's rows share: the message's ts as
// Python's str() writes it, or its position when it has none.
func toolID(timestamp jsonvalue.Value, position int) string {
	if timestamp.IsNull() {
		return strconv.Itoa(position)
	}
	return timestamp.Text()
}

// partialText marks the text of a message still being streamed.
func partialText(text string, message jsonvalue.Value) string {
	if message.Get("partial").IsEmpty() {
		return text
	}
	if text == "" {
		return "[partial]"
	}
	return text + " [partial]"
}

// isText reports whether a value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// Mapped is the row one ui_messages.json message gives: its turn type,
// text and tool name.
type Mapped struct {
	TurnType, Text, ToolName string
}

// IsToolUseKind reports whether a message kind is a tool call.
func IsToolUseKind(kind string) bool {
	switch kind {
	case "tool", "command", "use_mcp_server", "browser_action_launch", "browser_action",
		"new_task", "use_subagents", "condense", "summarize_task":
		return true
	}
	return false
}

// isUserKind reports whether a message kind is the user's own text.
func isUserKind(kind string) bool {
	return kind == "task" || kind == "user_feedback" || kind == "user_feedback_diff"
}

// isAssistantKind reports whether a message kind is the model's reply.
func isAssistantKind(kind string) bool {
	switch kind {
	case "text", "completion_result", "plan_completion_result", "followup",
		"plan_mode_respond", "act_mode_respond":
		return true
	}
	return false
}

// resultTool returns the tool name of a tool result kind, and false for a
// kind that is not one.
func resultTool(kind string) (string, bool) {
	switch kind {
	case "command_output":
		return "execute_command", true
	case "browser_action_result":
		return "browser_action", true
	case "codebase_search_result":
		return "codebaseSearch", true
	case "subtask_result":
		return "new_task", true
	case "mcp_server_response":
		return "", true
	}
	return "", false
}

// MapUI returns the row of one ui_messages.json message of a kind, or
// false to skip it. first is set for the file's first message, whose
// "text" kind is the user's task.
func MapUI(kind string, message jsonvalue.Value, first bool) (Mapped, bool) {
	text := messageText(message.Get("text"))
	switch {
	case isUserKind(kind) || (kind == "text" && first):
		if images := message.Get("images"); images.IsArray() && images.Len() > 0 {
			text = Join(jsonvalue.NewString(text), jsonvalue.NewString("["+strconv.Itoa(images.Len())+" image(s)]"))
		}
		return Mapped{TurnType: timeline.TurnUser, Text: text}, text != ""
	case kind == "reasoning":
		return Mapped{TurnType: timeline.TurnThinking, Text: text}, text != ""
	case isAssistantKind(kind):
		text = assistantText(text)
		return Mapped{TurnType: timeline.TurnAssistant, Text: text}, text != ""
	case IsToolUseKind(kind):
		name, body := toolUseText(kind, text)
		return Mapped{TurnType: timeline.TurnToolUse, Text: body, ToolName: name}, body != "" || name != ""
	}
	if name, ok := resultTool(kind); ok {
		if kind == "browser_action_result" {
			parsed, _ := LoadsOrNone(text)
			text = Join(parsed.Get("currentUrl"), parsed.Get("logs"), screenshot(parsed))
		}
		return Mapped{TurnType: timeline.TurnToolResult, Text: text, ToolName: name}, text != ""
	}
	return Mapped{TurnType: timeline.TurnSystem, Text: systemText(kind, text, message)}, true
}

// messageText returns a message's text field: a string as it is, "" for
// none, anything else as CompactJSON.
func messageText(value jsonvalue.Value) string {
	switch {
	case value.IsString():
		return value.StringValue()
	case value.IsNull():
		return ""
	}
	return parser.JSONText(value)
}

// screenshot returns "[screenshot]" when a browser result holds one.
func screenshot(parsed jsonvalue.Value) jsonvalue.Value {
	if !parsed.Get("screenshot").IsEmpty() {
		return jsonvalue.NewString("[screenshot]")
	}
	return jsonvalue.NewString("")
}

// assistantText returns a reply's text: for JSON text (a followup or a
// plan response) its question or response and the choices offered.
func assistantText(text string) string {
	parsed, ok := LoadsOrNone(text)
	if !ok {
		return text
	}
	choices := FirstTruthy(parsed.Get("options"), parsed.Get("suggest"))
	if choices.IsArray() {
		choices = jsonvalue.NewString(choiceList(choices))
	}
	question := FirstTruthy(parsed.Get("question"), parsed.Get("response"), jsonvalue.NewString(""))
	return Join(question, FirstTruthy(choices, jsonvalue.NewString("")))
}

// choiceList returns the choices of a followup joined by "; ": a string as
// it is, an object as its answer or else its CompactJSON.
func choiceList(choices jsonvalue.Value) string {
	parts := make([]string, 0, choices.Len())
	for _, choice := range choices.Items() {
		switch {
		case choice.IsString():
			parts = append(parts, choice.StringValue())
		case choice.IsObject() && !choice.Get("answer").IsEmpty():
			parts = append(parts, choice.Get("answer").Text())
		case choice.IsObject():
			parts = append(parts, parser.JSONText(choice))
		default:
			parts = append(parts, choice.Text())
		}
	}
	return strings.Join(parts, "; ")
}

// systemText returns the text of a message kept as a system row: the
// request counts of an API request, a context condensation's counts and
// summary, a checkpoint's hash, else "<kind>: <text>" or the kind alone.
func systemText(kind, text string, message jsonvalue.Value) string {
	switch kind {
	case "api_req_started", "api_req_finished", "api_req_deleted":
		parsed, _ := LoadsOrNone(text)
		detail := FormatCounts(parsed, "tokensIn", "tokensOut", "cacheWrites", "cacheReads",
			"cost", "apiProtocol", "cancelReason", "streamingFailedMessage")
		if detail == "" {
			return kind
		}
		return kind + ": " + detail
	case "condense_context":
		condense := message.Get("contextCondense")
		if condense.IsObject() && condense.Len() > 0 {
			detail := FormatCounts(condense, "prevContextTokens", "newContextTokens", "cost")
			return "condense_context: " + detail + " | " + parser.Text(condense.Get("summary"))
		}
	case "checkpoint_created":
		if hash := message.Get("lastCheckpointHash"); !hash.IsEmpty() {
			return "checkpoint_created: " + hash.Text()
		}
	}
	if text == "" {
		return kind
	}
	return kind + ": " + text
}

// toolUseText returns the tool name and text of a tool call message.
func toolUseText(kind, text string) (string, string) {
	parsed, ok := LoadsOrNone(text)
	switch {
	case kind == "tool" && !ok:
		return "tool", text
	case kind == "tool":
		return parser.Text(FirstTruthy(parsed.Get("tool"), jsonvalue.NewString("tool"))), fileToolText(parsed)
	case kind == "use_mcp_server" && !ok:
		return "use_mcp_server", text
	case kind == "use_mcp_server":
		name := FirstTruthy(parsed.Get("toolName"), parsed.Get("type"), jsonvalue.NewString("use_mcp_server"))
		return name.Text(), Join(parsed.Get("serverName"), parsed.Get("uri"), parsed.Get("arguments"))
	case kind == "browser_action" && ok:
		return "browser_action", Join(parsed.Get("action"), parsed.Get("coordinate"), parsed.Get("text"))
	case kind == "use_subagents" && ok && parsed.Get("prompts").IsArray():
		prompts := parsed.Get("prompts").Items()
		parts := make([]string, len(prompts))
		for i, prompt := range prompts {
			parts[i] = prompt.Text()
		}
		return "use_subagents", strings.Join(parts, " ; ")
	}
	switch kind {
	case "command":
		return "execute_command", text
	case "browser_action_launch", "browser_action":
		return "browser_action", text
	}
	return kind, text
}

// fileToolText returns the text of a "tool" message: its path, the paths
// of a batch, the search and mode fields, the reason and the diff or
// content.
func fileToolText(parsed jsonvalue.Value) string {
	paths := ""
	if batch := parsed.Get("batchFiles"); batch.IsArray() {
		var parts []string
		for _, entry := range batch.Items() {
			if entry.IsObject() {
				parts = append(parts, entry.Get("path").Text())
			}
		}
		paths = strings.Join(parts, "; ")
	}
	return Join(parsed.Get("path"), jsonvalue.NewString(paths), parsed.Get("regex"),
		parsed.Get("filePattern"), parsed.Get("query"), parsed.Get("mode"), parsed.Get("reason"),
		FirstTruthy(parsed.Get("diff"), parsed.Get("content")))
}
