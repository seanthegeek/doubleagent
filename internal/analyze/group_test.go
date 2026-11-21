package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// repeatedIDFiles returns a collection whose stores repeat an id: a Hermes
// state.db with two sessions rows of one id, an old one and a new one five
// months later, with usage rows, messages and a deletion fence of the id;
// and a nanobot session database with two rows of one key, with
// their presets, and an undated and a dated message of the key.
func repeatedIDFiles(t *testing.T) []collectedFile {
	t.Helper()
	read := func(parts ...string) string {
		data, err := osfile.ReadFile(filepath.Join(append([]string{"..", "parser"}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return []collectedFile{
		{"alice", "/home/alice", "hermes", ".hermes/state.db", read("hermes", "testdata", "repeated_session_rows.db")},
		{"alice", "/home/alice", "nanobot", ".nanobot/sessions/0123456789abcdef0123456789abcdef/sessions.sqlite3",
			read("nanobot", "testdata", "repeated_key_presets.db")},
	}
}

// repeatedIDRun runs timeline over the repeated-id collection with the
// options given and returns each row of timeline.jsonl that has a session
// as "agent time project branch model text", each line of sessions.jsonl
// as "agent session project first last", and the summary printed.
func repeatedIDRun(t *testing.T, options ...string) ([]string, []string, string) {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, repeatedIDFiles(t))
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	code, stdout, stderr := runCommand(env, args...)
	if code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	rows := recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "agent", "timestamp_utc",
		"project_path", "git_branch", "model", "text")
	sessions := recordsOf(t, filepath.Join(output, SessionsFile), "session_id", "agent", "session_id",
		"project_path", "first_timestamp_utc", "last_timestamp_utc")
	return rows, sessions, stdout
}

// recordsOf returns, for each line of a JSONL file whose required field
// is not empty, the named fields joined by a space.
func recordsOf(t *testing.T, path, required string, fields ...string) []string {
	t.Helper()
	data, err := osfile.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		record, err := jsonvalue.DecodeString(line)
		if err != nil {
			t.Fatal(err)
		}
		if record.Get(required).StringValue() == "" {
			continue
		}
		var values []string
		for _, field := range fields {
			values = append(values, record.Get(field).StringValue())
		}
		lines = append(lines, strings.Join(values, " "))
	}
	return lines
}

// The attributes the rows of a repeated id are shown with: each parent's
// own, at the parent's time or at a time of the row's own.
const (
	hermesOld   = "hermes 2026-01-01T00:00:00.000Z /proj/old branch-old "
	hermesNew   = "hermes 2026-06-01T00:00:00.000Z /proj/new branch-new "
	nanobotOld  = "nanobot 2026-01-01T00:00:00.000Z   "
	nanobotNew  = "nanobot 2026-06-01T00:00:00.000Z   "
	early       = "hermes 2026-02-01T00:00:00.000Z "
	dated       = "hermes 2026-06-02T00:00:00.000Z "
	late        = "hermes 2026-06-03T00:00:00.000Z "
	deleted     = "hermes 2026-06-04T00:00:00.000Z "
	nanobotHi   = "nanobot 2026-06-02T00:00:00.000Z   "
	deletedText = "session deleted: s1 (from state.db state_meta gateway.retired_session.v1, retired_at)"
)

func TestTheRowsOfARepeatedIDGoWithTheFirstParentTheFilterKeeps(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter they go with the first row of the id", nil, []string{
			hermesOld + "model-old session start | old | source=cli", hermesOld + "usage-undated model: usage-undated",
			nanobotOld + "model-old session start: key=k", nanobotOld + "model-old model: model-old (preset)",
			nanobotOld + "model-old undated hello", early + "/proj/old branch-old model-old early message",
			hermesNew + "model-new session start | new | source=cli",
			hermesNew + "model-new model: usage-dated -> model-new", nanobotNew + "model-new session start: key=k",
			nanobotNew + "model-new model: model-new (preset)",
			dated + "/proj/old branch-old usage-dated model: usage-undated -> usage-dated",
			nanobotHi + "model-old dated hi", late + "/proj/old branch-old model-old late message",
			deleted + "/proj/old branch-old model-old " + deletedText}},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"}, []string{
			hermesNew + "usage-undated model: usage-undated", hermesNew + "model-new session start | new | source=cli",
			hermesNew + "model-new model: usage-dated -> model-new", nanobotNew + "model-new undated hello",
			nanobotNew + "model-new session start: key=k", nanobotNew + "model-new model: model-new (preset)",
			dated + "/proj/new branch-new usage-dated model: usage-undated -> usage-dated",
			nanobotHi + "model-new dated hi", late + "/proj/new branch-new model-new late message",
			deleted + "/proj/new branch-new model-new " + deletedText}},
		{"a window before the second row keeps them with the first", []string{"--until", "2026-03-01"}, []string{
			hermesOld + "model-old session start | old | source=cli", hermesOld + "usage-undated model: usage-undated",
			nanobotOld + "model-old session start: key=k", nanobotOld + "model-old model: model-old (preset)",
			nanobotOld + "model-old undated hello", early + "/proj/old branch-old model-old early message"}},
		{"a pattern the first row does not match moves them to the second",
			[]string{"--match", "new|usage|message|deleted|hello|hi"}, []string{
				early + "/proj/new branch-new model-new early message", hermesNew + "usage-undated model: usage-undated",
				hermesNew + "model-new session start | new | source=cli",
				hermesNew + "model-new model: usage-dated -> model-new", nanobotNew + "model-new undated hello",
				nanobotNew + "model-new model: model-new (preset)",
				dated + "/proj/new branch-new usage-dated model: usage-undated -> usage-dated",
				nanobotHi + "model-new dated hi", late + "/proj/new branch-new model-new late message",
				deleted + "/proj/new branch-new model-new " + deletedText}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, sessions, _ := repeatedIDRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
			checkSessionsAgree(t, rows, sessions)
			withheld, withheldSessions, _ := repeatedIDRun(t, append([]string{"--no-secrets"}, test.options...)...)
			if !slices.Equal(withheld, rows) || !slices.Equal(withheldSessions, sessions) {
				t.Errorf("withholding secrets changed the rows: %q, sessions %q", withheld, withheldSessions)
			}
		})
	}
}

