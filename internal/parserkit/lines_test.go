package parserkit

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// numbered is one record a JSON line reader yielded, as compact JSON.
type numbered struct {
	line   int
	record string
}

// writeFile writes data to a new file under the test's directory and
// returns its path.
func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Each invalid UTF-8 byte of a line becomes one U+FFFD.
func TestJSONLinesKeepGoodRecordsAndReportBadLines(t *testing.T) {
	data := []byte("{\"a\":1}\n\n  \t\x0b{\"b\":2}\x0c \r\n[1,2]\n{\"c\":\n\"str\"\r\n" +
		"{\"d\":\"\xff\xe2\x82\"}\n{\"e\":3}")
	path := writeFile(t, "lines.jsonl", data)
	var problems Problems
	var got []numbered
	for line, record := range JSONLines(path, &problems) {
		got = append(got, numbered{line, record.JSON()})
	}
	want := []numbered{{1, `{"a":1}`}, {3, `{"b":2}`}, {7, "{\"d\":\"\ufffd\ufffd\ufffd\"}"}, {8, `{"e":3}`}}
	if !slices.Equal(got, want) {
		t.Errorf("records %v, want %v", got, want)
	}
	wantProblems := Problems{
		{Line: 4, Message: "unparseable line (not a JSON object)", Raw: "[1,2]"},
		{Line: 5, Message: "unparseable line (invalid JSON at line 1, column 6: unexpected end of JSON input)",
			Raw: `{"c":`},
		{Line: 6, Message: "unparseable line (not a JSON object)", Raw: `"str"`},
	}
	if !slices.Equal(problems, wantProblems) {
		t.Errorf("problems %+v, want %+v", problems, wantProblems)
	}
	var fromMemory Problems
	count := 0
	for range JSONRecords(SplitLines(data), &fromMemory) {
		count++
	}
	if count != len(want) || !slices.Equal(fromMemory, wantProblems) {
		t.Errorf("lines in memory gave %d records and %+v", count, fromMemory)
	}
}

func TestLinesKeepTerminatorsAndStopWhenTheLoopDoes(t *testing.T) {
	path := writeFile(t, "lines.txt", []byte("a\r\nb\n\nc"))
	var problems Problems
	var got []string
	for line, raw := range Lines(path, &problems) {
		got = append(got, strings.Repeat("#", line)+string(raw))
		if line == 3 {
			break
		}
	}
	if want := []string{"#a\r\n", "##b\n", "###\n"}; !slices.Equal(got, want) || len(problems) > 0 {
		t.Errorf("got %q and %v, want %q", got, problems, want)
	}
}

func TestLinesOfAnUnreadablePathAreOneProblemAtLineZero(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name, path, want string
	}{
		{"a missing file", filepath.Join(directory, "absent.jsonl"),
			"absent.jsonl could not be read (no such file or directory)"},
		{"a directory", directory, filepath.Base(directory) + " could not be read (is a directory)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			for range Lines(test.path, &problems) {
				t.Error("a line was yielded")
			}
			if want := (Problems{{Line: 0, Message: test.want}}); !slices.Equal(problems, want) {
				t.Errorf("got %+v, want %+v", problems, want)
			}
		})
	}
}

func TestLineTextAndTailTextDropOneLineEnd(t *testing.T) {
	lines := []struct{ raw, want string }{
		{"a\r\n", "a"}, {"a\n", "a"}, {"a\r", "a"}, {"a\n\n", "a\n"}, {"\xffa\r\r\n", "\ufffda\r"},
		{"a", "a"},
	}
	for _, line := range lines {
		if got := LineText([]byte(line.raw)); got != line.want {
			t.Errorf("LineText(%q) = %q, want %q", line.raw, got, line.want)
		}
	}
	tails := []struct {
		text  string
		start int
		want  string
	}{
		{"abc\r\n", 1, "bc"}, {"abc\n\n", 0, "abc\n"}, {"abc", 3, ""}, {"abc", 9, ""},
	}
	for _, tail := range tails {
		if got := TailText(tail.text, tail.start); got != tail.want {
			t.Errorf("TailText(%q, %d) = %q, want %q", tail.text, tail.start, got, tail.want)
		}
	}
}

func FuzzJSONRecords(fuzz *testing.F) {
	for _, seed := range []string{"{\"a\":1}\n\n[1]\n{\"c\":", " \x0b{}\x0c\r\n\xff", "\"s\"\n{"} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		var problems Problems
		lines := 0
		for line, record := range JSONRecords(SplitLines(data), &problems) {
			if !record.IsObject() || line <= lines {
				t.Errorf("line %d after %d gave %s", line, lines, record.JSON())
			}
			lines = line
		}
		for _, problem := range problems {
			if problem.Line < 1 || problem.Message == "" {
				t.Errorf("problem %+v", problem)
			}
		}
	})
}
