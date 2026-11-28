package zed

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// thinkingText returns the text of a Thinking part: an object's text, or
// any other value as its row text (jsonvalue.Value.Text).
func thinkingText(thinking jsonvalue.Value) string {
	if thinking.IsObject() {
		return parserkit.Text(thinking.Get("text"))
	}
	return thinking.Text()
}

// agentPart adds the items of one content part of an Agent message. A tool
// call is followed by its result, marked done, so the timeline reads in
// order; a later call of an id already done gets no result.
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
		toolUseID := parserkit.Text(toolUse.Get("id"))
		out.items = append(out.items, item{timeline.TurnToolUse, toolInputText(toolUse),
			parserkit.Text(toolUse.Get("name")), toolUseID})
		if result := results.Get(toolUseID); result.IsObject() && !done[toolUseID] {
			done[toolUseID] = true
			p.toolResult(out, toolUseID, result)
		}
	case part.Has("Image"):
		out.add(timeline.TurnAssistant, "[image]")
	default:
		out.bad(fmt.Sprintf("unknown Agent content variant, content[%d]", k), part)
	}
}

// agentMessage adds the items of an Agent message: its text, thinking,
// tool calls each followed by its result, images, then the results no call
// named. A result follows only the first call naming its id, so it is
// written once however many calls repeat the id.
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
	results = parserkit.ObjectOrEmpty(results)
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
			out.bad("Agent tool_results entry "+member.Key+" is not an object",
				member.Value)
		}
	}
}

// legacyTools adds the tool use and tool result items of a legacy message;
// a tool_uses or tool_results that is not an array adds a problem.
func (p *Parser) legacyTools(out *transcript, message jsonvalue.Value) {
	for k, toolUse := range out.array(message.Get("tool_uses"), "tool_uses") {
		if !toolUse.IsObject() {
			out.bad(fmt.Sprintf("tool_uses[%d] is not an object", k), toolUse)
			continue
		}
		out.items = append(out.items, item{timeline.TurnToolUse, toolInputText(toolUse),
			parserkit.Text(toolUse.Get("name")), parserkit.Text(toolUse.Get("id"))})
	}
	for k, result := range out.array(message.Get("tool_results"), "tool_results") {
		if !result.IsObject() {
			out.bad(fmt.Sprintf("tool_results[%d] is not an object", k), result)
			continue
		}
		text := contentText(result.Get("content"))
		if !result.Get("is_error").IsEmpty() {
			text = "[error] " + text
		}
		p.resultItems(out, text, "", parserkit.Text(result.Get("tool_use_id")), result)
	}
}

// legacyMessage adds the items of a message of the 0.1.0 and 0.2.0 shape:
// role and text, segments, tool_uses and tool_results. A segments that is
// not an array adds a problem.
func (p *Parser) legacyMessage(out *transcript, message jsonvalue.Value) {
	role := strings.ToLower(parserkit.Text(message.Get("role")))
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
	for k, segment := range out.array(message.Get("segments"), "segments") {
		segmentType := ""
		if segment.IsObject() {
			segmentType = strings.ToLower(parserkit.Text(segment.Get("type")))
		}
		switch segmentType {
		case "text":
			out.add(kind, parserkit.Text(segment.Get("text")))
		case "thinking":
			out.add(timeline.TurnThinking, parserkit.Text(segment.Get("text")))
		case "redactedthinking":
			out.add(timeline.TurnThinking, "[redacted thinking]")
		default:
			out.bad(fmt.Sprintf("unknown segment variant, segments[%d]", k), segment)
		}
	}
	p.legacyTools(out, message)
}
