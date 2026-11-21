package copilotdesktop

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeat is a key one table of data.db holds in more than one row, which
// only damage or an edit writes: projects.id, workspaces.id and
// workspace_session_aliases.session_id each name one row, the one the rows
// joined through the key are placed by. Its rows are a group
// (timeline.Group): each stored row of the key is a parent, with a
// `parser:` row saying so, and the rows that take their project path
// through the key are members, shown once with the path of the stored row
// the group goes with.
type repeat struct {
	group   *timeline.Group
	table   string
	column  string
	members string
	count   int
}

// repeatsOf returns the repeat of each key of column that more than one of
// rows holds, members naming the rows that take a path through it. A NULL
// key names no row, so it has no repeat.
func repeatsOf(table, column, members string, rows []sqlitefile.Row) map[string]*repeat {
	counts := map[string]int{}
	for _, row := range rows {
		if value := row.Get(column); !value.IsNull() {
			counts[parser.ColumnText(value)]++
		}
	}
	found := map[string]*repeat{}
	for key, count := range counts {
		if count > 1 {
			found[key] = &repeat{group: timeline.NewGroup(), table: table, column: column, members: members,
				count: count}
		}
	}
	return found
}

// start makes row, the row of one stored row of the key, a new parent of
// the group and returns the parent's `parser:` row, a copy of row saying
// that the key repeats. A nil repeat makes no parent and returns nothing.
func (r *repeat) start(row *timeline.Row) []timeline.Row {
	if r == nil {
		return nil
	}
	parent := r.group.Parent(row)
	notice := *row
	r.notice(parent, &notice)
	return []timeline.Row{notice}
}

// notice makes row the parent's `parser:` row, whose text says that the
// table holds the key in more than one row and whether the members are
// shown with this one.
func (r *repeat) notice(parent *timeline.Parent, row *timeline.Row) {
	prefix := parser.ProblemPrefix + r.table + " holds " + strconv.Itoa(r.count) + " rows of this row's " +
		r.column + "; its " + r.members + " are shown once, with "
	parent.Texts(row, timeline.RowText{Text: prefix + "this row"},
		timeline.RowText{Text: prefix + "another of them"})
}

// member makes row a member of the group, taking the project path of the
// stored row the group goes with; a nil repeat leaves the row as it is.
func (r *repeat) member(row *timeline.Row) {
	if r != nil {
		r.group.Member(row, timeline.TakeProject)
	}
}

// links is how the rows of data.db reach a project path: the projects by
// id, each workspace's project and each session's workspace, every key by
// its first stored row, with the repeat of each key that more than one
// stored row holds.
type links struct {
	projects         projectIndex
	workspaceProject map[string]string
	aliases          map[string]string
	workspaceRepeats map[string]*repeat
	aliasRepeats     map[string]*repeat
}

// newLinks indexes the workspaces and the session aliases, each key by its
// first stored row.
func newLinks(projects projectIndex, workspaces, aliases []sqlitefile.Row) links {
	found := links{projects: projects, workspaceProject: map[string]string{}, aliases: map[string]string{},
		workspaceRepeats: repeatsOf("workspaces", "id", "sessions", workspaces),
		aliasRepeats:     repeatsOf("workspace_session_aliases", "session_id", "session's rows", aliases)}
	for _, workspace := range workspaces {
		workspaceID := parser.ColumnText(workspace.Get("id"))
		if _, met := found.workspaceProject[workspaceID]; !met {
			found.workspaceProject[workspaceID] = parser.ColumnText(workspace.Get("project_id"))
		}
	}
	for _, alias := range aliases {
		sessionID := parser.ColumnText(alias.Get("session_id"))
		if _, met := found.aliases[sessionID]; !met {
			found.aliases[sessionID] = parser.ColumnText(alias.Get("workspace_id"))
		}
	}
	return found
}

// workspacePath returns the project path of a workspace id through its
// first stored row and its project's first stored row.
func (l links) workspacePath(workspaceID string) string {
	return l.projects.pathOf(l.workspaceProject[workspaceID])
}

// sessionPlace returns the project path of a session's rows, through its
// alias, the alias's workspace and the workspace's project, and the repeat
// whose group the rows are members of: that of the first link of the
// chain whose key repeats. A row holds one place in one group, so the
// links after that one give their first stored row's path.
func (l links) sessionPlace(sessionID string) (string, *repeat) {
	workspaceID := l.aliases[sessionID]
	path := l.workspacePath(workspaceID)
	if found := l.aliasRepeats[sessionID]; found != nil {
		return path, found
	}
	if found := l.workspaceRepeats[workspaceID]; found != nil {
		return path, found
	}
	return path, l.projects.repeats[l.workspaceProject[workspaceID]]
}

// workspaceRepeat returns the repeat of a stored workspace row's id, nil
// when one row holds it or the id is NULL.
func (l links) workspaceRepeat(workspace sqlitefile.Row) *repeat {
	if workspaceID := workspace.Get("id"); !workspaceID.IsNull() {
		return l.workspaceRepeats[parser.ColumnText(workspaceID)]
	}
	return nil
}

// projectPlace returns the project path of a row naming a project id and
// the repeat of the id, nil when one row holds it.
func (l links) projectPlace(projectID string) (string, *repeat) {
	return l.projects.pathOf(projectID), l.projects.repeats[projectID]
}

// aliasRows emits the `parser:` row of each stored row of a session id the
// alias table repeats, at its created_at, with the session id and the
// project path of the row's workspace: the parents of the id's group.
func (p *Parser) aliasRows(artifact *input.Artifact, aliases []sqlitefile.Row, joined links,
	emit parser.Emit) {
	for _, alias := range aliases {
		sessionID := parser.ColumnText(alias.Get("session_id"))
		found := joined.aliasRepeats[sessionID]
		if found == nil || alias.Get("session_id").IsNull() {
			continue
		}
		row := p.row(artifact, "", parser.ColumnTime(alias.Get("created_at")))
		row.SessionID = sessionID
		row.ProjectPath = joined.workspacePath(parser.ColumnText(alias.Get("workspace_id")))
		found.notice(found.group.Parent(&row), &row)
		emit(row)
	}
}
