package analyze

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// copilotRepeatStore returns a session store whose sessions table holds
// one session id in two rows, an old one in /proj/a on br-a and a new one
// in /proj/b on br-b, and the session's turn and changed file a day after
// the new row.
func copilotRepeatStore(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, cwd TEXT, branch TEXT, " +
			"summary TEXT, created_at TEXT)", Rows: [][]any{
			{"s1", "/proj/a", "br-a", "Name A", "2026-01-01T00:00:00.000Z"},
			{"s1", "/proj/b", "br-b", "Name B", "2026-06-01T00:00:00.000Z"}}},
		sqlitetest.Table{Name: "turns", SQL: "CREATE TABLE turns (session_id TEXT, turn_index INTEGER, " +
			"user_message TEXT, assistant_response TEXT, timestamp TEXT)", Rows: [][]any{
			{"s1", int64(0), "USER-TEXT", "REPLY-TEXT", "2026-06-02T00:00:00.000Z"}}},
		sqlitetest.Table{Name: "session_files", SQL: "CREATE TABLE session_files (session_id TEXT, " +
			"file_path TEXT, tool_name TEXT, first_seen_at TEXT)", Rows: [][]any{
			{"s1", "/src/f", "edit", "2026-06-02T00:00:01.000Z"}}})
}

// The rows of the session store above: each stored sessions row's own
// rows, and the session's rows with the project and branch of one stored
// row.
const (
	copilotShown = "parser: sessions holds 2 rows of this row's id; its turns, files, references, " +
		"checkpoints and tool events are shown once, with "
	copilotRowA = "2026-01-01T00:00:00.000Z s1 /proj/a br-a "
	copilotRowB = "2026-06-01T00:00:00.000Z s1 /proj/b br-b "
)

// copilotMembers returns the session's rows in the store above, with the
// project and branch given.
func copilotMembers(attributes string) []string {
	return []string{"2026-06-02T00:00:00.000Z s1 " + attributes + " USER-TEXT",
		"2026-06-02T00:00:00.000Z s1 " + attributes + " REPLY-TEXT",
		"2026-06-02T00:00:01.000Z s1 " + attributes + " file changed: /src/f tool=edit"}
}

func TestTheRowsOfARepeatedCopilotCLISessionGoWithTheFirstStoredRowTheFilterKeeps(t *testing.T) {
	first, second := copilotMembers("/proj/a br-a"), copilotMembers("/proj/b br-b")
	for _, test := range []repeatCase{
		{"without a filter they go with the first row of the id", nil, append([]string{
			copilotRowA + "session name: Name A", copilotRowA + copilotShown + "this row",
			copilotRowB + "session name: Name B", copilotRowB + copilotShown + "another of them"}, first...)},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"},
			append([]string{copilotRowB + "session name: Name B", copilotRowB + copilotShown + "this row"},
				second...)},
		{"a pattern only the second row and the rows match moves them to the second",
			[]string{"--match", "Name B|-TEXT|changed"}, append([]string{copilotRowB + "session name: Name B"},
				second...)},
		{"a pattern only the first row and the rows match keeps them with the first",
			[]string{"--match", "Name A|-TEXT|changed"}, append([]string{copilotRowA + "session name: Name A"},
				first...)},
		{"a pattern neither row matches keeps them with the first", []string{"--match", "-TEXT|changed"}, first},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := repeatedIDRunOf(t, "copilot-cli", []collectedFile{{"alice", "/home/alice", "copilot-cli",
				".copilot/session-store.db", copilotRepeatStore(t)}}, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// A stored row of a repeated session id with no summary gives no session
// name, but its `parser:` row still shows it, with its own project, and
// the session's rows go with it when the filter keeps only that row.
func TestAStoredSessionRowWithNoSummaryStillShowsItsRepeat(t *testing.T) {
	store := buildDatabase(t,
		sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, cwd TEXT, branch TEXT, " +
			"summary TEXT, created_at TEXT)", Rows: [][]any{
			{"s1", "/proj/a", "br-a", "Name A", "2026-01-01T00:00:00.000Z"},
			{"s1", "/proj/b", "br-b", nil, "2026-06-01T00:00:00.000Z"}}},
		sqlitetest.Table{Name: "turns", SQL: "CREATE TABLE turns (session_id TEXT, turn_index INTEGER, " +
			"user_message TEXT, assistant_response TEXT, timestamp TEXT)", Rows: [][]any{
			{"s1", int64(0), "USER-TEXT", nil, "2026-06-02T00:00:00.000Z"}}})
	rows := repeatedIDRunOf(t, "copilot-cli", []collectedFile{{"alice", "/home/alice", "copilot-cli",
		".copilot/session-store.db", store}}, "--since", "2026-03-01")
	want := []string{copilotRowB + copilotShown + "this row", "2026-06-02T00:00:00.000Z s1 /proj/b br-b USER-TEXT"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}
