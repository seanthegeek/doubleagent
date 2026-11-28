package analyze

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// opencodeSessionSQL is the SQL of a session table (session or
// session_v2) whose id is not declared unique, so it can hold an id twice.
func opencodeSessionSQL(name string) string {
	return "CREATE TABLE " + name + " (id text, project_id text, directory text, title text, " +
		"time_created integer, time_updated integer, time_archived integer)"
}

// opencodeSessionRepeatV1 returns a 1.x database whose session table holds
// one session id in two rows, an old one in /proj/a and a new one in
// /proj/b archived a second later, and the session's user message, its
// text part and its share, each a day after the new row.
func opencodeSessionRepeatV1(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_x", nil, "/proj/a", "A", opencodeOld, opencodeOld, nil},
			{"ses_x", nil, "/proj/b", "B", opencodeNew, opencodeNew, opencodeNew + 1000}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_1", "ses_x", opencodePart, opencodePart, `{"role":"user"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"prt_1", "msg_1", "ses_x", opencodePart, opencodePart, `{"type":"text","text":"USER-TEXT"}`}}},
		sqlitetest.Table{Name: "session_share", SQL: "CREATE TABLE session_share (session_id text, id text, " +
			"url text, time_created integer)", Rows: [][]any{
			{"ses_x", "shr_1", "https://opncd.example/s/x", opencodePart + 1000}}})
}

// opencodeSessionRepeatV2 returns a 2.x database whose session_v2 holds
// one session id in two rows, an old one in /proj/c and a new one in
// /proj/d, and the session's user record a day after the new row; its
// session table holds another session in session_v2, ses_m, as a
// migration leaves it, in two rows, and both tables name it in one
// directory.
func opencodeSessionRepeatV2(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_v2", SQL: opencodeSessionSQL("session_v2"), Rows: [][]any{
			{"ses_y", nil, "/proj/c", "C", opencodeOld, opencodeOld, nil},
			{"ses_y", nil, "/proj/d", "D", opencodeNew, opencodeNew, nil},
			{"ses_m", nil, "/proj/m", "M", opencodeOld, opencodeOld, nil}}},
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_m", nil, "/proj/legacy", "M1", opencodeOld, opencodeOld, nil},
			{"ses_m", nil, "/proj/legacy", "M2", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_y", "ses_y", "user", 0, opencodePart, opencodePart, `{"text":"V2-TEXT"}`}}})
}

