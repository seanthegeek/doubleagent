package openhands

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows of every file kind are proved against the Python analyzer by the
// parser replay (internal/parity/replay), over the recorded fixtures; these
// tests port what the Python tests call directly. Expected values come from
// running the analyzer's functions.

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
		{"any one separator character", "2026-10-01X08:00:00", "2026-10-01T08:00:00.000000"},
		{"a lower case t", "2026-10-01t08:00:00", "2026-10-01T08:00:00.000000"},
		{"a date alone", "2026-10-01", "2026-10-01T00:00:00.000000"},
		{"an hour alone", "2026-10-01T08", "2026-10-01T08:00:00.000000"},
		{"hours and minutes", "2026-10-01T08:30", "2026-10-01T08:30:00.000000"},
		{"one fraction digit", "2026-10-01T08:00:00.1", "2026-10-01T08:00:00.100000"},
		{"four fraction digits", "2026-10-01T08:00:00.1234", "2026-10-01T08:00:00.123400"},
		{"a comma fraction", "2026-10-01T08:00:00,5", "2026-10-01T08:00:00.500000"},
		{"a long fraction is cut", "2026-10-01T08:00:00.123456789012", "2026-10-01T08:00:00.123456"},
		{"hour 24 is the next midnight", "2026-10-01T24:00:00", "2026-10-02T00:00:00.000000"},
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

func TestFloorDivideIsPythonsFloatFloorDivision(t *testing.T) {
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
	const want = "scratch directory: %s (conversation worktree under the Agent Server's " +
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
				expected = fmt.Sprintf(want, test.directory)
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

func TestCompareNumbersOrdersDigitStringsOfAnySize(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"00001", "1", 0}, {"00002", "00010", -1}, {"99999", "100000", -1},
		{"123456789012345678901234567890", "123456789012345678901234567891", -1}, {"2", "1", 1},
	}
	for _, test := range tests {
		if got := compareNumbers(test.left, test.right); (got > 0) != (test.want > 0) ||
			(got < 0) != (test.want < 0) {
			t.Errorf("compareNumbers(%q, %q) = %d, want sign %d", test.left, test.right, got, test.want)
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
	if _, decoded, reason := loadJSON(path, new(parser.Problems)); decoded || reason == "" {
		t.Errorf("decoded %v with reason %q", decoded, reason)
	}
}
