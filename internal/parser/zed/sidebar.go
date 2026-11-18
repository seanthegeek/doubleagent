package zed

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The two scoped_kv_store namespaces read.
const (
	draftsNamespace    = "agent_draft_prompts"
	worktreesNamespace = "created_git_worktrees"
)

// archivedRow is one row of archived_git_worktrees with the thread_id of
// one thread linked to it, NULL when none is.
type archivedRow struct {
	sqlitefile.Row
	thread sqlitefile.Value
}

// sidebarStore holds the rows of a db.sqlite, each table's in the order
// the parser reads them; a table the database lacks has none.
type sidebarStore struct {
	sidebar   []sqlitefile.Row
	trusted   []sqlitefile.Row
	keyValues []sqlitefile.Row
	archived  []archivedRow
	terminals []sqlitefile.Row
	// byKey holds the sidebar rows by thread key, the last row of a key
	// winning.
	byKey map[string]sqlitefile.Row
}

// tableReader runs the queries of one database in turn, keeping the first
// error: once one fails the rest return nothing.
type tableReader struct {
	database *sqlitefile.Database
	names    parser.Tables
	err      error
}

// rows returns the rows query gives over the table called name, none when
// the database has no such table or an earlier query failed.
func (r *tableReader) rows(name string, query func(*sqlitefile.Table) sqlitefile.Query) []sqlitefile.Row {
	if r.err != nil || !r.names.Has(name) {
		return nil
	}
	table, err := r.database.Table(name)
	if err != nil {
		r.err = err
		return nil
	}
	rows, err := query(table).All()
	r.err = err
	return rows
}

// readSidebar reads every table of a db.sqlite the parser uses. An error
// in any of them stops the whole file, as one query failing did.
func readSidebar(database *sqlitefile.Database) (*sidebarStore, error) {
	names, err := parser.TablesOf(database)
	if err != nil {
		return nil, err
	}
	reader := &tableReader{database: database, names: names}
	store := &sidebarStore{byKey: map[string]sqlitefile.Row{}}
	store.sidebar = reader.rows("sidebar_threads", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select().OrderBy("updated_at")
	})
	store.trusted = reader.rows("trusted_worktrees", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select()
	})
	store.keyValues = reader.rows("scoped_kv_store", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("namespace", "key", "value").Where(func(row sqlitefile.Row) bool {
			return row.Equals("namespace", sqlitefile.TextValue(draftsNamespace)) ||
				row.Equals("namespace", sqlitefile.TextValue(worktreesNamespace))
		})
	})
	if reader.err == nil && names.Has("archived_git_worktrees") {
		store.archived, reader.err = readArchived(database, names.Has("thread_archived_worktrees"))
	}
	store.terminals = reader.rows("sidebar_terminal_threads", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select().OrderBy("created_at")
	})
	if reader.err != nil {
		return nil, reader.err
	}
	for _, row := range store.sidebar {
		store.byKey[threadKey(row.Get("thread_id"))] = row
	}
	return store, nil
}

// readArchived returns the archived worktrees by id, each joined to the
// threads linked to it (a left join on archived_worktree_id). Several
// threads linked to one worktree come in the order of their thread ids.
func readArchived(database *sqlitefile.Database, linked bool) ([]archivedRow, error) {
	table, err := database.Table("archived_git_worktrees")
	if err != nil {
		return nil, err
	}
	worktrees, err := table.Select().OrderBy("id").All()
	if err != nil || !linked {
		return asArchived(worktrees), err
	}
	links, err := database.Table("thread_archived_worktrees")
	if err != nil {
		return nil, err
	}
	lookup, err := links.Select("thread_id").Lookup("archived_worktree_id")
	if err != nil {
		return nil, err
	}
	var joined []archivedRow
	for _, worktree := range worktrees {
		matches := lookup.Find(worktree, "id")
		slices.SortStableFunc(matches, func(left, right sqlitefile.Row) int {
			return sqlitefile.Compare(left.Get("thread_id"), right.Get("thread_id"))
		})
		if len(matches) == 0 {
			joined = append(joined, archivedRow{Row: worktree})
		}
		for _, link := range matches {
			joined = append(joined, archivedRow{Row: worktree, thread: link.Get("thread_id")})
		}
	}
	return joined, nil
}

