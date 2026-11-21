package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// codexRepeatState returns a state database whose threads table holds one
// thread id in two rows, an old one in /proj/a on br-a and a new one in
// /proj/b on br-b, and a guardian review feedback record of the thread a
// day after the new row.
func codexRepeatState(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id TEXT, rollout_path TEXT, " +
			"created_at INTEGER, cwd TEXT, title TEXT, git_branch TEXT, first_user_message TEXT)", Rows: [][]any{
			{"th", "/r1", int64(1767225600), "/proj/a", "A", "br-a", nil},
			{"th", "/r2", int64(1780272000), "/proj/b", "B", "br-b", nil}}},
		sqlitetest.Table{Name: "guardian_review_feedback", SQL: "CREATE TABLE guardian_review_feedback " +
			"(id TEXT, thread_id TEXT, record TEXT)", Rows: [][]any{
			{"019e85a1-4800-7000-8000-000000000001", "th", `{"reviewed_thread_id":"th","status":"approved",` +
				`"target_item_id":"c1","decision":"FEEDBACK-TEXT","action":"run"}`}}})
}

// codexRepeatSiblings returns the files beside the state database above: a
// log database with a /feedback report sent for the thread, and a thread
// history with a user message of it, a second and two seconds after the
// feedback record.
func codexRepeatSiblings(t *testing.T) []collectedFile {
	t.Helper()
	logs := buildDatabase(t, sqlitetest.Table{Name: "logs", SQL: "CREATE TABLE logs (ts INTEGER, " +
		"target TEXT, feedback_log_body TEXT, thread_id TEXT)", Rows: [][]any{
		{int64(1780358401), "codex_feedback", "feedback event uploaded to Sentry thread_id=th " +
			"uploaded_attachments=1", "th"}}})
	history := buildDatabase(t, sqlitetest.Table{Name: "thread_items", SQL: "CREATE TABLE thread_items " +
		"(thread_id TEXT, turn_id TEXT, item_id TEXT, rollout_ordinal INTEGER, created_at_ms INTEGER, " +
		"item_json TEXT)", Rows: [][]any{{"th", "u", "i1", int64(1), int64(1780358402000),
		`{"type":"userMessage","content":[{"type":"text","text":"HISTORY-TEXT"}]}`}}})
	return []collectedFile{
		{"alice", "/home/alice", "codex-cli", ".codex/logs_1.sqlite", logs},
		{"alice", "/home/alice", "codex-cli", ".codex/thread_history_1.sqlite", history}}
}

// repeatedIDRunOf runs timeline over a collection holding the files, for
// the agent given, with the options given, and returns each row of
// timeline.jsonl as "time session project branch text".
func repeatedIDRunOf(t *testing.T, agent string, files []collectedFile, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", agent}, options...), collection, output)
	code, _, stderr := runCommand(env, args...)
	if code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "git_branch", "text")
}

// repeatCase is one run over the stores of a repeated id: its name as a
// sentence, the options of the run and the rows it must give.
type repeatCase struct {
	name    string
	options []string
	want    []string
}

// The rows of the Codex stores above: each stored threads row's record,
// and the rows of the thread in the three databases, with the project and
// branch of one stored row.
const (
	codexRecordA = "2026-01-01T00:00:00.000Z th /proj/a br-a thread record: th title=A rollout=/r1 branch=br-a"
	codexRecordB = "2026-06-01T00:00:00.000Z th /proj/b br-b thread record: th title=B rollout=/r2 branch=br-b"
	codexReview  = "2026-06-02T00:00:00.000Z th %s permission decision: approved (auto_review feedback " +
		"model=-) exec c1 turn=- reviewer -: FEEDBACK-TEXT action: run"
	codexReport = "2026-06-02T00:00:01.000Z th %s session synced: feedback report uploaded to Sentry, " +
		"attachments 1 failed - (from logs_1.sqlite logs, ts)"
	codexHistory = "2026-06-02T00:00:02.000Z th %s HISTORY-TEXT"
)

