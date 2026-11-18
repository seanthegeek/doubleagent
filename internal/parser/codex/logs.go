package codex

import (
	"cmp"
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// logField is one key=value field of a tracing body: a bare value or a
// quoted string, after the start or a space or tab.
var logField = regexp.MustCompile(`(?:^|[ \t])([A-Za-z_][A-Za-z0-9_.]*)=("(?:[^"\\]|\\.)*"|[^ \t\r\n]+)`)

// The approval decisions of the ChatGPT desktop app as Codex Desktop logs
// them (research/codex-cli.md, Other stores and Standing grants and
// approval records): the core's submission of an exec or patch decision,
// with the governed item's id, and the app server's response to an
// approval request, with only a request id.
var (
	execApproval = regexp.MustCompile(`op=ExecApproval \{ id: "([^"]*)"` +
		`(?:, turn_id: (?:Some\("([^"]*)"\)|None))?.*?decision: ([A-Za-z_]+)`)
	patchApproval   = regexp.MustCompile(`op=PatchApproval \{ id: "([^"]*)".*?decision: ([A-Za-z_]+)`)
	requestApproval = regexp.MustCompile(`CommandExecutionRequestApproval \{ request_id: ` +
		`(?:[A-Za-z]+\()?"?([^,)"]*)"?\)?,.*?decision: ([A-Za-z_]+)`)
	guardianSpan = regexp.MustCompile(`guardian_review\{([^}]*)\}`)
)

// The log rows the parser reads, by target and marker in the body.
const (
	feedbackTarget  = "codex_feedback"
	feedbackMark    = "feedback event uploaded to Sentry"
	handlersTarget  = "codex_core::session::handlers"
	execMark        = "op=ExecApproval {"
	patchMark       = "op=PatchApproval {"
	outgoingTarget  = "codex_app_server::outgoing_message"
	requestMark     = "CommandExecutionRequestApproval {"
	guardianMark    = "guardian_review{"
	noThreadPrefix  = "no-active-thread-"
	logsTable       = "logs"
	bodyColumn      = "feedback_log_body"
	firstBodyColumn = "message"
)

// logKinds are the log rows read, as target, marker and kind; a row is the
// first kind whose target and marker it has.
var logKinds = [][3]string{
	{feedbackTarget, feedbackMark, "feedback"},
	{handlersTarget, execMark, "exec"},
	{handlersTarget, patchMark, "patch"},
	{outgoingTarget, requestMark, "request"},
}

// logRecord is one log row the parser reads: its rowid, ts, ts_nanos and
// thread_id columns, target, body and kind.
type logRecord struct {
	rowid                          int
	timestamp, nanoseconds, thread sqlitefile.Value
	target, body, kind             string
}

// logFields returns the key=value fields of a tracing body, a quoted value
// without its quotes, a repeated key taking its last value.
func logFields(body string) map[string]string {
	fields := map[string]string{}
	for _, match := range logField.FindAllStringSubmatch(body, -1) {
		value := match[2]
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		fields[match[1]] = value
	}
	return fields
}

// logsTableOf returns the logs table and the name of its body column, or
// nil when the database has no such table or it lacks the body or target
// column. The body is feedback_log_body, message in the first migration.
func logsTableOf(database *sqlitefile.Database) (*sqlitefile.Table, string, error) {
	names, err := database.TableNames()
	if err != nil || !names[logsTable] {
		return nil, "", err
	}
	table, err := database.Table(logsTable)
	if err != nil {
		return nil, "", err
	}
	have := map[string]bool{}
	for _, column := range table.Columns() {
		if column.Generated == sqlitefile.NotGenerated {
			have[column.Name] = true
		}
	}
	body := bodyColumn
	if !have[body] {
		body = firstBodyColumn
	}
	if !have[body] || !have["target"] {
		return nil, "", nil
	}
	return table, body, nil
}

// readLogs returns the log rows of a kind logKinds names, in rowid order.
func readLogs(database *sqlitefile.Database) ([]logRecord, error) {
	table, body, err := logsTableOf(database)
	if table == nil || err != nil {
		return nil, err
	}
	columns := []string{"target", body}
	for _, column := range table.Columns() {
		if slices.Contains([]string{"ts", "ts_nanos", "thread_id"}, column.Name) &&
			column.Generated == sqlitefile.NotGenerated {
			columns = append(columns, column.Name)
		}
	}
	rows, err := table.Select(columns...).Where(func(row sqlitefile.Row) bool {
		return logKind(row, body) != ""
	}).OrderBy("rowid").All()
	if err != nil {
		return nil, err
	}
	records := make([]logRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, logRecord{rowid: int(row.Rowid), timestamp: row.Get("ts"),
			nanoseconds: row.Get("ts_nanos"), thread: row.Get("thread_id"),
			target: row.Get("target").Text(), body: parser.NonEmptyColumn(row.Get(body)),
			kind: logKind(row, body)})
	}
	return records, nil
}

