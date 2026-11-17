package vscode

import (
	"bytes"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// asciiWhitespace is what Python's bytes.strip() removes from a line.
const asciiWhitespace = " \t\n\r\v\f"

// sessionStart is the key of requestLines that holds the line of the
// session's own creation.
const sessionStart = -1

// requestLines maps a request's position to the log line that created it.
type requestLines map[int64]int

// replay replays a .jsonl mutation log and returns the session state. A
// line that does not decode, or whose mutation does not apply, is a problem
// with the line's text and is skipped; a line holding anything but an
// object is skipped silently.
func replay(path string, lines requestLines, problems *parser.Problems) pyjson.Value {
	var state *node
	for number, raw := range parser.Lines(path, problems) {
		stripped := bytes.Trim(raw, asciiWhitespace)
		if len(stripped) == 0 {
			continue
		}
		record, err := pyjson.DecodeString(pytext.DecodeUTF8(stripped))
		if err != nil {
			problems.AddRaw(number, parser.UnparseableLine(err), parser.LineText(raw))
			continue
		}
		if !record.IsObject() {
			continue
		}
		next, err := applyMutation(state, record)
		if err != nil {
			problems.AddRaw(number, "mutation not applied ("+err.Error()+")", parser.LineText(raw))
			continue
		}
		state = next
		track(record, state, number, lines)
	}
	return state.value()
}

// requestCount returns how many requests the state holds.
func requestCount(state *node) int64 {
	if state == nil || !state.isObject {
		return 0
	}
	requests := state.members["requests"]
	if requests == nil || !requests.isArray {
		return 0
	}
	return int64(len(requests.items))
}

// track remembers the log line that created each request: every request
// of a replaced state, the requests a push or set of the whole list made,
// and a request set by its index.
func track(record pyjson.Value, state *node, number int, lines requestLines) {
	kind := record.Get("kind")
	path := record.Get("k").Items()
	switch {
	case equalsInt(kind, 0):
		clear(lines)
		lines[sessionStart] = number
		for i := range requestCount(state) {
			lines[i] = number
		}
	case len(path) == 1 && path[0].IsString() && path[0].Text() == "requests":
		total := requestCount(state)
		start := int64(0)
		if equalsInt(kind, 2) {
			if given, ok := pyInt(record.Get("i")); ok {
				start = given
			} else {
				start = total - pushedCount(record.Get("v"))
			}
		}
		for i := range lines {
			if i >= start {
				delete(lines, i)
			}
		}
		for i := max(start, 0); i < total; i++ {
			lines[i] = number
		}
	case equalsInt(kind, 1) && len(path) == 2 && path[0].IsString() && path[0].Text() == "requests" &&
		isIntLike(path[1]):
		index, _ := pyInt(path[1])
		lines[index] = number
	}
}

// pushedCount returns len(v or []) for a pushed value: the items of an
// array, the members of an object, the characters of a string, and 0 for
// a value Python counts as false. Python cannot take the length of another
// value; it is counted as 0 here.
func pushedCount(value pyjson.Value) int64 {
	if value.IsString() {
		return int64(pytext.CodePointLen(value.Text()))
	}
	return int64(value.Len())
}
