package codex

import (
	"fmt"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// stateDatabase is the name of the threads projection, which the readers
// of the other databases beside it look up for each thread's project.
const stateDatabase = "state_5.sqlite"

// threadColumns, enrollmentColumns and feedbackColumns are the columns
// read from state_5.sqlite, each when the table has it.
var (
	threadColumns = []string{"id", "rollout_path", "created_at", "created_at_ms", "cwd", "title",
		"name", "git_branch", "git_origin_url", "model", "archived", "source", "thread_source",
		"first_user_message"}
	enrollmentColumns = []string{"remote_control_enabled", "server_id", "environment_id",
		"server_name", "app_server_client_name", "account_id", "updated_at"}
	feedbackColumns = []string{"id", "thread_id", "record"}
)

// selectRows returns every row of a table in rowid order with the wanted
// columns it has, a column a migration has not added reading as NULL; no
// rows and no error when the database has no such table.
func selectRows(database *sqlitefile.Database, name string, wanted []string) ([]sqlitefile.Row, error) {
	names, err := database.TableNames()
	if err != nil || !names[name] {
		return nil, err
	}
	table, err := database.Table(name)
	if err != nil {
		return nil, err
	}
	var columns []string
	for _, column := range table.Columns() {
		if column.Generated == sqlitefile.NotGenerated && slices.Contains(wanted, column.Name) {
			columns = append(columns, column.Name)
		}
	}
	slices.SortStableFunc(columns, func(left, right string) int {
		return slices.Index(wanted, left) - slices.Index(wanted, right)
	})
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s has none of the columns read", name)
	}
	return table.Select(columns...).OrderBy("rowid").All()
}

// columnText returns a column as Python's str(value or "") writes it.
func columnText(value sqlitefile.Value) string {
	if !value.Truthy() {
		return ""
	}
	return value.Str()
}

// compactColumn returns a column's text compacted, "" for NULL.
func compactColumn(value sqlitefile.Value) string {
	if value.IsNull() {
		return ""
	}
	return strings.TrimSpace(value.Str())
}

// isColumnText reports whether a column holds the text text.
func isColumnText(value sqlitefile.Value, text string) bool {
	return value.Type == sqlitefile.Text && value.Text() == text
}

// threadCreated returns a thread's creation time: created_at_ms when set,
// else created_at.
func threadCreated(thread sqlitefile.Row) sqlitefile.Value {
	if created := thread.Get("created_at_ms"); !created.IsNull() {
		return created
	}
	return thread.Get("created_at")
}

// readThreads returns the threads rows in creation order, ties in id
// order then rowid order.
func readThreads(database *sqlitefile.Database) ([]sqlitefile.Row, error) {
	threads, err := selectRows(database, "threads", threadColumns)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(threads, func(left, right sqlitefile.Row) int {
		if order := sqlitefile.Compare(createdKey(left), createdKey(right)); order != 0 {
			return order
		}
		return strings.Compare(columnText(left.Get("id")), columnText(right.Get("id")))
	})
	return threads, nil
}

// createdKey returns a thread's creation time as a sort key, 0 when it has
// none.
func createdKey(thread sqlitefile.Row) sqlitefile.Value {
	if created := threadCreated(thread); created.Truthy() {
		return created
	}
	return sqlitefile.IntValue(0)
}

// isSubagentThread reports whether a threads row is a subagent's.
func isSubagentThread(thread sqlitefile.Row) bool {
	source := columnText(thread.Get("source"))
	return isColumnText(thread.Get("thread_source"), "subagent") ||
		(strings.HasPrefix(source, "{") && strings.Contains(source, `"subagent"`))
}

// threadText returns the "thread record:" text of a threads row.
func threadText(thread sqlitefile.Row) string {
	text := "thread record: " + columnText(thread.Get("id")) + " title=" +
		columnText(thread.Get("title")) + " rollout=" + columnText(thread.Get("rollout_path"))
	for _, field := range [][2]string{{"name", "name"}, {"git_branch", "branch"}, {"model", "model"}} {
		if value := thread.Get(field[0]); value.Truthy() {
			text += " " + field[1] + "=" + value.Str()
		}
	}
	if thread.Get("archived").Truthy() {
		text += " archived"
	}
	return text
}

