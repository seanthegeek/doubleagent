package claudecode

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Stubs naming one persisted output by spellings the disk opens it by
// (another case, or an accent written as a combining mark) read it once
// for all the session's transcripts. A spelling the disk tells apart
// names no file, and the case says so.
func TestEverySpellingOfOnePersistedOutputReadsItOnce(t *testing.T) {
	const name = "été.txt"
	tests := []struct{ name, spelled string }{
		{"an ASCII capital", "\u00e9T\u00e9.txt"},
		{"accented capitals", "ÉTÉ.txt"},
		{"accents written as combining marks", "été.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := sessionFiles{transcripts: [][2]string{{"s1.jsonl", name}, {"s1/subagents/agent-a.jsonl", test.spelled},
				{"s1/subagents/agent-b.jsonl", test.spelled}}, outputs: map[string]string{name: "output"}}
			probe := filepath.Join(t.TempDir(), name)
			if err := osfile.WriteFile(probe, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := osfile.Lstat(filepath.Join(filepath.Dir(probe), test.spelled)); err != nil {
				t.Skip("this disk tells the two spellings apart, so they open two names")
			}
			if _, reads := sessionRows(t, map[string]sessionFiles{"s1": files}, []string{"s1"}); reads != 1 {
				t.Errorf("the output was read %d times, want once", reads)
			}
		})
	}
}

// Placeholders naming one paste-cache file by case spellings of its hash
// show it once, at the first; a disk that tells the spellings apart has
// one file for the first alone, and the test says so.
func TestEverySpellingOfOnePasteHashShowsItOnce(t *testing.T) {
	home := t.TempDir()
	if err := osfile.MkdirAll(filepath.Join(home, "paste-cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(home, "paste-cache", "abc.txt"), []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := osfile.Lstat(filepath.Join(home, "paste-cache", "ABC.txt")); err != nil {
		t.Skip("this disk tells names apart by case, so no two spellings open one file")
	}
	pasted := decode(t, `{"1": {"contentHash": "abc"}, "2": {"contentHash": "ABC"}, "3": {"contentHash": "Abc"}}`)
	want := "cached [Pasted text #2, shown above] [Pasted text #3, shown above]"
	var problems parserkit.Problems
	var out timeline.TextBuilder
	New().expandPastes(&out, &input.Artifact{Home: &input.Home{}}, home, "[Pasted text #1] [Pasted text #2] [Pasted text #3]", pasted, &problems)
	rows := make([]timeline.Row, 1)
	out.Set(&rows[0])
	timeline.Resolve(rows)
	if got := rows[0].Text; got != want || len(problems) != 0 {
		t.Errorf("got %q, %v; want %q", got, problems, want)
	}
}

// History lines naming one paste-cache file by case spellings of its hash
// share one text: the first line shows it and a later one points at it.
func TestHistoryLinesSpellingOnePasteHashShareItsText(t *testing.T) {
	probe := filepath.Join(t.TempDir(), "h1.txt")
	if err := osfile.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := osfile.Lstat(filepath.Join(filepath.Dir(probe), "H1.txt")); err != nil {
		t.Skip("this disk tells names apart by case, so no two spellings open one file")
	}
	line := func(display, hash string) string {
		return `{"display": "` + display + `", "pastedContents": {"1": {"contentHash": "` + hash + `"}}, ` +
			`"timestamp": 1790000001000, "sessionId": "s", "project": "/p"}`
	}
	rows := historyRows(t, line("one [Pasted text #1]", "h1"), line("two [Pasted text #1]", "H1"))
	timeline.Resolve(rows)
	want := []string{"one PASTE-B", "two [Pasted text #1, shown above in /home/u/.claude/history.jsonl line 1]"}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		if row.Text != want[i] {
			t.Errorf("row %d: got %q, want %q", i, row.Text, want[i])
		}
	}
}
