package continuedev

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// headerKeys are the session fields recovered from a session file that
// does not parse.
func headerKeys() []string {
	return []string{"sessionId", "title", "workspaceDirectory", "mode", "chatModelTitle"}
}

// historyPattern finds the opening of the history array of a damaged
// session file.
var historyPattern = regexp.MustCompile(`"history"[ \t\n\r]*:[ \t\n\r]*\[`)

// LoadSession returns a session file's object and a problem message, ""
// when it parsed. A file that does not parse yields its header keys and
// every history item that decodes before the damage, with the problem
// saying how many were recovered.
func LoadSession(text string) (jsonvalue.Value, string) {
	document, err := jsonvalue.DecodeString(text)
	if err == nil {
		if document.IsObject() {
			return document, ""
		}
		return jsonvalue.NewObject(), "session file is not a JSON object"
	}
	if start := strings.IndexByte(text, '{'); start >= 0 {
		prefix, end, prefixErr := jsonvalue.DecodePrefix(text, start)
		if prefixErr == nil && prefix.IsObject() {
			return prefix, "trailing data after the session object at offset " +
				strconv.Itoa(utf8.RuneCountInString(text[:end]))
		}
	}
	recovered := jsonvalue.NewObject()
	for _, key := range headerKeys() {
		if value, ok := headerValue(text, key); ok {
			recovered.Set(key, value)
		}
	}
	history := recoverHistory(text)
	recovered.Set("history", jsonvalue.NewArray(history...))
	return recovered, "session file did not parse (" + err.Error() + "); recovered " +
		strconv.Itoa(len(history)) + " history item(s)"
}

// headerValue returns the decoded string value of the first "<key>":
// "<string>" pair in text, or false when there is none or it does not
// decode.
func headerValue(text, key string) (jsonvalue.Value, bool) {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"[ \t\n\r]*:[ \t\n\r]*("(?:[^"\\]|\\.)*")`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.DecodeString(match[1])
	return value, err == nil
}

// recoverHistory returns the object items of the history array of a
// damaged session file, up to the first that does not decode.
func recoverHistory(text string) []jsonvalue.Value {
	location := historyPattern.FindStringIndex(text)
	if location == nil {
		return nil
	}
	var history []jsonvalue.Value
	position := location[1]
	for {
		for position < len(text) && strings.IndexByte(" \t\r\n,", text[position]) >= 0 {
			position++
		}
		if position >= len(text) || text[position] == ']' {
			return history
		}
		item, end, err := jsonvalue.DecodePrefix(text, position)
		if err != nil {
			return history
		}
		position = end
		if item.IsObject() {
			history = append(history, item)
		}
	}
}

// MessageText returns the text of a chat message's content, image parts
// written as "[image]".
func MessageText(content jsonvalue.Value) string {
	if !content.IsArray() {
		return parser.TextOf(content)
	}
	parts := make([]jsonvalue.Value, 0, content.Len())
	for _, part := range content.Items() {
		if part.IsObject() && isText(part.Get("type"), "imageUrl") {
			part = jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
				jsonvalue.Member{Key: "text", Value: jsonvalue.NewString("[image]")})
		}
		parts = append(parts, part)
	}
	return parser.TextOf(jsonvalue.NewArray(parts...))
}

// isText reports whether a value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}
