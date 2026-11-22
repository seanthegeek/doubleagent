package continuedev

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// promptModels emits a model row for each prompt log that changes the
// model, and returns the item's model: the last prompt log's, else the
// session's. A promptLogs that is not an array adds a problem.
func (s *sessionFile) promptModels(position int, item jsonvalue.Value) string {
	model := s.model
	for _, log := range parser.ArrayMember(item.Get("promptLogs"), "promptLogs", position, &s.problems) {
		title := log.Get("modelTitle")
		if title.IsEmpty() {
			continue
		}
		model = title.Text()
		if change := s.models.Change("model", s.base.SessionID, model); change != "" {
			row := s.row(position, model, timeline.TurnSystem)
			row.Text = change
			s.emit(row)
		}
	}
	return model
}

// toolCalls emits an assistant item's tool calls, then, for each call
// state, its result when no tool message answers the call and its
// permission decision. The calls come from the message, else from the
// states; a toolCalls or toolCallStates that is not an array adds a problem.
func (s *sessionFile) toolCalls(position int, model string, item, message jsonvalue.Value) {
	var states []jsonvalue.Value
	for _, state := range parser.ArrayMember(item.Get("toolCallStates"), "toolCallStates", position, &s.problems) {
		if state.IsObject() {
			states = append(states, state)
		}
	}
	calls := parser.ArrayMember(message.Get("toolCalls"), "toolCalls", position, &s.problems)
	if len(calls) == 0 {
		calls = nil
		for _, state := range states {
			if state.Get("toolCall").IsObject() {
				calls = append(calls, state.Get("toolCall"))
			}
		}
	}
	for _, call := range calls {
		if !call.IsObject() {
			continue
		}
		row := s.row(position, model, timeline.TurnToolUse)
		row.ToolUseID = parser.Text(call.Get("id"))
		row.ToolName = parser.Text(call.Get("function").Get("name"))
		s.names[row.ToolUseID] = row.ToolName
		s.text(row, argumentsText(call.Get("function").Get("arguments")))
	}
	for _, state := range states {
		s.callState(position, model, state)
	}
}

// argumentsText returns a call's arguments as row text: a string as it is,
// "" for none, anything else as CompactJSON.
func argumentsText(arguments jsonvalue.Value) string {
	switch {
	case arguments.IsString():
		return arguments.StringValue()
	case arguments.IsNull():
		return ""
	}
	return parser.JSONText(arguments)
}

// callState emits the result row of a call state no tool message answers,
// when it has output or a status beyond the running ones, and the
// permission decision it records.
func (s *sessionFile) callState(position int, model string, state jsonvalue.Value) {
	callID := parser.Text(state.Get("toolCallId"))
	if callID == "" {
		return
	}
	output := outputText(state.Get("output"))
	status := parser.Text(state.Get("status"))
	running := status == "" || status == "done" || status == "generated" || status == "generating" ||
		status == "calling"
	if !s.answered[callID] && (output != "" || !running) {
		row := s.row(position, model, timeline.TurnToolResult)
		row.ToolUseID, row.ToolName = callID, s.names[callID]
		text := output
		if status != "" && status != "done" {
			text = "[" + status + "] " + output
		}
		s.text(row, text)
	}
	if decision := stateDecision(status, state.Get("output"), output); decision != "" {
		row := s.row(position, model, timeline.TurnSystem)
		row.ToolUseID, row.ToolName = callID, s.names[callID]
		row.Text = "permission decision: " + decision
		s.emit(row)
	}
}

// outputText returns the text of a tool output, a list of context items:
// each item's content, its CompactJSON when it is not text, or its name
// when it has none, joined by LF.
func outputText(output jsonvalue.Value) string {
	if !output.IsArray() {
		return parser.TextOf(output)
	}
	var parts []string
	for _, item := range output.Items() {
		part := item.Text()
		if item.IsObject() {
			switch content := item.Get("content"); {
			case content.IsString():
				part = content.StringValue()
			case !content.IsNull():
				part = parser.JSONText(content)
			default:
				part = parser.Text(item.Get("name"))
			}
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

// stateDecision returns the decision a tool call state records, as the
// text after "permission decision: ", or "". A canceled state names the
// reason when its output is one of the CLI's denial texts
// (extensions/cli/src/stream/streamChatResponse.helpers.ts:519-527); an
// errored one whose output is the extension's policy violation was
// refused by policy.
func stateDecision(status string, output jsonvalue.Value, text string) string {
	if status == "canceled" {
		switch strings.TrimSpace(text) {
		case "Permission denied by user":
			return "canceled reason=user"
		case "Command blocked by security policy":
			return "canceled reason=policy"
		}
		return "canceled"
	}
	if status != "errored" {
		return ""
	}
	for _, item := range output.Items() {
		if parser.IsText(item.Get("name"), policyViolation) {
			return "errored reason=policy"
		}
	}
	return ""
}
