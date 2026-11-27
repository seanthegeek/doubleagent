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

// openclawAgentRel is where OpenClaw keeps an agent's database.
const openclawAgentRel = ".openclaw/agents/main/agent/openclaw-agent.sqlite"

// openclawRepeatEvents is the transcript_events table of the session s1:
// its header in /p on 1 July, a question and an answer naming no model.
func openclawRepeatEvents() sqlitetest.Table {
	return sqlitetest.Table{Name: "transcript_events", SQL: "CREATE TABLE transcript_events (session_id TEXT, " +
		"seq INTEGER, event_json TEXT, event_zstd BLOB, created_at INTEGER)", Rows: [][]any{
		{"s1", 0, `{"type":"session","version":3,"id":"s1","timestamp":"2026-07-01T00:00:00Z","cwd":"/p"}`,
			nil, 1782864000000},
		{"s1", 1, `{"type":"message","id":"u1","timestamp":"2026-07-01T00:00:01Z",` +
			`"message":{"role":"user","content":"the question"}}`, nil, 1782864001000},
		{"s1", 2, `{"type":"message","id":"a1","timestamp":"2026-07-01T00:00:02Z",` +
			`"message":{"role":"assistant","content":[{"type":"text","text":"the answer"}]}}`, nil, 1782864002000}}}
}

// openclawRepeatedWindow returns an OpenClaw agent database whose
// session_windows table holds the session id s1 twice, as only damage
// writes, since the id is the table's key: a row started on 1 January
// with old-model, then one started on 1 June with new-model; and the
// session's transcript in July.
func openclawRepeatedWindow(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_windows", SQL: "CREATE TABLE session_windows (session_id TEXT, " +
			"session_key TEXT, display_name TEXT, model TEXT, started_at INTEGER)", Rows: [][]any{
			{"s1", "agent:main:s1", "old", "old-model", 1767225600000},
			{"s1", "agent:main:s1", "new", "new-model", 1780272000000}}},
		openclawRepeatEvents())
}

// openclawRun runs timeline over a collection holding the OpenClaw agent
// database given, with the options given, and returns each row of
// timeline.jsonl that has a session as "time session project model text".
func openclawRun(t *testing.T, data string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "openclaw", openclawAgentRel, data}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "openclaw"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, timelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "model", "text")
}

// The rows of the repeated window above: each stored row's own, at its
// start with its model, and the transcript's.
const (
	openclawWindowShown = "parser: session_windows holds 2 rows of this row's id; its transcript rows are " +
		"shown once, with "
	openclawOldWindow = "2026-01-01T00:00:00.000Z s1 /p old-model "
	openclawNewWindow = "2026-06-01T00:00:00.000Z s1 /p new-model "
	openclawStart     = "2026-07-01T00:00:00.000Z s1 /p "
	openclawQuestion  = "2026-07-01T00:00:01.000Z s1 /p  the question"
	openclawAnswer    = "2026-07-01T00:00:02.000Z s1 /p "
)

// openclawWindowRow returns the `parser:` row of one stored window,
// chosen or not.
func openclawWindowRow(stored string, chosen bool) string {
	which, name, model := "another of them", "new", "new-model"
	if chosen {
		which = "this row"
	}
	if stored == openclawOldWindow {
		name, model = "old", "old-model"
	}
	return stored + openclawWindowShown + which + " (key=agent:main:s1 display_name=" + name + " model=" +
		model + ")"
}

// openclawWindowTranscript returns the transcript's rows, the start and
// the answer with model.
func openclawWindowTranscript(model string) []string {
	return []string{openclawStart + model + " session start: version=3", openclawQuestion,
		openclawAnswer + model + " the answer"}
}

