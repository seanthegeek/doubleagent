package openhands

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every file kind are proved by the parser replay
// (internal/parity/replay) over the recorded fixtures; these tests cover the
// helpers the rows are built from.

func TestNaiveReadsAZonelessISOTimeAndNothingElse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"padding and seven fraction digits", " 2026-10-01T08:00:00.1234567\t", "2026-10-01T08:00:00.123456"},
		{"microseconds", "2026-10-01T08:00:00.123456", "2026-10-01T08:00:00.123456"},
		{"seconds", "2026-10-01T08:00:00", "2026-10-01T08:00:00.000000"},
		{"a space separator", "2026-10-01 08:00:00", "2026-10-01T08:00:00.000000"},
		{"another separator character", "2026-10-01X08:00:00", ""},
		{"a lower case t", "2026-10-01t08:00:00", "2026-10-01T08:00:00.000000"},
		{"a date alone", "2026-10-01", "2026-10-01T00:00:00.000000"},
		{"an hour alone", "2026-10-01T08", ""},
		{"hours and minutes", "2026-10-01T08:30", "2026-10-01T08:30:00.000000"},
		{"one fraction digit", "2026-10-01T08:00:00.1", "2026-10-01T08:00:00.100000"},
		{"four fraction digits", "2026-10-01T08:00:00.1234", "2026-10-01T08:00:00.123400"},
		{"a comma fraction", "2026-10-01T08:00:00,5", "2026-10-01T08:00:00.500000"},
		{"a long fraction is cut", "2026-10-01T08:00:00.123456789012", "2026-10-01T08:00:00.123456"},
		{"hour 24", "2026-10-01T24:00:00", ""},
		{"a zero year", "0000-01-01T00:00:00", ""},
		{"the first year", "0001-01-01T00:00:00", "0001-01-01T00:00:00.000000"},
		{"trailing space is padding", "2026-10-01T08:00:00 ", "2026-10-01T08:00:00.000000"},
		{"Z means a zone", "2026-10-01T08:00:00Z", ""},
		{"an offset means a zone", "2026-10-01T08:00:00+01:00", ""},
		{"a dangling dot", "2026-10-01T08:00:00.", ""},
		{"month 13", "2026-13-01T00:00:00", ""},
		{"February 30", "2026-02-30", ""},
		{"a one digit hour", "2026-10-01T8:00:00", ""},
		{"second 60", "2026-10-01T08:00:60", ""},
		{"a letter after the fraction", "2026-10-01T08:00:00.12345678a", ""},
		{"blank text", "  ", ""},
		{"a non-ASCII digit after the fraction", "2026-10-01T08:00:00.123456\u0663", ""},
		{"a no-break space after the time", "2026-10-01T08:00:00\u00a0", ""},
		{"a line separator after the time", "2026-10-01T08:00:00\u2028", ""},
		{"a next line after the time", "2026-10-01T08:00:00\u0085", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			moment, ok := naive(jsonvalue.NewString(test.text))
			got := ""
			if ok {
				got = moment.Format("2006-01-02T15:04:05.000000")
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFloorDivideFloorsFromTheRemainder(t *testing.T) {
	tests := []struct {
		left float64
		want float64
	}{
		{0, 0}, {59.999999, 0}, {840, 0}, {839.9999999999999, 0}, {18060, 20}, {-17940, -20},
		{-3540.5, -4}, {899.9999999999999, 0}, {1799.9999999999998, 1}, {-0.0001, -1},
		{12405.678, 13},
	}
	for _, test := range tests {
		if got := floorDivide(test.left, 900); got != test.want {
			t.Errorf("floorDivide(%v, 900) = %v, want %v", test.left, got, test.want)
		}
	}
}

func TestScratchTextNeedsAPathBelowTheWorktreeRoot(t *testing.T) {
	const worktreeNote = " (conversation worktree under the Agent Server's " +
		"default conversation_worktree_root /tmp/conversation-worktrees)"
	tests := []struct {
		directory string
		scratch   bool
	}{
		{"/tmp/conversation-worktrees/abc", true},
		{"/tmp/conversation-worktrees/", false},
		{"/tmp/conversation-worktrees///", false},
		{"/private/tmp/conversation-worktrees/x/y", true},
		{"/tmp/other", false},
		{"/tmp/conversation-worktrees//x/", true},
	}
	for _, test := range tests {
		t.Run(test.directory, func(t *testing.T) {
			expected := ""
			if test.scratch {
				expected = "scratch directory: " + test.directory + worktreeNote
			}
			if got := scratchText(test.directory); got != expected {
				t.Errorf("got %q, want %q", got, expected)
			}
		})
	}
}

func TestFormatOffsetWritesSignedHoursAndMinutes(t *testing.T) {
	for seconds, want := range map[int]string{0: "UTC+00:00", 3600: "UTC+01:00", -18000: "UTC-05:00",
		19800: "UTC+05:30", -12600: "UTC-03:30", 45: "UTC+00:00"} {
		if got := formatOffset(seconds); got != want {
			t.Errorf("formatOffset(%d) = %q, want %q", seconds, got, want)
		}
	}
}

func TestWantsEventStateHistoryAndLegacyFiles(t *testing.T) {
	const conversation = "0123456789abcdef0123456789abcdef"
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
	}{
		{".openhands/agent-canvas/dev_conversations/" + conversation + "/events/event-00001-abcdef12.json", true, false},
		{".openhands/conversations/" + conversation + "/events/event-00001-abcdef12.json", true, false},
		{".openhands/conversations/" + conversation + "/events/event-0001-abcdef12.json", false, false},
		{".openhands/conversations/" + conversation + "/events/event-00001-abc.json", false, false},
		{".openhands/conversations/" + conversation + "/base_state.json", true, false},
		{".openhands/conversations/" + conversation + "/meta.json", false, false},
		{".openhands/projects/" + digest + "/prompt_history.json", true, true},
		{".openhands/projects/ABC/prompt_history.json", false, false},
		{".openhands/sessions/s1/events/3.json", true, false},
		{".openhands/users/u/conversations/s1/events/3.json", true, false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || subject.Secondary(artifact) != test.secondary {
				t.Errorf("wants %v secondary %v", subject.Wants(artifact), subject.Secondary(artifact))
			}
		})
	}
}

