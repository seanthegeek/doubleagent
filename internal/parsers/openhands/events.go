package openhands

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// eventRowMaker builds a row of the event being read.
type eventRowMaker func(turn, text, toolName, toolUseID string) timeline.Row

// eventRows returns the rows of one event, by its kind.
func (p *Parser) eventRows(artifact *input.Artifact, conv *conversation, event jsonvalue.Value,
	options parserkit.Options) []timeline.Row {
	var rows []timeline.Row
	row := func(turn, text, toolName, toolUseID string) timeline.Row {
		made := p.row(artifact, conv, turn)
		made.ToolName, made.ToolUseID = toolName, toolUseID
		made.Text = timeline.Compact(text)
		rows = append(rows, made)
		return made
	}
	kind := parserkit.Text(event.Get("kind"))
	switch kind {
	case "MessageEvent":
		messageRows(event, options, row)
	case "ActionEvent":
		actionRows(event, options, row)
	case "ObservationEvent":
		observation := event.Get("observation")
		text := parserkit.TextOf(observation.Get("content"))
		if !observation.Get("is_error").IsEmpty() {
			text = "[error] " + text
		}
		row(timeline.TurnToolResult, text, parserkit.Text(event.Get("tool_name")),
			parserkit.Text(event.Get("tool_call_id")))
	case "UserRejectObservation":
		rejectionRows(event, row)
	case "AgentErrorEvent":
		row(timeline.TurnToolResult, "[error] "+parserkit.TextOr(event.Get("error"), ""),
			parserkit.Text(event.Get("tool_name")), parserkit.Text(event.Get("tool_call_id")))
	case "ACPToolCallEvent":
		acpRows(event, row)
	case "SecurityAnalysisEvent":
		if event.Get("risks").IsObject() && event.Get("risks").Len() > 0 {
			securityRows(artifact, event, conv, row)
		} else {
			otherRow(kind, event, row)
		}
	default:
		otherKindRows(kind, event, row)
	}
	return rows
}

// otherKindRows emits the row of an event kind that is only described.
func otherKindRows(kind string, event jsonvalue.Value, row eventRowMaker) {
	switch kind {
	case "SystemPromptEvent":
		count := 0
		if tools := event.Get("tools"); tools.IsArray() {
			count = tools.Len()
		}
		row(timeline.TurnSystem, "system prompt, "+strconv.Itoa(count)+" tools: "+
			parserkit.TextOf(event.Get("system_prompt")), "", "")
	case "Condensation", "CondensationSummaryEvent":
		forgotten := ""
		if ids := event.Get("forgotten_event_ids"); ids.IsArray() {
			forgotten = strconv.Itoa(ids.Len()) + " events forgotten"
		}
		row(timeline.TurnSystem, parserkit.JoinNonEmpty(" | ", kind+":", forgotten, parserkit.Text(event.Get("summary"))), "", "")
	case "HookExecutionEvent":
		hookRow(event, row)
	case "ConversationStateUpdateEvent":
		// The full_state value is a whole ConversationState, secrets included;
		// only its key is reported.
		value, shown := event.Get("value"), ""
		if !value.IsObject() && !value.IsArray() && !parserkit.IsText(event.Get("key"), "full_state") {
			shown = value.Text()
		}
		text := "state update: " + parserkit.Text(event.Get("key"))
		if shown != "" {
			text += "=" + shown
		}
		row(timeline.TurnSystem, text, "", "")
	default:
		otherRow(kind, event, row)
	}
}

// otherRow emits the row of an event of any other kind: its kind and the
// rest of its fields as compact JSON.
func otherRow(kind string, event jsonvalue.Value, row eventRowMaker) {
	var rest []jsonvalue.Member
	for _, member := range event.Members() {
		switch member.Key {
		case "kind", "id", "timestamp", "source", "parent_id":
		default:
			rest = append(rest, member)
		}
	}
	text := ""
	if len(rest) > 0 {
		text = parserkit.JSONText(jsonvalue.NewObject(rest...))
	}
	if kind == "" {
		kind = "event"
	}
	row(timeline.TurnSystem, parserkit.JoinNonEmpty(" | ", kind+":", text), "", "")
}

