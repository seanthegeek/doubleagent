package opencode

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeat is a message id a table holds in more than one row, which only
// damage or an edit writes, since the id is unique in OpenCode's tables.
// Its rows are a group (timeline.Group): each stored row of the id is a
// parent, with a `parser:` row of its own saying so, at its time and with
// its session, project and model, beside its own model, agent and error
// rows; the rows of the id's parts are members, gathered once and shown
// with the parent the group goes with.
type repeat struct {
	group *timeline.Group
	table string
	count int
}

// newRepeat returns the group of a message id table holds in count rows.
func newRepeat(table string, count int) *repeat {
	return &repeat{group: timeline.NewGroup(), table: table, count: count}
}

// link places the rows of one stored row of a repeated id in the id's
// group: as the row's own, as members, or as rows the id's parts replace
// (shown only when the group goes with another row). A nil link places
// nothing.
type link struct {
	repeat *repeat
	parent *timeline.Parent
	// members says the rows being added are the id's parts' rows;
	// replaced that they are the row's own content, which those replace.
	members, replaced bool
}

// start makes row, a row of the stored row's session, project, model and
// time, the `parser:` row of a new parent of the repeated id and returns
// the link of the parent's rows.
func (r *repeat) start(row *timeline.Row) *link {
	row.TurnType = timeline.TurnSystem
	parent := r.group.Parent(row)
	prefix := parserkit.ProblemPrefix + r.table + " holds " + strconv.Itoa(r.count) +
		" rows of this row's id; its parts are shown once, with "
	parent.Texts(row, timeline.RowText{Text: prefix + "this row"},
		timeline.RowText{Text: prefix + "another of them"})
	return &link{repeat: r, parent: parent}
}

// place gives row its place in the group; inherited says its time is the
// stored row's, as it has none of its own.
func (l *link) place(row *timeline.Row, inherited bool) {
	switch {
	case l == nil:
	case l.members:
		takes := timeline.TakeSession | timeline.TakeProject | timeline.TakeModel
		if inherited {
			takes |= timeline.TakeTime
		}
		l.repeat.group.Member(row, takes)
	case l.replaced:
		l.parent.Row(row, timeline.WhenNotChosen)
	default:
		l.parent.Row(row, timeline.Always)
	}
}

// problem makes row, a `parser:` row of one of the id's parts, a member
// taking the session and project: it has no time or model of its own.
func (l *link) problem(row *timeline.Row) {
	if l != nil {
		l.repeat.group.Member(row, timeline.TakeSession|timeline.TakeProject)
	}
}

// sessionRepeat is a session id one session table holds in more than one
// row, which only damage or an edit writes, since the id is the table's
// key. Its rows are a group (timeline.Group): each stored row of the id is
// a parent, giving its own session-start, archive and permission rows and
// a `parser:` row saying so, with its own project; the session's
// messages, shares and problems are members, shown once with the project
// of the row the group goes with.
type sessionRepeat struct {
	group *timeline.Group
	table string
	count int
}

// newSessionRepeat returns the group of a session id table holds in count
// rows.
func newSessionRepeat(table string, count int) *sessionRepeat {
	return &sessionRepeat{group: timeline.NewGroup(), table: table, count: count}
}

// start makes start, the "session start:" row of one stored row of the
// id, a new parent of the group, and notice, a copy of it, that parent's
// `parser:` row, and returns the parent.
func (r *sessionRepeat) start(start, notice *timeline.Row) *timeline.Parent {
	parent := r.group.Parent(start)
	prefix := parserkit.ProblemPrefix + r.table + " holds " + strconv.Itoa(r.count) +
		" rows of this row's id; its messages and shares are shown once, with "
	parent.Texts(notice, timeline.RowText{Text: prefix + "this row"},
		timeline.RowText{Text: prefix + "another of them"})
	return parent
}

// projectRepeat is a project id the project table holds in more than one
// row, which only damage or an edit writes, since the id is the table's
// key. Its rows are a group (timeline.Group): each stored row of the id
// is a parent, with a `parser:` row of its own at its time and with its
// worktree; the rows whose project path is the project's worktree (the
// rows of a session naming no directory of its own and the saved grants
// of the project) are members, shown once with the worktree of the row the
// group goes with.
type projectRepeat struct {
	group *timeline.Group
	count int
}

// newProjectRepeat returns the group of a project id the table holds in
// count rows.
func newProjectRepeat(count int) *projectRepeat {
	return &projectRepeat{group: timeline.NewGroup(), count: count}
}

// member makes row a member of the group, taking takes; a nil repeat
// leaves the row as it is.
func (r *projectRepeat) member(row *timeline.Row, takes timeline.Takes) {
	if r != nil && takes != 0 {
		r.group.Member(row, takes)
	}
}

// projectNotices emits the `parser:` row of each stored row of a repeated
// project id, at its time_created and with its worktree: the parents of
// the id's group, registered before any of its members.
func (s *store) projectNotices() {
	for _, row := range s.repeatedProjects {
		found := s.projectRepeats[s.projectKey(row.Get("id"))]
		worktree := parserkit.NonEmptyColumn(row.Get("worktree"))
		if worktree == "/" {
			worktree = ""
		}
		notice := s.systemAt(newScope("", worktree), row.Rowid, row.Get("time_created"), "")
		prefix := parserkit.ProblemPrefix + "project holds " + strconv.Itoa(found.count) +
			" rows of this row's id; its sessions and saved grants are shown once, with "
		found.group.Parent(&notice).Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		s.emit(notice)
	}
}
