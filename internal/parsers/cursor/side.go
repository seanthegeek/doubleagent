package cursor

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	return cmp.Or(asText(value), "-")
}

// joinProject gives a row the session id and, when the home's
// store knows it, its project and branch, those of the stored row the
// group goes with when the store repeats the id.
func joinProject(row *timeline.Row, sessionID string, index *stateIndex) {
	row.SessionID = sessionID
	if found, ok := index.projects[sessionID]; ok {
		row.ProjectPath, row.GitBranch = found.path, found.branch
	}
	index.member(sessionID, row, timeline.TakeProject|timeline.TakeBranch)
}

// ftsBody is one conversation_fts_content row: its rowid and c1, the body
// (c0 is the title).
type ftsBody struct {
	rowid int64
	body  sqlitefile.Value
}

// ftsBodies returns the conversation_fts_content rows by id, in rowid
// order; more than one of an id only damage writes, the id being the
// content table's INTEGER PRIMARY KEY. The full-text table's content table
// is read as a plain table, so the module is never needed; none when it or
// a column is missing. A row whose id is NULL is left out: NULL equals
// nothing in SQL, so no conversation's fts_rowid finds it.
func ftsBodies(store *store) map[string][]ftsBody {
	bodies := map[string][]ftsBody{}
	table := store.table("conversation_fts_content")
	if table == nil || !table.Has("id") || !table.Has("c1") {
		return bodies
	}
	rows, err := table.Select("id", "c1").All()
	if store.keep(err) {
		return bodies
	}
	for _, row := range rows {
		if key := parserkit.ColumnKey(row.Get("id")); key != "" {
			bodies[key] = append(bodies[key], ftsBody{rowid: row.Rowid, body: row.Get("c1")})
		}
	}
	for _, stored := range bodies {
		slices.SortStableFunc(stored, func(left, right ftsBody) int { return cmp.Compare(left.rowid, right.rowid) })
	}
	return bodies
}

// storedBodies emits a row of its own, with no session or time, for each
// stored row of a full-text id the content table holds more than once:
// which of them a conversation naming the id holds is not known, so none
// gives its text to the conversation's rows.
func (p *Parser) storedBodies(artifact *input.Artifact, ident string, stored []ftsBody, emit parserkit.Emit) {
	for _, found := range stored {
		emit(p.row(artifact, timeline.TurnSystem, "cached conversation text (conversation-search.db, fts_rowid="+ident+
			"): "+timeline.Compact(asText(found.body)), "", found.rowid))
	}
}

// bodyJoin joins the conversations of conversation-search.db to their
// full-text bodies: the shared text of each id once written, and the
// conversations that named an id an earlier one names.
type bodyJoin struct {
	owner    *Parser
	artifact *input.Artifact
	bodies   map[string][]ftsBody
	written  map[string]*timeline.SharedText
	repeats  int
	emit     parserkit.Emit
}

// sharedBody returns the shared text of a full-text body, made once per
// fts_rowid however many conversations name it; nil for an empty body.
func sharedBody(body sqlitefile.Value) *timeline.SharedText {
	text := timeline.Compact(asText(body))
	if text == "" {
		return nil
	}
	return timeline.NewSharedText(text)
}

// cached gives a conversation its cached text row through row, sharing
// the text of its fts_rowid's body; a body id the content table holds
// more than once gives a parser: row instead, its stored rows each giving
// a row of their own once (storedBodies).
func (j *bodyJoin) cached(record sqlitefile.Row, source string, row func(string, *timeline.SharedText)) {
	key := parserkit.ColumnKey(record.Get("fts_rowid"))
	if key == "" {
		return
	}
	stored := j.bodies[key]
	shared, met := j.written[key]
	if len(stored) > 1 {
		row(parserkit.ProblemPrefix+"conversation_fts_content holds "+strconv.Itoa(len(stored))+
			" rows of this row's fts_rowid; each gives its own row", nil)
		if !met {
			j.written[key] = nil
			j.owner.storedBodies(j.artifact, asText(record.Get("fts_rowid")), stored, j.emit)
		}
		return
	}
	if !met {
		var body sqlitefile.Value
		if len(stored) == 1 {
			body = stored[0].body
		}
		shared = sharedBody(body)
		j.written[key] = shared
	} else if shared != nil {
		j.repeats++
	}
	if shared != nil {
		row("cached conversation text (conversation-search.db, source="+cmp.Or(source, "-")+"): ", shared)
	}
}

// bodyPointer is the pointer of a later conversation's row of a full-text
// body: the conversation row of the source line given holds its text.
func bodyPointer(holder *timeline.Row, _ bool) string {
	return "[shown in full above, in the row of source line " + strconv.Itoa(holder.SourceLine) + "]"
}

// sharedBodiesText returns the problem text of n conversations whose
// fts_rowid names a full-text row an earlier conversation names.
func sharedBodiesText(n int) string {
	if n == 1 {
		return "1 conversation names a full-text row an earlier conversation names"
	}
	return strconv.Itoa(n) + " conversations name a full-text row an earlier conversation names"
}

// search emits the rows of conversation-search.db: a conversation's cloud
// origin, its index entry and its cached text. The conversations that join
// one full-text body share its text: the output writes it in full in the
// first of their rows it keeps and a pointer to that row in each later
// one, and the later conversations joining it are counted as one problem.
func (p *Parser) search(artifact *input.Artifact, store *store, emit parserkit.Emit) {
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
	joined := &bodyJoin{owner: p, artifact: artifact, bodies: bodies, written: map[string]*timeline.SharedText{},
		emit: emit}
	for _, record := range records {
		sessionID := asText(record.Get("id"))
		row := func(text string, shared *timeline.SharedText) {
			made := p.row(artifact, timeline.TurnSystem, text, parserkit.ColumnTime(record.Get("updated_at")), record.Rowid)
			if shared != nil {
				var out timeline.TextBuilder
				out.Write(text)
				out.Share(shared, "[cached conversation text]", bodyPointer)
				out.Set(&made)
			}
			joinProject(&made, sessionID, index)
			made.Model = index.models[sessionID]
			index.cloudMember(sessionID, &made, timeline.TakeProject|timeline.TakeModel)
			emit(made)
		}
		source := asText(record.Get("source"))
		if source == cloudCache {
			row(cloudOrigin, nil)
		}
		if title := timeline.Compact(asText(record.Get("title"))); title != "" {
			row("conversation index: "+title+" source="+cmp.Or(source, "-")+" archived="+
				columnField(record.Get("is_archived")), nil)
		}
		joined.cached(record, source, row)
	}
	if joined.repeats > 0 {
		var problems parserkit.Problems
		problems.Add(0, sharedBodiesText(joined.repeats))
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
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

// tracking emits a row per row of the tables of ai-code-tracking.db.
func (p *Parser) tracking(artifact *input.Artifact, store *store, emit parserkit.Emit) {
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
			row := p.row(artifact, timeline.TurnSystem, text, parserkit.ColumnTime(when), record.Rowid)
			joinProject(&row, sessionID, index)
			row.Model = asText(record.Get("model"))
			index.cloudMember(sessionID, &row, timeline.TakeProject)
			if tracked.name == "scored_commits" {
				row.GitBranch = asText(record.Get("branchName"))
			}
			emit(row)
		}
	}
}
