package cursor

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// optionIDs returns the ids of a prompt's allowlist options.
func optionIDs(review jsonvalue.Value) []jsonvalue.Value {
	var ids []jsonvalue.Value
	for _, option := range review.Get("allowlistOptions").Items() {
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
		ids = append(ids, ident.Text())
	}
	return "permission prompt pending: " + cmp.Or(name, "-") + " reason=" + parserkit.SettingOrDash(extra.Get("blockReason")) +
		" options=" + cmp.Or(strings.Join(ids, ","), "-")
}

// optionNames returns the name of each allowlistOptions entry by its id:
// its commandName, else the id. An id two entries give, which the app
// never writes, has the names of both, each once, in the order met, so
// no entry's name stands in for another's.
func optionNames(options jsonvalue.Value) map[string][]string {
	names, met := map[string][]string{}, map[[2]string]bool{}
	for _, option := range options.Items() {
		if ident := option.Get("id"); option.IsObject() && !ident.IsNull() {
			name := ident.Text()
			if command := option.Get("commandName"); !command.IsEmpty() {
				name = command.Text()
			}
			if key := [2]string{ident.Text(), name}; !met[key] {
				met[key] = true
				names[ident.Text()] = append(names[ident.Text()], name)
			}
		}
	}
	return names
}

// permissionText returns the "permission decision:" text of an answered
// prompt; " allowlisted=" with the applied allowlist entries follows when
// there are any, which marks an allow-always decision.
func permissionText(extra, review jsonvalue.Value) string {
	text := "permission decision: " + parserkit.SettingOrDash(review.Get("selectedOption")) + " outcome=" +
		parserkit.SettingOrDash(extra.Get("status")) + " approval=" + parserkit.SettingOrDash(review.Get("approvalType")) + " reason=" +
		parserkit.SettingOrDash(extra.Get("blockReason"))
	applied := review.Get("appliedAllowlistOptionIds")
	if !applied.IsArray() || applied.Len() == 0 {
		return text
	}
	names := optionNames(review.Get("allowlistOptions"))
	// An id the list names again is written as itself: the app writes
	// each once, and its name written per repeat would grow with the
	// square of the input.
	var shown []string
	written := map[string]bool{}
	for _, ident := range applied.Items() {
		name, ok := names[ident.Text()]
		if ok && !written[ident.Text()] {
			shown = append(shown, strings.Join(name, "|"))
		} else {
			shown = append(shown, ident.Text())
		}
		written[ident.Text()] = true
	}
	return text + " allowlisted=" + strings.Join(shown, ",")
}

// reviewText returns the text of a tool call's permission review and
// whether its prompt is still open: a turn the user stopped (whose
// selectedOption is a preset), an open prompt, or an answer.
func reviewText(name string, tool, extra, review jsonvalue.Value) (string, bool) {
	if parserkit.IsText(review.Get("status"), "None") && parserkit.IsText(extra.Get("status"), "rejected") {
		return stoppedText, false
	}
	if !parserkit.IsText(review.Get("status"), "Done") || parserkit.IsText(tool.Get("status"), "loading") {
		return pendingText(name, extra, review), true
	}
	return permissionText(extra, review), false
}

// tool emits the rows of a tool call bubble: the call, its permission
// prompt or decision, and its result.
func (p *Parser) tool(stamp *stamper, bubble, entry jsonvalue.Value, rowid int64, model string) {
	tool := bubble.Get("toolFormerData")
	extra := tool.Get("additionalData")
	name, callID := parserkit.Text(tool.Get("name")), parserkit.Text(tool.Get("toolCallId"))
	started := firstOrLast(bubble.Get("startedAtMs"), extra.Get("startedAtMs"), bubble.Get("createdAt"),
		entry.Get("createdAt"))
	row := func(turnType, text string, when jsonvalue.Value) {
		made := stamp.row(turnType, text, timeutil.FromValue(when), rowid, model)
		made.ToolName, made.ToolUseID = name, callID
		stamp.emit(made)
	}
	arguments := parserkit.FirstNonEmpty(tool.Get("params"), tool.Get("rawArgs"))
	argsText := arguments.StringValue()
	if !arguments.IsString() && !arguments.IsEmpty() {
		argsText = parserkit.JSONText(arguments)
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
	if result := tool.Get("result"); parserkit.Present(result) {
		resultText = timeline.Compact(parserkit.SettingValue(result))
	}
	rejected := parserkit.IsText(extra.Get("status"), "rejected") && !pending
	if resultText != "" || rejected {
		text := resultText
		if rejected {
			text = "[rejected] " + text
		}
		row(timeline.TurnToolResult, timeline.Compact(text), firstOrLast(bubble.Get("completedAtMs"), started))
	}
}
