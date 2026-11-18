package vscode

import (
	"bytes"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// sessionStart is the key of requestLines that holds the line of the
// session's own creation.
const sessionStart = -1

// requestLines maps a request's position to the log line that created it.
type requestLines map[int64]int

// replay replays a .jsonl mutation log and returns the session state. A
// line that does not decode, or whose mutation does not apply, is a problem
// with the line's text and is skipped; a line holding anything but an
// object is skipped silently. The entries the replay moves are counted in
// work.
func replay(path string, lines requestLines, problems *parser.Problems, work parser.Work) jsonvalue.Value {
	var state *node
	for number, raw := range parser.Lines(path, problems) {
		stripped := bytes.TrimSpace(raw)
		if len(stripped) == 0 {
			continue
		}
		record, err := jsonvalue.DecodeString(textlines.ValidUTF8(stripped))
		if err != nil {
			problems.AddRaw(number, parser.UnparseableLine(err), parser.LineText(raw))
			continue
		}
		if !record.IsObject() {
			continue
		}
		before := requestCount(state)
		next, err := applyMutation(state, record, work)
		if err != nil {
			problems.AddRaw(number, "mutation not applied ("+err.Error()+")", parser.LineText(raw))
			continue
		}
		state = next
		track(record, before, requestCount(state), number, lines, work)
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

// track remembers the log line that created each request, given how many
// requests there were before and after the record was applied: every
// request of a replaced state or of a list set whole, the requests a push
// added at the end of the list (whatever the truncation before it), a
// request set by its index, and a deleted request's followers moved down
// one place. Before the record, lines holds a line for each of the before
// requests and no other, so only those from the first changed one on are
// looked at; the entries set or moved are counted in work.
func track(record jsonvalue.Value, before, after int64, number int, lines requestLines, work parser.Work) {
	kind := record.Get("kind")
	path := record.Get("k").Items()
	switch {
	case isKind(kind, 0):
		clear(lines)
		lines[sessionStart] = number
		for i := range after {
			lines[i] = number
		}
		work.Add("entries moved", int(after))
	case len(path) == 1 && isRequests(path[0]):
		start := int64(0)
		if isKind(kind, 2) {
			start = after - pushedCount(record.Get("v"))
		}
		for i := start; i < before; i++ {
			delete(lines, i)
		}
		for i := start; i < after; i++ {
			lines[i] = number
		}
		work.Add("entries moved", int(max(before, after)-start))
	case len(path) == 2 && isRequests(path[0]) && isKind(kind, 1) && path[1].IsInt():
		index, _ := integer(path[1])
		lines[index] = number
	case len(path) == 2 && isRequests(path[0]) && isKind(kind, 3):
		if removed, ok := listIndex(path[1], int(before)); ok {
			for i := int64(removed); i < after; i++ {
				if line, ok := lines[i+1]; ok {
					lines[i] = line
				} else {
					delete(lines, i)
				}
			}
			delete(lines, after)
			work.Add("entries moved", int(after)-removed)
		}
	}
}

// isRequests reports whether a path key is "requests".
func isRequests(key jsonvalue.Value) bool {
	return key.IsString() && key.StringValue() == "requests"
}

// pushedCount returns how many items a push of value adds: each item of
// an array, or the value itself as one item.
func pushedCount(value jsonvalue.Value) int64 {
	if value.IsArray() {
		return int64(value.Len())
	}
	return 1
}
