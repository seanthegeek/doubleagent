package openclaw

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
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
		prefix := parser.ProblemPrefix + "session_windows holds " + strconv.Itoa(len(stored)) +
			" rows of this row's id; its transcript rows are shown once, with "
		for _, window := range stored {
			described := a.describeRow(window)
			notice := a.row(newSession(sessionID), int(window.Rowid),
				timeutil.FirstNonEmpty(parser.ColumnTime(window.Get("started_at")),
					parser.ColumnTime(window.Get("created_at"))), timeline.TurnSystem, "")
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
