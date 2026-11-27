package sqlitefile

import (
	"slices"
	"strings"
)

// The parsers' queries, rewritten with the helpers as a parser would write
// them. Each runs over any database holding its tables and returns the rows
// in the column order of the SQL query given in its comment, and whether the
// database has what it reads. testdata/tables.txt and
// testdata/fixture_tables.txt record what SQLite returns for each query.

// siteFunction is one rewritten query.
type siteFunction func(database *Database) ([][]Value, bool, error)

// sites returns the rewritten queries by the names the dumps use.
func sites() map[string]siteFunction {
	return map[string]siteFunction{
		"chatgpt_desktop":   chatgptDesktopSite,
		"cursor_prefixed":   cursorPrefixedSite,
		"cursor_one":        cursorOneSite,
		"cursor_substr":     cursorSubstrSite,
		"zed_join":          zedJoinSite,
		"openclaw_join":     openclawJoinSite,
		"openclaw_cold":     openclawColdSite,
		"codex_logs":        codexLogsSite,
		"goose_like":        gooseLikeSite,
		"hermes_names":      hermesNamesSite,
		"cline_fts":         clineFTSSite,
		"opencode_messages": opencodeMessagesSite,
		"crush_files":       crushFilesSite,
		"functions":         functionsSite,
		"comparisons":       comparisonsSite,
		"order_mixed":       orderMixedSite,
		"order_people":      orderPeopleSite,
	}
}

// tablesOf opens the named tables, reporting false when one is missing.
func tablesOf(database *Database, names ...string) ([]*Table, bool, error) {
	present, err := tableNames(database)
	if err != nil {
		return nil, false, err
	}
	var tables []*Table
	for _, name := range names {
		if !present[name] {
			return nil, false, nil
		}
		table, err := database.Table(name)
		if err != nil {
			return nil, true, err
		}
		tables = append(tables, table)
	}
	return tables, true, nil
}

// withRowid returns each row's rowid followed by its values.
func withRowid(rows []Row) [][]Value {
	var out [][]Value
	for _, row := range rows {
		out = append(out, append([]Value{IntValue(row.Rowid)}, row.Values()...))
	}
	return out
}

// chatgptDesktopSite is the ChatGPT desktop parser's query:
// select rowid as _rowid, * from local_thread_catalog order by rowid.
func chatgptDesktopSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "local_thread_catalog")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select().All()
	return withRowid(rows), true, err
}

// cursorPrefixedSite is the Cursor parser's query: select rowid, key, value from
// cursorDiskKV where key >= ? and key < ? order by key.
func cursorPrefixedSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "cursorDiskKV")
	if !ok || err != nil {
		return nil, ok, err
	}
	prefix := "composerData:"
	end := prefix[:len(prefix)-1] + string(rune(prefix[len(prefix)-1]+1))
	rows, err := tables[0].Select("key", "value").Where(func(row Row) bool {
		low, ok := row.Cmp("key", TextValue(prefix))
		high, _ := row.Cmp("key", TextValue(end))
		return ok && low >= 0 && high < 0
	}).OrderBy("key").All()
	return withRowid(rows), true, err
}

// cursorOneSite is the Cursor parser's query: select rowid, value from cursorDiskKV where key = ?.
func cursorOneSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "cursorDiskKV")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("value").Where(func(row Row) bool {
		return row.Equals("key", TextValue("bubbleId:1:2"))
	}).All()
	return withRowid(rows), true, err
}

// cursorSubstrSite is the Cursor parser's query: select rowid, key, value from ItemTable
// where substr(key, 1, ?) = ? order by key.
func cursorSubstrSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "ItemTable")
	if !ok || err != nil {
		return nil, ok, err
	}
	prefix := "cloudAgentRepository.agents."
	rows, err := tables[0].Select("key", "value").Where(func(row Row) bool {
		key := row.Get("key")
		return key.Type == Text && strings.HasPrefix(key.Text(), prefix)
	}).OrderBy("key").All()
	return withRowid(rows), true, err
}

// zedJoinSite is the Zed parser's query: select a.rowid, a.*, t.thread_id from
// archived_git_worktrees a left join thread_archived_worktrees t on
// t.archived_worktree_id = a.id order by a.id, with one worktree's links
// ordered by thread id (difference 105).
func zedJoinSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "archived_git_worktrees", "thread_archived_worktrees")
	if !ok || err != nil {
		return nil, ok, err
	}
	links, err := tables[1].Select("thread_id").Lookup("archived_worktree_id")
	if err != nil {
		return nil, true, err
	}
	archived, err := tables[0].Select().OrderBy("id").All()
	var out [][]Value
	for _, worktree := range archived {
		matches := links.Find(worktree, "id")
		slices.SortStableFunc(matches, func(left, right Row) int {
			return Compare(left.Get("thread_id"), right.Get("thread_id"))
		})
		head := append([]Value{IntValue(worktree.Rowid)}, worktree.Values()...)
		if len(matches) == 0 {
			out = append(out, append(head, Value{}))
		}
		for _, link := range matches {
			out = append(out, append(slices.Clone(head), link.Get("thread_id")))
		}
	}
	return out, true, err
}

