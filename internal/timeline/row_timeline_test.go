package timeline

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

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

func TestSummariseGivesTheDocumentedSessions(t *testing.T) {
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

func TestProjectPathsHaveOneSpellingAndKey(t *testing.T) {
	for _, test := range pathCases() {
		t.Run(strconv.Quote(test.input)+" has one spelling", func(t *testing.T) {
			if got := CanonicalProjectPath(test.input); got != test.canonical {
				t.Errorf("CanonicalProjectPath = %q, want %q", got, test.canonical)
			}
			if got := CanonicalProjectPath(test.canonical); got != test.canonical {
				t.Errorf("CanonicalProjectPath is not idempotent: %q gives %q", test.canonical, got)
			}
			if got := ProjectKey(test.input); got != test.key {
				t.Errorf("ProjectKey = %q, want %q (a Unicode lowering gives %q)", got, test.key, test.unicodeKey)
			}
			if got := IsWindowsPath(test.input); got != test.windows {
				t.Errorf("IsWindowsPath = %v, want %v", got, test.windows)
			}
		})
	}
}

func TestCompactTrimsUnicodeWhitespace(t *testing.T) {
	for _, test := range compactCases() {
		t.Run(strconv.Quote(test.input)+" is stripped", func(t *testing.T) {
			if got := Compact(test.input); got != test.want {
				t.Errorf("Compact(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestCompactValueWritesTheValueAsRowText(t *testing.T) {
	cases := []struct {
		name, document, want string
	}{
		{"null gives nothing", `null`, ""},
		{"a string is stripped", `"  hi there \n"`, "hi there"},
		{"an integer is its digits", `42`, "42"},
		{"a float is written as the evidence wrote it", `1E5`, "1E5"},
		{"true is true", `true`, "true"},
		{"a list is compact JSON", `[1, "a", null]`, `[1,"a",null]`},
		{"an object is compact JSON", `{"k": false}`, `{"k":false}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.document)
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

// failingWriter is an io.Writer whose every write fails with err.
type failingWriter struct {
	err error
}

// Write fails.
func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

// controlText returns every ASCII control character, DEL, every C1 control
// and the two Unicode line separators.
func controlText() string {
	var text strings.Builder
	for char := rune(0); char < 0x20; char++ {
		text.WriteRune(char)
	}
	text.WriteRune(0x7f)
	for char := rune(0x80); char < 0xa0; char++ {
		text.WriteRune(char)
	}
	text.WriteString("\u2028\u2029")
	return text.String()
}

func TestWritersEscapeEveryControlCharacter(t *testing.T) {
	control := controlText()
	writers := []struct {
		name  string
		write func(*bytes.Buffer) error
	}{
		{"timeline rows", func(out *bytes.Buffer) error {
			return WriteRows(out, []Row{{Text: control, SourceFile: control, Host: control}})
		}},
		{"session summaries", func(out *bytes.Buffer) error {
			return WriteSessions(out, []SessionSummary{{SessionID: control,
				Models: []string{control}}})
		}},
	}
	for _, test := range writers {
		t.Run("the "+test.name+" escape every byte below 0x20 and write DEL as it is", func(t *testing.T) {
			var out bytes.Buffer
			if err := test.write(&out); err != nil {
				t.Fatal(err)
			}
			line, ok := strings.CutSuffix(out.String(), "\n")
			if !ok || strings.ContainsFunc(line, func(char rune) bool { return char < 0x20 }) {
				t.Fatalf("a control character is written raw: %q", out.String())
			}
			for _, want := range []string{`\u0000`, `\u001b`, `\n`, `\t`, "\x7f", "\u009b",
				`\u2028\u2029`} {
				if !strings.Contains(line, want) {
					t.Errorf("%q not in %q", want, line)
				}
			}
		})
	}
}