// opencodeDatabaseRun runs timeline over a collection holding the files,
// for the agent given, with the options given, and returns each row of
// timeline.jsonl as "time session project text".
func opencodeDatabaseRun(t *testing.T, agent string, files []collectedFile, options ...string) []string {
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
	return recordsOf(t, filepath.Join(output, timelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "text")
}

// opencodeSessionRun runs timeline over a collection holding the 1.x and
// 2.x databases with the session ids above, for the agent given at the
// paths given, with the options given, and returns each row of
// timeline.jsonl as "time session project text".
func opencodeSessionRun(t *testing.T, agent, legacyRel, currentRel string, options ...string) []string {
	t.Helper()
	return opencodeDatabaseRun(t, agent, []collectedFile{
		{"alice", "/home/alice", agent, legacyRel, opencodeSessionRepeatV1(t)},
		{"alice", "/home/alice", agent, currentRel, opencodeSessionRepeatV2(t)}}, options...)
}

// The rows of the OpenCode stores above: each stored session row's own
// rows, and the rows of its session with the project of one stored row.
const (
	sessionShown     = "parser: session holds 2 rows of this row's id; its messages and shares are shown once, with "
	sessionShownV2   = "parser: session_v2 holds 2 rows of this row's id; its messages and shares are shown once, with "
	sessionStartA    = "2026-01-01T00:00:00.000Z ses_x /proj/a session start: A directory=/proj/a"
	sessionStartB    = "2026-06-01T00:00:00.000Z ses_x /proj/b session start: B directory=/proj/b"
	sessionStartC    = "2026-01-01T00:00:00.000Z ses_y /proj/c session start: C directory=/proj/c"
	sessionStartD    = "2026-06-01T00:00:00.000Z ses_y /proj/d session start: D directory=/proj/d"
	sessionStartM    = "2026-01-01T00:00:00.000Z ses_m /proj/m session start: M directory=/proj/m"
	sessionRowA      = "2026-01-01T00:00:00.000Z ses_x /proj/a "
	sessionRowB      = "2026-06-01T00:00:00.000Z ses_x /proj/b "
	sessionRowC      = "2026-01-01T00:00:00.000Z ses_y /proj/c "
	sessionRowD      = "2026-06-01T00:00:00.000Z ses_y /proj/d "
	sessionArchivedB = "2026-06-01T00:00:01.000Z ses_x /proj/b session archived (from opencode.db session, " +
		"time_archived)"
	sessionMembers = "2026-06-02T00:00:00.000Z "
	sessionShare   = "2026-06-02T00:00:01.000Z "
)

// opencodeSessionCases returns the runs over the OpenCode stores above,
// for the database file name given: without a filter, with a window or a
// pattern that keeps only the later stored row of each id, with a
// pattern that keeps only the earlier, and with one that keeps neither.
func opencodeSessionCases(label string) []opencodeRepeatCase {
	archived := strings.Replace(sessionArchivedB, "opencode.db", label, 1)
	members := func(legacy, current string) []string {
		return []string{sessionMembers + "ses_y " + current + " V2-TEXT",
			sessionMembers + "ses_x " + legacy + " USER-TEXT",
			sessionShare + "ses_x " + legacy + " session synced: share shr_1 url https://opncd.example/s/x (from " +
				label + " session_share, time_created)"}
	}
	return []opencodeRepeatCase{
		{"without a filter they go with the first row of the id", nil, append([]string{
			sessionStartC, sessionRowC + sessionShownV2 + "this row", sessionStartM, sessionStartA,
			sessionRowA + sessionShown + "this row", sessionStartD, sessionRowD + sessionShownV2 + "another of them",
			sessionStartB, sessionRowB + sessionShown + "another of them", archived}, members("/proj/a", "/proj/c")...)},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"}, append([]string{
			sessionStartD, sessionRowD + sessionShownV2 + "this row", sessionStartB, sessionRowB + sessionShown + "this row",
			archived}, members("/proj/b", "/proj/d")...)},
		{"a pattern only the second row's rows match moves them to the second",
			[]string{"--match", "directory=/proj/[bd]|-TEXT"}, append([]string{sessionStartD, sessionStartB},
				members("/proj/b", "/proj/d")[:2]...)},
		{"a pattern only the first row's rows match keeps them with the first",
			[]string{"--match", "directory=/proj/[ac]|-TEXT"}, append([]string{sessionStartC, sessionStartA},
				members("/proj/a", "/proj/c")[:2]...)},
		{"a pattern neither row's rows match keeps them with the first", []string{"--match", "-TEXT"},
			members("/proj/a", "/proj/c")[:2]},
	}
}

