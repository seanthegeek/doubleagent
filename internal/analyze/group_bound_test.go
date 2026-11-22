package analyze

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// longValue is a value of a stored row longer than the bound the kit cuts
// every field but the text to.
var longValue = "/" + strings.Repeat("a", 100_000)

// opencodeLongDirectory returns a 1.x OpenCode database whose session table
// holds one session id in two rows, each with a directory of longValue's
// length, and n messages of the session with one text part each.
func opencodeLongDirectory(t *testing.T, n int) string {
	t.Helper()
	var messages, parts [][]any
	for i := range n {
		message := "msg_" + strconv.Itoa(i)
		messages = append(messages, []any{message, "ses_x", opencodePart + i, opencodePart + i, `{"role":"assistant"}`})
		parts = append(parts, []any{"prt_" + strconv.Itoa(i), message, "ses_x", opencodePart + i, opencodePart + i,
			`{"type":"text","text":"hi"}`})
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"ses_x", nil, longValue, "A", opencodeOld, opencodeOld, nil},
			{"ses_x", nil, longValue + "b", "B", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: messages},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: parts})
}

// ollamaLongModel returns an Ollama database whose messages repeat one
// assistant message id, each with a model name of longValue's length, and
// n tool calls of the id, which take the model of the message the group
// goes with.
func ollamaLongModel(t *testing.T, n int) string {
	t.Helper()
	var calls [][]any
	for i := range n {
		calls = append(calls, []any{i + 1, 1, "search", `{}`})
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, chat_id TEXT, role TEXT, " +
			"content TEXT, thinking TEXT, model_name TEXT, created_at TEXT)", Rows: [][]any{
			{1, "c", "assistant", "a", "", longValue, "2026-01-01 00:00:00"},
			{1, "c", "assistant", "b", "", longValue + "x", "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "tool_calls", SQL: "CREATE TABLE tool_calls (id INTEGER, message_id INTEGER, " +
			"function_name TEXT, function_arguments TEXT)", Rows: calls})
}

// codexLongCwd returns a Codex state database whose threads table holds
// one thread id in two rows, each with a cwd of longValue's length, and a
// guardian review of the thread, a member read in the same parse, and, as
// the state database's neighbour, a log database with n /feedback
// reports of the thread, which are the members of the id's group; the
// state database first, or last with stateLast set.
func codexLongCwd(t *testing.T, n int, stateLast bool) []collectedFile {
	t.Helper()
	state := buildDatabase(t, sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id TEXT, " +
		"rollout_path TEXT, created_at INTEGER, cwd TEXT, title TEXT, git_branch TEXT, first_user_message TEXT)",
		Rows: [][]any{{"th", "/r1", int64(1767225600), longValue, "A", "br-a", nil},
			{"th", "/r2", int64(1780272000), longValue + "b", "B", "br-b", nil}}},
		sqlitetest.Table{Name: "guardian_review_feedback", SQL: "CREATE TABLE guardian_review_feedback " +
			"(id TEXT, thread_id TEXT, record TEXT)", Rows: [][]any{
			{"019e85a1-4800-7000-8000-000000000001", "th", `{"reviewed_thread_id":"th","status":"approved",` +
				`"target_item_id":"c1","decision":"FEEDBACK-TEXT","action":"run"}`}}})
	var reports [][]any
	for i := range n {
		reports = append(reports, []any{int64(1780358401 + i), "codex_feedback",
			"feedback event uploaded to Sentry thread_id=th uploaded_attachments=1", "th"})
	}
	logs := buildDatabase(t, sqlitetest.Table{Name: "logs", SQL: "CREATE TABLE logs (ts INTEGER, " +
		"target TEXT, feedback_log_body TEXT, thread_id TEXT)", Rows: reports})
	files := []collectedFile{{"alice", "/home/alice", "codex-cli", ".codex/state_5.sqlite", state},
		{"alice", "/home/alice", "codex-cli", ".codex/logs_1.sqlite", logs}}
	if stateLast {
		files[0], files[1] = files[1], files[0]
	}
	return files
}

// copilotLongCwd returns a Copilot CLI session store whose sessions table
// holds one session id in two rows, each with a cwd of longValue's length,
// and n turns of the session with a prompt alone, which take the project
// of the stored row the group goes with.
func copilotLongCwd(t *testing.T, n int) []collectedFile {
	t.Helper()
	var turns [][]any
	for i := range n {
		turns = append(turns, []any{"s1", int64(i), "ask", nil, "2026-06-02T00:00:00.000Z"})
	}
	store := buildDatabase(t,
		sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, cwd TEXT, branch TEXT, " +
			"summary TEXT, created_at TEXT)", Rows: [][]any{
			{"s1", longValue, "br-a", "Name A", "2026-01-01T00:00:00.000Z"},
			{"s1", longValue + "b", "br-b", "Name B", "2026-06-01T00:00:00.000Z"}}},
		sqlitetest.Table{Name: "turns", SQL: "CREATE TABLE turns (session_id TEXT, turn_index INTEGER, " +
			"user_message TEXT, assistant_response TEXT, timestamp TEXT)", Rows: turns})
	return []collectedFile{{"alice", "/home/alice", "copilot-cli", ".copilot/session-store.db", store}}
}

