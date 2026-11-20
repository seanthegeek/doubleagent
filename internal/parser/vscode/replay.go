package vscode

import (
	"bytes"
	"errors"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// sessionStart is the key of requestLines that holds the line of the
// session's own creation.
const sessionStart = -1

// errNotObject is the reason a log line holding a JSON value other than an
// object is a problem, worded as the kit's JSON line reader words it.
var errNotObject = errors.New("not a JSON object")

// requestLines maps a request's position to the log line that created it.
type requestLines map[int64]int

// replay replays a .jsonl mutation log and returns the session state,
// filling lines with the log line that made each request and the line of
// the session's own creation. A line that does not decode, holds a JSON
// value other than an object, or whose mutation does not apply, is a
// problem with the line's text and is skipped. The entries the replay
// changes are counted in work.
func replay(path string, lines requestLines, problems *parser.Problems, work parser.Work) jsonvalue.Value {
	var state *node
	started, startLine := false, 0
	for number, raw := range parser.Lines(path, problems) {
		stripped := bytes.TrimSpace(raw)
		if len(stripped) == 0 {
			continue
		}
		record, err := jsonvalue.DecodeString(textlines.ValidUTF8(stripped))
		if err == nil && !record.IsObject() {
			err = errNotObject
		}
		if err != nil {
			problems.AddRaw(number, parser.UnparseableLine(err), parser.LineText(raw))
			continue
		}
		next, err := applyMutation(state, record, number, work)
		if err != nil {
			problems.AddRaw(number, "mutation not applied ("+err.Error()+")", parser.LineText(raw))
			continue
		}
		state = next
		if isKind(record.Get("kind"), 0) {
			started, startLine = true, number
		}
	}
	trackLines(state, started, startLine, lines)
	return state.value()
}

// trackLines fills lines from the replayed state: the log line that made
// each request, which every node carries from the record that made it (a
// replaced state, a list set whole, a push, a set by index), so a delete
// moves no line, and the session start's line once a state was set whole.
func trackLines(state *node, started bool, startLine int, lines requestLines) {
	clear(lines)
	if started {
		lines[sessionStart] = startLine
	}
	if requests := requestList(state); requests != nil {
		for i, request := range requests.items.items() {
			lines[int64(i)] = request.line
		}
	}
}

// requestList returns the state's requests array, nil when it has none.
func requestList(state *node) *node {
	if state == nil || !state.isObject {
		return nil
	}
	if requests := state.members["requests"]; requests != nil && requests.isArray {
		return requests
	}
	return nil
}