func TestTheRowsOfARepeatedOpenCodeSessionGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	for _, test := range opencodeSessionCases("opencode.db") {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeSessionRun(t, "opencode", ".local/share/opencode/opencode.db",
				".local/share/opencode/opencode-dev.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// Kilo Code's CLI keeps OpenCode's stores under its own directory, read
// by the same reader, so a repeated session id there is shown the same way.
func TestTheRowsOfARepeatedKiloCodeSessionGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	for _, test := range opencodeSessionCases("kilo.db") {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeSessionRun(t, "kilo-code", ".local/share/kilo/kilo.db",
				".local/share/kilo/kilo-dev.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// opencodeDoubleRepeat returns a 2.x database whose session_v2 holds one
// session id in two rows, an old one in /proj/c and a new one in /proj/d,
// and whose session_message holds, in that session, a user record and an
// assistant message id in two records with one session_part item, all a
// day after the new row.
func opencodeDoubleRepeat(t *testing.T) string {
	t.Helper()
	assistant := `{"content":[{"type":"text","text":"OWN"}],"model":{"providerID":"p","id":"m"}}`
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_v2", SQL: opencodeSessionSQL("session_v2"), Rows: [][]any{
			{"ses_y", nil, "/proj/c", "C", opencodeOld, opencodeOld, nil},
			{"ses_y", nil, "/proj/d", "D", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_y", "ses_y", "user", 0, opencodePart, opencodePart, `{"text":"V2-TEXT"}`},
			{"msg_v", "ses_y", "assistant", 1, opencodePart, opencodePart, assistant},
			{"msg_v", "ses_y", "assistant", 2, opencodePart, opencodePart, assistant}}},
		sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, " +
			"session_id text, type text, ordinal integer, data text)", Rows: [][]any{
			{"prt_v", "msg_v", "ses_y", "text", 0, `{"text":"ITEM-TEXT"}`}}})
}

// A row holds one place in one group, so the rows of a message id
// repeated in a session whose id is repeated too stay with the message's
// group, which has the session's first stored row's project; the
// session's other rows go with the stored row the filter keeps.
func TestARepeatedMessageInARepeatedSessionKeepsTheFirstSessionRowsProject(t *testing.T) {
	rows := opencodeDatabaseRun(t, "opencode", []collectedFile{{"alice", "/home/alice", "opencode",
		".local/share/opencode/opencode.db", opencodeDoubleRepeat(t)}}, "--since", "2026-03-01")
	message := "parser: session_message holds 2 rows of this row's id; its parts are shown once, with "
	want := []string{sessionStartD, sessionRowD + sessionShownV2 + "this row",
		sessionMembers + "ses_y /proj/d V2-TEXT", sessionMembers + "ses_y /proj/c " + message + "this row",
		sessionMembers + "ses_y /proj/c model: m provider=p", sessionMembers + "ses_y /proj/c ITEM-TEXT",
		sessionMembers + "ses_y /proj/c " + message + "another of them", sessionMembers + "ses_y /proj/c OWN"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}

// opencodeUnplacedRepeat returns a 1.x database whose session table holds
// one session id in two rows, an old one naming no directory and a new
// one in /proj/b, and two user messages of the session a day after the
// new row: one with its own path.cwd, /own, and one without.
func opencodeUnplacedRepeat(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_x", nil, nil, "A", opencodeOld, opencodeOld, nil},
			{"ses_x", nil, "/proj/b", "B", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_1", "ses_x", opencodePart, opencodePart, `{"role":"user","path":{"cwd":"/own"}}`},
			{"msg_2", "ses_x", opencodePart + 1000, opencodePart + 1000, `{"role":"user"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"prt_1", "msg_1", "ses_x", opencodePart, opencodePart, `{"type":"text","text":"CWD-TEXT"}`},
			{"prt_2", "msg_2", "ses_x", opencodePart + 1000, opencodePart + 1000, `{"type":"text","text":"BARE-TEXT"}`}}})
}

// A 1.x message of a session whose first stored row names no project
// takes its own path.cwd, and keeps it whichever stored row of a repeated
// session id the filter keeps; a message without one takes the project of
// the stored row the filter keeps.
func TestAMessagesOwnDirectoryStaysWhenTheFirstSessionRowNamesNone(t *testing.T) {
	for _, test := range []opencodeRepeatCase{
		{"with a pattern both rows' start rows match", nil, []string{
			"2026-01-01T00:00:00.000Z ses_x  session start: A", sessionStartB,
			"2026-06-02T00:00:00.000Z ses_x /own CWD-TEXT", "2026-06-02T00:00:01.000Z ses_x  BARE-TEXT"}},
		{"with a window past the first row", []string{"--since", "2026-03-01"}, []string{sessionStartB,
			"2026-06-02T00:00:00.000Z ses_x /own CWD-TEXT", "2026-06-02T00:00:01.000Z ses_x /proj/b BARE-TEXT"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeDatabaseRun(t, "opencode", []collectedFile{{"alice", "/home/alice", "opencode",
				".local/share/opencode/opencode.db", opencodeUnplacedRepeat(t)}},
				append([]string{"--match", "session start|-TEXT"}, test.options...)...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// opencodeDamagedRepeat returns a 1.x database whose session table holds
// one session id in two rows, an old one in /proj/a and a new one in
// /proj/b whose permission value is not a JSON array, and a message of the
// session whose data does not decode.
func opencodeDamagedRepeat(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: "CREATE TABLE session (id text, directory text, title text, " +
			"time_created integer, time_updated integer, permission text)", Rows: [][]any{
			{"ses_x", "/proj/a", "A", opencodeOld, opencodeOld, nil},
			{"ses_x", "/proj/b", "B", opencodeNew, opencodeNew, "RULES"}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_1", "ses_x", opencodePart, opencodePart, "MESSAGE"}}})
}

// A problem of one stored session row is one of that row's own rows, with
// its project; a problem of the session's records is a row of the
// session, with the project of the stored row its group goes with.
func TestTheProblemsOfARepeatedSessionTakeTheirOwnersProject(t *testing.T) {
	message := " ses_x %s parser: unparseable record 1 (message.data: invalid JSON at line 1, column 1: " +
		"invalid character 'M' looking for beginning of value): MESSAGE"
	permission := " ses_x /proj/b parser: unparseable record 2 (session.permission: not a JSON array): RULES"
	for _, test := range []opencodeRepeatCase{
		{"a pattern only the record's problem matches keeps it with the first row", []string{"--match", "MESSAGE"},
			[]string{fmt.Sprintf(message, "/proj/a")}},
		{"a pattern the second row's problem matches moves it to the second", []string{"--match", "unparseable"},
			[]string{fmt.Sprintf(message, "/proj/b"), permission}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeDatabaseRun(t, "opencode", []collectedFile{{"alice", "/home/alice", "opencode",
				".local/share/opencode/opencode.db", opencodeDamagedRepeat(t)}}, test.options...)
			slices.Sort(rows)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
