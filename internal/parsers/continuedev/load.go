package continuedev

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// headerKeys are the session fields recovered from a session file that
// does not parse.
func headerKeys() []string {
	return []string{"sessionId", "title", "workspaceDirectory", "mode", "chatModelTitle"}
}

// headerPatterns finds the first "<key>": "<string>" pair of each header key.
var headerPatterns = func() map[string]*regexp.Regexp {
	patterns := map[string]*regexp.Regexp{}
	for _, key := range headerKeys() {
		patterns[key] = regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"[ \t\n\r]*:[ \t\n\r]*("(?:[^"\\]|\\.)*")`)
	}
	return patterns
}()

// historyPattern finds the opening of the history array of a damaged
// session file.
var historyPattern = regexp.MustCompile(`"history"[ \t\n\r]*:[ \t\n\r]*\[`)

// errNotObject is the problem of a session file that decodes to something
// other than an object.
var errNotObject = errors.New("session file is not a JSON object")

// headerValue returns the decoded string value of the first "<key>":
// "<string>" pair in text, or false when there is none or it does not
// decode.
func headerValue(text, key string) (jsonvalue.Value, bool) {
	match := headerPatterns[key].FindStringSubmatch(text)
	if match == nil {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.DecodeString(match[1])
	return value, err == nil
}

// recoverHistory returns the object items of the history array of a
// damaged session file, up to the first that does not decode.
func recoverHistory(artifact *input.Artifact, text string) []jsonvalue.Value {
	location := historyPattern.FindStringIndex(text)
	if location == nil {
		return nil
	}
	return parserkit.ObjectsFrom(artifact, text, location[1])
}

// LoadSession returns a session file's object, always an object, and the
// problem that kept it from decoding whole, nil when it did. A file whose
// object is followed by other data keeps the object, and the problem gives
// the offset where the data starts, counted in characters (runes). A file
// that does not decode yields its header keys and every history item that
// decodes before the damage, with the problem saying how many were
// recovered. Repeated member names are noted in the parse of artifact.
func LoadSession(artifact *input.Artifact, text string) (jsonvalue.Value, error) {
	document, err := parserkit.DecodeJSON(artifact, text)
	if err == nil {
		if document.IsObject() {
			return document, nil
		}
		return jsonvalue.NewObject(), errNotObject
	}
	if start := strings.IndexByte(text, '{'); start >= 0 {
		document := jsonvalue.NewDocument(text)
		prefix, end, prefixErr := document.DecodePrefix(start)
		parserkit.NoteRepeats(artifact, document.TakeRepeats())
		if prefixErr == nil && prefix.IsObject() {
			return prefix, fmt.Errorf("trailing data after the session object at offset %d",
				utf8.RuneCountInString(text[:end]))
		}
	}
	recovered := jsonvalue.NewObject()
	for _, key := range headerKeys() {
		if value, ok := headerValue(text, key); ok {
			recovered.Set(key, value)
		}
	}
	history := recoverHistory(artifact, text)
	recovered.Set("history", jsonvalue.NewArray(history...))
	return recovered, fmt.Errorf("session file did not parse (%w); recovered %d history item(s)",
		err, len(history))
}

// MessageText returns the text of a chat message's content, image parts
// written as "[image]".
func MessageText(content jsonvalue.Value) string {
	if !content.IsArray() {
		return parserkit.TextOf(content)
	}
	parts := make([]jsonvalue.Value, 0, content.Len())
	for _, part := range content.Items() {
		if part.IsObject() && parserkit.IsText(part.Get("type"), "imageUrl") {
			part = jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
				jsonvalue.Member{Key: "text", Value: jsonvalue.NewString("[image]")})
		}
		parts = append(parts, part)
	}
	return parserkit.TextOf(jsonvalue.NewArray(parts...))
}