// checkSessionsAgree checks that each session of sessions.jsonl spans the
// times of the rows kept of its agent and names the project of one of them.
func checkSessionsAgree(t *testing.T, rows, sessions []string) {
	t.Helper()
	for _, session := range sessions {
		fields := strings.Split(session, " ")
		agent, project, first, last := fields[0], fields[2], fields[3], fields[4]
		var times, projects []string
		for _, row := range rows {
			if values := strings.Split(row, " "); values[0] == agent {
				times, projects = append(times, values[1]), append(projects, values[2])
			}
		}
		if len(times) == 0 || first != slices.Min(times) || last != slices.Max(times) ||
			!slices.Contains(projects, project) {
			t.Errorf("session %q does not agree with the rows %q", session, rows)
		}
	}
}

// Choosing the parents runs the filter once more over the rows as the
// parsers gave them, and that pass counts nothing: the summary counts
// each row the filter drops once: the rows of each id's first row and the
// early message outside the window, and the two problem rows undated.
func TestChoosingTheParentsCountsNoRowAsFiltered(t *testing.T) {
	_, _, stdout := repeatedIDRun(t, "--since", "2026-03-01")
	if want := "filtered:  4 outside the window, 2 without a timestamp, 0 not matching\n"; !strings.Contains(stdout, want) {
		t.Errorf("summary %q, want the line %q", stdout, want)
	}
}

