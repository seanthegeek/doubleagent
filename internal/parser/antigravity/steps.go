package antigravity

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// stepRow returns a row of a step at a time, with its model.
func (r *conversationReader) stepRow(index int64, timestamp, model, turnType, text string) timeline.Row {
	row := r.row(index)
	row.TimestampUTC, row.Model, row.TurnType = timestamp, model, turnType
	row.Text = timeline.Compact(text)
	return row
}

// stepRows keeps the rows of one step: a prompt, a reply with its
// reasoning and tool calls, or one line for any other step, then a
// permission decision when the user rejected a command or MCP call.
func (r *conversationReader) stepRows(index int64, stored sqlitefile.Row, step, metadata message, model string) {
	created := metadata.text("created_at")
	completed := metadata.first("completed_at", "finished_generating_at")
	if completed == "" {
		completed = created
	}
	kind := r.owner.stepKind(step)
	payload := step.child(kind)
	switch kind {
	case "user_input":
		r.userInput(index, created, model, payload, metadata)
		return
	case "planner_response":
		r.plannerResponse(index, created, completed, model, payload)
		return
	}
	text := "step " + enumText(stored.Get("step_type"), stepTypes())
	if kind != "" {
		text = describePayload(kind, payload)
	}
	if status := enumText(stored.Get("status"), stepStatuses()); status != "done" {
		text = "[" + status + "] " + text
	}
	if kind == "run_command" && payload.truthy("ran_in_sandbox") {
		text += " sandbox=true"
	}
	row := r.stepRow(index, created, model, timeline.TurnSystem, text)
	if call := metadata.child("tool_call"); !call.empty() {
		row = r.stepRow(index, completed, model, timeline.TurnToolResult, text)
		row.ToolName, row.ToolUseID = call.first("name", "original_name"), call.text("id")
	}
	r.add(row)
	if (kind == "run_command" || kind == "mcp_tool") && payload.truthy("user_rejected") {
		r.rejection(row, payload)
	}
}

// stepKind returns the kind of a step: the first payload kind it holds, in
// declaration order, or "".
func (p *Parser) stepKind(step message) string {
	for _, name := range p.payloadKinds {
		if step.has(name) {
			return name
		}
	}
	return ""
}

// rejection keeps the user's rejection of a command or MCP call as its own
// row, beside the result that also says "[rejected by user]".
func (r *conversationReader) rejection(result timeline.Row, payload message) {
	decision := result
	decision.TurnType, decision.Text = timeline.TurnSystem, "permission decision: user_rejected"
	if decision.ToolName == "" {
		decision.ToolName = payload.child("tool_call").text("name")
	}
	r.add(decision)
}

// userInput keeps the row of a prompt: a system row for one the user did
// not type.
func (r *conversationReader) userInput(index int64, created, model string, payload, metadata message) {
	turnType := timeline.TurnUser
	if source, ok := metadata.intValue("source"); ok && source == stepSourceUserImplicit {
		turnType = timeline.TurnSystem
	}
	text := payload.first("query", "user_response")
	if payload.truthy("is_queued_message") {
		text = "[queued] " + text
	}
	r.add(r.stepRow(index, created, model, turnType, text))
}

// plannerResponse keeps the rows of a reply: its reasoning when asked for,
// its text and its tool calls.
func (r *conversationReader) plannerResponse(index int64, created, completed, model string, payload message) {
	if thinking := payload.first("raw_thinking", "thinking"); thinking != "" && r.options.IncludeThinking {
		r.add(r.stepRow(index, created, model, timeline.TurnThinking, thinking))
	}
	if text := payload.first("modified_response", "response"); text != "" {
		r.add(r.stepRow(index, completed, model, timeline.TurnAssistant, text))
	}
	for _, call := range payload.children("tool_calls") {
		row := r.stepRow(index, completed, model, timeline.TurnToolUse, toolArgsSummary(call.text("arguments_json")))
		row.ToolName, row.ToolUseID = call.first("name", "original_name"), call.text("id")
		r.add(row)
	}
}

// permissionRows keeps one system row per grant in a step's permissions:
// the steps.permissions column, else the Step's own field.
func (r *conversationReader) permissionRows(index int64, column sqlitefile.Value, step, metadata message) {
	permissions := step.child("permissions")
	if !column.IsEmpty() {
		decoded, ok := r.owner.decodeMessage(blobOf(column), "TrajectoryPermissions")
		if !ok {
			row := r.row(index)
			row.TurnType = timeline.TurnSystem
			row.Text = "parser: the permissions of step " + strconv.FormatInt(index, 10) + " could not be decoded"
			r.add(row)
			return
		}
		permissions = decoded
	}
	call := metadata.child("tool_call")
	timestamp := metadata.first("completed_at", "finished_generating_at", "created_at")
	for _, text := range permissionTexts(permissions) {
		row := r.row(index)
		row.TimestampUTC, row.TurnType = timestamp, timeline.TurnSystem
		row.ToolName, row.ToolUseID = call.first("name", "original_name"), call.text("id")
		row.Text = timeline.Compact(text)
		r.add(row)
	}
}
