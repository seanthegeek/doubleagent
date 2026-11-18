package input

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestReadLinesBoundsEachLine(t *testing.T) {
	type line struct {
		number int
		text   string
		long   bool
	}
	tests := []struct {
		name, data string
		limit      int
		want       []line
	}{
		{"lines are numbered and lose their LF", "a\n\nb", 4,
			[]line{{1, "a", false}, {2, "", false}, {3, "b", false}}},
		{"a line of exactly the bound is kept", "abcd\nx\n", 4,
			[]line{{1, "abcd", false}, {2, "x", false}}},
		{"a longer line is one long line and the rest is read", "abcde\nx\n", 4,
			[]line{{1, "", true}, {2, "x", false}}},
		{"a long last line without an LF is long", "x\nabcde", 4,
			[]line{{1, "x", false}, {2, "", true}}},
		{"a line longer than the reader's buffer is long", strings.Repeat("z", 200000) + "\nok\n", 100000,
			[]line{{1, "", true}, {2, "ok", false}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []line
			err := readLines(t.Context(), strings.NewReader(test.data), test.limit,
				func(number int, text []byte, long bool) { got = append(got, line{number, string(text), long}) })
			if err != nil || !slices.Equal(got, test.want) {
				t.Errorf("got %v (%v), want %v", got, err, test.want)
			}
		})
	}
}

func TestAManifestLineTooLongToBeARowIsOneProblem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/fs/home/a/.claude/x", "x")
	row := collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x")
	writeManifest(t, dir, string(bytes.Repeat([]byte(" "), maxManifestLine+1)), row)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, []string{"manifest.jsonl line 1: longer than 1048576 bytes, not read"})
	wantEqual(t, "artifacts", len(collection.Artifacts), 1)
}

func TestACollectionSummaryLargerThanItsBoundIsANote(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir)
	writeFile(t, dir+"/collection.json", `{"hostname": "`+strings.Repeat("h", maxSummary)+`"}`)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "notes", collection.Notes, []string{"collection.json unreadable: larger than the analyzer reads"})
}

func TestAManifestThatIsADirectoryLeavesANote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/fs/home/a/.claude/x", "x")
	writeFile(t, dir+"/manifest.jsonl/inside", "x")
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantEqual(t, "kind", collection.Kind, KindLoose)
	wantSlice(t, "notes", collection.Notes,
		[]string{"manifest.jsonl is not a regular file; reading the input as a loose tree"})
}