// stamp returns a row of the session with the project and branch of its
// threads row, when there is one. The model stays empty: the threads row
// holds the model of the thread's last turn, not of each event.
func (p *Parser) stamp(artifact *input.Artifact, thread *sqlitefile.Row, sessionID string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SessionID = sessionID
	if thread != nil {
		row.ProjectPath = columnText(thread.Get("cwd"))
		row.GitBranch = columnText(thread.Get("git_branch"))
	}
	return row
}

// stateTables is what the parser reads from state_5.sqlite.
type stateTables struct {
	threads, enrollments, feedback []sqlitefile.Row
}

// readState returns the tables of state_5.sqlite, or the message of the
// problem row that replaces them.
func readState(path string) (stateTables, string) {
	database, message := parser.OpenDatabase(path)
	if message != "" {
		return stateTables{}, message
	}
	defer database.Close()
	var tables stateTables
	var err error
	if tables.threads, err = readThreads(database); err != nil {
		return stateTables{}, parser.DatabaseError(err)
	}
	if tables.enrollments, err = selectRows(database, "remote_control_enrollments",
		enrollmentColumns); err != nil {
		return stateTables{}, parser.DatabaseError(err)
	}
	if tables.feedback, err = selectRows(database, "guardian_review_feedback",
		feedbackColumns); err != nil {
		return stateTables{}, parser.DatabaseError(err)
	}
	return tables, ""
}

// parseState emits the rows of state_5.sqlite or of a copy under
// db-backups/: each thread's record and first prompt, then a "remote
// control:" row per enrollment, then a "permission decision:" row per
// guardian review feedback record.
func (p *Parser) parseState(artifact *input.Artifact, emit parser.Emit) {
	tables, message := readState(artifact.DiskPath)
	if message != "" {
		emit(p.ProblemRow(artifact, message))
		return
	}
	p.threadRows(artifact, tables.threads, emit)
	for _, enrollment := range tables.enrollments {
		row := p.SystemRow(artifact, enrollmentText(enrollment))
		row.SourceLine = int(enrollment.Rowid)
		row.TimestampUTC = parser.ColumnTime(enrollment.Get("updated_at"))
		emit(row)
	}
	p.feedbackRows(artifact, tables.feedback, threadsByID(tables.threads), emit)
}

// threadsByID returns the threads rows by their id as text, the last row
// of a repeated id winning.
func threadsByID(threads []sqlitefile.Row) map[string]*sqlitefile.Row {
	byID := make(map[string]*sqlitefile.Row, len(threads))
	for i := range threads {
		byID[columnText(threads[i].Get("id"))] = &threads[i]
	}
	return byID
}

// enrollmentText returns the "remote control:" text of an enrollment, "-"
// for an empty column.
func enrollmentText(enrollment sqlitefile.Row) string {
	labels := []string{"enabled", "server", "environment", "host", "client", "account"}
	parts := make([]string, len(labels))
	for i, label := range labels {
		value, shown := enrollment.Get(enrollmentColumns[i]), "-"
		if !value.IsNull() && !isColumnText(value, "") {
			shown = value.Str()
		}
		parts[i] = label + "=" + shown
	}
	return "remote control: " + strings.Join(parts, " ")
}

// threadRows emits a thread record per threads row, then its git remote
// and its first prompt when it has them.
func (p *Parser) threadRows(artifact *input.Artifact, threads []sqlitefile.Row, emit parser.Emit) {
	for i := range threads {
		thread := &threads[i]
		sessionID := columnText(thread.Get("id"))
		row := p.stamp(artifact, thread, sessionID)
		row.SourceLine = int(thread.Rowid)
		row.TurnType = timeline.TurnSystem
		row.TimestampUTC = parser.ColumnTime(threadCreated(*thread))
		row.Text = timeline.Compact(threadText(*thread))
		emit(row)
		if origin := compactColumn(thread.Get("git_origin_url")); origin != "" {
			remote := row
			remote.Text = timeline.GitRemotePrefix + origin
			emit(remote)
		}
		first := compactColumn(thread.Get("first_user_message"))
		if first == "" {
			continue
		}
		prompt := p.stamp(artifact, thread, sessionID)
		prompt.SourceLine = row.SourceLine
		prompt.TurnType, prompt.Text = timeline.TurnUser, first
		if isSubagentThread(*thread) {
			prompt.TurnType, prompt.Text = timeline.TurnSystem, "subagent task: "+first
		}
		emit(prompt)
	}
}

