package vscode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// request emits the rows of one request: changes of the model, mode and
// permission level, the prompt, a confirmation, the response's parts, and
// an error or a cancel.
func (s *sessionRows) request(request jsonvalue.Value, line int, options parserkit.Options) {
	model := parserkit.Text(request.Get("modelId"))
	asked := request.Get("timestamp")
	s.change("model", "model", modelText(model, request.Get("modelConfiguration")), asked, line, model)
	modeInfo := request.Get("modeInfo")
	s.change("permission mode", "mode", shownSetting(modeInfo.Get("telemetryModeId")), asked, line, model)
	s.change("permission level", "level", shownSetting(modeInfo.Get("permissionLevel")), asked, line, model)
	answered := request.Get("responseTimestamp")
	if answered.IsEmpty() {
		answered = asked
	}
	text := promptText(request)
	if !request.Get("isSystemInitiated").IsEmpty() {
		s.emit(s.row(timeline.TurnSystem, "system initiated: "+text, asked, line, model))
	} else {
		s.emit(s.row(timeline.TurnUser, text, asked, line, model))
	}
	if confirmation := request.Get("confirmation"); confirmation.IsString() && confirmation.StringValue() != "" {
		s.emit(s.row(timeline.TurnSystem, "permission decision: confirmation="+confirmation.StringValue(), asked,
			line, model))
	}
	response := request.Get("response")
	parts := response.Items()
	if response.IsString() || response.IsObject() {
		parts = []jsonvalue.Value{response}
	}
	responses := responseRows{session: s, timestamp: answered, line: line, model: model}
	responses.parts(parts, options)
	failure := request.Get("result").Get("errorDetails")
	switch {
	case failure.IsObject() && !failure.Get("message").IsEmpty():
		s.emit(s.row(timeline.TurnSystem, "error: "+failure.Get("message").Text(), answered, line, model))
	case !request.Get("isCanceled").IsEmpty():
		s.emit(s.row(timeline.TurnSystem, "canceled", answered, line, model))
	}
}

// promptText returns a request's message text, with the names of its
// attached variables.
func promptText(request jsonvalue.Value) string {
	message := request.Get("message")
	text := message.StringValue()
	if message.IsObject() {
		text = parserkit.Text(message.Get("text"))
	}
	var names []string
	for _, variable := range parserkit.Elements(request.Get("variableData").Get("variables")) {
		if name := variable.Get("name"); !name.IsEmpty() {
			names = append(names, name.Text())
		}
	}
	if len(names) > 0 {
		text += "\n[attached: " + strings.Join(names, ", ") + "]"
	}
	return text
}

// responseRows builds the rows of one request's response, each at the
// response's time.
type responseRows struct {
	session   *sessionRows
	timestamp jsonvalue.Value
	line      int
	model     string
	run       []string // the text parts not yet emitted
}

// row emits a row of the response.
func (r *responseRows) row(turnType, text, toolName, toolUseID string) {
	row := r.session.row(turnType, text, r.timestamp, r.line, r.model)
	row.ToolName, row.ToolUseID = toolName, toolUseID
	r.session.emit(row)
}

// flush emits the text parts gathered so far as one assistant row.
func (r *responseRows) flush() {
	joined := strings.TrimSpace(strings.Join(r.run, ""))
	r.run = r.run[:0]
	if joined != "" {
		r.row(timeline.TurnAssistant, joined, "", "")
	}
}

// parts emits the rows of a response's parts: runs of text parts join
// into one assistant row, and any other part ends the run.
func (r *responseRows) parts(parts []jsonvalue.Value, options parserkit.Options) {
	for _, part := range parts {
		if part.IsString() {
			r.run = append(r.run, part.StringValue())
			continue
		}
		if !part.IsObject() {
			continue
		}
		kind := part.Get("kind")
		switch {
		case kind.IsNull() && part.Has("value"):
			r.run = append(r.run, mdText(part))
			continue
		case parserkit.IsText(kind, "markdownContent"):
			r.run = append(r.run, mdText(part.Get("content")))
			continue
		case parserkit.IsText(kind, "inlineReference"):
			name := parserkit.Text(part.Get("name"))
			if name == "" {
				name = uriText(part.Get("inlineReference"))
			}
			r.run = append(r.run, name)
			continue
		}
		r.flush()
		r.part(kind, part, options)
	}
	r.flush()
}

// part emits the rows of a response part that is not text.
func (r *responseRows) part(kind, part jsonvalue.Value, options parserkit.Options) {
	switch {
	case parserkit.IsText(kind, "thinking"):
		if thought := mdText(part.Get("value")); options.IncludeThinking && thought != "" {
			r.row(timeline.TurnThinking, thought, "", "")
		}
	case parserkit.IsText(kind, "toolInvocationSerialized"):
		toolUseID, toolName := parserkit.Text(part.Get("toolCallId")), parserkit.Text(part.Get("toolId"))
		r.row(timeline.TurnToolUse, toolCallText(part), toolName, toolUseID)
		r.row(timeline.TurnToolResult, toolResultText(part), toolName, toolUseID)
		if decision := confirmedText(part); decision != "" {
			r.row(timeline.TurnSystem, decision, toolName, toolUseID)
		}
	case parserkit.IsText(kind, "confirmation"):
		r.row(timeline.TurnSystem, confirmationPartText(part), "", "")
	case parserkit.IsText(kind, "textEditGroup"):
		r.row(timeline.TurnSystem, "edit: "+uriText(part.Get("uri")), "", "")
	}
}
