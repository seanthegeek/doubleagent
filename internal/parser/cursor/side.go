package cursor

import (
	"math"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// searchColumns are the conversations columns of conversation-search.db
// read.
func searchColumns() []string {
	return []string{"fts_rowid", "id", "title", "source", "is_archived", "updated_at"}
}

// trackingTable is one table of ai-code-tracking.db and the columns read.
type trackingTable struct {
	name    string
	columns []string
}

// trackingTables are the tables of ai-code-tracking.db read, in order.
func trackingTables() []trackingTable {
	return []trackingTable{
		{"ai_code_hashes", []string{"fileName", "source", "model", "requestId", "conversationId", "timestamp"}},
		{"conversation_summaries", []string{"conversationId", "title", "tldr", "model", "updatedAt"}},
		{"ai_deleted_files", []string{"gitPath", "composerId", "conversationId", "model", "deletedAt"}},
		{"scored_commits", []string{"commitHash", "branchName", "linesAdded", "linesDeleted", "scoredAt"}},
	}
}

// columnField returns a column value as row text, "-" when it is NULL or
// "".
func columnField(value sqlitefile.Value) string {
	return orDash(asText(value))
}

// joinProject gives a row the session id and, when the home's store knows
// it, its project and branch.
func joinProject(row *timeline.Row, sessionID string, index *stateIndex) {
	row.SessionID = sessionID
	if found, ok := index.projects[sessionID]; ok {
		row.ProjectPath, row.GitBranch = found.path, found.branch
	}
}

// valueKey returns a column value as a key that is equal for equal
// values: a number by its value whatever its storage class, text and blobs
// by their bytes, and NULL.
func valueKey(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		return "n" + strconv.FormatInt(value.Int, 10)
	case sqlitefile.Float:
		if value.Float == math.Trunc(value.Float) && math.Abs(value.Float) < 1<<63 {
			return "n" + strconv.FormatInt(int64(value.Float), 10)
		}
		return "f" + strconv.FormatFloat(value.Float, 'g', -1, 64)
	case sqlitefile.Text:
		return "t" + string(value.Bytes)
	case sqlitefile.Blob:
		return "b" + string(value.Bytes)
	}
	return "null"
}

// ftsBodies returns the conversation_fts_content id -> c1 (the body; c0 is
// the title). The full-text table's content table is read as a plain
// table, so the module is never needed; none when it or a column is
// missing.
func ftsBodies(store *store) map[string]sqlitefile.Value {
	bodies := map[string]sqlitefile.Value{}
	table := store.table("conversation_fts_content")
	if table == nil || !table.Has("id") || !table.Has("c1") {
		return bodies
	}
	rows, err := table.Select("id", "c1").All()
	if store.keep(err) {
		return bodies
	}
	for _, row := range rows {
		bodies[valueKey(row.Get("id"))] = row.Get("c1")
	}
	return bodies
}

// search emits the rows of conversation-search.db: a conversation's cloud
// origin, its index entry and its cached text.
func (p *Parser) search(artifact *input.Artifact, store *store, emit parser.Emit) {
	table := store.table("conversations")
	if table == nil {
		return
	}
	index := p.homeIndex(artifact)
	bodies := ftsBodies(store)
	records, err := selectPresent(table, searchColumns())
	if store.keep(err) {
		return
	}
	for _, record := range records {
		sessionID := asText(record.Get("id"))
		row := func(text string) {
			made := p.row(artifact, timeline.TurnSystem, text, parser.ColumnTime(record.Get("updated_at")), record.Rowid)
			joinProject(&made, sessionID, index)
			made.Model = index.models[sessionID]
			emit(made)
		}
		source := asText(record.Get("source"))
		if source == cloudCache {
			row(cloudOrigin)
		}
		if title := timeline.Compact(asText(record.Get("title"))); title != "" {
			row("conversation index: " + title + " source=" + orDash(source) + " archived=" +
				columnField(record.Get("is_archived")))
		}
		if body := timeline.Compact(asText(bodies[valueKey(record.Get("fts_rowid"))])); body != "" {
			row("cached conversation text (conversation-search.db, source=" + orDash(source) + "): " + body)
		}
	}
}

// tracking emits a row per row of the tables of ai-code-tracking.db.
func (p *Parser) tracking(artifact *input.Artifact, store *store, emit parser.Emit) {
	index := p.homeIndex(artifact)
	for _, tracked := range trackingTables() {
		table := store.table(tracked.name)
		if table == nil {
			continue
		}
		records, err := selectPresent(table, tracked.columns)
		if store.keep(err) {
			return
		}
		for _, record := range records {
			text, when, sessionID := trackingText(tracked.name, record)
			row := p.row(artifact, timeline.TurnSystem, text, parser.ColumnTime(when), record.Rowid)
			joinProject(&row, sessionID, index)
			row.Model = asText(record.Get("model"))
			if tracked.name == "scored_commits" {
				row.GitBranch = asText(record.Get("branchName"))
			}
			emit(row)
		}
	}
}

// trackingText returns the text, time and session id of a row of
// ai-code-tracking.db.
func trackingText(table string, record sqlitefile.Row) (string, sqlitefile.Value, string) {
	field := func(column string) string { return columnField(record.Get(column)) }
	session := record.Get("conversationId")
	if session.IsEmpty() {
		session = record.Get("composerId")
	}
	sessionID := asText(session)
	switch table {
	case "ai_code_hashes":
		return "ai code: " + field("fileName") + " source=" + field("source") + " model=" + field("model") +
			" request=" + field("requestId"), record.Get("timestamp"), sessionID
	case "conversation_summaries":
		return "conversation summary: " + field("title") + ": " + field("tldr"), record.Get("updatedAt"), sessionID
	case "ai_deleted_files":
		return "file deleted by agent: " + field("gitPath") + " model=" + field("model"), record.Get("deletedAt"),
			sessionID
	}
	return "scored commit: " + field("commitHash") + " branch=" + field("branchName") + " +" +
		field("linesAdded") + " -" + field("linesDeleted"), record.Get("scoredAt"), ""
}
