package zed

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	// group is the group of an id threads link to that the table repeats,
	// nil for any other id: each stored row of the id is a parent, and each
	// link (link true) a member.
	group *timeline.Group
	link  bool
}

// linkGroup is the first thread link of one archived worktree row, by
// which the rows of an id the link column compares as one are grouped.
type linkGroup struct {
	rowid int64
	found bool
}

// sidebarStore holds the rows of a db.sqlite, each table's in the order
// the parser reads them; a table the database lacks has none.
type sidebarStore struct {
	sidebar   []sqlitefile.Row
	trusted   []sqlitefile.Row
	keyValues []sqlitefile.Row
	archived  []archivedRow
	terminals []sqlitefile.Row
	// repeatedArchived counts the archived worktree rows repeating an
	// earlier row's id, which give their rows unlinked.
	repeatedArchived int
	// byKey holds the sidebar rows by thread key, the first row of a key
	// winning: the stored row a repeated key goes with without a filter.
	byKey map[string]sqlitefile.Row
	// repeats holds the thread keys the table repeats, each a group.
	repeats map[string]*sidebarRepeat
	// repositories indexes the main repository of each worktree path, made
	// on first use.
	repositories *repositoryIndex
	// work counts the entries indexed, which a test measures.
	work parserkit.Work
	// carried cuts the values a join copies onto many rows: an archived
	// worktree's columns and a created worktree's main repository.
	carried *parserkit.Carried
}

// tableReader runs the queries of one database in turn, keeping the first
// error: once one fails the rest return nothing.
type tableReader struct {
	database *sqlitefile.Database
	names    parserkit.Tables
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

// joinArchived joins each worktree to its links, given each worktree's
// first link and the number of worktrees sharing it, and returns the rows
// and the number of worktrees repeating an earlier one's links.
func joinArchived(worktrees []sqlitefile.Row, firsts []linkGroup, counts map[int64]int,
	lookup *sqlitefile.Lookup) ([]archivedRow, int) {
	var joined []archivedRow
	groups, repeats := map[int64]*timeline.Group{}, 0
	for i, worktree := range worktrees {
		first := firsts[i]
		group, shown := groups[first.rowid]
		switch {
		case !first.found:
			joined = append(joined, archivedRow{Row: worktree})
			continue
		case shown:
			repeats++
			joined = append(joined, archivedRow{Row: worktree, group: group})
			continue
		case counts[first.rowid] > 1:
			group = timeline.NewGroup()
			joined = append(joined, archivedRow{Row: worktree, group: group})
		}
		groups[first.rowid] = group
		matches := lookup.Find(worktree, "id")
		slices.SortStableFunc(matches, func(left, right sqlitefile.Row) int {
			return sqlitefile.Compare(left.Get("thread_id"), right.Get("thread_id"))
		})
		for _, link := range matches {
			joined = append(joined, archivedRow{Row: worktree, thread: link.Get("thread_id"), group: group,
				link: group != nil})
		}
	}
	return joined, repeats
}

// asArchived returns worktrees with no linked thread.
func asArchived(worktrees []sqlitefile.Row) []archivedRow {
	rows := make([]archivedRow, len(worktrees))
	for i, worktree := range worktrees {
		rows[i] = archivedRow{Row: worktree}
	}
	return rows
}

// readArchived returns the archived worktrees by id, each joined to the
// threads linked to it (a left join on archived_worktree_id), and the
// number of rows repeating an earlier row's id. Several threads linked to
// one worktree come in the order of their thread ids. The id is the
// table's key, so only damage repeats one: the links of a repeated id
// come once, after its first row, as members of a group whose parents are
// the id's rows, each giving its own row unlinked.
func readArchived(database *sqlitefile.Database, linked bool) ([]archivedRow, int, error) {
	table, err := database.Table("archived_git_worktrees")
	if err != nil {
		return nil, 0, err
	}
	worktrees, err := table.Select().OrderBy("id").All()
	if err != nil || !linked {
		return asArchived(worktrees), 0, err
	}
	links, err := database.Table("thread_archived_worktrees")
	if err != nil {
		return nil, 0, err
	}
	lookup, err := links.Select("thread_id").Lookup("archived_worktree_id")
	if err != nil {
		return nil, 0, err
	}
	firsts, counts := make([]linkGroup, len(worktrees)), map[int64]int{}
	for i, worktree := range worktrees {
		first, found := lookup.First(worktree, "id")
		firsts[i] = linkGroup{rowid: first.Rowid, found: found}
		if found {
			counts[first.Rowid]++
		}
	}
	joined, repeats := joinArchived(worktrees, firsts, counts, lookup)
	return joined, repeats, nil
}

// readSidebar reads every table of a db.sqlite the parser uses. An error
// in any of them stops the whole file, as one query failing did.
func readSidebar(database *sqlitefile.Database) (*sidebarStore, error) {
	names, err := parserkit.TablesOf(database)
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
		store.archived, store.repeatedArchived, reader.err = readArchived(database,
			names.Has("thread_archived_worktrees"))
	}
	store.terminals = reader.rows("sidebar_terminal_threads", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select().OrderBy("created_at")
	})
	if reader.err != nil {
		return nil, reader.err
	}
	for _, row := range store.sidebar {
		if _, held := store.byKey[threadKey(row.Get("thread_id"))]; !held {
			store.byKey[threadKey(row.Get("thread_id"))] = row
		}
	}
	store.repeats = repeatedKeys(store.sidebar)
	return store, nil
}