// codexMembers returns the rows of the thread in the three databases, with
// the project and branch given.
func codexMembers(attributes string) []string {
	var rows []string
	for _, row := range []string{codexReview, codexReport, codexHistory} {
		rows = append(rows, strings.Replace(row, "%s", attributes, 1))
	}
	return rows
}

func TestTheRowsOfARepeatedCodexThreadGoWithTheFirstStoredRowTheFilterKeeps(t *testing.T) {
	first, second := codexMembers("/proj/a br-a"), codexMembers("/proj/b br-b")
	for _, test := range []repeatCase{
		{"without a filter they go with the first row of the id", nil,
			append([]string{codexRecordA, codexRecordB}, first...)},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"},
			append([]string{codexRecordB}, second...)},
		{"a pattern only the second row and the rows match moves them to the second",
			[]string{"--match", "title=B|-TEXT|Sentry"}, append([]string{codexRecordB}, second...)},
		{"a pattern only the first row and the rows match keeps them with the first",
			[]string{"--match", "title=A|-TEXT|Sentry"}, append([]string{codexRecordA}, first...)},
		{"a pattern neither row matches keeps them with the first", []string{"--match", "-TEXT|Sentry"}, first},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := append([]collectedFile{{"alice", "/home/alice", "codex-cli", ".codex/state_5.sqlite",
				codexRepeatState(t)}}, codexRepeatSiblings(t)...)
			rows := repeatedIDRunOf(t, "codex-cli", files, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// The log and history databases are read before or after the state
// database beside them, as the collection lists them; either way their
// rows go with the stored row the filter keeps.
func TestARepeatedCodexThreadsRowsMoveWhicheverDatabaseIsReadFirst(t *testing.T) {
	for _, test := range []repeatCase{
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"},
			append([]string{codexRecordB}, codexMembers("/proj/b br-b")...)},
		{"a pattern only the first row and the rows match keeps them with the first",
			[]string{"--match", "title=A|-TEXT|Sentry"}, append([]string{codexRecordA}, codexMembers("/proj/a br-a")...)},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := append(codexRepeatSiblings(t), collectedFile{"alice", "/home/alice", "codex-cli",
				".codex/state_5.sqlite", codexRepeatState(t)})
			rows := repeatedIDRunOf(t, "codex-cli", files, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// A thread history item that does not decode gives a problem row of its
// thread, with the project of the stored row the thread's group goes with.
func TestAProblemOfARepeatedCodexThreadTakesTheKeptStoredRowsProject(t *testing.T) {
	history := buildDatabase(t, sqlitetest.Table{Name: "thread_items", SQL: "CREATE TABLE thread_items " +
		"(thread_id TEXT, turn_id TEXT, item_id TEXT, rollout_ordinal INTEGER, created_at_ms INTEGER, " +
		"item_json TEXT)", Rows: [][]any{{"th", "u", "i1", int64(1), int64(1780358402000), "BROKEN-ITEM"}}})
	problem := " th %s  parser: unparseable record 1 (invalid JSON at line 1, column 1: invalid character 'B' " +
		"looking for beginning of value): BROKEN-ITEM"
	for _, test := range []repeatCase{
		{"a pattern only the problem matches keeps it with the first row", []string{"--match", "BROKEN"},
			[]string{strings.Replace(problem, "%s", "/proj/a", 1)}},
		{"a pattern the second row matches moves it to the second", []string{"--match", "title=B|BROKEN"},
			[]string{codexRecordB, strings.Replace(problem, "%s", "/proj/b", 1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := repeatedIDRunOf(t, "codex-cli", []collectedFile{
				{"alice", "/home/alice", "codex-cli", ".codex/state_5.sqlite", codexRepeatState(t)},
				{"alice", "/home/alice", "codex-cli", ".codex/thread_history_1.sqlite", history}}, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
