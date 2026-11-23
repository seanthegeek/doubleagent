package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserregistry"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// opencodeProjectRepeat returns a 1.x database whose project table holds
// the id prj_1 in two rows, as only damage writes: /proj/a and, five
// months later, /proj/b; a session of the project naming no directory of
// its own, with a user message, a saved grant of the project and a
// worktree OpenCode made for it, each a day after the later row.
func opencodeProjectRepeat(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "project", SQL: "CREATE TABLE project (id text, worktree text, " +
			"time_created integer, time_updated integer)", Rows: [][]any{
			{"prj_1", "/proj/a", opencodeOld, opencodeOld}, {"prj_1", "/proj/b", opencodeNew, opencodeNew}}},
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_p", "prj_1", nil, "P", opencodePart, opencodePart, nil}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_1", "ses_p", opencodePart, opencodePart, `{"role":"user"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"prt_1", "msg_1", "ses_p", opencodePart, opencodePart, `{"type":"text","text":"PROJECT-TEXT"}`}}},
		sqlitetest.Table{Name: "permission", SQL: "CREATE TABLE permission (id text, project_id text, " +
			"action text, resource text, time_created integer)", Rows: [][]any{
			{"per_1", "prj_1", "bash", "ls", opencodePart}}},
		sqlitetest.Table{Name: "worktree", SQL: "CREATE TABLE worktree (directory text, project_id text, " +
			"strategy text, time_created integer)", Rows: [][]any{{"/wt", "prj_1", nil, opencodePart}}})
}

// opencodeProjectRun runs timeline over a collection holding
// opencodeProjectRepeat at rel, for the agent given, with the options
// given, and returns every row of timeline.jsonl as "time session project
// text".
func opencodeProjectRun(t *testing.T, agent, rel string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", agent, rel, opencodeProjectRepeat(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parserregistry.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", agent}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc", "session_id",
		"project_path", "text")
}

// opencodeProjectCases returns the runs over opencodeProjectRepeat in the
// database file name given: without a filter, with a window that keeps
// only the later stored row or only the earlier, and with a pattern that
// keeps neither. The stored rows give only their `parser:` rows, whose
// texts a pattern matches alike, so only a window tells them apart.
func opencodeProjectCases(label string) []opencodeRepeatCase {
	const (
		notice = "parser: project holds 2 rows of this row's id; its sessions and saved grants are shown once, with "
		old    = "2026-01-01T00:00:00.000Z  /proj/a " + notice
		newer  = "2026-06-01T00:00:00.000Z  /proj/b " + notice
		joined = "2026-06-02T00:00:00.000Z "
	)
	worktree := joined + " /wt scratch directory: /wt (git worktree of project prj_1 strategy=-) (from " + label +
		" worktree, time_created)"
	members := func(path string) []string {
		return []string{joined + "ses_p " + path + " session start: P", worktree,
			joined + " " + path + " permission setting: bash ls=allow (from " + label + " permission, time_created)",
			joined + "ses_p " + path + " PROJECT-TEXT"}
	}
	matched := func(path string) []string {
		return []string{joined + "ses_p " + path + " session start: P", joined + "ses_p " + path + " PROJECT-TEXT"}
	}
	return []opencodeRepeatCase{
		{"without a filter they go with the first row", nil,
			slices.Concat([]string{old + "this row", newer + "another of them"}, members("/proj/a"))},
		{"a window that keeps only the later row moves them to it", []string{"--since", "2026-03-01"},
			slices.Concat([]string{newer + "this row"}, members("/proj/b"))},
		{"a window that keeps only the earlier row keeps them with it", []string{"--until", "2026-03-01"},
			[]string{old + "this row"}},
		{"a pattern neither row matches leaves them with the first", []string{"--match", "P$|-TEXT"},
			matched("/proj/a")},
	}
}

func TestTheRowsOfARepeatedOpenCodeProjectGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	for _, test := range opencodeProjectCases("opencode.db") {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeProjectRun(t, "opencode", ".local/share/opencode/opencode.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// Kilo Code's CLI keeps OpenCode's stores under its own directory, read
// by the same reader, so a repeated project id there is shown the same way.
func TestTheRowsOfARepeatedKiloCodeProjectGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	for _, test := range opencodeProjectCases("kilo.db") {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeProjectRun(t, "kilo-code", ".local/share/kilo/kilo.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// opencodeProjectAndSessionRepeat returns a 1.x database whose project
// table holds prj_1 in two rows, /proj/a and, five months later, /proj/b,
// and whose session table holds a session of the project naming no
// directory of its own in two rows, with a user message, a day after the
// later project row.
func opencodeProjectAndSessionRepeat(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "project", SQL: "CREATE TABLE project (id text, worktree text, " +
			"time_created integer, time_updated integer)", Rows: [][]any{
			{"prj_1", "/proj/a", opencodeOld, opencodeOld}, {"prj_1", "/proj/b", opencodeNew, opencodeNew}}},
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_q", "prj_1", nil, "Q1", opencodePart, opencodePart, nil},
			{"ses_q", "prj_1", nil, "Q2", opencodePart, opencodePart, nil}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_1", "ses_q", opencodePart, opencodePart, `{"role":"user"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"prt_1", "msg_1", "ses_q", opencodePart, opencodePart, `{"type":"text","text":"Q-TEXT"}`}}})
}

// A row holds one place in one group, so the rows of a session whose id
// and whose project's id both repeat follow the session's group alone,
// and its stored rows, the group's parents, keep the worktree of the
// project's first stored row, whichever project row the filter keeps.
func TestARepeatedSessionOfARepeatedProjectKeepsTheFirstProjectRowsWorktree(t *testing.T) {
	rows := opencodeDatabaseRun(t, "opencode", []collectedFile{{"alice", "/home/alice", "opencode",
		".local/share/opencode/opencode.db", opencodeProjectAndSessionRepeat(t)}}, "--since", "2026-03-01")
	const session = "parser: session holds 2 rows of this row's id; its messages and shares are shown once, with "
	joined := "2026-06-02T00:00:00.000Z ses_q /proj/a "
	want := []string{joined + "session start: Q1", joined + session + "this row", joined + "Q-TEXT",
		joined + "session start: Q2", joined + session + "another of them"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}
