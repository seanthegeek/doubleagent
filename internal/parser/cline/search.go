package cline

import (
	"errors"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
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

// readSearch reads the search index, or returns the message of the row that
// replaces it. The FTS5 table session_search is read from its content
// table; one that cannot be read leaves indexed_sessions read alone.
func readSearch(path string) (searchIndex, string) {
	database, message := openDatabase(path, searchName)
	if message != "" {
		return searchIndex{}, message
	}
	defer database.Close()
	names, err := database.TableNames()
	if err != nil {
		return searchIndex{}, unreadable(searchName, err)
	}
	var index searchIndex
	if names["indexed_sessions"] {
		if index.indexed, err = selectAll(database, "indexed_sessions"); err != nil {
			return searchIndex{}, unreadable(searchName, err)
		}
	}
	if names["session_search"] {
		index.documents, err = selectAll(database, "session_search", "session_id", "document_id",
			"ordinal", "role", "started_at", "workspace_root", "title", "content")
		switch {
		case errors.Is(err, sqlitefile.ErrMalformed):
			return searchIndex{}, unreadable(searchName, err)
		case err != nil:
			index.problem = "session_search unreadable (" + err.Error() + "), indexed_sessions read alone"
		}
	}
	return index, ""
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
func (p *Parser) searchDatabase(artifact *input.Artifact, emit parser.Emit) {
	index, message := readSearch(artifact.DiskPath)
	if message != "" {
		emit(p.ProblemRow(artifact, message))
		return
	}
	if index.problem != "" {
		emit(p.ProblemRow(artifact, index.problem))
	}
	var order []string
	bySession := map[string][]sqlitefile.Row{}
	add := func(sessionID string) {
		if _, ok := bySession[sessionID]; !ok {
			order = append(order, sessionID)
			bySession[sessionID] = nil
		}
	}
	meta := map[string]sqlitefile.Row{}
	for _, indexed := range index.indexed {
		sessionID := columnStr(indexed.Get("session_id"))
		add(sessionID)
		meta[sessionID] = indexed
	}
	for _, document := range index.documents {
		sessionID := columnStr(document.Get("session_id"))
		add(sessionID)
		bySession[sessionID] = append(bySession[sessionID], document)
	}
	for _, sessionID := range order {
		p.searchSession(artifact, sessionID, meta[sessionID], bySession[sessionID], emit)
	}
}

// columnStr returns a column as Python's str(value or "") gives it.
func columnStr(value sqlitefile.Value) string {
	if !value.Truthy() {
		return ""
	}
	return value.Str()
}

// orColumn returns the first value Python counts as true, else the last.
func orColumn(values ...sqlitefile.Value) sqlitefile.Value {
	for _, value := range values[:len(values)-1] {
		if value.Truthy() {
			return value
		}
	}
	return values[len(values)-1]
}

// searchSession emits the index row of one session, then a row per
// message document in ordinal order.
func (p *Parser) searchSession(artifact *input.Artifact, sessionID string, indexed sqlitefile.Row,
	documents []sqlitefile.Row, emit parser.Emit) {
	head, hasHead, messages := splitHead(documents)
	first := head
	if !hasHead && len(messages) > 0 {
		first = messages[0]
	}
	workspace := columnStr(first.Get("workspace_root"))
	project := p.searchProject(artifact, sessionID, workspace)
	row := p.BaseRow(artifact)
	row.TimestampUTC = parser.ColumnTime(first.Get("started_at"))
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
			row.Text = timeline.Compact(content.Str())
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
		leftOrdinal, rightOrdinal := ordinal(left.Get("ordinal")), ordinal(right.Get("ordinal"))
		switch {
		case leftOrdinal < rightOrdinal:
			return -1
		case leftOrdinal > rightOrdinal:
			return 1
		}
		return 0
	})
	return head, hasHead, messages
}

// searchProject returns a search session's project: its manifest's when
// the manifest is there, else the documents' workspace root.
func (p *Parser) searchProject(artifact *input.Artifact, sessionID, workspace string) string {
	if sessionID == "" {
		return workspace
	}
	var ignored parser.Problems
	manifest, _ := parser.LoadJSON(artifact.Home.PathOf(
		sessionsDir(artifact.Rel)+"/"+sessionID+"/"+sessionID+".json"), &ignored)
	if !manifest.IsObject() {
		return workspace
	}
	return parser.Str(clinelegacy.FirstTruthy(manifest.Get("cwd"), manifest.Get("workspace_root")))
}

// searchDetail returns the "key=value" fields of a search session's index
// row: title, workspace root, prompt, document count and indexing time.
func searchDetail(indexed, head sqlitefile.Row, workspace string) string {
	fields := []struct {
		key   string
		value sqlitefile.Value
	}{
		{"title", orColumn(indexed.Get("title"), head.Get("title"))},
		{"workspace_root", sqlitefile.TextValue(workspace)},
		{"prompt", head.Get("content")},
		{"document_count", indexed.Get("document_count")},
		{"indexed_at", indexed.Get("indexed_at")},
	}
	detail := ""
	for _, field := range fields {
		empty := field.value.Type == sqlitefile.Text && len(field.value.Bytes) == 0
		if field.value.IsNull() || empty {
			continue
		}
		if detail != "" {
			detail += " "
		}
		detail += field.key + "=" + field.value.Str()
	}
	return detail
}
