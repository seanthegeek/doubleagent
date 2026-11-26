package dedup

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

const (
	sessionID  = "11111111-2222-4333-8444-555555555555"
	otherID    = "99999999-0000-4000-8000-000000000000"
	backupA    = "/home/alice/.app/backups/1/history.jsonl"
	backupB    = "/home/alice/.app/backups/2/history.jsonl"
	backupC    = "/home/alice/.app/backups/3/history.jsonl"
	historyLog = "/home/alice/.claude/history.jsonl"
	baseTime   = "2026-10-01T10:00:00.000Z"
)

// change sets one field of a test row.
type change func(row *timeline.Row)

// set returns the change that gives the named field of a row a value.
func set(field string, value any) change {
	return func(row *timeline.Row) {
		switch field {
		case "text":
			row.Text = value.(string)
		case "line":
			row.SourceLine = value.(int)
		case "file":
			row.SourceFile = value.(string)
		case "session":
			row.SessionID = value.(string)
		case "project":
			row.ProjectPath = value.(string)
		case "turn":
			row.TurnType = value.(string)
		case "id":
			row.ToolUseID = value.(string)
		case "agent":
			row.Agent = value.(string)
		case "user":
			row.User = value.(string)
		case "host":
			row.Host = value.(string)
		default:
			panic("no field " + field)
		}
	}
}

// primaryRow returns a transcript row at a time.
func primaryRow(moment string, changes ...change) timeline.Row {
	row := timeline.Row{TimestampUTC: moment, Host: "h1", User: "alice", Agent: "claude-code",
		SessionID: sessionID, ProjectPath: "/srv/proj", TurnType: "user",
		Text:       "delete the logs",
		SourceFile: "/home/alice/.claude/projects/-srv-proj/" + sessionID + ".jsonl",
		SourceLine: 2}
	for _, apply := range changes {
		apply(&row)
	}
	return row
}

// historyRow returns a primary row moved to the history file, line 1.
func historyRow(moment string, changes ...change) timeline.Row {
	return primaryRow(moment, append([]change{set("file", historyLog), set("line", 1)}, changes...)...)
}

// recovered returns a kept secondary row as Deduplicate returns it.
func recovered(row timeline.Row) timeline.Row {
	if row.TurnType != timeline.TurnSystem {
		row.Recovered = "yes"
	}
	return row
}

// withTolerance returns secondary rows each with the same tolerance.
func withTolerance(tolerance float64, rows ...timeline.Row) []Secondary {
	out := make([]Secondary, len(rows))
	for i, row := range rows {
		out[i] = Secondary{row, tolerance}
	}
	return out
}

// deduplicateCase is one call of Deduplicate and what it returns.
type deduplicateCase struct {
	name      string
	primary   []timeline.Row
	secondary []Secondary
	kept      []timeline.Row // nil: every primary row, then the recovered secondary rows in want
	want      []timeline.Row // the secondary rows kept, before they are marked recovered
	dropped   map[string]int // nil: not checked
}

// rows is shorthand for a slice of rows.
func rows(list ...timeline.Row) []timeline.Row { return list }

// sessionAndProjectScopeCases are the tests of how a row's session and project
// decide whether it matches a primary row.
func sessionAndProjectScopeCases() []deduplicateCase {
	return []deduplicateCase{
		{name: "a history row with the transcript's session matches it",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("project", "/elsewhere"))),
			dropped:   map[string]int{"claude-code": 1}},
		{name: "a row one millisecond before the epoch matches like any other",
			primary:   rows(primaryRow("1969-12-31T23:59:59.999Z")),
			secondary: withTolerance(toleranceSeconds, historyRow("1969-12-31T23:59:59.999Z")),
			dropped:   map[string]int{"claude-code": 1}},
		{name: "a row without a session matches by project",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("session", ""))),
			dropped:   map[string]int{"claude-code": 1}},
		{name: "a row without a session in another project is kept",
			primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("session", ""),
				set("project", "/srv/other"))),
			want: rows(historyRow(baseTime, set("session", ""), set("project", "/srv/other")))},
		{name: "a row of another session in the same project is kept",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("session", otherID))),
			want:      rows(historyRow(baseTime, set("session", otherID)))},
		{name: "an undated row matches by session",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("project", "/elsewhere"))),
			dropped:   map[string]int{"claude-code": 1}},
		{name: "an undated row of another session is kept",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("session", otherID))),
			want:      rows(historyRow("", set("session", otherID)))},
		{name: "an undated row without a session matches by project",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("session", "")))},
		{name: "an undated row without a session in another project is kept",
			primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("session", ""),
				set("project", "/srv/other"))),
			want: rows(historyRow("", set("session", ""), set("project", "/srv/other")))},
		{name: "an undated row without session or project matches any row of its identity",
			primary: rows(primaryRow(baseTime, set("project", "/srv/other"))),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("session", ""),
				set("project", "")))},
		{name: "an unscoped undated row of another user is kept",
			primary: rows(primaryRow(baseTime, set("project", "/srv/other"))),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("session", ""), set("project", ""),
				set("user", "bob"))),
			want: rows(historyRow("", set("session", ""), set("project", ""), set("user", "bob")))},
	}
}

