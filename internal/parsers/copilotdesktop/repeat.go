package copilotdesktop

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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

// keyed is the stored rows of one table in reading order, with the
// collation its key column declares, by which two values of the key are
// one key exactly when SQL's = finds them equal (parserkit.CollatedKey).
type keyed struct {
	rows      []sqlitefile.Row
	column    string
	collation string
}

// key returns the key a value has as a value of the table's key column,
// "" for NULL, which names no row.
func (k keyed) key(value sqlitefile.Value) string {
	return parserkit.CollatedKey(value, k.collation)
}

// repeatsOf returns the repeat of each key of the table that more than
// one of its rows holds, members naming the rows that take a path through
// it. A NULL key names no row, so it has no repeat.
func repeatsOf(table, members string, from keyed) map[string]*repeat {
	counts := map[string]int{}
	for _, row := range from.rows {
		if key := from.key(row.Get(from.column)); key != "" {
			counts[key]++
		}
	}
	found := map[string]*repeat{}
	for key, count := range counts {
		if count > 1 {
			found[key] = &repeat{group: timeline.NewGroup(), table: table, column: from.column, members: members,
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
	prefix := parserkit.ProblemPrefix + r.table + " holds " + strconv.Itoa(r.count) + " rows of this row's " +
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
// stored row holds. A value naming a key is keyed as the key's own column
// compares it, and a NULL one names nothing.
type links struct {
	projects         projectIndex
	workspaces       keyed
	aliases          keyed
	workspaceProject map[string]sqlitefile.Value
	aliasWorkspace   map[string]sqlitefile.Value
	workspaceRepeats map[string]*repeat
	aliasRepeats     map[string]*repeat
}

// newLinks indexes the workspaces and the session aliases, each key by its
// first stored row.
func newLinks(projects projectIndex, workspaces, aliases keyed) links {
	found := links{projects: projects, workspaces: workspaces, aliases: aliases,
		workspaceProject: firstValues(workspaces, "project_id"), aliasWorkspace: firstValues(aliases, "workspace_id"),
		workspaceRepeats: repeatsOf("workspaces", "sessions", workspaces),
		aliasRepeats:     repeatsOf("workspace_session_aliases", "session's rows", aliases)}
	return found
}

// firstValues returns, by key, the value of column in the first stored row
// of each key of the table; a row whose key is NULL is left out.
func firstValues(from keyed, column string) map[string]sqlitefile.Value {
	values := map[string]sqlitefile.Value{}
	for _, row := range from.rows {
		key := from.key(row.Get(from.column))
		if _, met := values[key]; !met && key != "" {
			values[key] = row.Get(column)
		}
	}
	return values
}

// workspacePath returns the project path of a workspace id through its
// first stored row and its project's first stored row.
func (l links) workspacePath(workspaceID sqlitefile.Value) string {
	key := l.workspaces.key(workspaceID)
	if key == "" {
		return ""
	}
	return l.projects.pathOf(l.workspaceProject[key])
}

// sessionPlace returns the project path of a session's rows, through its
// alias, the alias's workspace and the workspace's project, and the repeat
// whose group the rows are members of: that of the first link of the
// chain whose key repeats. A row holds one place in one group, so the
// links after that one give their first stored row's path.
func (l links) sessionPlace(sessionID sqlitefile.Value) (string, *repeat) {
	key := l.aliases.key(sessionID)
	if key == "" {
		return "", nil
	}
	workspaceID, ok := l.aliasWorkspace[key]
	if !ok {
		return "", nil
	}
	path := l.workspacePath(workspaceID)
	if found := l.aliasRepeats[key]; found != nil {
		return path, found
	}
	if found := l.workspaceRepeats[l.workspaces.key(workspaceID)]; found != nil {
		return path, found
	}
	_, project := l.projectPlace(l.workspaceProject[l.workspaces.key(workspaceID)])
	return path, project
}

// workspaceRepeat returns the repeat of a stored workspace row's id, nil
// when one row holds it or the id is NULL.
func (l links) workspaceRepeat(workspace sqlitefile.Row) *repeat {
	return l.workspaceRepeats[l.workspaces.key(workspace.Get("id"))]
}

// projectPlace returns the project path of a row naming a project id and
// the repeat of the id, nil when one row holds it.
func (l links) projectPlace(projectID sqlitefile.Value) (string, *repeat) {
	return l.projects.pathOf(projectID), l.projects.repeats[l.projects.stored.key(projectID)]
}

// aliasRows emits the `parser:` row of each stored row of a session id the
// alias table repeats, at its created_at, with the session id and the
// project path of the row's workspace: the parents of the id's group.
func (p *Parser) aliasRows(artifact *input.Artifact, joined links, emit parserkit.Emit) {
	for _, alias := range joined.aliases.rows {
		found := joined.aliasRepeats[joined.aliases.key(alias.Get("session_id"))]
		if found == nil {
			continue
		}
		row := p.row(artifact, "", parserkit.ColumnTime(alias.Get("created_at")))
		row.SessionID = parserkit.ColumnText(alias.Get("session_id"))
		row.ProjectPath = joined.workspacePath(alias.Get("workspace_id"))
		found.notice(found.group.Parent(&row), &row)
		emit(row)
	}
}
