package opencode

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The refusal texts a tool's error carries. 1.x and Kilo Code store the
// error's message as the tool part's state.error string; 2.0 stores
// {type, message} on the tool item.
const (
	rejectedV1 = "The user rejected permission to use this specific tool call."
	feedbackV1 = "The user rejected permission to use this specific tool call with the following feedback: "
	ruleV1     = "The user has specified a rule which prevents you from using this specific tool call."
	declinedV2 = "The user declined this tool call"
)

// toolKeys returns the tool input fields that best identify the action, in
// order of preference.
func toolKeys() []string {
	return []string{"command", "filePath", "file_path", "pattern", "path", "url", "query",
		"description", "prompt"}
}

// ToolSummary returns the most identifying argument of a tool input: the
// first non-empty string among command, filePath, file_path, pattern
// (joined as "<pattern> | <path>" when a path is given too), path, url,
// query, description and prompt. A string input is itself; null, "" and
// an empty object give ""; any other input is its compact JSON.
func ToolSummary(toolInput jsonvalue.Value) string {
	if toolInput.IsString() {
		return toolInput.StringValue()
	}
	if toolInput.IsObject() {
		for _, key := range toolKeys() {
			argument := toolInput.Get(key)
			if !isFilled(argument) {
				continue
			}
			if place := toolInput.Get("path"); key == "pattern" && isFilled(place) {
				return argument.StringValue() + " | " + place.StringValue()
			}
			return argument.StringValue()
		}
		if toolInput.Len() == 0 {
			return ""
		}
	}
	if toolInput.IsNull() {
		return ""
	}
	return parserkit.JSONText(toolInput)
}

// isFilled reports whether value is a non-empty string.
func isFilled(value jsonvalue.Value) bool {
	return value.IsString() && value.StringValue() != ""
}

// textOf returns a string value's text, "" for any other value.
func textOf(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return ""
}

// join returns the non-empty parts joined by single spaces.
func join(parts ...string) string {
	return parserkit.JoinNonEmpty(" ", parts...)
}

// modelName returns "<provider>/<model>", or whichever of the two is a
// non-empty string.
func modelName(provider, model jsonvalue.Value) string {
	providerText, modelText := textOf(provider), textOf(model)
	if providerText != "" && modelText != "" {
		return providerText + "/" + modelText
	}
	if modelText != "" {
		return modelText
	}
	return providerText
}

// messageRef returns the provider and model id of a 1.x message: the
// assistant's own providerID and modelID, else the user message's model
// object (its modelID, else its id).
func messageRef(message jsonvalue.Value) (jsonvalue.Value, jsonvalue.Value) {
	if parserkit.IsText(message.Get("role"), "assistant") {
		return message.Get("providerID"), message.Get("modelID")
	}
	if model := message.Get("model"); model.IsObject() {
		return model.Get("providerID"), parserkit.FirstNonEmpty(model.Get("modelID"), model.Get("id"))
	}
	return jsonvalue.NewNull(), jsonvalue.NewNull()
}

// errorText returns an error value as row text: a string as it is; for an
// object its name (else type) and its data.message (else message) joined,
// else the object as compact JSON; "" for an empty value.
func errorText(raw jsonvalue.Value) string {
	if raw.IsString() {
		return raw.StringValue()
	}
	failure := parserkit.ObjectOrEmpty(raw)
	data := parserkit.ObjectOrEmpty(failure.Get("data"))
	name := parserkit.FirstNonEmpty(failure.Get("name"), failure.Get("type"))
	message := parserkit.FirstNonEmpty(data.Get("message"), failure.Get("message"))
	if text := join(parserkit.Text(name), parserkit.Text(message)); text != "" {
		return text
	}
	if !raw.IsEmpty() {
		return parserkit.JSONText(raw)
	}
	return ""
}

// decisionText returns the "permission decision:" text of a tool error that
// records a refusal, else "". The 2.0 permission.rejected type covers both
// a deny rule and a rejection with feedback, which its message does not
// tell apart, so it keeps the agent's own term.
func decisionText(failure jsonvalue.Value) string {
	var kind, message string
	switch {
	case failure.IsString():
		message = failure.StringValue()
	case failure.IsObject():
		kind, message = textOf(failure.Get("type")), textOf(failure.Get("message"))
	default:
		return ""
	}
	switch {
	case kind == "permission.rejected":
		return "permission decision: rejected (permission.rejected)"
	case (kind == "" || kind == "aborted") && message == declinedV2:
		return "permission decision: rejected"
	case strings.HasPrefix(message, feedbackV1):
		return "permission decision: rejected with feedback"
	case strings.HasPrefix(message, rejectedV1):
		return "permission decision: rejected"
	case strings.HasPrefix(message, ruleV1):
		return "permission decision: denied by rule"
	}
	return ""
}

// rule is one rule of a session's permission column.
type rule struct {
	permission, pattern, effect string
}

// sessionRules returns the rules of a session's permission column text, a
// JSON array of 2.0 {action, resource, effect} or 1.x and Kilo Code
// {permission, pattern, action} objects, and the number of entries of
// neither shape (not objects, or objects with neither key), which give no
// rule; false when the text is not a JSON array. A missing or empty field
// reads "-".
func sessionRules(artifact *input.Artifact, text string) ([]rule, int, bool) {
	rules, err := parserkit.DecodeJSON(artifact, text)
	if err != nil || !rules.IsArray() {
		return nil, 0, false
	}
	var found []rule
	skipped := 0
	for _, entry := range rules.Items() {
		entry = parserkit.ObjectOrEmpty(entry)
		var keys [3]string
		switch {
		case entry.Has("effect"):
			keys = [3]string{"action", "resource", "effect"}
		case entry.Has("permission"):
			keys = [3]string{"permission", "pattern", "action"}
		default:
			skipped++
			continue
		}
		found = append(found, rule{permission: parserkit.SettingOrDash(entry.Get(keys[0])),
			pattern: parserkit.SettingOrDash(entry.Get(keys[1])), effect: parserkit.SettingOrDash(entry.Get(keys[2]))})
	}
	return found, skipped, true
}

// notRules returns the problem text of a permission column holding n
// entries that are not rules: "<column> holds <n> entries that are not
// permission rules", "1 entry that is not" for one.
func notRules(column string, n int) string {
	if n == 1 {
		return column + " holds 1 entry that is not a permission rule"
	}
	return column + " holds " + strconv.Itoa(n) + " entries that are not permission rules"
}