// feedbackRows emits a "permission decision:" row per
// guardian_review_feedback record, at the time in its UUIDv7 id, under the
// reviewed thread, with the governed item as the call id; a record that is
// not a JSON object is a problem row after the others.
func (p *Parser) feedbackRows(artifact *input.Artifact, feedback []sqlitefile.Row,
	threads map[string]*sqlitefile.Row, emit parser.Emit) {
	var problems parser.Problems
	for _, entry := range feedback {
		rowid := int(entry.Rowid)
		record, ok := decodeColumnObject(entry.Get("record"), rowid, &problems)
		if !ok {
			continue
		}
		sessionID := parser.Str(record.Get("reviewed_thread_id"))
		if sessionID == "" {
			sessionID = columnText(entry.Get("thread_id"))
		}
		row := p.stamp(artifact, threads[sessionID], sessionID)
		row.SourceLine = rowid
		row.TurnType = timeline.TurnSystem
		row.TimestampUTC = uuid7Time(entry.Get("id"))
		row.ToolUseID = parser.Str(record.Get("target_item_id"))
		row.Text = timeline.Compact(feedbackText(record))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// decodeColumnObject returns the JSON object a column holds as text or as
// a UTF-8 blob, or false after adding the problem "unparseable record
// <rowid> (<reason>)" with the stored value.
func decodeColumnObject(value sqlitefile.Value, rowid int, problems *parser.Problems) (jsonvalue.Value, bool) {
	raw := value.Str()
	if value.Type == sqlitefile.Blob {
		raw = textlines.ValidUTF8(value.Bytes)
	}
	record, err := jsonvalue.DecodeString(raw)
	if err == nil && !record.IsObject() {
		err = errNotObject
	}
	if err != nil {
		problems.AddRaw(rowid, parser.UnparseableRecord(rowid, err), raw)
		return jsonvalue.NewNull(), false
	}
	return record, true
}

// feedbackText returns the "permission decision:" text of a guardian
// review's feedback record. A denied record whose decision is the
// reviewer's JSON verdict reads as the review thread's own row for that
// verdict, so the two pair.
func feedbackText(record jsonvalue.Value) string {
	item := strOr(record.Get("target_item_id"), "-")
	reviewer := strOr(record.Get("reviewer_thread_id"), "-")
	decision := record.Get("decision")
	if isText(record.Get("status"), "denied") && decision.IsString() {
		if verdict, ok := guardianVerdict(decision.StringValue()); ok {
			return guardianText(verdict, item, reviewer)
		}
	}
	return "permission decision: " + strOr(record.Get("status"), "-") +
		" (auto_review feedback model=" + strOr(record.Get("model"), "-") + ") exec " + item +
		" turn=" + strOr(record.Get("reviewed_turn_id"), "-") + " reviewer " + reviewer + ": " +
		strOr(decision, "-") + " action: " + parser.Str(record.Get("action"))
}

// siblingThreads returns the threads rows of the state_5.sqlite beside the
// artifact, by id; none when there is no such database or it cannot be
// read, which that database's own parse reports.
func siblingThreads(artifact *input.Artifact) map[string]*sqlitefile.Row {
	path := parser.Beside(artifact, stateDatabase)
	if sqlitefile.Problem(path) != nil {
		return map[string]*sqlitefile.Row{}
	}
	database, err := sqlitefile.OpenPath(path)
	if err != nil {
		return map[string]*sqlitefile.Row{}
	}
	defer database.Close()
	threads, err := readThreads(database)
	if err != nil {
		return map[string]*sqlitefile.Row{}
	}
	return threadsByID(threads)
}
