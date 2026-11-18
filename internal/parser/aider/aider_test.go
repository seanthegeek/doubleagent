package aider

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows of the three history files are proved by the parser replay
// (internal/parity/replay) over the recorded fixtures; these tests cover the
// line readers and helpers the rows are built from.

func TestMarkerLinesTakeASCIIDigitsAndSpaceOrTab(t *testing.T) {
	markers := []struct {
		name  string
		line  string
		match func(string) bool
	}{
		{"chat header", "# aider chat started at 2026-10-01 08:00:00", func(line string) bool {
			_, ok := chatHeader(line)
			return ok
		}},
		{"input timestamp", "# 2026-10-01 08:00:00.123456", isInputStamp},
		{"LLM marker", "TO LLM 2026-10-01T08:00:00", func(line string) bool {
			_, _, ok := llmMarker(line)
			return ok
		}},
		{"LLM response marker", "LLM RESPONSE 2026-10-01T08:00:00", func(line string) bool {
			_, _, ok := llmMarker(line)
			return ok
		}},
	}
	for _, marker := range markers {
		t.Run(marker.name, func(t *testing.T) {
			if !marker.match(marker.line) || !marker.match(marker.line+" \t") {
				t.Errorf("%q is not a marker", marker.line)
			}
			for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
				if marker.match(marker.line + space) {
					t.Errorf("%q matched", marker.line+space)
				}
			}
			for digit, arabic := range map[string]string{"0": "\u0660", "1": "\u0661"} {
				if marker.match(strings.ReplaceAll(marker.line, digit, arabic)) {
					t.Errorf("a line with %q matched", arabic)
				}
			}
		})
	}
}

func TestInputHistoryLineOfUnicodeSpaceIsReported(t *testing.T) {
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		t.Run(fmt.Sprintf("U+%04X", []rune(space)[0]), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.history")
			data := "# 2026-10-01 08:00:00\n+hi\n" + space + "\n \t\n"
			if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			var problems parser.Problems
			entries := ReadInputHistory(path, &problems)
			if len(entries) != 1 || entries[0] != (InputEntry{1, "2026-10-01 08:00:00", "hi"}) {
				t.Errorf("entries %+v", entries)
			}
			want := parser.Problem{Line: 3, Message: "unparseable line (not a FileHistory line)",
				Raw: space}
			if len(problems) != 1 || problems[0] != want {
				t.Errorf("problems %+v, want %+v", problems, want)
			}
		})
	}
}

func TestInputHistoryJoinsPlusLinesAndKeepsAnEntryWithoutATimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.history")
	data := "+orphan\n\n# 2026-10-01 08:00:00.5  \n+a\n+b\n# 2026-10-01 09:00:00\n"
	if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var problems parser.Problems
	got := ReadInputHistory(path, &problems)
	want := []InputEntry{{1, "", "orphan"}, {3, "2026-10-01 08:00:00.5", "a\nb"},
		{6, "2026-10-01 09:00:00", ""}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] ||
		len(problems) != 0 {
		t.Errorf("got %+v, %+v", got, problems)
	}
}

func TestSourcePathHelpersKeepTheHostsSeparator(t *testing.T) {
	tests := []struct {
		original string
		dir      string
		sibling  string
	}{
		{"/alice/.aider.chat.history.md", "/alice", "/alice/x"},
		{`C:\Users\a\proj\.aider.input.history`, `C:\Users\a\proj`, `C:\Users\a\proj\x`},
		{"/.aider.llm.history", "/", "/x"},
		{".aider.llm.history", "", "x"},
		{`C:\x/y\.aider.llm.history`, `C:\x/y`, `C:\x/y\x`},
	}
	for _, test := range tests {
		t.Run(test.original, func(t *testing.T) {
			if got := projectDir(test.original); got != test.dir {
				t.Errorf("projectDir %q, want %q", got, test.dir)
			}
			if got := sibling(test.original, "x"); got != test.sibling {
				t.Errorf("sibling %q, want %q", got, test.sibling)
			}
		})
	}
}

func TestWantsAndSecondaryNameTheThreeFilesAtAnyDepth(t *testing.T) {
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
	}{
		{".aider.chat.history.md", true, false},
		{"proj/.aider.chat.history.md", true, false},
		{"proj/.aider.input.history", true, true},
		{".aider.llm.history", true, true},
		{"x.aider.llm.history", false, false},
		{".aider.chat.history.md.bak", false, false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || (test.wants && subject.Secondary(artifact) != test.secondary) {
				t.Errorf("wants %v secondary %v", subject.Wants(artifact), subject.Secondary(artifact))
			}
		})
	}
}

func TestAnnouncementLinesNameTheModelFromItsFirstVisibleCharacter(t *testing.T) {
	tests := []struct {
		line string
		want []modelChange
	}{
		{"Main model: gpt-4o with diff edit format, infinite output",
			[]modelChange{{"main", "gpt-4o", "", true}}},
		{"Model: gpt-4o with whole edit format", []modelChange{{"main", "gpt-4o", "", true},
			{"weak", "gpt-4o", " (weak model)", false}}},
		{"Editor model: sonnet with editor-diff edit format",
			[]modelChange{{"editor", "sonnet", " (editor model)", true}}},
		{"Weak model: haiku", []modelChange{{"weak", "haiku", " (weak model)", true}}},
		{"Weak model:  haiku", nil},
		{"Main model:  gpt with diff edit format", nil},
		{"Main model: gpt with   edit format", nil},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			if got := subject.announcement(test.line); !slices.Equal(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}
