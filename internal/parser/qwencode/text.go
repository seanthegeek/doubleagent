package qwencode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The markers a channel prompt's quoted group messages are wrapped in, one
// "- [<sender>] <text>" line each.
const (
	groupHistoryMarker   = "[Chat messages since your last reply - for context]"
	currentMessageMarker = "[Current message - respond to this]"
)

// quotedLine is one quoted group message of a channel prompt.
var quotedLine = regexp.MustCompile(`^- \[([^\]\r\n]*)\] (.*)\z`)

// argumentKeys are the arguments of a function call that identify it best,
// in the order they are looked for.
func argumentKeys() []string {
	return []string{"command", "absolute_path", "file_path", "path", "pattern", "url", "query",
		"prompt", "description"}
}

// isSet reports whether a value is neither null nor the empty string, the
// test Python's `value not in (None, "")` makes.
func isSet(value jsonvalue.Value) bool {
	return !value.IsNull() && !(value.IsString() && value.StringValue() == "")
}

// textOrJSON returns a string as it is and any other value as compact
// JSON.
func textOrJSON(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return parser.JSONText(value)
}

// orElse returns the first value Python counts as true, else the last, as
// Python's `a or b` does.
func orElse(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values[:len(values)-1] {
		if !value.IsEmpty() {
			return value
		}
	}
	return values[len(values)-1]
}

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// dash returns a value as row text, "-" when it is null or empty.
func dash(value jsonvalue.Value) string {
	if !isSet(value) {
		return "-"
	}
	return value.Text()
}

// argsSummary returns the most identifying argument of a function call,
// else the arguments as compact JSON; "" for none.
func argsSummary(arguments jsonvalue.Value) string {
	if arguments.IsObject() {
		for _, key := range argumentKeys() {
			if value := arguments.Get(key); isSet(value) {
				return textOrJSON(value)
			}
		}
		return parser.JSONText(arguments)
	}
	if !isSet(arguments) {
		return ""
	}
	return textOrJSON(arguments)
}

// messageParts returns the object parts of a record's message.
func messageParts(record jsonvalue.Value) []jsonvalue.Value {
	var out []jsonvalue.Value
	for _, part := range record.Get("message").Get("parts").Items() {
		if part.IsObject() {
			out = append(out, part)
		}
	}
	return out
}

// responseText returns the text of a function response's response: its
// non-empty output, error or llmContent string, else the response as
// compact JSON, "" for none.
func responseText(response jsonvalue.Value) string {
	if response.IsObject() {
		for _, key := range []string{"output", "error", "llmContent"} {
			if value := response.Get(key); value.IsString() && value.StringValue() != "" {
				return value.StringValue()
			}
		}
		return parser.JSONText(response)
	}
	if !isSet(response) {
		return ""
	}
	return textOrJSON(response)
}

// systemText returns the row text of a system record: its subtype and a
// summary of its payload.
func systemText(subtype string, payload jsonvalue.Value) string {
	field := func(key string) string { return parser.Text(payload.Get(key)) }
	var detail string
	switch subtype {
	case "slash_command":
		detail = field("phase") + " " + field("rawCommand")
	case "session_model":
		detail = field("modelId") + " auth=" + field("authType")
	case "custom_title":
		detail = field("customTitle")
	case "turn_result":
		detail = field("state") + " " + field("resultText")
	case "rewind":
		detail = "truncated " + payload.Get("truncatedCount").Text() + " record(s)"
	case "chat_compression":
		if info := payload.Get("info"); !info.IsNull() {
			detail = parser.JSONText(info)
		}
	default:
		if !payload.IsEmpty() {
			detail = parser.JSONText(payload)
		}
	}
	if detail = strings.TrimSpace(detail); detail != "" {
		return subtype + ": " + detail
	}
	return subtype
}

// injectedOrigin returns the row prefix of a user record the user did not
// type, else "": "<subtype> (provenance=<provenance> agent=<agentName>
// agentId=<agentId>)", leaving out what is not recorded. A record is the
// user's when its provenance is absent or real_user, unless its subtype is
// one only the system or another agent writes.
func injectedOrigin(record jsonvalue.Value) string {
	provenance := parser.Text(record.Get("provenance"))
	subtype := parser.Text(record.Get("subtype"))
	injected := subtype == "notification" || subtype == "cron" || subtype == "agent_message"
	if (provenance == "" || provenance == "real_user") && !injected {
		return ""
	}
	author := record.Get("systemPayload").Get("author")
	var keys []string
	for _, pair := range []struct {
		key   string
		value jsonvalue.Value
	}{
		{"provenance", jsonvalue.NewString(provenance)},
		{"agent", orElse(record.Get("agentName"), author.Get("name"))},
		{"agentId", orElse(record.Get("agentId"), author.Get("agentId"))},
	} {
		if isSet(pair.value) {
			keys = append(keys, pair.key+"="+pair.value.Text())
		}
	}
	label := subtype
	if label == "" {
		label = "user"
	}
	if len(keys) > 0 {
		return label + " (" + strings.Join(keys, " ") + ")"
	}
	return label
}

// quotedGroupMessages returns the "group message: [<sender>] <text>" texts
// a channel prompt quotes, else none. The quote is the block between the
// group history marker at the start of a line and a blank line before the
// current message marker; a block with a line of another form is not the
// channel's and gives none.
func quotedGroupMessages(text string) []string {
	opening := groupHistoryMarker + "\n"
	start := strings.Index(text, opening)
	for start > 0 && text[start-1] != '\n' {
		next := strings.Index(text[start+1:], opening)
		if next < 0 {
			start = -1
			break
		}
		start += 1 + next
	}
	if start < 0 {
		return nil
	}
	body := start + len(opening)
	end := strings.Index(text[body:], "\n\n"+currentMessageMarker)
	if end < 0 {
		return nil
	}
	var out []string
	for line := range strings.SplitSeq(text[body:body+end], "\n") {
		found := quotedLine.FindStringSubmatch(line)
		if found == nil {
			return nil
		}
		out = append(out, "group message: ["+found[1]+"] "+found[2])
	}
	return out
}