// logKind returns the kind of a log row: the first logKinds entry whose
// target equals the row's and whose marker its body holds, else "".
func logKind(row sqlitefile.Row, body string) string {
	for _, kind := range logKinds {
		if row.Equals("target", sqlitefile.TextValue(kind[0])) &&
			sqlitefile.Instr(row.Get(body), sqlitefile.TextValue(kind[1])) > 0 {
			return kind[2]
		}
	}
	return ""
}

// readLogDatabase returns the log rows of the database at path, or an
// error whose text is the problem row that replaces them.
func readLogDatabase(path string) ([]logRecord, error) {
	database, err := openDatabase(path)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	records, err := readLogs(database)
	if err != nil {
		return nil, databaseError(err)
	}
	return records, nil
}

// openDatabase opens the database at path through parser.OpenDatabase, or
// returns an error whose text is the problem row that says why it cannot.
func openDatabase(path string) (*sqlitefile.Database, error) {
	database, message := parser.OpenDatabase(path)
	if message != "" {
		return nil, errors.New(message)
	}
	return database, nil
}

// databaseError returns an error met reading a database as an error whose
// text is its problem row (parser.DatabaseError).
func databaseError(err error) error {
	return errors.New(parser.DatabaseError(err))
}

// parseLogs emits the rows of logs_<N>.sqlite: one per sent /feedback
// report and one per approval decision, at the log time, then a problem
// row per approval line the patterns cannot read.
func (p *Parser) parseLogs(artifact *input.Artifact, emit parser.Emit) {
	records, err := readLogDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, err.Error()))
		return
	}
	threads := map[string]*sqlitefile.Row{}
	if len(records) > 0 {
		threads = siblingThreads(artifact)
	}
	var problems parser.Problems
	decided := decisions(records, &problems)
	name := filepath.Base(artifact.DiskPath)
	for _, record := range records {
		var found decision
		if record.kind == "feedback" {
			found = feedbackDecision(record, name)
		} else if known, ok := decided[record.rowid]; ok {
			found = known
			found.text += " (from " + name + " logs, ts)"
		} else {
			continue
		}
		var thread *sqlitefile.Row
		if found.sessionID != "" {
			thread = threads[found.sessionID]
		}
		row := p.stamp(artifact, thread, found.sessionID)
		row.SourceLine = record.rowid
		row.TurnType = timeline.TurnSystem
		row.TimestampUTC = logTime(record)
		row.ToolUseID = found.toolUseID
		row.Text = found.text
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// decision is the session, governed item and text of one log row's row.
type decision struct {
	sessionID, toolUseID, text string
}

// feedbackDecision returns the row of a sent /feedback report; a report
// sent with no thread open has no session.
func feedbackDecision(record logRecord, name string) decision {
	fields := logFields(record.body)
	sessionID := fields["thread_id"]
	if sessionID == "" {
		sessionID = parser.NonEmptyColumn(record.thread)
	}
	if strings.HasPrefix(sessionID, noThreadPrefix) {
		sessionID = ""
	}
	text := "session synced: feedback report uploaded to Sentry, attachments " +
		cmp.Or(fields["uploaded_attachments"], "-") + " failed " + cmp.Or(fields["attachments_failed"], "-") +
		" (from " + name + " logs, ts)"
	return decision{sessionID: sessionID, text: text}
}

// logTime returns a log row's time: ts in seconds plus ts_nanos when that
// is a nanosecond count, "" when ts is not a number.
func logTime(record logRecord) string {
	seconds, ok := numberOf(record.timestamp)
	if !ok {
		return ""
	}
	if nanos := record.nanoseconds; nanos.Type == sqlitefile.Integer && nanos.Int >= 0 &&
		nanos.Int < 1_000_000_000 {
		return timeutil.FromEpoch(seconds + float64(nanos.Int)/1e9)
	}
	return parser.ColumnTime(record.timestamp)
}

// numberOf returns an integer or float column as a float.
func numberOf(value sqlitefile.Value) (float64, bool) {
	switch value.Type {
	case sqlitefile.Integer:
		return float64(value.Int), true
	case sqlitefile.Float:
		return value.Float, true
	}
	return 0, false
}

// approvalMatch returns the approval pattern's groups in a log row of the
// kind, or nil; a row of that kind the pattern cannot read adds the problem
// "unreadable approval log row (<target>)" with its body.
func approvalMatch(record logRecord, kind string, problems *parser.Problems) []string {
	if record.kind != kind {
		return nil
	}
	pattern := map[string]*regexp.Regexp{"exec": execApproval, "patch": patchApproval}[kind]
	if pattern == nil {
		pattern = requestApproval
	}
	match := pattern.FindStringSubmatch(record.body)
	if match == nil {
		problems.AddRaw(record.rowid, "unreadable approval log row ("+record.target+")", record.body)
	}
	return match
}

// decisionSession returns the thread of an approval row: its body's
// thread_id field, else its thread_id column.
func decisionSession(record logRecord) string {
	if session := logFields(record.body)["thread_id"]; session != "" {
		return session
	}
	return parser.NonEmptyColumn(record.thread)
}

// decisions returns the row of each approval line by rowid, without its
// source suffix. A request row with an unpaired exec row within 2 seconds
// is left out, since the exec row names the thread and the governed item
// for the same decision in other words; a patch row stands alone, since
// the request rows read are command approvals only.
func decisions(records []logRecord, problems *parser.Problems) map[int]decision {
	out := map[int]decision{}
	var execs []logRecord
	for _, record := range records {
		if match := approvalMatch(record, "exec", problems); match != nil {
			out[record.rowid] = decision{decisionSession(record), match[1], "permission decision: " +
				match[3] + " exec " + match[1] + " turn=" + cmp.Or(match[2], "-")}
			execs = append(execs, record)
		}
	}
	for _, record := range records {
		if match := approvalMatch(record, "patch", problems); match != nil {
			out[record.rowid] = decision{decisionSession(record), match[1],
				"permission decision: " + match[2] + " patch " + match[1]}
		}
	}
	pairs := newExecPairs(execs)
	for _, record := range records {
		match := approvalMatch(record, "request", problems)
		if match == nil || pairs.take(record.timestamp) >= 0 {
			continue
		}
		out[record.rowid] = decision{text: "permission decision: " + match[2] + " request " + match[1]}
	}
	return out
}

// guardianLog is one log row holding a guardian review span: the whole
// row, for its ts, whether its body is text a query could return, and the
// tool_call_id values of its spans of the reviewed thread, in order.
type guardianLog struct {
	row      sqlitefile.Row
	readable bool
	calls    []string
}

// guardianLogs returns, per logs_<N>.sqlite file in the agent's home in
// name order, its rows that hold a guardian review span, in rowid order,
// with the calls of thread parent's spans. A database that cannot be read
// gives none; its own parse reports why.
func guardianLogs(home, parent string) [][]guardianLog {
	var databases [][]guardianLog
	for _, logPath := range logDatabases(home) {
		rows, body := guardianRows(logPath)
		logs := make([]guardianLog, 0, len(rows))
		for _, row := range rows {
			text := row.Get(body)
			each := guardianLog{row: row, readable: text.Type != sqlitefile.Text || utf8.Valid(text.Bytes)}
			for _, match := range guardianSpan.FindAllStringSubmatch(parser.NonEmptyColumn(text), -1) {
				fields := logFields(match[1])
				if call := fields["tool_call_id"]; fields["thread_id"] == parent && call != "" {
					each.calls = append(each.calls, call)
				}
			}
			logs = append(logs, each)
		}
		databases = append(databases, logs)
	}
	return databases
}

// guardianCalls returns the distinct tool_call_id values of the guardian
// logs, in log order, of the rows logged from 2 seconds before start to 2
// seconds after end (both timeline timestamps), the ts compared as the
// database compares it. A database with a row in that time whose body is
// not valid text gives none, as a query of those rows fails. None when
// either time cannot be read, without reading the logs.
func guardianCalls(logs func() [][]guardianLog, start, end string) []string {
	startMillis, startOK := timeutil.EpochMillis(start)
	endMillis, endOK := timeutil.EpochMillis(end)
	if !startOK || !endOK {
		return nil
	}
	low, high := floorDiv(startMillis, 1000)-2, floorDiv(endMillis, 1000)+2
	var calls []string
	seen := map[string]bool{}
	for _, database := range logs() {
		inTime := slices.DeleteFunc(slices.Clone(database), func(each guardianLog) bool {
			lowOrder, lowOK := each.row.Cmp("ts", sqlitefile.IntValue(low))
			highOrder, highOK := each.row.Cmp("ts", sqlitefile.IntValue(high))
			return !lowOK || !highOK || lowOrder < 0 || highOrder > 0
		})
		if slices.ContainsFunc(inTime, func(each guardianLog) bool { return !each.readable }) {
			continue
		}
		for _, each := range inTime {
			for _, call := range each.calls {
				if !seen[call] {
					seen[call] = true
					calls = append(calls, call)
				}
			}
		}
	}
	return calls
}

// floorDiv returns n divided by divisor, rounded toward minus infinity.
func floorDiv(n, divisor int64) int64 {
	quotient := n / divisor
	if n%divisor != 0 && n < 0 {
		quotient--
	}
	return quotient
}

// logDatabases returns the paths of the logs_*.sqlite entries of home, in
// name order; none when home cannot be listed.
func logDatabases(home string) []string {
	entries, err := parser.ReadDir(home)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		if matched, _ := path.Match("logs_*.sqlite", entry.Name()); matched {
			paths = append(paths, filepath.Join(home, entry.Name()))
		}
	}
	slices.Sort(paths)
	return paths
}

