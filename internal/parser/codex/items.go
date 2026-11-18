package codex

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// itemTime says which time of a turn item a row takes: when the item
// started or when it completed.
type itemTime uint8

// The two times of a turn item.
const (
	atStart itemTime = iota
	atEnd
)

// itemRow is one row of a turn item: its turn type, which of the item's
// times it takes, its tool name and call id, and its text.
type itemRow struct {
	turnType  string
	when      itemTime
	toolName  string
	toolUseID string
	text      string
}

// toolPair returns the tool_use row, at the item's start, and the
// tool_result row, at its end, of a tool item.
func toolPair(itemID, name, call, result string) []itemRow {
	return []itemRow{
		{turnType: timeline.TurnToolUse, when: atStart, toolName: name, toolUseID: itemID, text: call},
		{turnType: timeline.TurnToolResult, when: atEnd, toolUseID: itemID, text: result},
	}
}

// systemItem returns the one system row of an item that is not a tool
// call, at the item's end.
func systemItem(text string) []itemRow {
	return []itemRow{{turnType: timeline.TurnSystem, when: atEnd, text: text}}
}

// itemRows returns the rows of an item_completed turn item in the rollout's
// turn-item form: a tool item gives a tool_use and a tool_result on the
// item's id, anything else one system row.
func itemRows(item jsonvalue.Value) []itemRow {
	itemType := parser.Text(item.Get("type"))
	itemID := parser.Text(item.Get("id"))
	switch {
	case itemType == "CommandExecution":
		output := withExit(parser.Text(item.Get("aggregated_output")), item.Get("exit_code"))
		return toolPair(itemID, "shell", joinedCommand(item.Get("command")), output)
	case itemType == "McpToolCall":
		name := parser.Text(item.Get("server")) + "." + parser.Text(item.Get("tool"))
		result := itemFailure(item)
		if !item.Get("result").IsEmpty() {
			result = resultText(item.Get("result"))
		}
		return toolPair(itemID, name, parser.JSONText(item.Get("arguments")), result)
	case itemType == "DynamicToolCall":
		result := resultText(item.Get("content_items"))
		if result == "" {
			result = itemFailure(item)
		}
		return toolPair(itemID, parser.Text(item.Get("tool")),
			parser.JSONText(item.Get("arguments")), result)
	case itemType == "FileChange":
		return fileChangeRows(item, itemID)
	case itemType == "FunctionCallOutput":
		return []itemRow{{turnType: timeline.TurnToolResult, when: atEnd,
			toolName: parser.Text(item.Get("name")), toolUseID: itemID,
			text: parser.TextOf(item.Get("output"))}}
	case itemType == "WebSearch" || (itemType == "Extension" && parser.IsText(item.Get("kind"), "web.search")):
		results := item.Get("results")
		if results.IsEmpty() {
			results = jsonvalue.NewArray()
		}
		return toolPair(itemID, "web_search", parser.Text(item.Get("query")), parser.JSONText(results))
	}
	return systemItemRows(item, itemType)
}

// systemItemRows returns the system row of a turn item that is not a tool
// call: a sleep, an image generation, a plan, a subagent's activity, or any
// other item as its JSON.
func systemItemRows(item jsonvalue.Value, itemType string) []itemRow {
	switch {
	case itemType == "Extension" && parser.IsText(item.Get("kind"), "clock.sleep"):
		duration := ""
		if value, ok := item.Lookup("durationMs"); ok {
			duration = value.Text()
		}
		return systemItem("sleep: " + duration + " ms")
	case itemType == "ImageGeneration" || itemType == "Extension":
		return systemItem(imageText(item, itemType))
	case itemType == "Plan":
		return systemItem("plan: " + parser.Text(item.Get("text")))
	case itemType == "SubAgentActivity":
		return systemItem("subagent activity: " + parser.Text(item.Get("kind")) + " " +
			parser.Text(item.Get("agent_path")) + " " + parser.Text(item.Get("agent_thread_id")))
	}
	rest := without(item, "type", "id", "result")
	return systemItem("item " + itemType + ": " + parser.JSONText(rest))
}

// imageText returns the text of an image generation item, or of an
// Extension item of another kind, which is shown as its JSON.
func imageText(item jsonvalue.Value, itemType string) string {
	if itemType == "Extension" && !parser.IsText(item.Get("kind"), "image_gen.generation") {
		return "item Extension: " + parser.JSONText(without(item, "type", "id"))
	}
	prompt := parser.Text(parser.FirstNonEmpty(item.Get("revised_prompt"), item.Get("revisedPrompt")))
	text := "image generation: " + prompt + " status=" + parser.Text(item.Get("status"))
	if saved := parser.Text(parser.FirstNonEmpty(item.Get("saved_path"), item.Get("savedPath"))); saved != "" {
		text += " saved=" + saved
	}
	return text
}

// fileChangeRows returns the pair of a FileChange item: the changed paths,
// sorted, and its output or status.
func fileChangeRows(item jsonvalue.Value, itemID string) []itemRow {
	paths := ""
	if changes := item.Get("changes"); changes.IsObject() {
		paths = strings.Join(slices.Sorted(slices.Values(changes.Keys())), ", ")
	}
	var output []string
	for _, key := range []string{"stdout", "stderr"} {
		if value := item.Get(key); !value.IsEmpty() {
			output = append(output, value.Text())
		}
	}
	result := strings.Join(output, "\n")
	if result == "" {
		result = parser.Text(item.Get("status"))
	}
	return toolPair(itemID, "apply_patch", paths, result)
}

// joinedCommand returns a command as text: a list's parts joined by
// spaces, anything else as its text.
func joinedCommand(command jsonvalue.Value) string {
	if !command.IsArray() {
		return parser.Text(command)
	}
	parts := make([]string, 0, command.Len())
	for _, part := range command.Items() {
		parts = append(parts, part.Text())
	}
	return strings.Join(parts, " ")
}

// itemFailure returns the "[error] ..." text of a failed tool item, or ""
// when it records no error.
func itemFailure(item jsonvalue.Value) string {
	failure := item.Get("error")
	if failure.IsObject() {
		message := failure.Get("message")
		if !message.IsEmpty() {
			return "[error] " + message.Text()
		}
		return "[error] " + parser.JSONText(failure)
	}
	if !failure.IsEmpty() {
		return "[error] " + failure.Text()
	}
	return ""
}

// resultText returns the text of a tool server's call result or a dynamic
// tool's output: an object's "content" as message content, an object
// without one as JSON, anything else as message content.
func resultText(value jsonvalue.Value) string {
	if value.IsObject() {
		if content, ok := value.Lookup("content"); ok {
			return parser.TextOf(content)
		}
		return parser.JSONText(value)
	}
	return parser.TextOf(value)
}

// withExit prefixes a command's output with "[exit <code>] " when its exit
// code is neither missing nor zero.
func withExit(text string, code jsonvalue.Value) string {
	if isNullOrZero(code) {
		return text
	}
	return "[exit " + code.Text() + "] " + text
}
