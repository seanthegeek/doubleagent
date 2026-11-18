package opencode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	return parser.JSONText(toolInput)
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

// objectOf returns value when it is an object, else an empty object, so a
// chain of lookups reads a missing level as empty.
func objectOf(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}

// join returns the non-empty parts joined by single spaces.
func join(parts ...string) string {
	return parser.JoinNonEmpty(" ", parts...)
}

// labelled returns label followed by the value's text when the value is
// not empty, else "": the "key=value" parts of a row text.
func labelled(label string, value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.Text()
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
	if parser.IsText(message.Get("role"), "assistant") {
		return message.Get("providerID"), message.Get("modelID")
	}
	if model := message.Get("model"); model.IsObject() {
		return model.Get("providerID"), parser.FirstNonEmpty(model.Get("modelID"), model.Get("id"))
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
	failure := objectOf(raw)
	data := objectOf(failure.Get("data"))
	name := parser.FirstNonEmpty(failure.Get("name"), failure.Get("type"))
	message := parser.FirstNonEmpty(data.Get("message"), failure.Get("message"))
	if text := join(parser.Text(name), parser.Text(message)); text != "" {
		return text
	}
	if !raw.IsEmpty() {
		return parser.JSONText(raw)
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
// {permission, pattern, action} objects, or false when the text is not a
// JSON array. Entries of neither shape are skipped; a missing or empty
// field reads "-".
func sessionRules(text string) ([]rule, bool) {
	rules, err := jsonvalue.DecodeString(text)
	if err != nil || !rules.IsArray() {
		return nil, false
	}
	var found []rule
	for _, entry := range rules.Items() {
		entry = objectOf(entry)
		var keys [3]string
		switch {
		case entry.Has("effect"):
			keys = [3]string{"action", "resource", "effect"}
		case entry.Has("permission"):
			keys = [3]string{"permission", "pattern", "action"}
		default:
			continue
		}
		found = append(found, rule{permission: ruleField(entry, keys[0]),
			pattern: ruleField(entry, keys[1]), effect: ruleField(entry, keys[2])})
	}
	return found, true
}

// ruleField returns a rule's field as text, "-" when it is missing, null or
// the empty string.
func ruleField(entry jsonvalue.Value, key string) string {
	value := entry.Get(key)
	if value.IsNull() || parser.IsText(value, "") {
		return "-"
	}
	return value.Text()
}