// guardianRows returns, in rowid order, the whole log rows of the
// database at logPath whose body holds a guardian review span, and the
// name of the body column; none when the database cannot be read. The
// rows are taken as the scan meets them, unprojected, so a body that is
// not valid text fails only the times that would have returned it.
func guardianRows(logPath string) ([]sqlitefile.Row, string) {
	if sqlitefile.Problem(logPath) != nil {
		return nil, ""
	}
	database, err := sqlitefile.OpenPath(logPath)
	if err != nil {
		return nil, ""
	}
	defer database.Close()
	table, body, err := logsTableOf(database)
	if table == nil || err != nil {
		return nil, ""
	}
	var rows []sqlitefile.Row
	err = table.Select().Where(func(row sqlitefile.Row) bool {
		if sqlitefile.Instr(row.Get(body), sqlitefile.TextValue(guardianMark)) > 0 {
			rows = append(rows, row)
		}
		return false
	}).Each(func(sqlitefile.Row) error { return nil })
	if err != nil {
		return nil, ""
	}
	slices.SortStableFunc(rows, func(left, right sqlitefile.Row) int { return cmp.Compare(left.Rowid, right.Rowid) })
	return rows, body
}

// uuid7Time returns the capture time in a UUIDv7's first 48 bits, the
// milliseconds since the epoch, as a timestamp; "" for any other value.
func uuid7Time(value sqlitefile.Value) string {
	hexes := strings.ReplaceAll(parser.NonEmptyColumn(value), "-", "")
	if len(hexes) != 32 || hexes[12] != '7' {
		return ""
	}
	milliseconds, err := strconv.ParseUint(hexes[:12], 16, 64)
	if err != nil {
		return ""
	}
	return timeutil.FromEpoch(float64(milliseconds) / 1000)
}