// asArchived returns worktrees with no linked thread.
func asArchived(worktrees []sqlitefile.Row) []archivedRow {
	rows := make([]archivedRow, len(worktrees))
	for i, worktree := range worktrees {
		rows[i] = archivedRow{Row: worktree}
	}
	return rows
}

// parseSidebar emits the rows of a db.sqlite: sidebar threads, trusted
// worktrees, drafts and created worktrees, archived worktrees and sidebar
// terminals, then the problems met.
func (p *Parser) parseSidebar(artifact *input.Artifact, database *sqlitefile.Database,
	emit parser.Emit) {
	store, err := readSidebar(database)
	if err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	var problems parser.Problems
	for _, row := range store.sidebar {
		p.sidebarRow(artifact, row, emit)
	}
	for _, row := range store.trusted {
		emit(p.trustedRow(artifact, row))
	}
	for _, row := range store.keyValues {
		if row.Equals("namespace", sqlitefile.TextValue(draftsNamespace)) {
			p.draft(artifact, row, store, &problems, emit)
		} else {
			p.createdWorktree(artifact, row, store, &problems, emit)
		}
	}
	for _, row := range store.archived {
		emit(p.archivedWorktree(artifact, row, store))
	}
	for _, row := range store.terminals {
		text := "sidebar terminal: " + firstColumn(row.Get("custom_title"), row.Get("title")) +
			" cwd=" + cmp.Or(firstColumn(row.Get("working_directory")), "-")
		emit(p.stateRow(artifact, text, parser.ColumnTime(row.Get("created_at")), row.Rowid, "",
			firstFolder(row.Get("folder_paths"))))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// sidebarRow emits the row of an external agent's thread, which has no
// transcript in threads.db, or of a native thread's sidebar state. A draft
// (no session_id) has no row of its own, only its draft prompt.
func (p *Parser) sidebarRow(artifact *input.Artifact, row sqlitefile.Row, emit parser.Emit) {
	title := firstColumn(row.Get("title_override"), row.Get("title"))
	project := firstFolder(row.Get("folder_paths"))
	sessionID := parser.NonEmptyColumn(row.Get("session_id"))
	switch {
	case !row.Get("agent_id").IsEmpty():
		archived := ""
		if !row.Get("archived").IsEmpty() {
			archived = " archived"
		}
		text := "external agent thread (no transcript in threads.db): agent=" +
			row.Get("agent_id").String() + " title=" + title + archived
		timestamp := parser.ColumnTime(row.Get("updated_at"))
		if interacted := row.Get("interacted_at"); !interacted.IsEmpty() {
			timestamp = parser.ColumnTime(interacted)
		}
		emit(p.stateRow(artifact, text, timestamp, row.Rowid, sessionID, project))
	case sessionID != "":
		text := fmt.Sprintf("sidebar thread: title=%s archived=%t interacted_at=%s main_worktree=%s",
			title, !row.Get("archived").IsEmpty(), cmp.Or(parser.ColumnTime(row.Get("interacted_at")), "-"),
			cmp.Or(firstFolder(row.Get("main_worktree_paths")), "-"))
		emit(p.stateRow(artifact, text, parser.ColumnTime(row.Get("updated_at")), row.Rowid,
			sessionID, project))
	}
}

// trustedRow returns the `permission setting:` row of one trusted project
// path.
func (p *Parser) trustedRow(artifact *input.Artifact, row sqlitefile.Row) timeline.Row {
	path := parser.NonEmptyColumn(row.Get("absolute_path"))
	host := parser.NonEmptyColumn(row.Get("host_name"))
	if host != "" && !row.Get("user_name").IsEmpty() {
		host = row.Get("user_name").String() + "@" + host
	}
	if host != "" {
		host = " host=" + host
	}
	text := "permission setting: trusted_worktrees." + path + "=true" + host + " (from " +
		artifact.Rel + " trusted_worktrees, file mtime)"
	return p.stateRow(artifact, text, parser.FileMtime(artifact), row.Rowid, "", path)
}
