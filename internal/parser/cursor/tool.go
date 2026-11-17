package cursor

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// tool emits the rows of a tool call bubble: the call, its permission
// prompt or decision, and its result.
func (p *Parser) tool(stamp *stamper, bubble, entry jsonvalue.Value, rowid int64, model string) {
	tool := bubble.Get("toolFormerData")
	extra := tool.Get("additionalData")
	name, callID := parser.Str(tool.Get("name")), parser.Str(tool.Get("toolCallId"))
	started := firstTruthy(bubble.Get("startedAtMs"), extra.Get("startedAtMs"), bubble.Get("createdAt"),
		entry.Get("createdAt"))
	row := func(turnType, text string, when jsonvalue.Value) {
		made := stamp.row(turnType, text, timeutil.FromValue(when), rowid, model)
		made.ToolName, made.ToolUseID = name, callID
		stamp.emit(made)
	}
	arguments := firstTruthy(tool.Get("params"), tool.Get("rawArgs"))
	argsText := arguments.StringValue()
	if !arguments.IsString() && !arguments.IsEmpty() {
		argsText = parser.CompactJSON(arguments)
	}
	row(timeline.TurnToolUse, timeline.Compact(argsText), started)
	review := extra.Get("reviewData")
	pending := false
	if review.IsObject() {
		var text string
		text, pending = reviewText(name, tool, extra, review)
		row(timeline.TurnSystem, text, started)
	}
	resultText := ""
	if result := tool.Get("result"); !absent(result) {
		resultText = timeline.Compact(textOrJSON(result))
	}
	rejected := isText(extra.Get("status"), "rejected") && !pending
	if resultText != "" || rejected {
		text := resultText
		if rejected {
			text = "[rejected] " + text
		}
		row(timeline.TurnToolResult, timeline.Compact(text), firstTruthy(bubble.Get("completedAtMs"), started))
	}
}

// reviewText returns the text of a tool call's permission review and
// whether its prompt is still open: a turn the user stopped (whose
// selectedOption is a preset), an open prompt, or an answer.
func reviewText(name string, tool, extra, review jsonvalue.Value) (string, bool) {
	if isText(review.Get("status"), "None") && isText(extra.Get("status"), "rejected") {
		return stoppedText, false
	}
	if !isText(review.Get("status"), "Done") || isText(tool.Get("status"), "loading") {
		return pendingText(name, extra, review), true
	}
	return permissionText(extra, review), false
}

// fieldOf returns a decoded value as row text, "-" when it is null or "".
func fieldOf(value jsonvalue.Value) string {
	return orDash(jsonText(value))
}

// optionIDs returns the ids of a prompt's allowlist options.
func optionIDs(review jsonvalue.Value) []jsonvalue.Value {
	var ids []jsonvalue.Value
	for _, option := range parser.Elements(review.Get("allowlistOptions")) {
		if ident := option.Get("id"); option.IsObject() && !ident.IsNull() {
			ids = append(ids, ident)
		}
	}
	return ids
}

// pendingText returns the "permission prompt pending:" text of a prompt
// still open at the copy: its selectedOption is a preset, not a decision.
func pendingText(name string, extra, review jsonvalue.Value) string {
	var ids []string
	for _, ident := range optionIDs(review) {
		ids = append(ids, jsonText(ident))
	}
	return "permission prompt pending: " + orDash(name) + " reason=" + fieldOf(extra.Get("blockReason")) +
		" options=" + orDash(strings.Join(ids, ","))
}

// permissionText returns the "permission decision:" text of an answered
// prompt; " allowlisted=" with the applied allowlist entries follows when
// there are any, which marks an allow-always decision.
func permissionText(extra, review jsonvalue.Value) string {
	text := "permission decision: " + fieldOf(review.Get("selectedOption")) + " outcome=" +
		fieldOf(extra.Get("status")) + " approval=" + fieldOf(review.Get("approvalType")) + " reason=" +
		fieldOf(extra.Get("blockReason"))
	applied := review.Get("appliedAllowlistOptionIds")
	if !applied.IsArray() || applied.Len() == 0 {
		return text
	}
	names := map[string]string{}
	for _, option := range parser.Elements(review.Get("allowlistOptions")) {
		if ident := option.Get("id"); option.IsObject() && !ident.IsNull() {
			name := ident.Text()
			if command := option.Get("commandName"); !command.IsEmpty() {
				name = command.Text()
			}
			names[ident.Text()] = name
		}
	}
	var shown []string
	for _, ident := range applied.Items() {
		if name, ok := names[ident.Text()]; ok {
			shown = append(shown, name)
		} else {
			shown = append(shown, ident.Text())
		}
	}
	return text + " allowlisted=" + strings.Join(shown, ",")
}
