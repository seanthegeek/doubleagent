package clinelegacy

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// ContentRow is one row of a model API message's content: its turn type,
// text, tool name and tool call id.
type ContentRow struct {
	TurnType, Text, ToolName, ToolUseID string
}

// BaseTurn returns the turn type of a message's role: user, assistant, and
// system for any other.
func BaseTurn(role pyjson.Value) string {
	switch {
	case isText(role, "user"):
		return timeline.TurnUser
	case isText(role, "assistant"):
		return timeline.TurnAssistant
	}
	return timeline.TurnSystem
}

// ParseAPI emits the rows of the API history, then a row per problem.
func (p *TaskParser) ParseAPI(artifact *input.Artifact, context *TaskContext,
	options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	for position, message := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !message.IsObject() {
			continue
		}
		timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(message.Get("ts")), context.Start)
		model := context.ModelAt(message.Get("ts"))
		row := func(turnType string) timeline.Row {
			row := p.Row(artifact, context, position, timestamp, model)
			row.TurnType = turnType
			return row
		}
		if p.apiMarker(message, options, row, emit) {
			continue
		}
		if reasoning := message.Get("reasoning_content"); options.IncludeThinking &&
			reasoning.IsString() && reasoning.Text() != "" {
			thinking := row(timeline.TurnThinking)
			thinking.Text = timeline.Compact(reasoning.Text())
			emit(thinking)
		}
		for _, content := range p.apiContentRows(message, position, options.IncludeThinking) {
			each := row(content.TurnType)
			each.ToolName, each.ToolUseID = content.ToolName, content.ToolUseID
			each.Text = timeline.Compact(content.Text)
			emit(each)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.TaskID, context.ProjectPath))
}

// apiMarker emits the row of a reasoning record (when thinking is kept),
// a summary or a truncation marker, and reports whether the message was
// one of those, which have no content rows.
func (p *TaskParser) apiMarker(message pyjson.Value, options parser.Options,
	row func(string) timeline.Row, emit parser.Emit) bool {
	switch {
	case isText(message.Get("type"), "reasoning"):
		if options.IncludeThinking {
			thinking := row(timeline.TurnThinking)
			thinking.Text = timeline.CompactValue(FirstTruthy(pyjson.NewString(parser.TextOf(message.Get("summary"))),
				message.Get("text"), message.Get("reasoning_content"),
				pyjson.NewString("[encrypted reasoning]")))
			emit(thinking)
		}
		return true
	case message.Get("isSummary").Truthy() || message.Get("isTruncationMarker").Truthy():
		label := "truncation marker"
		if message.Get("isSummary").Truthy() {
			label = "summary"
		}
		marker := row(timeline.TurnSystem)
		marker.Text = timeline.Compact(label + ": " + parser.TextOf(message.Get("content")))
		emit(marker)
		return true
	}
	return false
}

// apiContentRows returns the content rows of one API history message, by
// the family's own reading when it has one.
func (p *TaskParser) apiContentRows(message pyjson.Value, position int, includeThinking bool) []ContentRow {
	baseTurn := BaseTurn(message.Get("role"))
	if rower, ok := p.family.(APIRower); ok {
		return rower.APIContentRows(message, position, baseTurn, includeThinking)
	}
	return ContentRows(message.Get("content"), baseTurn, includeThinking)
}

// ContentRows returns a row per block of a model API message's content: a
// string as one row of the base turn, text, tool_use, tool_result,
// thinking (kept only with includeThinking), image and file blocks, and
// "[<type>]" for a block of another type. Content that is neither a string
// nor a list is one row of its CompactJSON.
func ContentRows(content pyjson.Value, baseTurn string, includeThinking bool) []ContentRow {
	switch {
	case content.IsString():
		if content.Text() == "" {
			return nil
		}
		return []ContentRow{{TurnType: baseTurn, Text: content.Text()}}
	case !content.IsArray():
		if !Present(content) {
			return nil
		}
		return []ContentRow{{TurnType: baseTurn, Text: parser.CompactJSON(content)}}
	}
	var rows []ContentRow
	for _, block := range content.Items() {
		if block.IsString() {
			if block.Text() != "" {
				rows = append(rows, ContentRow{TurnType: baseTurn, Text: block.Text()})
			}
			continue
		}
		if row, ok := blockRow(block, baseTurn, includeThinking); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// blockRow returns the row of one content block, or false for a block that
// gives none.
func blockRow(block pyjson.Value, baseTurn string, includeThinking bool) (ContentRow, bool) {
	if !block.IsObject() {
		return ContentRow{}, false
	}
	blockType := block.Get("type")
	switch {
	case isText(blockType, "text"):
		text := block.Get("text")
		return ContentRow{TurnType: baseTurn, Text: text.Str()}, text.Truthy()
	case isText(blockType, "tool_use"):
		name := parser.Str(block.Get("name"))
		return ContentRow{TurnType: timeline.TurnToolUse, ToolName: name,
			Text:      claudecode.ToolSummary(name, block.Get("input")),
			ToolUseID: parser.Str(FirstTruthy(block.Get("id"), block.Get("call_id")))}, true
	case isText(blockType, "tool_result"):
		return toolResultRow(block), true
	case isText(blockType, "thinking") || isText(blockType, "reasoning"):
		text := parser.Str(FirstTruthy(block.Get("thinking"), block.Get("text")))
		return ContentRow{TurnType: timeline.TurnThinking, Text: text}, includeThinking
	case isText(blockType, "redacted_thinking"):
		return ContentRow{TurnType: timeline.TurnThinking, Text: "[redacted thinking]"}, includeThinking
	case isText(blockType, "image"):
		return ContentRow{TurnType: baseTurn, Text: "[image]"}, true
	case isText(blockType, "file"):
		return ContentRow{TurnType: baseTurn, Text: Join(pyjson.NewString("[file]"), block.Get("path"))}, true
	case blockType.Truthy():
		return ContentRow{TurnType: baseTurn, Text: "[" + blockType.Str() + "]"}, true
	}
	return ContentRow{}, false
}

// toolResultRow returns the row of a tool_result block: its content's text
// (CompactJSON for content that is neither a string nor a list), marked
// "[error]" for a failed call.
func toolResultRow(block pyjson.Value) ContentRow {
	content := block.Get("content")
	body := parser.CompactJSON(content)
	if content.IsString() || content.IsArray() || content.IsNull() {
		body = parser.TextOf(content)
	}
	if block.Get("is_error").Truthy() {
		body = "[error] " + body
	}
	return ContentRow{TurnType: timeline.TurnToolResult, Text: body,
		ToolName: parser.Str(block.Get("name")), ToolUseID: parser.Str(block.Get("tool_use_id"))}
}
