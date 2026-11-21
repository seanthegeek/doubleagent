package analyze

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id",
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

// openclawGroupedStores returns the OpenClaw stores whose rows of a
// repeated id are grouped, for the property the parsers on the grouping
// mechanism keep (group_property_test.go).
func openclawGroupedStores() []groupedStore {
	file := func(text func(t *testing.T) string) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "openclaw", openclawAgentRel, text(t)}}
		}
	}
	return []groupedStore{
		{"openclaw windows", "openclaw", file(openclawRepeatedWindow)},
		{"openclaw relayed windows", "openclaw", file(openclawRelayedWindow)},
	}
}

// openclawCollection writes the store's collection and returns its
// directory and its rows without a filter.
func openclawCollection(t *testing.T, store groupedStore) (string, []timeline.Row) {
	t.Helper()
	directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
	return directory, timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
}

func TestAWindowShowsEveryRowOfARepeatedOpenClawIDInsideIt(t *testing.T) {
	for _, store := range openclawGroupedStores() {
		t.Run(store.name, func(t *testing.T) {
			directory, unfiltered := openclawCollection(t, store)
			for _, row := range unfiltered {
				for _, window := range []rowfilter.RowFilter{{Since: row.TimestampUTC}, {Until: row.TimestampUTC}} {
					filtered := timelineRows(t, directory, store.agent, window, nil, false)
					if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
						return row.TimestampUTC != "" && (window.Since == "" || row.TimestampUTC >= window.Since) &&
							(window.Until == "" || row.TimestampUTC <= window.Until)
					}); len(faults) > 0 {
						t.Errorf("--since %q --until %q: %q", window.Since, window.Until, faults)
					}
				}
			}
		})
	}
}

func TestAMatchShowsEveryRowOfARepeatedOpenClawIDItMatches(t *testing.T) {
	for _, store := range openclawGroupedStores() {
		t.Run(store.name, func(t *testing.T) {
			directory, unfiltered := openclawCollection(t, store)
			var texts []string
			for _, row := range timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, true) {
				if text := regexp.QuoteMeta(row.FullText()); text != "" && !slices.Contains(texts, text) {
					texts = append(texts, text)
				}
			}
			for i, first := range texts {
				for _, second := range texts[i:] {
					pattern := first + "|" + second
					filtered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, []string{pattern}, false)
					matched := regexp.MustCompile(pattern)
					if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
						return row.Matches(matched)
					}); len(faults) > 0 {
						t.Errorf("--match %q: %q", pattern, faults)
					}
				}
			}
		})
	}
}
