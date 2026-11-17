package zed

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// item is one transcript row of a thread before it gets the thread's
// context.
type item struct {
	turnType  string
	text      string
	toolName  string
	toolUseID string
}

// transcript gathers the items of one thread in order and the problems met
// in its messages, each at the thread's rowid.
type transcript struct {
	items    []item
	problems *parser.Problems
	rowid    int64
	position int
}

// add appends an item without a tool.
func (t *transcript) add(turnType, text string) {
	t.items = append(t.items, item{turnType: turnType, text: text})
}

// bad records a message part of a shape not recognised, with the part as
// compact JSON: "unparseable record <N> (<reason>)", N the message's
// 1-based position in messages.
func (t *transcript) bad(reason string, part pyjson.Value) {
	t.problems.AddRaw(int(t.rowid), fmt.Sprintf("unparseable record %d (%s)", t.position, reason),
		recordJSON(part))
}

// messages returns the items of a thread's messages in transcript order,
// adding a message, or a part of one, of a shape not recognised to
// problems.
func (p *Parser) messages(value pyjson.Value, rowid int64, problems *parser.Problems) []item {
	messages, ok := value.Lookup("messages")
	if !ok || messages.IsNull() {
		return nil
	}
	if !messages.IsArray() {
		problems.AddRaw(int(rowid), "unparseable record 0 (messages is not a list)",
			recordJSON(messages))
		return nil
	}
	out := &transcript{problems: problems, rowid: rowid}
	for i, message := range messages.Items() {
		out.position = i + 1
		p.message(out, message)
	}
	return out.items
}

// message adds the items of one message by its variant.
func (p *Parser) message(out *transcript, message pyjson.Value) {
	switch {
	case message.IsString():
		out.add(timeline.TurnSystem, lower(message.Text()))
	case !message.IsObject():
		out.bad("unknown message variant", message)
	case message.Has("User"):
		userMessage(out, message.Get("User"))
	case message.Has("Agent"):
		p.agentMessage(out, message.Get("Agent"))
	case message.Has("Compaction"):
		compaction := message.Get("Compaction")
		switch {
		case compaction.Get("Summary").IsString():
			out.add(timeline.TurnSystem, "compaction: "+compaction.Get("Summary").Text())
		case compaction.IsObject() && compaction.Has("ProviderNative"):
			out.add(timeline.TurnSystem, "compaction: provider native")
		default:
			out.bad("unknown Compaction variant", message)
		}
	case message.Has("role"):
		p.legacyMessage(out, message)
	default:
		out.bad("unknown message variant", message)
	}
}

// contentList returns a message's content as a list: a string or an object
// stands for a list of one, null for none. Any other value is reported
// under its role's name and gives none.
func contentList(out *transcript, content pyjson.Value, role string) []pyjson.Value {
	switch {
	case content.IsString(), content.IsObject():
		return []pyjson.Value{content}
	case content.IsArray():
		return content.Items()
	case !content.IsNull():
		out.bad(role+" content is not a list", content)
	}
	return nil
}

// userMessage adds one user row per prompt: the text, mentions and images
// of one User message joined, so sessions count prompts, not content items.
// A content object of an unknown variant stays in the text as JSON.
func userMessage(out *transcript, message pyjson.Value) {
	if !message.IsObject() {
		out.bad("User message is not an object", pyjson.NewObject(pyjson.Member{Key: "User", Value: message}))
		return
	}
	var parts []string
	for k, part := range contentList(out, message.Get("content"), "User") {
		switch {
		case part.IsString():
			parts = append(parts, part.Text())
		case !part.IsObject():
			out.bad(fmt.Sprintf("unknown User content variant, content[%d]", k), part)
		case part.Get("Text").IsString():
			parts = append(parts, part.Get("Text").Text())
		case part.Has("Mention"):
			parts = append(parts, mentionText(part.Get("Mention")))
		case part.Has("Image"):
			parts = append(parts, "[image]")
		default:
			parts = append(parts, parser.CompactJSON(part))
		}
	}
	out.add(timeline.TurnUser, strings.Join(parts, "\n"))
}

// toolResult adds the result row of one Agent message's tool call, keyed
// key in its tool_results, and its denial row when Zed wrote the result
// for a denied call.
func (p *Parser) toolResult(out *transcript, key string, result pyjson.Value) {
	text := contentText(result.Get("content"))
	if output, ok := result.Lookup("output"); text == "" && ok && !output.IsNull() {
		text = contentText(output)
		if output.IsObject() {
			text = parser.CompactJSON(output)
		}
	}
	if result.Get("is_error").Truthy() {
		text = "[error] " + text
	}
	toolUseID := cmp.Or(parser.Str(result.Get("tool_use_id")), key)
	p.resultItems(out, text, parser.Str(result.Get("tool_name")), toolUseID, result)
}

// resultItems adds a tool result row with the given text, and the denial
// row of the result when Zed wrote it for a denied call.
func (p *Parser) resultItems(out *transcript, text, name, toolUseID string, result pyjson.Value) {
	out.items = append(out.items, item{timeline.TurnToolResult, text, name, toolUseID})
	if decision := p.denialText(result); decision != "" {
		out.items = append(out.items, item{timeline.TurnSystem, decision, name, toolUseID})
	}
}
