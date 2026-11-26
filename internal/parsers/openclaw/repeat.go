package openclaw

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// windowRepeat is a session id session_windows holds in more than one row,
// which only damage or an edit writes, since the id is the table's key.
// Its rows are a group (timeline.Group): each stored row is a parent, with
// a `parser:` row of its own naming its key, columns and model at its
// start, and its relay rows; the rows of the id's transcripts whose model
// is the window's are members, taking the model of the stored row the
// group goes with. A row's text cannot follow that choice, so the id's
// session-start rows name no window's key or columns.
type windowRepeat struct {
	group   *timeline.Group
	parents []*timeline.Parent
	// notices are the parents' `parser:` rows, emitted once the
	// transcripts have given the session's working directory.
	notices []timeline.Row
}

// parent returns the parent of the i-th stored row, nil for a nil repeat.
func (r *windowRepeat) parent(i int) *timeline.Parent {
	if r == nil {
		return nil
	}
	return r.parents[i]
}

// repeatWindows makes the group of each window id stored more than once,
// its parents in table order.
func (a *agentDatabase) repeatWindows() {
	for _, sessionID := range a.windows.order {
		stored := a.windows.byID[sessionID]
		if len(stored) < 2 {
			continue
		}
		repeat := &windowRepeat{group: timeline.NewGroup()}
		prefix := parserkit.ProblemPrefix + "session_windows holds " + strconv.Itoa(len(stored)) +
			" rows of this row's id; its transcript rows are shown once, with "
		for _, window := range stored {
			described := a.describeRow(window)
			notice := a.row(newSession(sessionID), int(window.Rowid),
				cmp.Or(parserkit.ColumnTime(window.Get("started_at")),
					parserkit.ColumnTime(window.Get("created_at"))), timeline.TurnSystem, "")
			notice.Model = described.model
			parent := repeat.group.Parent(&notice)
			suffix := described.words()
			parent.Texts(&notice, timeline.RowText{Text: timeline.Compact(prefix + "this row" + suffix)},
				timeline.RowText{Text: timeline.Compact(prefix + "another of them" + suffix)})
			repeat.parents = append(repeat.parents, parent)
			repeat.notices = append(repeat.notices, notice)
		}
		a.windows.repeats[sessionID] = repeat
	}
}

// words returns " (<words>)", the words a stored window row's `parser:`
// row names it by: its key, its columns, its model provider and its model,
// those that are set; "" when none is.
func (described windowText) words() string {
	var words []string
	for _, pair := range [][2]string{{"key=", described.key}, {"", described.columns},
		{"model_provider=", described.provider}, {"model=", described.model}} {
		if pair[1] != "" {
			words = append(words, pair[0]+pair[1])
		}
	}
	if len(words) == 0 {
		return ""
	}
	return " (" + strings.Join(words, " ") + ")"
}

// windowNotices emits the `parser:` rows of the stored window rows of each
// repeated id, with the working directory of the id's transcript.
func (a *agentDatabase) windowNotices() {
	for _, sessionID := range a.windows.order {
		repeat := a.windows.repeats[sessionID]
		if repeat == nil {
			continue
		}
		for _, notice := range repeat.notices {
			notice.ProjectPath = a.store.cwds[sessionID]
			a.emit(notice)
		}
	}
}

// keyRepeat is a session key session_nodes holds in more than one row,
// which only damage or an edit writes, since the key is the table's key.
// Its rows are a group (timeline.Group): each stored row is a parent, with
// a `parser:` row of its own at its updated_at and the rows of its entry,
// each with its own generation; the key's channel deliveries are members,
// taking the session and project of the stored row the group goes with.
type keyRepeat struct {
	group *timeline.Group
	count int
	// rows are each stored row's entry rows, by the row's place in the
	// nodes read, waiting for the transcripts to give their generations'
	// working directories.
	rows map[int][]keyRow
}

// keyRow is a row of a stored row's entry and its generation.
type keyRow struct {
	sessionID string
	attachedRow
}

// waiting returns the attach function that keeps the entry rows of the
// i-th node read for the group.
func (r *keyRepeat) waiting(i int) func(sessionID string, line int, timestamp, text, model string) {
	return func(sessionID string, line int, timestamp, text, model string) {
		r.rows[i] = append(r.rows[i], keyRow{sessionID, attachedRow{line, timestamp, text, model, nil}})
	}
}

// repeatKeys makes the group of each session key nodes holds more than
// once.
func (a *agentDatabase) repeatKeys(nodes []node) {
	counts := map[string]int{}
	for _, each := range nodes {
		counts[each.sessionKey.String()]++
	}
	for key, count := range counts {
		if count > 1 {
			a.keys[key] = &keyRepeat{group: timeline.NewGroup(), count: count, rows: map[int][]keyRow{}}
		}
	}
}

// keyNotices registers the stored rows of each repeated session key as
// parents of its group, in table order, and emits each one's `parser:` row
// and entry rows, with their generations' working directories.
func (a *agentDatabase) keyNotices(nodes []node) {
	for i, each := range nodes {
		repeat := a.keys[each.sessionKey.String()]
		if repeat == nil {
			continue
		}
		notice := a.contextRow(parserkit.NonEmptyColumn(each.currentSessionID), each.rowid, each.updatedAt,
			timeline.TurnSystem, "")
		parent := repeat.group.Parent(&notice)
		prefix := parserkit.ProblemPrefix + "session_nodes holds " + strconv.Itoa(repeat.count) +
			" rows of this row's session key; its channel deliveries are shown once, with "
		parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		a.emit(notice)
		for _, waiting := range repeat.rows[i] {
			row := a.contextRow(waiting.sessionID, waiting.line, waiting.timestamp, timeline.TurnSystem,
				waiting.text)
			row.Model = waiting.model
			parent.Row(&row, timeline.Always)
			a.emit(row)
		}
	}
}