// openclawJoinSite is the OpenClaw parser's query: session_conversations l join
// conversations c using (conversation_id) order by l.first_seen_at, l.rowid.
func openclawJoinSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "session_conversations", "conversations")
	if !ok || err != nil {
		return nil, ok, err
	}
	byID, err := tables[1].Select("channel", "account_id", "kind", "peer_id", "thread_id").
		Lookup("conversation_id")
	if err != nil {
		return nil, true, err
	}
	links, err := tables[0].Select("session_id", "role", "first_seen_at", "conversation_id").
		OrderBy("first_seen_at", "rowid").All()
	var out [][]Value
	for _, link := range links {
		for _, conversation := range byID.Find(link, "conversation_id") {
			row := []Value{IntValue(link.Rowid), link.Get("session_id"), link.Get("role"),
				link.Get("first_seen_at")}
			out = append(out, append(row, conversation.Values()...))
		}
	}
	return out, true, err
}

// openclawColdSite is the OpenClaw parser's query: the sqlite-stored cold
// archives by archived_at, each followed by its chunks (a WITHOUT ROWID
// table) by chunk_index.
func openclawColdSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "session_transcript_cold_archives")
	if !ok || err != nil {
		return nil, ok, err
	}
	chunkTables, chunked, err := tablesOf(database, "session_transcript_cold_archive_chunks")
	if err != nil {
		return nil, true, err
	}
	archives, err := tables[0].Select("session_id", "archive_name", "archived_at", "archive_blob").
		Where(func(row Row) bool { return row.Equals("storage", TextValue("sqlite")) }).
		OrderBy("archived_at").All()
	var out [][]Value
	for _, archive := range archives {
		out = append(out, withRowid([]Row{archive})...)
		if !chunked {
			continue
		}
		chunks, err := chunkTables[0].Select("archive_name", "archive_blob").
			Where(func(row Row) bool { return row.Equals("session_id", archive.Get("session_id")) }).
			OrderBy("chunk_index").All()
		if err != nil {
			return out, true, err
		}
		for _, chunk := range chunks {
			out = append(out, chunk.Values())
		}
	}
	return out, true, err
}

// codexLogsSite is the Codex state parser's query: the logs rows whose target and body
// marker match one of the kinds, (target = ? and instr(body, ?) > 0) or ...
func codexLogsSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "logs")
	if !ok || err != nil {
		return nil, ok, err
	}
	logs := tables[0]
	body := "message"
	if logs.Has("feedback_log_body") {
		body = "feedback_log_body"
	}
	if !logs.Has(body) || !logs.Has("target") {
		return nil, false, nil
	}
	kinds := [][2]string{
		{"codex_feedback", "feedback event uploaded to Sentry"},
		{"codex_core::session::handlers", "op=ExecApproval {"},
		{"codex_core::session::handlers", "op=PatchApproval {"},
		{"codex_app_server::outgoing_message", "CommandExecutionRequestApproval {"},
	}
	selected := []string{"target", body}
	for _, column := range []string{"ts", "ts_nanos", "thread_id"} {
		if logs.Has(column) {
			selected = append(selected, column)
		}
	}
	rows, err := logs.Select(selected...).Where(func(row Row) bool {
		for _, kind := range kinds {
			if row.Equals("target", TextValue(kind[0])) && Instr(row.Get(body), TextValue(kind[1])) > 0 {
				return true
			}
		}
		return false
	}).All()
	var out [][]Value
	for _, row := range rows {
		values := []Value{IntValue(row.Rowid)}
		for _, column := range []string{"ts", "ts_nanos", "thread_id"} {
			values = append(values, row.Get(column))
		}
		out = append(out, append(values, row.Get("target"), row.Get(body)))
	}
	return out, true, err
}

// gooseLikeSite is the Goose parser's query: select session_id, content_json from
// messages where content_json like '%toolConfirmationResponse%'.
func gooseLikeSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "messages")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("session_id", "content_json").Where(func(row Row) bool {
		return Like(row.Get("content_json"), "%toolConfirmationResponse%")
	}).All()
	return valuesOf(rows), true, err
}

// valuesOf returns each row's values.
func valuesOf(rows []Row) [][]Value {
	var out [][]Value
	for _, row := range rows {
		out = append(out, row.Values())
	}
	return out
}

// hermesNamesSite is the Hermes parser's query: select name, response_id from
// conversations, which the parser folds into a map where the last
// duplicate wins.
func hermesNamesSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "conversations")
	if !ok || err != nil || !tables[0].Has("name") || !tables[0].Has("response_id") {
		return nil, ok && err != nil, err
	}
	rows, err := tables[0].Select("name", "response_id").All()
	return valuesOf(rows), true, err
}

