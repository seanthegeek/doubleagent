package clinelegacy

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
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
func BaseTurn(role jsonvalue.Value) string {
	switch {
	case parser.IsText(role, "user"):
		return timeline.TurnUser
	case parser.IsText(role, "assistant"):
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
			reasoning.IsString() && reasoning.StringValue() != "" {
			thinking := row(timeline.TurnThinking)
			thinking.Text = timeline.Compact(reasoning.StringValue())
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
func (p *TaskParser) apiMarker(message jsonvalue.Value, options parser.Options,
	row func(string) timeline.Row, emit parser.Emit) bool {
	switch {
	case parser.IsText(message.Get("type"), "reasoning"):
		if options.IncludeThinking {
			thinking := row(timeline.TurnThinking)
			thinking.Text = timeline.CompactValue(parser.FirstNonEmpty(jsonvalue.NewString(parser.TextOf(message.Get("summary"))),
				message.Get("text"), message.Get("reasoning_content"),
				jsonvalue.NewString("[encrypted reasoning]")))
			emit(thinking)
		}
		return true
	case !message.Get("isSummary").IsEmpty() || !message.Get("isTruncationMarker").IsEmpty():
		label := "truncation marker"
		if !message.Get("isSummary").IsEmpty() {
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
func (p *TaskParser) apiContentRows(message jsonvalue.Value, position int, includeThinking bool) []ContentRow {
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
func ContentRows(content jsonvalue.Value, baseTurn string, includeThinking bool) []ContentRow {
	switch {
	case content.IsString():
		if content.StringValue() == "" {
			return nil
		}
		return []ContentRow{{TurnType: baseTurn, Text: content.StringValue()}}
	case !content.IsArray():
		if !Present(content) {
			return nil
		}
		return []ContentRow{{TurnType: baseTurn, Text: parser.JSONText(content)}}
	}
	var rows []ContentRow
	for _, block := range content.Items() {
		if block.IsString() {
			if block.StringValue() != "" {
				rows = append(rows, ContentRow{TurnType: baseTurn, Text: block.StringValue()})
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
func blockRow(block jsonvalue.Value, baseTurn string, includeThinking bool) (ContentRow, bool) {
	if !block.IsObject() {
		return ContentRow{}, false
	}
	blockType := block.Get("type")
	switch {
	case parser.IsText(blockType, "text"):
		text := block.Get("text")
		return ContentRow{TurnType: baseTurn, Text: text.Text()}, !text.IsEmpty()
	case parser.IsText(blockType, "tool_use"):
		name := parser.Text(block.Get("name"))
		return ContentRow{TurnType: timeline.TurnToolUse, ToolName: name,
			Text:      claudecode.ToolSummary(name, block.Get("input")),
			ToolUseID: parser.Text(parser.FirstNonEmpty(block.Get("id"), block.Get("call_id")))}, true
	case parser.IsText(blockType, "tool_result"):
		return toolResultRow(block), true
	case parser.IsText(blockType, "thinking") || parser.IsText(blockType, "reasoning"):
		text := parser.Text(parser.FirstNonEmpty(block.Get("thinking"), block.Get("text")))
		return ContentRow{TurnType: timeline.TurnThinking, Text: text}, includeThinking
	case parser.IsText(blockType, "redacted_thinking"):
		return ContentRow{TurnType: timeline.TurnThinking, Text: "[redacted thinking]"}, includeThinking
	case parser.IsText(blockType, "image"):
		return ContentRow{TurnType: baseTurn, Text: "[image]"}, true
	case parser.IsText(blockType, "file"):
		return ContentRow{TurnType: baseTurn, Text: Join(jsonvalue.NewString("[file]"), block.Get("path"))}, true
	case !blockType.IsEmpty():
		return ContentRow{TurnType: baseTurn, Text: "[" + blockType.Text() + "]"}, true
	}
	return ContentRow{}, false
}

// toolResultRow returns the row of a tool_result block: its content's text
// (CompactJSON for content that is neither a string nor a list), marked
// "[error]" for a failed call.
func toolResultRow(block jsonvalue.Value) ContentRow {
	content := block.Get("content")
	body := parser.JSONText(content)
	if content.IsString() || content.IsArray() || content.IsNull() {
		body = parser.TextOf(content)
	}
	if !block.Get("is_error").IsEmpty() {
		body = "[error] " + body
	}
	return ContentRow{TurnType: timeline.TurnToolResult, Text: body,
		ToolName: parser.Text(block.Get("name")), ToolUseID: parser.Text(block.Get("tool_use_id"))}
}