// thinkingText returns the reasoning of a message or action: its
// reasoning_content and the text of its thinking blocks. A thinking_blocks
// that is not an array adds a problem.
func thinkingText(record jsonvalue.Value, problems *parserkit.Problems) string {
	var parts []string
	if reasoning := record.Get("reasoning_content"); reasoning.IsString() {
		parts = append(parts, reasoning.StringValue())
	}
	for _, block := range parserkit.ArrayMember(record.Get("thinking_blocks"), "thinking_blocks", 1, problems) {
		if thinking := block.Get("thinking"); thinking.IsString() {
			parts = append(parts, thinking.StringValue())
		}
	}
	return parserkit.JoinNonEmpty(" | ", parts...)
}

// problemRows emits a system row per problem of the event.
func problemRows(problems parserkit.Problems, row eventRowMaker) {
	for _, problem := range problems {
		row(timeline.TurnSystem, parserkit.ProblemPrefix+problem.Message, "", "")
	}
}

// messageRows emits the rows of a MessageEvent: thinking, the message or the
// tool result it carries, its tool calls, then the problem of a member of
// the wrong kind.
func messageRows(event jsonvalue.Value, options parserkit.Options, row eventRowMaker) {
	message := event.Get("llm_message")
	role := parserkit.Text(parserkit.FirstNonEmpty(message.Get("role"), event.Get("source")))
	var problems parserkit.Problems
	if options.IncludeThinking {
		if thinking := thinkingText(message, &problems); thinking != "" {
			row(timeline.TurnThinking, thinking, "", "")
		}
	}
	text := parserkit.TextOf(message.Get("content"))
	switch {
	case role == "tool":
		row(timeline.TurnToolResult, text, parserkit.Text(message.Get("name")),
			parserkit.Text(message.Get("tool_call_id")))
	case text != "" || role == "user" || role == "system":
		turn := timeline.TurnSystem
		if role == "user" || role == "assistant" {
			turn = role
		}
		row(turn, text, "", "")
	}
	for _, call := range parserkit.ArrayMember(message.Get("tool_calls"), "tool_calls", 1, &problems) {
		function := call.Get("function")
		row(timeline.TurnToolUse,
			parserkit.Text(parserkit.FirstNonEmpty(call.Get("arguments"), function.Get("arguments"))),
			parserkit.Text(parserkit.FirstNonEmpty(call.Get("name"), function.Get("name"))),
			parserkit.Text(call.Get("id")))
	}
	problemRows(problems, row)
}

// actionRows emits the thinking and the tool call of an ActionEvent, then
// the problem of a thinking_blocks of the wrong kind.
func actionRows(event jsonvalue.Value, options parserkit.Options, row eventRowMaker) {
	var problems parserkit.Problems
	if options.IncludeThinking {
		if thinking := parserkit.JoinNonEmpty(" | ", parserkit.TextOf(event.Get("thought")), thinkingText(event, &problems)); thinking != "" {
			row(timeline.TurnThinking, thinking, "", "")
		}
	}

	text := actionText(event)
	if risk := event.Get("security_risk"); parserkit.Present(risk) {
		text += " security_risk=" + risk.Text()
	}
	row(timeline.TurnToolUse, text, parserkit.Text(event.Get("tool_name")),
		parserkit.Text(event.Get("tool_call_id")))
	problemRows(problems, row)
}

// actionText returns a terminal action's command, a file edit's "<command>
// <path>", else the arguments.
func actionText(event jsonvalue.Value) string {
	action := event.Get("action")
	command, hasCommand := nonEmptyText(action.Get("command"))
	if path, ok := nonEmptyText(action.Get("path")); ok {
		if hasCommand {
			return command + " " + path
		}
		return path
	}
	if hasCommand {
		return command
	}
	if arguments, ok := nonEmptyText(event.Get("tool_call").Get("arguments")); ok {
		return arguments
	}
	var rest []jsonvalue.Member
	for _, member := range action.Members() {
		if member.Key != "kind" {
			rest = append(rest, member)
		}
	}
	if len(rest) > 0 {
		return parserkit.JSONText(jsonvalue.NewObject(rest...))
	}
	return parserkit.Text(action.Get("kind"))
}

