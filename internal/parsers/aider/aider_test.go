package aider

import (
	"fmt"
	"math/bits"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of the three history files are checked by the golden output of
// the analysis commands (internal/analyze), over inputs that hold every
// parsed agent; these tests cover the line readers and helpers the rows are
// built from.

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
			var problems parserkit.Problems
			entries := ReadInputHistory(path, &problems)
			if len(entries) != 1 || entries[0] != (InputEntry{1, "2026-10-01 08:00:00", "hi"}) {
				t.Errorf("entries %+v", entries)
			}
			want := parserkit.Problem{Line: 3, Message: "unparseable line (not a FileHistory line)",
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
	var problems parserkit.Problems
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

func TestAnInputHistoryIsReadWhateverTheASCIICaseOfItsName(t *testing.T) {
	const rel = "MyProj/.Aider.Input.HISTORY"
	path := filepath.Join(t.TempDir(), filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte("# 2026-10-01 08:00:00\n+hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	if !New().Wants(artifact) || len(rows) != 1 || rows[0].TurnType != timeline.TurnUser || rows[0].Text != "hi" ||
		rows[0].ProjectPath != "/h/MyProj" {
		t.Errorf("rows %+v, want one user row hi in /h/MyProj", rows)
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

func TestAChatRunIsReadInOneTestForTextPerLine(t *testing.T) {
	for _, blanks := range []int{1000, 2000, 4000} {
		t.Run(fmt.Sprintf("%d blank lines", blanks), func(t *testing.T) {
			var rows []timeline.Row
			reader := &chatReader{parser: New(), artifact: &input.Artifact{Original: "/p/" + chatName},
				emit: func(row timeline.Row) { rows = append(rows, row) }, models: map[string]string{},
				work: parserkit.CountingWork()}
			reader.line(1, "# aider chat started at 2025-01-01 10:00:00")
			for number := 2; number <= blanks+1; number++ {
				reader.line(number, "")
			}
			reader.line(blanks+2, "the reply")
			reader.flush()
			if checks := reader.work.Count("text checks"); checks != blanks+1 {
				t.Errorf("%d tests for text over %d lines, want one per line", checks, blanks+1)
			}
			if last := rows[len(rows)-1]; last.Text != "the reply" || last.SourceLine != blanks+2 {
				t.Errorf("last row %q at line %d, want the reply at line %d", last.Text, last.SourceLine, blanks+2)
			}
		})
	}
}

func TestAChatRunStartsAtItsFirstLineWithText(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		line  int
	}{
		{"text at once", []string{"#### hello", "#### more"}, 2},
		{"blank user lines first", []string{"####", "####   ", "#### hello"}, 4},
		{"blank assistant lines first", []string{"", "  ", "the reply", ""}, 4},
		{"a blank line after the text keeps the start", []string{"> ran", ">", "> more"}, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rows []timeline.Row
			reader := &chatReader{parser: New(), artifact: &input.Artifact{Original: "/p/" + chatName},
				emit: func(row timeline.Row) { rows = append(rows, row) }, models: map[string]string{}}
			reader.line(1, "# aider chat started at 2025-01-01 10:00:00")
			for i, line := range test.lines {
				reader.line(i+2, line)
			}
			reader.flush()
			if len(rows) != 2 || rows[1].SourceLine != test.line {
				t.Errorf("rows %+v, want a second row at line %d", rows, test.line)
			}
		})
	}
}

// scanInput is the plain form of matchInput's search, the test's measure
// of it: from position next, entries timed before the header are passed
// over, the search ends at the first timed at or after the next header,
// and the first holding the text matches. It returns the match, or -1.
func scanInput(inputs []InputEntry, next int, header, nextHeader, text string) int {
	for j := next; j < len(inputs); j++ {
		stamp := inputs[j].Timestamp[:19]
		if stamp < header {
			continue
		}
		if nextHeader != "" && stamp >= nextHeader {
			return -1
		}
		if normalise(inputs[j].Text) == normalise(text) {
			return j
		}
	}
	return -1
}

func TestAPromptFindsItsInputEntryAsTheEntriesAreSearchedInOrder(t *testing.T) {
	stamp := func(minute int) string { return fmt.Sprintf("2025-01-01 10:%02d:00.000000", minute) }
	inputs := []InputEntry{
		{Timestamp: stamp(1), Text: "early"}, {Timestamp: stamp(12), Text: "fix  the\nbug"},
		{Timestamp: stamp(14), Text: "again"}, {Timestamp: stamp(25), Text: "later"},
		{Timestamp: stamp(13), Text: "again"}, {Timestamp: stamp(11), Text: "again"},
	}
	tests := []struct {
		name, header, nextHeader, text string
		next, want                     int
	}{
		{"whitespace runs are one space", stamp(10)[:19], stamp(20)[:19], "fix the bug", 0, 1},
		{"an entry before the header is passed over", stamp(10)[:19], "", "early", 0, -1},
		{"an entry of the next session ends the search", stamp(10)[:19], stamp(20)[:19], "again", 3, -1},
		{"the search starts after the last match", stamp(10)[:19], stamp(20)[:19], "again", 4, 4},
		{"the first entry holding the text matches", stamp(10)[:19], stamp(20)[:19], "again", 2, 2},
		{"the last session searches to the end", stamp(20)[:19], "", "later", 0, 3},
		{"no entry holds the text", stamp(10)[:19], "", "missing", 0, -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &chatReader{inputs: inputs, header: test.header, nextHeader: test.nextHeader,
				nextInput: test.next}
			reader.matchInput(test.text)
			got := reader.nextInput - 1
			if reader.nextInput == test.next {
				got = -1
			}
			if got != test.want {
				t.Errorf("matched %d, want %d", got, test.want)
			}
			if plain := scanInput(inputs, test.next, test.header, test.nextHeader, test.text); plain != test.want {
				t.Errorf("the plain search matched %d, want %d", plain, test.want)
			}
		})
	}
}

func TestAPromptSearchAgreesWithThePlainSearchOverMixedTimes(t *testing.T) {
	var inputs []InputEntry
	state := uint32(7)
	random := func(n int) int {
		state = state*1103515245 + 12345
		return int(state>>16) % n
	}
	for range 300 {
		inputs = append(inputs, InputEntry{Timestamp: fmt.Sprintf("2025-01-01 10:%02d:00", random(60)),
			Text: fmt.Sprintf("prompt %d", random(8))})
	}
	for trial := range 2000 {
		header := fmt.Sprintf("2025-01-01 10:%02d:00", random(60))
		nextHeader := ""
		if trial%3 != 0 {
			nextHeader = fmt.Sprintf("2025-01-01 10:%02d:00", random(60))
		}
		next, text := random(len(inputs)+1), fmt.Sprintf("prompt %d", random(9))
		reader := &chatReader{inputs: inputs, header: header, nextHeader: nextHeader, nextInput: next}
		reader.matchInput(text)
		want := scanInput(inputs, next, header, nextHeader, text)
		if (want < 0 && reader.nextInput != next) || (want >= 0 && reader.nextInput != want+1) {
			t.Fatalf("trial %d: next input %d, want the match %d", trial, reader.nextInput, want)
		}
	}
}

func TestAPromptSearchGrowsWithTheLogarithmOfTheEntries(t *testing.T) {
	for _, count := range []int{1024, 2048, 4096} {
		t.Run(strconv.Itoa(count)+" entries", func(t *testing.T) {
			inputs := make([]InputEntry, count)
			for i := range inputs {
				inputs[i] = InputEntry{Timestamp: "2025-01-01 10:30:00", Text: "prompt " + strconv.Itoa(i%2)}
			}
			reader := &chatReader{inputs: inputs, header: "2025-01-01 10:00:00", nextHeader: "2025-01-01 11:00:00",
				work: parserkit.CountingWork()}
			for range count {
				reader.matchInput("a prompt no entry holds")
				reader.matchInput("prompt 0")
			}
			steps, limit := reader.work.Count("search steps"), 2*count*4*(bits.Len(uint(count))+1)
			if steps == 0 || steps > limit {
				t.Errorf("%d steps for %d searches, want at most %d", steps, 2*count, limit)
			}
		})
	}
}