// repeatedWorktrees returns the problem text of n archived worktree rows
// that repeat an earlier row's id and so give their rows unlinked.
func repeatedWorktrees(n int) string {
	rows := "1 row repeating"
	if n > 1 {
		rows = strconv.Itoa(n) + " rows repeating"
	}
	return "archived_git_worktrees holds " + rows + " an earlier row's id; its linked threads are shown once"
}

// trustedRow returns the `permission setting:` row of one trusted project
// path.
func (p *Parser) trustedRow(artifact *input.Artifact, row sqlitefile.Row) timeline.Row {
	path := parserkit.NonEmptyColumn(row.Get("absolute_path"))
	host := parserkit.NonEmptyColumn(row.Get("host_name"))
	if host != "" && !row.Get("user_name").IsEmpty() {
		host = row.Get("user_name").String() + "@" + host
	}
	if host != "" {
		host = " host=" + host
	}
	text := "permission setting: trusted_worktrees." + path + "=true" + host + " (from " +
		artifact.Rel + " trusted_worktrees, file mtime)"
	return p.stateRow(artifact, text, parserkit.FileMtime(artifact), row.Rowid, "", path)
}

// parseSidebar emits the rows of a db.sqlite: sidebar threads, trusted
// worktrees, drafts and created worktrees, archived worktrees and sidebar
// terminals, then the problems met.
func (p *Parser) parseSidebar(artifact *input.Artifact, database *sqlitefile.Database,
	emit parserkit.Emit) {
	store, err := readSidebar(database)
	if err != nil {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
		return
	}
	store.work = p.Work
	store.carried = parserkit.NewCarried(artifact)
	var problems parserkit.Problems
	for _, row := range store.sidebar {
		p.sidebarRows(artifact, row, store, emit)
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
		p.archivedWorktree(artifact, row, store, emit)
	}
	if store.repeatedArchived > 0 {
		problems.Add(0, repeatedWorktrees(store.repeatedArchived))
	}
	for _, row := range store.terminals {
		text := "sidebar terminal: " + firstColumn(row.Get("custom_title"), row.Get("title")) +
			" cwd=" + cmp.Or(firstColumn(row.Get("working_directory")), "-")
		emit(p.stateRow(artifact, text, parserkit.ColumnTime(row.Get("created_at")), row.Rowid, "",
			firstFolder(row.Get("folder_paths"))))
	}
	store.carried.Report(0, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// sidebarRow returns the row of an external agent's thread, which has no
// transcript in threads.db, or of a native thread's sidebar state, and
// whether there is one: a draft (no session_id) has no row of its own,
// only its draft prompt.
func (p *Parser) sidebarRow(artifact *input.Artifact, row sqlitefile.Row) (timeline.Row, bool) {
	title := firstColumn(row.Get("title_override"), row.Get("title"))
	project := firstFolder(row.Get("folder_paths"))
	sessionID := parserkit.NonEmptyColumn(row.Get("session_id"))
	switch {
	case !row.Get("agent_id").IsEmpty():
		archived := ""
		if !row.Get("archived").IsEmpty() {
			archived = " archived"
		}
		text := "external agent thread (no transcript in threads.db): agent=" +
			row.Get("agent_id").String() + " title=" + title + archived
		timestamp := parserkit.ColumnTime(row.Get("updated_at"))
		if interacted := row.Get("interacted_at"); !interacted.IsEmpty() {
			timestamp = parserkit.ColumnTime(interacted)
		}
		return p.stateRow(artifact, text, timestamp, row.Rowid, sessionID, project), true
	case sessionID != "":
		text := fmt.Sprintf("sidebar thread: title=%s archived=%t interacted_at=%s main_worktree=%s",
			title, !row.Get("archived").IsEmpty(), cmp.Or(parserkit.ColumnTime(row.Get("interacted_at")), "-"),
			cmp.Or(firstFolder(row.Get("main_worktree_paths")), "-"))
		return p.stateRow(artifact, text, parserkit.ColumnTime(row.Get("updated_at")), row.Rowid,
			sessionID, project), true
	}
	return timeline.Row{}, false
}
