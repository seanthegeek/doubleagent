package timeline

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

func TestAppendRowWritesPythonsLine(t *testing.T) {
	for i, test := range rowLineCases() {
		t.Run("row "+strconv.Itoa(i)+" is written as Python writes it", func(t *testing.T) {
			if got := string(AppendRow(nil, &test.row)); got != test.want {
				t.Errorf("AppendRow =\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

func TestWriteRowsWritesOneLinePerRow(t *testing.T) {
	cases := rowLineCases()
	rows := make([]Row, len(cases))
	var want strings.Builder
	for i, test := range cases {
		rows[i] = test.row
		want.WriteString(test.want)
	}
	var out bytes.Buffer
	if err := WriteRows(&out, rows); err != nil {
		t.Fatal(err)
	}
	if out.String() != want.String() {
		t.Errorf("WriteRows =\n%s\nwant\n%s", out.String(), want.String())
	}
}

func TestWritersReturnTheWriteError(t *testing.T) {
	failing := failingWriter{errors.New("disk full")}
	if err := WriteRows(failing, []Row{{}}); !errors.Is(err, failing.err) {
		t.Errorf("WriteRows error = %v, want %v", err, failing.err)
	}
	if err := WriteSessions(failing, []SessionSummary{{}}); !errors.Is(err, failing.err) {
		t.Errorf("WriteSessions error = %v, want %v", err, failing.err)
	}
}

func TestSummariseGivesPythonsSessions(t *testing.T) {
	for _, test := range sessionCases() {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteSessions(&out, Summarise(test.rows, test.marks)); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("sessions =\n%s\nwant\n%s", out.String(), test.want)
			}
		})
	}
}

func TestSummariseWithoutMarkAgentsLeavesMarksEmpty(t *testing.T) {
	rows := []Row{
		{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: TurnUser},
		{Host: "h", User: "u", Agent: "copilot-cli", SessionID: "c", TurnType: TurnUser},
	}
	for _, summary := range Summarise(rows, MarkAgents{}) {
		marks := []string{summary.Synced, summary.Relayed, summary.Origin, summary.Recovered}
		if !slices.Equal(marks, []string{"", "", "", ""}) {
			t.Errorf("%s marks = %q, want all empty", summary.Agent, marks)
		}
	}
}

func TestSessionWithOnlyParserRowsIsNeverRecovered(t *testing.T) {
	bad := Row{Host: "h", User: "u", Agent: "claude-code", SessionID: "s", TurnType: TurnSystem,
		Text: "parser: unparseable line (bad JSON): {", Recovered: "yes"}
	secondary := MarkAgents{Secondary: map[string]bool{"claude-code": true}}
	if got := Summarise([]Row{bad}, secondary)[0].Recovered; got != "no" {
		t.Errorf("Recovered with the agent reading a secondary source = %q, want no", got)
	}
	if got := Summarise([]Row{bad}, MarkAgents{})[0].Recovered; got != "" {
		t.Errorf("Recovered otherwise = %q, want empty", got)
	}
}

func TestProjectPathsMatchPython(t *testing.T) {
	for _, test := range pathCases() {
		t.Run(strconv.Quote(test.input)+" is spelled as Python spells it", func(t *testing.T) {
			if got := CanonicalProjectPath(test.input); got != test.canonical {
				t.Errorf("CanonicalProjectPath = %q, want %q", got, test.canonical)
			}
			if got := CanonicalProjectPath(test.canonical); got != test.canonical {
				t.Errorf("CanonicalProjectPath is not idempotent: %q gives %q", test.canonical, got)
			}
			if got := ProjectKey(test.input); got != test.key {
				t.Errorf("ProjectKey = %q, want %q (Python %q)", got, test.key, test.pythonKey)
			}
			if got := IsWindowsPath(test.input); got != test.windows {
				t.Errorf("IsWindowsPath = %v, want %v", got, test.windows)
			}
		})
	}
}

func TestCompactStripsAsPythonDoes(t *testing.T) {
	for _, test := range compactCases() {
		t.Run(strconv.Quote(test.input)+" is stripped", func(t *testing.T) {
			if got := Compact(test.input); got != test.want {
				t.Errorf("Compact(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestCompactValueWritesTheValueAsPythonsStr(t *testing.T) {
	cases := []struct {
		name, document, want string
	}{
		{"null gives nothing", `null`, ""},
		{"a string is stripped", `"  hi there \n"`, "hi there"},
		{"an integer is its digits", `42`, "42"},
		{"a float is written as repr writes it", `1E5`, "100000.0"},
		{"true is Python's True", `true`, "True"},
		{"a list is Python's list", `[1, "a", null]`, "[1, 'a', None]"},
		{"an object is Python's dict", `{"k": false}`, "{'k': False}"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, err := pyjson.DecodeString(test.document)
			if err != nil {
				t.Fatal(err)
			}
			if got := CompactValue(value); got != test.want {
				t.Errorf("CompactValue(%s) = %q, want %q", test.document, got, test.want)
			}
		})
	}
}

func TestSortRowsPutsUndatedRowsLastAndKeepsTies(t *testing.T) {
	rows := []Row{
		{Text: "undated b", SourceFile: "/b"},
		{Text: "late", TimestampUTC: "2026-10-01T09:00:00.000Z"},
		{Text: "undated a", SourceFile: "/a", SourceLine: 2},
		{Text: "early line 10", TimestampUTC: "2026-10-01T08:00:00.000Z", SourceFile: "/a",
			SourceLine: 10},
		{Text: "early line 9", TimestampUTC: "2026-10-01T08:00:00.000Z", SourceFile: "/a",
			SourceLine: 9},
		{Text: "undated a again", SourceFile: "/a", SourceLine: 2},
		{Text: "early other file", TimestampUTC: "2026-10-01T08:00:00.000Z", SourceFile: "/0"},
		{Text: "undated a first line", SourceFile: "/a", SourceLine: 1},
	}
	SortRows(rows)
	var got []string
	for _, row := range rows {
		got = append(got, row.Text)
	}
	want := []string{"early other file", "early line 9", "early line 10", "late",
		"undated a first line", "undated a", "undated a again", "undated b"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %q, want %q", got, want)
	}
}

func TestColumnsNameTheFieldsInOrder(t *testing.T) {
	check := func(t *testing.T, value any, columns []string) {
		kind := reflect.TypeOf(value)
		if kind.NumField() != len(columns) {
			t.Fatalf("%s has %d fields for %d columns", kind.Name(), kind.NumField(), len(columns))
		}
		for i, column := range columns {
			name := strings.ReplaceAll(strings.ToLower(kind.Field(i).Name), "_", "")
			if strings.ReplaceAll(column, "_", "") != name {
				t.Errorf("field %d of %s is %s, column %s", i, kind.Name(), kind.Field(i).Name,
					column)
			}
		}
	}
	t.Run("the row's fields follow the timeline columns", func(t *testing.T) {
		check(t, Row{}, Columns())
	})
	t.Run("the summary's fields follow the session columns", func(t *testing.T) {
		check(t, SessionSummary{}, SessionColumns())
	})
	t.Run("the turn types are the six documented ones", func(t *testing.T) {
		want := []string{"user", "assistant", "tool_use", "tool_result", "system", "thinking"}
		if !slices.Equal(TurnTypes(), want) {
			t.Errorf("TurnTypes() = %q", TurnTypes())
		}
	})
}

// failingWriter is an io.Writer whose every write fails with err.
type failingWriter struct {
	err error
}

// Write fails.
func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}
