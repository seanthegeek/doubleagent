package kilocode

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The rows each file gives are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the patterns and helpers on their own. The array reader
// is the kit's parserkit.ArrayElements, tested there.

func TestTheLogHeadTakesASCIIDigitsAndSpaceOrTab(t *testing.T) {
	head := "INFO%s2026-10-01T08:00:00 +3ms "
	tests := []struct {
		name, line string
		want       bool
	}{
		{"a space and a tab", strings.Replace(head, "%s", " \t", 1), true},
		{"an Arabic-Indic digit", strings.ReplaceAll(strings.Replace(head, "%s", " ", 1), "3", "\u0663"), false},
		{"a no-break space", strings.Replace(head, "%s", "\u00a0", 1), false},
		{"a line separator", strings.Replace(head, "%s", "\u2028", 1), false},
		{"a next-line character", strings.Replace(head, "%s", "\u0085", 1), false},
		{"a lower-case level", "info 2026-10-01T08:00:00 +3ms ", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := logHeadPattern.MatchString(test.line); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestRemoteFieldsStopAtUnicodeWhitespace(t *testing.T) {
	ended := " sessionID=s startedAt=t durationMs=%s reason=r remote session ended"
	tests := []struct {
		name    string
		pattern string
		line    string
		want    []string
	}{
		{"an ASCII duration", "ended", strings.Replace(ended, "%s", "3", 1), []string{"s", "t", "3", "r"}},
		{"an Arabic-Indic duration", "ended", strings.Replace(ended, "%s", "\u0663", 1), nil},
		{"a no-break space ends a field", "started",
			" sessionID=a\u00a0b startedAt=t remote session started", nil},
		{"a zero-width space does not", "started",
			" sessionID=a\u200bb startedAt=t remote session started", []string{"a\u200bb", "t", "", ""}},
		{"a model and a directory with a space", "started",
			" sessionID=s startedAt=t model=m directory=/x y remote session started",
			[]string{"s", "t", "m", "/x y"}},
		{"an ideographic space ends a field", "enabled", " connected=a\u3000b remote connection enabled", nil},
		{"a file separator does not", "enabled", " connected=a\x1cb remote connection enabled",
			[]string{"a\x1cb"}},
	}
	patterns := map[string]func(string) []string{
		"ended":   endedPattern.FindStringSubmatch,
		"started": startedPattern.FindStringSubmatch,
		"enabled": enabledPattern.FindStringSubmatch,
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := patterns[test.pattern](test.line)
			if got != nil {
				got = got[1:]
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestATaskFileIsFoundUnderAnyUserDirectory(t *testing.T) {
	rel := "User/globalStorage/kilocode.kilo-code/tasks/t1/api_conversation_history.json"
	tests := []struct {
		rel  string
		want bool
	}{
		{rel, true},
		{".config/VSCodium/" + rel, true},
		{".config/Code/X" + rel, false},
		{strings.Replace(rel, "api_conversation_history", "ui_messages", 1), false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := isTaskFile(test.rel); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestWantsKilosFilesAndEveryDatabaseIsPrimary(t *testing.T) {
	tests := []struct {
		rel              string
		wants, secondary bool
	}{
		{".local/share/kilo/kilo.db", true, false},
		{".local/share/kilo/kilo-beta.db", true, false},
		{".local/share/kilo/opencode-next.db", true, false},
		{".local/share/kilo/opencode.db", true, false},
		{".local/share/opencode/opencode.db", false, false},
		{".local/share/kilo/storage/session_share/ses_1.json", true, false},
		{".local/share/kilo/log/2026-10-03T100000.log", true, false},
		{".local/share/kilo/retention/state.json", true, false},
		{".local/share/kilo/memory/p/sessions/a.md", true, false},
		{".config/kilo/kilo.jsonc", true, false},
		{".config/kilo/other.json", false, false},
		{".local/state/kilo-sandbox-policy/a/b.json", true, false},
		{".local/state/kilo-sandbox-policy/.a/b.json", false, false},
		{".local/state/kilo-sandbox-preference/b.json", true, false},
		{".local/state/kilo/prompt-history.jsonl", true, true},
		{".local/state/kilo/session.json", true, true},
		{".local/state/kilo/prompt-stash.jsonl", true, false},
		{".local/share/kilo/storage/message/s/m.json", true, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.wants {
				t.Errorf("wants %v, want %v", got, test.wants)
			}
			if got := parserkit.IsSecondary(subject, artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
		})
	}
	if subject.Agent() != "kilo-code" || subject.Name() != "kilo-code" ||
		parserkit.MarksOf(subject) != (parserkit.Marks{Sync: true}) {
		t.Errorf("agent %q, name %q, marks %+v", subject.Agent(), subject.Name(), parserkit.MarksOf(subject))
	}
}

func TestDigestFieldsStopAtTheSummaryLine(t *testing.T) {
	lines := []string{"# Session  ", "# Session s1", "Topic: \tt ", "Updated: 2026", "  ## Summary\t",
		"Topic: late"}
	fields, summaryAt := digestFields(lines)
	if summaryAt != 4 || fields["# Session "] != "s1" || fields["Topic: "] != "t" || fields["Updated: "] != "2026" {
		t.Errorf("got %q at %d", fields, summaryAt)
	}
	if _, summaryAt := digestFields([]string{"# Session s1"}); summaryAt != -1 {
		t.Errorf("got %d without a summary line", summaryAt)
	}
}

func TestNonSpaceIsTheComplementOfUnicodeIsSpace(t *testing.T) {
	pattern := regexp.MustCompile(`\A` + nonSpace + `\z`)
	for char := rune(0); char <= 0x3000; char++ {
		if got, want := pattern.MatchString(string(char)), !unicode.IsSpace(char); got != want {
			t.Errorf("U+%04X: nonSpace matches %v, want %v", char, got, want)
		}
	}
}