func TestTheTranscriptOfARepeatedOpenClawWindowGoesWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter it takes the first stored row's model", nil, slices.Concat(
			[]string{openclawWindowRow(openclawOldWindow, true), openclawWindowRow(openclawNewWindow, false)},
			openclawWindowTranscript("old-model"))},
		{"a window past the first row moves it to the second", []string{"--since", "2026-03-01"},
			slices.Concat([]string{openclawWindowRow(openclawNewWindow, true)}, openclawWindowTranscript("new-model"))},
		{"a window keeping only the first row keeps it there", []string{"--until", "2026-03-01"},
			[]string{openclawWindowRow(openclawOldWindow, true)}},
		{"a pattern only the second row matches moves it to the second",
			[]string{"--match", "display_name=new|the answer"},
			[]string{openclawWindowRow(openclawNewWindow, true), openclawAnswer + "new-model the answer"}},
		{"a pattern only the first row matches keeps it with the first",
			[]string{"--match", "display_name=old|the answer"},
			[]string{openclawWindowRow(openclawOldWindow, true), openclawAnswer + "old-model the answer"}},
		{"a pattern no stored row's rows match keeps it with the first", []string{"--match", "the answer"},
			[]string{openclawAnswer + "old-model the answer"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := openclawRun(t, openclawRepeatedWindow(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// openclawRelayedWindow returns an OpenClaw agent database whose
// session_windows table holds the session id s1 twice, once on telegram
// with old-model and once on slack with new-model, and the session's
// transcript.
func openclawRelayedWindow(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_windows", SQL: "CREATE TABLE session_windows (session_id TEXT, " +
			"session_key TEXT, channel TEXT, model TEXT, started_at INTEGER)", Rows: [][]any{
			{"s1", "agent:main:s1", "telegram", "old-model", 1767225600000},
			{"s1", "agent:main:s1", "slack", "new-model", 1780272000000}}},
		openclawRepeatEvents())
}

func TestEachStoredRowOfARepeatedOpenClawWindowGivesItsOwnRelayRow(t *testing.T) {
	relay := func(stored, channel string) string {
		return stored + "session relayed: " + channel + " account - - (from openclaw-agent.sqlite session_windows, " +
			"started_at)"
	}
	oldRelay := relay("2026-01-01T00:00:00.000Z s1 /p  ", "telegram")
	newRelay := relay("2026-06-01T00:00:00.000Z s1 /p  ", "slack")
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"a pattern only the second row's relay matches moves the answer to the second",
			[]string{"--match", "session relayed: slack|the answer"},
			[]string{newRelay, openclawAnswer + "new-model the answer"}},
		{"a pattern matching both relays keeps the answer with the first",
			[]string{"--match", "session relayed|the answer"},
			[]string{oldRelay, newRelay, openclawAnswer + "old-model the answer"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := openclawRun(t, openclawRelayedWindow(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// openclawRepeatedNode returns an OpenClaw agent database whose
// session_nodes table holds the session key k twice, as only damage
// writes, since the key is the table's key: a row of the generation s-old
// updated on 1 January, then one of s-new updated on 1 June, each with a
// permission setting of its own; each generation's transcript header, in /old and
// /new; and one delivery from k on telegram in July.
func openclawRepeatedNode(t *testing.T) string {
	t.Helper()
	header := func(sessionID, cwd string) []any {
		return []any{sessionID, 0, `{"type":"session","version":3,"id":"` + sessionID +
			`","timestamp":"2026-07-01T00:00:00Z","cwd":"` + cwd + `"}`, nil, 1782864000000}
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_nodes", SQL: "CREATE TABLE session_nodes (session_key TEXT, " +
			"current_session_id TEXT, entry_json TEXT, updated_at INTEGER)", Rows: [][]any{
			{"k", "s-old", `{"execHost":"gateway"}`, 1767225600000},
			{"k", "s-new", `{"execHost":"node"}`, 1780272000000}}},
		sqlitetest.Table{Name: "transcript_events", SQL: "CREATE TABLE transcript_events (session_id TEXT, " +
			"seq INTEGER, event_json TEXT, event_zstd BLOB, created_at INTEGER)",
			Rows: [][]any{header("s-new", "/new"), header("s-old", "/old")}},
		sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (conversation_id TEXT, " +
			"channel TEXT)", Rows: [][]any{{"c", "telegram"}}},
		sqlitetest.Table{Name: "conversation_deliveries", SQL: "CREATE TABLE conversation_deliveries " +
			"(operation_kind TEXT, conversation_id TEXT, source_session_key TEXT, status TEXT, created_at INTEGER)",
			Rows: [][]any{{"send", "c", "k", "sent", 1783000000000}}})
}

// The rows of the repeated session key above.
const (
	openclawNodeShown = "parser: session_nodes holds 2 rows of this row's session key; its channel deliveries " +
		"are shown once, with "
	openclawOldNode = "2026-01-01T00:00:00.000Z s-old /old  "
	openclawNewNode = "2026-06-01T00:00:00.000Z s-new /new  "
	openclawSetting = openclawOldNode + "permission setting: execHost=gateway (from openclaw-agent.sqlite " +
		"session_nodes.entry_json, updated_at)"
	openclawNewSetting = openclawNewNode + "permission setting: execHost=node (from openclaw-agent.sqlite " +
		"session_nodes.entry_json, updated_at)"
	openclawDelivery = "channel delivery: send sent channel telegram conversation c platform message - " +
		"(from openclaw-agent.sqlite conversation_deliveries, created_at)"
	openclawDeliveryAt = "2026-07-02T13:46:40.000Z "
)

// openclawNodeRows returns the stored rows' own rows, with the first or
// the second chosen.
func openclawNodeRows(firstChosen bool) []string {
	old, later := "this row", "another of them"
	if !firstChosen {
		old, later = later, old
	}
	return []string{openclawOldNode + openclawNodeShown + old, openclawSetting,
		openclawNewNode + openclawNodeShown + later, openclawNewSetting}
}

func TestTheDeliveriesOfARepeatedOpenClawSessionKeyGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	starts := []string{"2026-07-01T00:00:00.000Z s-new /new  session start: version=3",
		"2026-07-01T00:00:00.000Z s-old /old  session start: version=3"}
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter they take the first stored row's session", nil, slices.Concat(openclawNodeRows(true),
			starts, []string{openclawDeliveryAt + "s-old /old  " + openclawDelivery})},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"},
			slices.Concat([]string{openclawNewNode + openclawNodeShown + "this row", openclawNewSetting}, starts,
				[]string{openclawDeliveryAt + "s-new /new  " + openclawDelivery})},
		{"a window keeping only the first row keeps them there", []string{"--until", "2026-03-01"},
			[]string{openclawOldNode + openclawNodeShown + "this row", openclawSetting}},
		{"a pattern only the second row's setting matches moves them to the second",
			[]string{"--match", "execHost=node|channel delivery"},
			[]string{openclawNewSetting, openclawDeliveryAt + "s-new /new  " + openclawDelivery}},
		{"a pattern only the first row's setting matches keeps them with the first",
			[]string{"--match", "execHost=gateway|channel delivery"},
			[]string{openclawSetting, openclawDeliveryAt + "s-old /old  " + openclawDelivery}},
		{"a pattern no stored row's rows match keeps them with the first", []string{"--match", "channel delivery"},
			[]string{openclawDeliveryAt + "s-old /old  " + openclawDelivery}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := openclawRun(t, openclawRepeatedNode(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
