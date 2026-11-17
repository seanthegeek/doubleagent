package zed

import (
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// agentMessage adds the items of an Agent message: its text, thinking,
// tool calls each followed by its result, images, then the results no call
// named.
func (p *Parser) agentMessage(out *transcript, message jsonvalue.Value) {
	if !message.IsObject() {
		out.bad("Agent message is not an object",
			jsonvalue.NewObject(jsonvalue.Member{Key: "Agent", Value: message}))
		return
	}
	results := message.Get("tool_results")
	if !results.IsNull() && !results.IsObject() {
		out.bad("Agent tool_results is not an object", results)
	}
	results = objectOrEmpty(results)
	done := map[string]bool{}
	for k, part := range contentList(out, message.Get("content"), "Agent") {
		p.agentPart(out, k, part, results, done)
	}
	for _, member := range results.Members() {
		switch {
		case done[member.Key]:
		case member.Value.IsObject():
			p.toolResult(out, member.Key, member.Value)
		default:
			out.bad(fmt.Sprintf("Agent tool_results entry %s is not an object", member.Key),
				member.Value)
		}
	}
}

// agentPart adds the items of one content part of an Agent message. A tool
// call is followed by its result, marked done, so the timeline reads in
// order.
func (p *Parser) agentPart(out *transcript, k int, part, results jsonvalue.Value,
	done map[string]bool) {
	switch {
	case part.IsString():
		out.add(timeline.TurnAssistant, part.StringValue())
	case !part.IsObject():
		out.bad(fmt.Sprintf("unknown Agent content variant, content[%d]", k), part)
	case part.Get("Text").IsString():
		out.add(timeline.TurnAssistant, part.Get("Text").StringValue())
	case part.Has("Thinking"):
		out.add(timeline.TurnThinking, thinkingText(part.Get("Thinking")))
	case part.Has("RedactedThinking"):
		out.add(timeline.TurnThinking, "[redacted thinking]")
	case part.Get("ToolUse").IsObject():
		toolUse := part.Get("ToolUse")
		toolUseID := parser.Str(toolUse.Get("id"))
		out.items = append(out.items, item{timeline.TurnToolUse, toolInputText(toolUse),
			parser.Str(toolUse.Get("name")), toolUseID})
		if result := results.Get(toolUseID); result.IsObject() {
			done[toolUseID] = true
			p.toolResult(out, toolUseID, result)
		}
	case part.Has("Image"):
		out.add(timeline.TurnAssistant, "[image]")
	default:
		out.bad(fmt.Sprintf("unknown Agent content variant, content[%d]", k), part)
	}
}

// thinkingText returns the text of a Thinking part: an object's text, or
// any other value as Python's str() writes it.
func thinkingText(thinking jsonvalue.Value) string {
	if thinking.IsObject() {
		return parser.Str(thinking.Get("text"))
	}
	return thinking.Text()
}

// legacyMessage adds the items of a message of the 0.1.0 and 0.2.0 shape:
// role and text, segments, tool_uses and tool_results.
func (p *Parser) legacyMessage(out *transcript, message jsonvalue.Value) {
	role := lower(parser.Str(message.Get("role")))
	kind := timeline.TurnSystem
	switch role {
	case "user":
		kind = timeline.TurnUser
	case "assistant":
		kind = timeline.TurnAssistant
	}
	if text := message.Get("text"); text.IsString() {
		if kind == timeline.TurnSystem {
			out.add(kind, role+": "+text.StringValue())
		} else {
			out.add(kind, text.StringValue())
		}
	}
	for k, segment := range parser.Elements(message.Get("segments")) {
		segmentType := ""
		if segment.IsObject() {
			segmentType = lower(parser.Str(segment.Get("type")))
		}
		switch segmentType {
		case "text":
			out.add(kind, parser.Str(segment.Get("text")))
		case "thinking":
			out.add(timeline.TurnThinking, parser.Str(segment.Get("text")))
		case "redactedthinking":
			out.add(timeline.TurnThinking, "[redacted thinking]")
		default:
			out.bad(fmt.Sprintf("unknown segment variant, segments[%d]", k), segment)
		}
	}
	p.legacyTools(out, message)
}

// legacyTools adds the tool use and tool result items of a legacy message.
func (p *Parser) legacyTools(out *transcript, message jsonvalue.Value) {
	for k, toolUse := range parser.Elements(message.Get("tool_uses")) {
		if !toolUse.IsObject() {
			out.bad(fmt.Sprintf("tool_uses[%d] is not an object", k), toolUse)
			continue
		}
		out.items = append(out.items, item{timeline.TurnToolUse, toolInputText(toolUse),
			parser.Str(toolUse.Get("name")), parser.Str(toolUse.Get("id"))})
	}
	for k, result := range parser.Elements(message.Get("tool_results")) {
		if !result.IsObject() {
			out.bad(fmt.Sprintf("tool_results[%d] is not an object", k), result)
			continue
		}
		text := contentText(result.Get("content"))
		if !result.Get("is_error").IsEmpty() {
			text = "[error] " + text
		}
		p.resultItems(out, text, "", parser.Str(result.Get("tool_use_id")), result)
	}
}