// timelineBytes runs timeline over a collection holding files with the
// options given and returns the size of timeline.jsonl.
func timelineBytes(t *testing.T, files []collectedFile, options ...string) int {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	data, err := osfile.ReadFile(filepath.Join(output, TimelineFile))
	if err != nil {
		t.Fatal(err)
	}
	return len(data)
}

// The members of a repeated id take their parent's fields as the kit cut
// them on the parent's own row, so a long project or model of a stored row
// costs its cut length in each member, whichever stored row the group goes
// with: each member costs about the bound, not the whole value.
func TestTheMembersOfARepeatedIDTakeTheirParentsFieldsCut(t *testing.T) {
	t.Parallel()
	stores := []struct {
		name  string
		files func(t *testing.T, n int) []collectedFile
		later string
	}{
		{"an OpenCode session's directory", func(t *testing.T, n int) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode.db",
				opencodeLongDirectory(t, n)}}
		}, "2026-03-01"},
		{"an Ollama message's model", func(t *testing.T, n int) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "ollama", "Library/Application Support/Ollama/db.sqlite",
				ollamaLongModel(t, n)}}
		}, "2026-03-01"},
		{"a Codex thread's cwd, its members in the log database read after it",
			func(t *testing.T, n int) []collectedFile { return codexLongCwd(t, n, false) }, "2026-03-01"},
		{"a Codex thread's cwd, its members in the log database read before it",
			func(t *testing.T, n int) []collectedFile { return codexLongCwd(t, n, true) }, "2026-03-01"},
		{"a Copilot CLI session's cwd", copilotLongCwd, "2026-03-01"},
	}
	for _, store := range stores {
		for _, options := range [][]string{nil, {"--since", store.later}} {
			t.Run(store.name+" "+strings.Join(options, " "), func(t *testing.T) {
				small, large := timelineBytes(t, store.files(t, 100), options...),
					timelineBytes(t, store.files(t, 200), options...)
				if perMember := (large - small) / 100; perMember > 2*parser.CarriedLimit {
					t.Errorf("100 members wrote %d bytes and 200 wrote %d: %d a member", small, large, perMember)
				}
			})
		}
	}
}

// A cut is reported by the rows of the stored row that holds the value,
// never by the members that take it, so a repeated Codex thread's long cwd
// gives one problem row counting the two stored rows, whichever of the
// state and log databases is read first.
func TestAMembersCutIsNotCountedWhicheverFileIsReadFirst(t *testing.T) {
	want := []string{"/home/alice/.codex/state_5.sqlite parser: project_path longer than 4096 bytes, " +
		"cut to 4096 bytes (2 times)"}
	for _, stateLast := range []bool{false, true} {
		t.Run("state database last "+strconv.FormatBool(stateLast), func(t *testing.T) {
			work := t.TempDir()
			collection := writeCollection(t, filepath.Join(work, "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, codexLongCwd(t, 3, stateLast))
			env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
				Parsers: parsers.New, Version: "0.0.0-test"}
			output := filepath.Join(work, "out")
			if code, _, stderr := runCommand(env, "timeline", collection, output); code != ExitFound {
				t.Fatal(stderr)
			}
			var got []string
			for _, row := range recordsOf(t, filepath.Join(output, TimelineFile), "text", "source_file", "text") {
				if strings.Contains(row, " cut to ") {
					got = append(got, row)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("problem rows %q, want %q", got, want)
			}
		})
	}
}
