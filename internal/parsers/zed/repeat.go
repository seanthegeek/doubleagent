package zed

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sidebarRepeat is a thread key sidebar_threads holds in more than one
// row, which only damage or an edit writes, since thread_id is the table's
// primary key. Its rows are a group (timeline.Group): each stored row of
// the key is a parent, with a `parser:` row of its own saying so, at its
// updated_at and with its session and project, beside its own sidebar row;
// the key's drafts and the rows of the archived worktrees linked to it are
// members, shown once with the stored row the group goes with.
type sidebarRepeat struct {
	group *timeline.Group
	count int
}

// repeatedKeys returns the thread keys of sidebar held in more than one
// row, each with a new group. An empty key names no thread and is never
// grouped.
func repeatedKeys(sidebar []sqlitefile.Row) map[string]*sidebarRepeat {
	counts := map[string]int{}
	for _, row := range sidebar {
		counts[threadKey(row.Get("thread_id"))]++
	}
	repeats := map[string]*sidebarRepeat{}
	for key, count := range counts {
		if key != "" && count > 1 {
			repeats[key] = &sidebarRepeat{group: timeline.NewGroup(), count: count}
		}
	}
	return repeats
}

// sidebarRows emits the rows of one sidebar_threads row: its own row
// (sidebarRow), if it has one, and for a repeated thread key first the
// `parser:` row that makes the stored row a parent of the key's group.
func (p *Parser) sidebarRows(artifact *input.Artifact, row sqlitefile.Row, store *sidebarStore,
	emit parserkit.Emit) {
	own, shown := p.sidebarRow(artifact, row)
	if repeat := store.repeats[threadKey(row.Get("thread_id"))]; repeat != nil {
		problem := p.stateRow(artifact, "", parserkit.ColumnTime(row.Get("updated_at")), row.Rowid,
			parserkit.NonEmptyColumn(row.Get("session_id")), firstFolder(row.Get("folder_paths")))
		parent := repeat.group.Parent(&problem)
		prefix := parserkit.ProblemPrefix + "sidebar_threads holds " + strconv.Itoa(repeat.count) +
			" rows of this row's thread_id; its drafts and archived worktree links are shown once, with "
		parent.Texts(&problem, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		emit(problem)
		if shown {
			parent.Row(&own, timeline.Always)
		}
	}
	if shown {
		emit(own)
	}
}

// member makes row a member of the group of key, when the sidebar repeats
// key, taking what takes names from the stored row the group goes with.
func (s *sidebarStore) member(key string, row *timeline.Row, takes timeline.Takes) {
	if repeat := s.repeats[key]; repeat != nil {
		repeat.group.Member(row, takes)
	}
}