// clineFTSSite is the Cline parser's query: select <columns> from session_search order
// by rowid, an fts5 table read from its content table.
func clineFTSSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "session_search")
	if !ok || err != nil {
		return nil, ok, err
	}
	columns := "session_id, document_id, ordinal, role, started_at, workspace_root, title, content"
	rows, err := tables[0].Select(strings.Split(columns, ", ")...).OrderBy("rowid").All()
	return valuesOf(rows), true, err
}

// opencodeMessagesSite is the OpenCode parser's query: session_message ordered by
// session_id, seq, time_created, ties in rowid order.
func opencodeMessagesSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "session_message")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("id", "session_id", "type", "seq", "time_created", "data").
		OrderBy("session_id", "seq", "time_created").All()
	return withRowid(rows), true, err
}

// crushFilesSite is the Crush parser's query: select rowid, session_id, path, version,
// length(content) as size, created_at from files order by created_at, rowid.
func crushFilesSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "files")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("session_id", "path", "version", "created_at").
		Compute("size", func(row Row) Value { return Length(row.Get("content")) }).
		OrderBy("created_at", "rowid").All()
	var out [][]Value
	for _, row := range rows {
		out = append(out, []Value{IntValue(row.Rowid), row.Get("session_id"), row.Get("path"),
			row.Get("version"), row.Get("size"), row.Get("created_at")})
	}
	return out, true, err
}

// functionsSite computes the functions query of tables.txt over samples. The
// functions read the stored values, which may be invalid text, so it reads
// them through the table's scan rather than a selection.
func functionsSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "samples")
	if !ok || err != nil {
		return nil, ok, err
	}
	var out [][]Value
	err = tables[0].scan(func(row Row) error {
		out = append(out, functionRow(row.Rowid, row.value(0)))
		return nil
	})
	return out, true, err
}

// functionRow computes one row of the functions query for one sample.
func functionRow(rowid int64, sample Value) []Value {
	row := []Value{IntValue(rowid), Length(sample)}
	for _, needle := range []Value{TextValue("b"), BlobValue([]byte("b")), TextValue("é"), TextValue("")} {
		row = append(row, nullOr(sample, IntValue(Instr(sample, needle))))
	}
	for _, pattern := range []string{"A%", "%c", "_b%", "%", "_", "é", "É", "_A", "__", "%B_",
		`a\%c`, "%5%"} {
		row = append(row, nullOr(sample, sqlBool(Like(sample, pattern))))
	}
	return row
}

// nullOr returns NULL when the sample is NULL, as SQL functions do, and result otherwise.
func nullOr(sample, result Value) Value {
	if sample.IsNull() {
		return Value{}
	}
	return result
}

// sqlBool returns a truth value as SQL returns it, 1 or 0.
func sqlBool(truth bool) Value {
	if truth {
		return IntValue(1)
	}
	return IntValue(0)
}

// comparisonsSite computes the comparisons query of tables.txt over mixed: SQL
// comparisons with the column's affinity applied to the literal.
func comparisonsSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "mixed")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select().All()
	tests := []struct {
		column  string
		operand Value
		holds   func(order int) bool
	}{
		{"x", IntValue(5), equalOrder}, {"x", TextValue("5"), equalOrder},
		{"y", IntValue(5), equalOrder}, {"y", TextValue("5"), equalOrder},
		{"z", TextValue("5"), equalOrder}, {"z", FloatValue(5), equalOrder},
		{"w", TextValue("5"), equalOrder}, {"w", IntValue(5), equalOrder},
		{"y", TextValue("b"), func(order int) bool { return order < 0 }},
		{"z", TextValue("3"), func(order int) bool { return order > 0 }},
		{"x", IntValue(2), func(order int) bool { return order >= 0 }},
	}
	var out [][]Value
	for _, row := range rows {
		values := []Value{IntValue(row.Rowid)}
		for _, test := range tests {
			order, ok := row.Cmp(test.column, test.operand)
			if !ok {
				values = append(values, Value{})
				continue
			}
			values = append(values, sqlBool(test.holds(order)))
		}
		out = append(out, values)
	}
	return out, true, err
}

// equalOrder reports a comparison result of equality.
func equalOrder(order int) bool {
	return order == 0
}

// orderMixedSite is select rowid from mixed order by x, y.
func orderMixedSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "mixed")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("rowid").OrderBy("x", "y").All()
	return valuesOf(rows), true, err
}

// orderPeopleSite is select rowid, name from people order by name, t,
// under the NOCASE and RTRIM collations the columns declare.
func orderPeopleSite(database *Database) ([][]Value, bool, error) {
	tables, ok, err := tablesOf(database, "people")
	if !ok || err != nil {
		return nil, ok, err
	}
	rows, err := tables[0].Select("rowid", "name").OrderBy("name", "t").All()
	return valuesOf(rows), true, err
}
