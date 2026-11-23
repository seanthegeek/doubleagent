package cline

import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// searchName is the search index's file name, as its problem rows name it.
const searchName = "session-search.db"

// searchIndex is what db/session-search.db holds: the indexed_sessions
// rows and the session_search documents, each in rowid order, and the
// problem of a documents table that could not be read.
type searchIndex struct {
	indexed   []sqlitefile.Row
	documents []sqlitefile.Row
	problem   string
}

// readSearch reads the search index, or returns an error whose text is the
// row that replaces it. The FTS5 table session_search is read from its
// content table; one that cannot be read leaves indexed_sessions read alone.
func readSearch(path string) (searchIndex, error) {
	database, err := openDatabase(path, searchName)
	if err != nil {
		return searchIndex{}, err
	}
	defer database.Close()
	names, err := parserkit.TablesOf(database)
	if err != nil {
		return searchIndex{}, unreadable(searchName, err)
	}
	var index searchIndex
	if names.Has("indexed_sessions") {
		if index.indexed, err = selectAll(database, "indexed_sessions"); err != nil {
			return searchIndex{}, unreadable(searchName, err)
		}
	}
	if names.Has("session_search") {
		index.documents, err = selectAll(database, "session_search", "session_id", "document_id",
			"ordinal", "role", "started_at", "workspace_root", "title", "content")
		switch {
		case errors.Is(err, sqlitefile.ErrMalformed):
			return searchIndex{}, unreadable(searchName, err)
		case err != nil:
			index.problem = "session_search unreadable (" + err.Error() + "), indexed_sessions read alone"
		}
	}
	return index, nil
}

// selectAll returns the named columns, or every column, of a table's rows
// in rowid order.
func selectAll(database *sqlitefile.Database, name string, columns ...string) ([]sqlitefile.Row, error) {
	table, err := database.Table(name)
	if err != nil {
		return nil, err
	}
	return table.Select(columns...).OrderBy("rowid").All()
}

// searchDatabase emits, per session of the search index, its index row
// and a row per message document.
func (p *Parser) searchDatabase(artifact *input.Artifact, emit parserkit.Emit) {
	index, err := readSearch(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, err.Error()))
		return
	}
	if index.problem != "" {
		emit(p.ProblemRow(artifact, index.problem))
	}
	var order []string
	bySession := map[string][]sqlitefile.Row{}
	manifests := map[parserkit.EntryKey]manifestProject{}
	add := func(sessionID string) {
		if _, ok := bySession[sessionID]; !ok {
			order = append(order, sessionID)
			bySession[sessionID] = nil
		}
	}
	meta := map[string]sqlitefile.Row{}
	for _, indexed := range index.indexed {
		sessionID := parserkit.NonEmptyColumn(indexed.Get("session_id"))
		add(sessionID)
		meta[sessionID] = indexed
	}
	for _, document := range index.documents {
		sessionID := parserkit.NonEmptyColumn(document.Get("session_id"))
		add(sessionID)
		bySession[sessionID] = append(bySession[sessionID], document)
	}
	for _, sessionID := range order {
		p.searchSession(artifact, sessionID, meta[sessionID], bySession[sessionID], manifests, emit)
	}
}

// searchSession emits the index row of one session, then a row per
// message document in ordinal order.
func (p *Parser) searchSession(artifact *input.Artifact, sessionID string, indexed sqlitefile.Row,
	documents []sqlitefile.Row, manifests map[parserkit.EntryKey]manifestProject, emit parserkit.Emit) {
	head, hasHead, messages := splitHead(documents)
	first := head
	if !hasHead && len(messages) > 0 {
		first = messages[0]
	}
	workspace := parserkit.NonEmptyColumn(first.Get("workspace_root"))
	project := p.searchProject(artifact, sessionID, workspace, manifests)
	row := p.BaseRow(artifact)
	row.TimestampUTC = parserkit.ColumnTime(first.Get("started_at"))
	row.SessionID, row.ProjectPath = sessionID, project
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact("session search index: " + searchDetail(indexed, head, workspace))
	emit(row)
	for _, document := range messages {
		row := p.BaseRow(artifact)
		row.SourceLine = ordinal(document.Get("ordinal")) + 1
		row.SessionID, row.ProjectPath = sessionID, project
		row.TurnType = timeline.TurnSystem
		if role := document.Get("role"); role.Type == sqlitefile.Text &&
			(role.Text() == timeline.TurnUser || role.Text() == timeline.TurnAssistant) {
			row.TurnType = role.Text()
		}
		if content := document.Get("content"); !content.IsNull() {
			row.Text = timeline.Compact(content.String())
		}
		emit(row)
	}
}

// splitHead returns a session's first "session" document, whether there is
// one, and its other documents in ordinal order.
func splitHead(documents []sqlitefile.Row) (sqlitefile.Row, bool, []sqlitefile.Row) {
	var head sqlitefile.Row
	var messages []sqlitefile.Row
	hasHead := false
	for _, document := range documents {
		if role := document.Get("role"); role.Type == sqlitefile.Text && role.Text() == "session" {
			if !hasHead {
				head, hasHead = document, true
			}
			continue
		}
		messages = append(messages, document)
	}
	slices.SortStableFunc(messages, func(left, right sqlitefile.Row) int {
		return cmp.Compare(ordinal(left.Get("ordinal")), ordinal(right.Get("ordinal")))
	})
	return head, hasHead, messages
}

// manifestProject is what a session manifest gives a search session: its
// project, when the manifest is a JSON object.
type manifestProject struct {
	project string
	object  bool
}

// searchProject returns a search session's project: its manifest's when
// the manifest is there, else the documents' workspace root. Each manifest
// is read once per index, in manifests by the entry its path opens, since
// the ids of many sessions can be spellings that open one file.
func (p *Parser) searchProject(artifact *input.Artifact, sessionID, workspace string,
	manifests map[parserkit.EntryKey]manifestProject) string {
	name, named := parserkit.Component(sessionID)
	if !named {
		return workspace
	}
	manifestPath, found := parserkit.Above(artifact, 1, "sessions", name, name+".json")
	if !found {
		return workspace
	}
	key := parserkit.KeyOf(manifestPath)
	read, seen := manifests[key]
	if !seen {
		p.Work.Add("search manifests read", 1)
		var ignored parserkit.Problems
		// The manifest is parsed as its own artifact, whose parse reports
		// its repeated member names.
		if manifest, _ := parserkit.LoadJSON(nil, manifestPath, &ignored); manifest.IsObject() {
			read = manifestProject{object: true,
				project: parserkit.Text(parserkit.FirstNonEmpty(manifest.Get("cwd"), manifest.Get("workspace_root")))}
		}
		manifests[key] = read
	}
	if !read.object {
		return workspace
	}
	return read.project
}

// searchDetail returns the "key=value" fields of a search session's index
// row: title, workspace root, prompt, document count and indexing time.
func searchDetail(indexed, head sqlitefile.Row, workspace string) string {
	title := indexed.Get("title")
	if title.IsEmpty() {
		title = head.Get("title")
	}
	fields := []struct {
		key   string
		value sqlitefile.Value
	}{
		{"title", title},
		{"workspace_root", sqlitefile.TextValue(workspace)},
		{"prompt", head.Get("content")},
		{"document_count", indexed.Get("document_count")},
		{"indexed_at", indexed.Get("indexed_at")},
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		empty := field.value.Type == sqlitefile.Text && len(field.value.Bytes) == 0
		if !field.value.IsNull() && !empty {
			parts = append(parts, field.key+"="+field.value.String())
		}
	}
	return strings.Join(parts, " ")
}
