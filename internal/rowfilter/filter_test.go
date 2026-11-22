package rowfilter

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// filterRows returns the rows the RowFilter tests filter.
func filterRows() []timeline.Row {
	row := func(timestamp, text, turnType, session, toolUseID string) timeline.Row {
		return timeline.Row{TimestampUTC: timestamp, Agent: "claude-code", SessionID: session,
			TurnType: turnType, Text: text, ToolUseID: toolUseID}
	}
	return []timeline.Row{
		row("2026-10-01T08:59:59.999Z", "before", "user", "s1", ""),
		row("2026-10-01T09:00:00.000Z", "first", "user", "s1", ""),
		row("2026-10-01T09:00:01.000Z", "curl http://x", "tool_use", "s1", "t1"),
		row("2026-10-01T09:00:02.000Z", "200 OK", "tool_result", "s1", "t1"),
		row("2026-10-01T09:00:03.000Z", "ls", "tool_use", "s2", "t1"),
		row("2026-10-01T10:00:00.000Z", "last", "user", "s1", ""),
		row("2026-10-01T10:00:00.001Z", "after", "user", "s1", ""),
		row("", "undated CURL", "user", "s1", ""),
	}
}

// texts returns the text of each row.
func texts(rows []timeline.Row) []string {
	out := []string{}
	for _, row := range rows {
		out = append(out, row.Text)
	}
	return out
}

// mustCompile compiles test patterns, failing the test on an error.
func mustCompile(t *testing.T, ignoreCase bool, expressions ...string) []*regexp.Regexp {
	t.Helper()
	patterns, err := CompilePatterns(expressions, ignoreCase)
	if err != nil {
		t.Fatal(err)
	}
	return patterns
}

func TestRowFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  func(t *testing.T) *RowFilter
		want    []string
		dropped [3]int // window, undated, match
	}{
		{"the window is inclusive at both ends", func(*testing.T) *RowFilter {
			return &RowFilter{Since: "2026-10-01T09:00:00.000Z", Until: "2026-10-01T10:00:00.000Z"}
		}, []string{"first", "curl http://x", "200 OK", "ls", "last"}, [3]int{2, 1, 0}},
		{"undated rows are kept when asked", func(*testing.T) *RowFilter {
			return &RowFilter{Since: "2026-10-01T09:00:00.000Z", KeepUndated: true}
		}, []string{"first", "curl http://x", "200 OK", "ls", "last", "after", "undated CURL"},
			[3]int{1, 0, 0}},
		{"a match keeps its tool partner in the same session only", func(t *testing.T) *RowFilter {
			return &RowFilter{Patterns: mustCompile(t, false, "curl")}
		}, []string{"curl http://x", "200 OK"}, [3]int{0, 0, 6}},
		{"patterns are alternatives and ignore case when asked", func(t *testing.T) *RowFilter {
			return &RowFilter{Patterns: mustCompile(t, true, "CURL", "^last$")}
		}, []string{"curl http://x", "200 OK", "last", "undated CURL"}, [3]int{0, 0, 4}},
		{"an inactive filter keeps every row", func(*testing.T) *RowFilter {
			return &RowFilter{}
		}, texts(filterRows()), [3]int{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter := test.filter(t)
			got := texts(filter.Apply(filterRows()))
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("kept %q, want %q", got, test.want)
			}
			dropped := [3]int{filter.DroppedWindow, filter.DroppedUndated, filter.DroppedMatch}
			if dropped != test.dropped {
				t.Errorf("dropped %v, want %v", dropped, test.dropped)
			}
		})
	}
}

func TestPartnersWithoutASessionShareTheirFile(t *testing.T) {
	rows := []timeline.Row{
		{Text: "rm -rf /tmp/x", TurnType: "tool_use", ToolUseID: "t1", SourceFile: "/a"},
		{Text: "done", TurnType: "tool_result", ToolUseID: "t1", SourceFile: "/a"},
		{Text: "done", TurnType: "tool_result", ToolUseID: "t1", SourceFile: "/b"},
		{Text: "other", TurnType: "tool_result", ToolUseID: "t2", SourceFile: "/a"},
	}
	filter := &RowFilter{Patterns: mustCompile(t, false, "rm -rf")}
	got := filter.Apply(rows)
	if !reflect.DeepEqual(got, rows[:2]) || filter.DroppedMatch != 2 {
		t.Errorf("kept %v dropping %d", got, filter.DroppedMatch)
	}
}

func TestActive(t *testing.T) {
	for _, test := range []struct {
		filter RowFilter
		want   bool
	}{
		{RowFilter{}, false},
		{RowFilter{KeepUndated: true}, false},
		{RowFilter{Since: "2026-10-01T00:00:00.000Z"}, true},
		{RowFilter{Until: "2026-10-01T00:00:00.000Z"}, true},
		{RowFilter{Patterns: []*regexp.Regexp{regexp.MustCompile("x")}}, true},
	} {
		if got := test.filter.Active(); got != test.want {
			t.Errorf("%+v Active() = %v", test.filter, got)
		}
	}
}

func TestCompilePatterns(t *testing.T) {
	if _, err := CompilePatterns([]string{"ok", "("}, false); err == nil ||
		err.Error() != "--match \"(\": error parsing regexp: missing closing ): `(`" {
		t.Errorf("error %v", err)
	}
	patterns := mustCompile(t, true, "rm -RF")
	if !patterns[0].MatchString("sudo RM -rf /") || patterns[0].MatchString("rm  -rf") {
		t.Error("an ignore-case pattern matched wrongly")
	}
}