func TestAJSONFileWithAByteOrderMarkDoesNotDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.json")
	if err := osfile.WriteFile(path, []byte("\xef\xbb\xbf{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, decoded, err := loadJSON(path, new(parser.Problems)); decoded || err == nil {
		t.Errorf("decoded %v with error %v", decoded, err)
	}
}

// writeTree writes each file of files, by path relative to home, and
// returns home.
func writeTree(t *testing.T, home string, files map[string]string) string {
	t.Helper()
	for rel, text := range files {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// parseRel returns the rows of the file at rel under home.
func parseRel(t *testing.T, home, rel string) []timeline.Row {
	t.Helper()
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
		Rel: rel, Home: &input.Home{DiskPath: home, Original: "/h"}}
	subject := New()
	if !subject.Wants(artifact) {
		t.Fatalf("%s is not wanted", rel)
	}
	var rows []timeline.Row
	if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// problemTexts returns the texts of the rows that report a problem.
func problemTexts(rows []timeline.Row) []string {
	var texts []string
	for _, row := range rows {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			texts = append(texts, row.Text)
		}
	}
	return texts
}

func TestAConversationWithNoEventReportsItsFilesOnItsState(t *testing.T) {
	const conversation = ".openhands/conversations/0123456789abcdef0123456789abcdef/"
	const event = conversation + "events/event-00000-0123abcd.json"
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"a state cut off", map[string]string{conversation + "base_state.json": `{"agent": {"llm": {"model": "gpt`},
			[]string{"parser: base_state.json could not be decoded (invalid JSON at line 1, column 33: unexpected end of JSON input)"}},
		{"a state that is not an object", map[string]string{conversation + "base_state.json": `[1]`},
			[]string{"parser: base_state.json is not a JSON object"}},
		{"a damaged meta.json", map[string]string{conversation + "base_state.json": `{}`,
			conversation + "meta.json": `{`},
			[]string{"parser: meta.json could not be decoded (invalid JSON at line 1, column 2: unexpected end of JSON input)"}},
		{"a whole state", map[string]string{conversation + "base_state.json": `{"agent":{"llm":{"model":"m"}}}`}, nil},
		{"the first event reports them instead", map[string]string{conversation + "base_state.json": `[1]`,
			event: `{"kind": "MessageEvent"}`}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), test.files)
			if got := problemTexts(parseRel(t, home, conversation+"base_state.json")); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestALegacySessionListsItsEventsOnceOrTwice(t *testing.T) {
	const events = ".openhands/sessions/s1/events/"
	for _, test := range []struct {
		name     string
		reversed bool
	}{{"lowest first", false}, {"lowest last", true}} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			var rels []string
			for i := range 200 {
				rel := events + strconv.Itoa(i) + ".json"
				files[rel] = `{"timestamp": "2024-01-01T00:00:00"}`
				rels = append(rels, rel)
			}
			if test.reversed {
				slices.Reverse(rels)
			}
			home := writeTree(t, t.TempDir(), files)
			subject := New()
			subject.Work = parser.CountingWork()
			var texts []string
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
					Rel: rel, Home: &input.Home{DiskPath: home, Original: "/h"}}
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
					t.Fatal(err)
				}
			}
			want := "legacy OpenHands 0.x session, 200 event files, not parsed; times are host local, emitted as if UTC"
			if len(texts) != 1 || texts[0] != want {
				t.Errorf("rows %q, want one %q", texts, want)
			}
			if listings := subject.Work.Count("legacy listings"); listings > 2 {
				t.Errorf("the events directory was listed %d times, want at most twice", listings)
			}
		})
	}
}

func TestALegacyEventFileThatIsNotAnObjectIsReported(t *testing.T) {
	const event = ".openhands/sessions/abc/events/0.json"
	tests := []struct {
		name, text string
		want       []string
	}{
		{"a whole event", `{"timestamp": "2024-05-01T10:00:00"}`, nil},
		{"an event cut off", `{"timestamp": "2024`,
			[]string{"parser: 0.json could not be decoded (invalid JSON at line 1, column 20: unexpected end of JSON input)"}},
		{"an event that is not an object", `[1]`, []string{"parser: 0.json is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), map[string]string{event: test.text})
			rows := parseRel(t, home, event)
			if got := problemTexts(rows); !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
			if len(rows) == 0 || !strings.HasPrefix(rows[0].Text, "legacy OpenHands 0.x session") {
				t.Errorf("rows %+v, want the session row first", rows)
			}
		})
	}
}