// zedWorktreeRun runs timeline with the options given over a collection
// holding Zed's testdata/repeated_worktree_2.db: two archived worktree rows
// of one id, at /w/copy0 and /w/copy1, and the threads t000 and t001 of
// the sessions s000 and s001 linked to the id. It returns each archived
// worktree row of timeline.jsonl as "session project worktree".
func zedWorktreeRun(t *testing.T, options ...string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("..", "parser", "zed", "testdata", "repeated_worktree_2.db"))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "zed", ".local/share/zed/db/0-stable/db.sqlite", string(data)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	var rows []string
	for _, line := range recordsOf(t, filepath.Join(output, TimelineFile), "text", "session_id", "project_path",
		"text") {
		if session, rest, found := strings.Cut(line, " /w/main archived worktree: "); found {
			worktree, _, _ := strings.Cut(rest, " ")
			rows = append(rows, session+" /w/main "+worktree)
		}
	}
	return rows
}

func TestTheLinksOfARepeatedZedWorktreeIDGoWithTheStoredRowTheMatchKeeps(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter they go with the first stored row", nil,
			[]string{"s000 /w/main /w/copy0", "s001 /w/main /w/copy0", " /w/main /w/copy1"}},
		{"a match on the later stored row moves them to it", []string{"--match", "/w/copy1 "},
			[]string{"s000 /w/main /w/copy1", "s001 /w/main /w/copy1"}},
		{"a match on the first stored row keeps them with it", []string{"--match", "/w/copy0 "},
			[]string{"s000 /w/main /w/copy0", "s001 /w/main /w/copy0"}},
		{"a match on neither stored row shows none of them", []string{"--match", "/w/absent|sidebar thread"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := zedWorktreeRun(t, test.options...); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

// sharedIDChat is an Agent Zero chat.json whose log names each of three
// history messages, a user's text, a tool result and a reply, in two log
// items: one on the first of January, one on the first of June.
const sharedIDChat = `{"id":"ctx","agents":[{"number":0,"history":` +
	`"{\"_cls\":\"History\",\"current\":{\"_cls\":\"Topic\",\"messages\":[` +
	`{\"_cls\":\"Message\",\"id\":\"u\",\"ai\":false,\"content\":{\"user_message\":\"HISTORY-USER\"}},` +
	`{\"_cls\":\"Message\",\"id\":\"t\",\"ai\":false,\"content\":{\"tool_name\":\"x\",\"tool_result\":\"HISTORY-RESULT\"}},` +
	`{\"_cls\":\"Message\",\"id\":\"a\",\"ai\":true,\"content\":\"HISTORY-REPLY\"}]}}"}],"log":{"logs":[` +
	`{"no":1,"id":"u","type":"user","timestamp":1767225600,"content":"january ask"},` +
	`{"no":2,"id":"t","type":"tool","timestamp":1767225600,"content":"january output"},` +
	`{"no":3,"id":"a","type":"agent","timestamp":1767225600,"kvps":{},"content":"january reply"},` +
	`{"no":4,"id":"u","type":"user","timestamp":1780272000,"content":"june ask"},` +
	`{"no":5,"id":"t","type":"tool","timestamp":1780272000,"content":"june output"},` +
	`{"no":6,"id":"a","type":"agent","timestamp":1780272000,"kvps":{},"content":"june reply"}]}}`

// sharedIDRun runs timeline over a collection holding sharedIDChat with the
// options given and returns each dated row of timeline.jsonl as "month
// turn text", the month "01" or "06".
func sharedIDRun(t *testing.T, options ...string) []string {
	t.Helper()
	return agentZeroRun(t, []collectedFile{agentZeroChat(sharedIDChat)}, options...)
}

// agentZeroChat returns the collected chat.json of the chat ctx holding text.
func agentZeroChat(text string) collectedFile {
	return collectedFile{"alice", "/home/alice", "agent-zero", "agent-zero/usr/chats/ctx/chat.json", text}
}

// agentZeroRun runs timeline over a collection holding the Agent Zero files
// with the options given and returns each dated row of timeline.jsonl as
// "month turn text", the month "01" or "06"; none when the run found none.
func agentZeroRun(t *testing.T, files []collectedFile, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	switch code, _, stderr := runCommand(env, args...); code {
	case ExitNothing:
		return nil
	case ExitFound:
	default:
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	rows := recordsOf(t, filepath.Join(output, TimelineFile), "timestamp_utc", "timestamp_utc", "turn_type", "text")
	for i, row := range rows {
		rows[i] = row[5:7] + row[strings.Index(row, " "):]
	}
	return rows
}

// The history text of an id the log items of a chat share is shown in the
// first of those items that the filters keep, and a pattern finds an item
// by the history text as well as by its own; the text is shown once, never
// in an item the filters drop. Both tool items of an id carry it as their
// tool_use_id, so the filters keep every row of either beside a kept row of
// the other, and the first keeps the result.
func TestAgentZeroHistoryTextIsShownInTheFirstItemTheFilterKeeps(t *testing.T) {
	january := []string{"01 user HISTORY-USER", "01 tool_use {}", "01 tool_result HISTORY-RESULT",
		"01 assistant HISTORY-REPLY"}
	june := []string{"06 user HISTORY-USER", "06 tool_use {}", "06 tool_result HISTORY-RESULT",
		"06 assistant HISTORY-REPLY"}
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter the history goes with the first item", nil,
			append(slices.Clone(january), "06 user june ask", "06 tool_use {}", "06 tool_result june output",
				"06 assistant june reply")},
		{"a pattern only the history matches shows it in the first item alone",
			[]string{"--match", "HISTORY"},
			append(slices.Clone(january), "06 tool_use {}", "06 tool_result june output")},
		{"a pattern only the later items' own content matches shows the history there",
			[]string{"--match", "june"}, []string{"01 tool_use {}", "01 tool_result HISTORY-RESULT",
				"06 user HISTORY-USER", "06 tool_use {}", "06 tool_result june output", "06 assistant HISTORY-REPLY"}},
		{"a window that keeps only the later items shows the history there",
			[]string{"--since", "2026-03-01"}, june},
		{"a window that keeps only the first items keeps the history with them",
			[]string{"--until", "2026-03-01"}, january},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := sharedIDRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
			for _, history := range []string{"HISTORY-USER", "HISTORY-RESULT", "HISTORY-REPLY"} {
				if count := strings.Count(strings.Join(rows, "\n"), history); count > 1 {
					t.Errorf("%s is shown %d times, want at most once", history, count)
				}
			}
		})
	}
}

