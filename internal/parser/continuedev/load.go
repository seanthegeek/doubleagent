package continuedev

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
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
func LoadSession(text string) (pyjson.Value, string) {
	document, err := pyjson.DecodeString(text)
	if err == nil {
		if document.IsObject() {
			return document, ""
		}
		return pyjson.NewObject(), "session file is not a JSON object"
	}
	if start := strings.IndexByte(text, '{'); start >= 0 {
		prefix, end, prefixErr := pyjson.DecodePrefix(text, start)
		if prefixErr == nil && prefix.IsObject() {
			return prefix, "trailing data after the session object at offset " +
				strconv.Itoa(pytext.CodePointLen(text[:end]))
		}
	}
	recovered := pyjson.NewObject()
	for _, key := range headerKeys() {
		if value, ok := headerValue(text, key); ok {
			recovered.Set(key, value)
		}
	}
	history := recoverHistory(text)
	recovered.Set("history", pyjson.NewArray(history...))
	return recovered, "session file did not parse (" + err.Error() + "); recovered " +
		strconv.Itoa(len(history)) + " history item(s)"
}

// headerValue returns the decoded string value of the first "<key>":
// "<string>" pair in text, or false when there is none or it does not
// decode.
func headerValue(text, key string) (pyjson.Value, bool) {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"[ \t\n\r]*:[ \t\n\r]*("(?:[^"\\]|\\.)*")`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return pyjson.NewNull(), false
	}
	value, err := pyjson.DecodeString(match[1])
	return value, err == nil
}

// recoverHistory returns the object items of the history array of a
// damaged session file, up to the first that does not decode.
func recoverHistory(text string) []pyjson.Value {
	location := historyPattern.FindStringIndex(text)
	if location == nil {
		return nil
	}
	var history []pyjson.Value
	position := location[1]
	for {
		for position < len(text) && strings.IndexByte(" \t\r\n,", text[position]) >= 0 {
			position++
		}
		if position >= len(text) || text[position] == ']' {
			return history
		}
		item, end, err := pyjson.DecodePrefix(text, position)
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
func MessageText(content pyjson.Value) string {
	if !content.IsArray() {
		return parser.TextOf(content)
	}
	parts := make([]pyjson.Value, 0, content.Len())
	for _, part := range content.Items() {
		if part.IsObject() && isText(part.Get("type"), "imageUrl") {
			part = pyjson.NewObject(pyjson.Member{Key: "type", Value: pyjson.NewString("text")},
				pyjson.Member{Key: "text", Value: pyjson.NewString("[image]")})
		}
		parts = append(parts, part)
	}
	return parser.TextOf(pyjson.NewArray(parts...))
}

// isText reports whether a value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}