// rejectionRows emits the result and the permission decision of a rejected
// action.
func rejectionRows(event jsonvalue.Value, row eventRowMaker) {
	toolName, toolUseID := parserkit.Text(event.Get("tool_name")), parserkit.Text(event.Get("tool_call_id"))
	source, reason := event.Get("rejection_source"), event.Get("rejection_reason")
	row(timeline.TurnToolResult, "[rejected by "+parserkit.TextOr(source, "user")+"] "+parserkit.TextOr(reason, ""),
		toolName, toolUseID)
	row(timeline.TurnSystem, parserkit.JoinNonEmpty(" ", "permission decision: rejected rejection_source="+
		parserkit.TextOr(source, "-"), parserkit.Text(reason)), toolName, toolUseID)
}

// acpRows emits the call and the result of an ACP tool call.
func acpRows(event jsonvalue.Value, row eventRowMaker) {
	name, callID := parserkit.TextOr(event.Get("tool_kind"), "acp"), parserkit.Text(event.Get("tool_call_id"))
	input := ""
	if raw := event.Get("raw_input"); !raw.IsNull() {
		input = parserkit.JSONText(raw)
	}
	row(timeline.TurnToolUse, parserkit.JoinNonEmpty(" | ", parserkit.Text(event.Get("title")), input), name, callID)
	output, content := "", ""
	switch raw := event.Get("raw_output"); {
	case raw.IsString():
		output = raw.StringValue()
	case !raw.IsNull():
		output = parserkit.JSONText(raw)
	}
	if event.Get("content").IsArray() {
		content = parserkit.TextOf(event.Get("content"))
	}
	text := parserkit.JoinNonEmpty(" | ", output, content)
	status := event.Get("status")
	if text != "" || parserkit.IsText(status, "completed") || parserkit.IsText(status, "failed") {
		if !event.Get("is_error").IsEmpty() || parserkit.IsText(status, "failed") {
			text = "[error] " + text
		}
		row(timeline.TurnToolResult, text, name, callID)
	}
}

// hookRow emits the row of a hook that ran.
func hookRow(event jsonvalue.Value, row eventRowMaker) {
	exit := ""
	if code := event.Get("exit_code"); !code.IsNull() {
		exit = "exit=" + code.Text()
	}
	blocked := ""
	if !event.Get("blocked").IsEmpty() {
		blocked = "blocked"
	}
	row(timeline.TurnSystem, parserkit.JoinNonEmpty(" | ", "hook "+parserkit.Text(event.Get("hook_event_type"))+": "+
		parserkit.Text(event.Get("hook_command")), exit, blocked, parserkit.Text(event.Get("stderr"))), "", "")
}

// securityRows emits one permission decision row per action the security
// analyzer rated; the analyzer and policy each row repeats are cut by the
// kit (parserkit.Carried).
func securityRows(artifact *input.Artifact, event jsonvalue.Value, conv *conversation, row eventRowMaker) {
	details := event.Get("details")
	carried := parserkit.NewCarried(artifact)
	analyzer := carried.Cut("analyzer", parserkit.TextOr(event.Get("analyzer"), "-"))
	policy := carried.Cut("policy", parserkit.TextOr(event.Get("policy"), "-"))
	for _, risk := range event.Get("risks").Members() {
		held := conv.actions[risk.Key]
		missing := ""
		if held[1] == "" {
			missing = "action_id=" + risk.Key
		}
		detailText := ""
		if detail := details.Get(risk.Key); !detail.IsEmpty() {
			detailText = "details=" + parserkit.JSONText(detail)
		}
		row(timeline.TurnSystem, parserkit.JoinNonEmpty(" ", "permission decision: analyzed risk="+risk.Value.Text()+
			" analyzer="+analyzer+" policy="+policy, missing, detailText), held[0], held[1])
	}
	var problems parserkit.Problems
	carried.Report(0, &problems)
	for _, problem := range problems {
		row(timeline.TurnSystem, parserkit.ProblemPrefix+problem.Message, "", "")
	}
}