// emptyLaterChat is an Agent Zero chat.json whose log names a reply's
// history message in two agent items: one on the first of January with
// content of its own, one on the first of June with neither a headline nor
// content.
const emptyLaterChat = `{"id":"ctx","agents":[{"number":0,"history":` +
	`"{\"_cls\":\"History\",\"current\":{\"_cls\":\"Topic\",\"messages\":[` +
	`{\"_cls\":\"Message\",\"id\":\"a\",\"ai\":true,\"content\":\"HISTORY-REPLY\"}]}}"}],"log":{"logs":[` +
	`{"no":1,"id":"a","type":"agent","timestamp":1767225600,"kvps":{},"content":"january reply"},` +
	`{"no":2,"id":"a","type":"agent","timestamp":1780272000,"kvps":{}}]}}`

// A later agent item of an id with neither a headline nor content of its
// own shows the history reply when the filters keep it and not the first
// item, and otherwise writes no row; the reply is shown once.
func TestAgentZeroReplyIsShownInAKeptLaterItemWithNoContentOfItsOwn(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter the reply goes with the first item", nil, []string{"01 assistant HISTORY-REPLY"}},
		{"a window that keeps only the later item shows the reply there",
			[]string{"--since", "2026-03-01"}, []string{"06 assistant HISTORY-REPLY"}},
		{"a window that keeps only the first item keeps the reply with it",
			[]string{"--until", "2026-03-01"}, []string{"01 assistant HISTORY-REPLY"}},
		{"a pattern the reply matches shows it in the first item alone",
			[]string{"--match", "HISTORY"}, []string{"01 assistant HISTORY-REPLY"}},
		{"a window that keeps neither item shows nothing", []string{"--since", "2026-09-01"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := agentZeroRun(t, []collectedFile{agentZeroChat(emptyLaterChat)}, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