// differingFieldCases are the tests that a row differing in text, turn type,
// user, host or agent never matches.
func differingFieldCases() []deduplicateCase {
	return []deduplicateCase{
		{name: "an undated row with other text is kept",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("text", "delete the logs!"))),
			want:      rows(historyRow("", set("text", "delete the logs!")))},
		{name: "a different turn type never matches",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("turn", "system"))),
			want:      rows(historyRow(baseTime, set("turn", "system")))},
		{name: "a different text never matches", primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("text", "delete the logs!"))),
			want:      rows(historyRow(baseTime, set("text", "delete the logs!")))},
		{name: "a different user never matches", primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("user", "bob"))),
			want:      rows(historyRow(baseTime, set("user", "bob")))},
		{name: "a different host never matches", primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("host", "h2"))),
			want:      rows(historyRow(baseTime, set("host", "h2")))},
		{name: "a different agent never matches", primary: rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("agent", "codex-cli"))),
			want:      rows(historyRow(baseTime, set("agent", "codex-cli")))},
	}
}

// countAndOrderCases are the tests of the counts of dropped rows and of the
// order of the rows kept.
func countAndOrderCases() []deduplicateCase {
	history := historyRow(baseTime)
	return []deduplicateCase{
		{name: "two identical primary rows both stay",
			primary:   rows(primaryRow(baseTime, set("line", 2)), primaryRow(baseTime, set("line", 9))),
			secondary: withTolerance(toleranceSeconds, history),
			dropped:   map[string]int{"claude-code": 1}},
		{name: "no secondary rows keeps the primary rows",
			primary: rows(primaryRow(baseTime, set("line", 2)), primaryRow(baseTime, set("line", 9))),
			dropped: map[string]int{}},
		{name: "drops are counted per agent",
			primary: rows(primaryRow(baseTime), primaryRow(baseTime, set("agent", "antigravity")),
				primaryRow(baseTime, set("agent", "codex-cli"))),
			secondary: withTolerance(toleranceSeconds, history, historyRow(baseTime,
				set("agent", "antigravity")), historyRow(baseTime, set("agent", "codex-cli"),
				set("text", "x"))),
			want:    rows(historyRow(baseTime, set("agent", "codex-cli"), set("text", "x"))),
			dropped: map[string]int{"claude-code": 1, "antigravity": 1}},
		{name: "primary rows come first in input order then the kept secondary rows",
			primary: rows(primaryRow("2026-10-01T10:00:05.000Z", set("text", "b")),
				primaryRow(baseTime, set("text", "a"))),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T11:00:00.000Z",
				set("text", "late")), historyRow(baseTime, set("text", "a")),
				historyRow("2026-10-01T09:00:00.000Z", set("text", "early"))),
			want: rows(historyRow("2026-10-01T11:00:00.000Z", set("text", "late")),
				historyRow("2026-10-01T09:00:00.000Z", set("text", "early")))},
	}
}

func TestDeduplicate(t *testing.T) {
	cases := slices.Concat(sessionAndProjectScopeCases(), differingFieldCases(), countAndOrderCases(),
		nearestLineCases(), windowEdgeCases(), undatedLineCases(), toleranceCases(), copyCases())
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			check(t, test)
		})
	}
}

// check runs one case and compares the rows kept and the counts.
func check(t *testing.T, test deduplicateCase) {
	t.Helper()
	primaryCopy := append([]timeline.Row(nil), test.primary...)
	secondaryCopy := append([]Secondary(nil), test.secondary...)
	kept, dropped := Deduplicate(test.primary, test.secondary, nil)
	want := test.kept
	if want == nil {
		want = append([]timeline.Row{}, test.primary...)
		for _, row := range test.want {
			want = append(want, recovered(row))
		}
	}
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("kept:\n%s\nwant:\n%s", describe(kept), describe(want))
	}
	if test.dropped != nil && !maps.Equal(dropped, test.dropped) {
		t.Errorf("dropped %v, want %v", dropped, test.dropped)
	}
	if !reflect.DeepEqual(primaryCopy, test.primary) ||
		!reflect.DeepEqual(secondaryCopy, test.secondary) {
		t.Error("Deduplicate changed its arguments")
	}
}

// describe lists rows one per line for a failure message.
func describe(list []timeline.Row) string {
	text := ""
	for _, row := range list {
		text += fmt.Sprintf("  %s %s %q %s:%d %q\n", row.TimestampUTC, row.TurnType, row.Text,
			row.SourceFile, row.SourceLine, row.Recovered)
	}
	return text
}
