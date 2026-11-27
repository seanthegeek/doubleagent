package pi

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionText returns a session file of sessionID whose header names
// parent and whose entries are the message entries of entries.
func sessionText(sessionID, parent string, entries ...string) string {
	lines := []string{`{"type":"session","version":3,"id":"` + sessionID + `","timestamp":"2026-01-01T00:00:00Z",` +
		`"cwd":"/w","parentSession":"` + parent + `"}`}
	for _, entry := range entries {
		lines = append(lines, `{"type":"message","id":"`+entry+`","parentId":null,`+
			`"timestamp":"2026-01-01T00:00:01Z","message":{"role":"user","content":"entry `+entry+`"}}`)
	}
	return strings.Join(lines, "\n") + "\n"
}

// writeHomeFile writes text at rel under home.
func writeHomeFile(t *testing.T, home, rel, text string) {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// parentSpelling is one case of the parent spelling test.
type parentSpelling struct {
	name     string
	parents  []string
	files    []string
	anyCase  bool
	reads    int
	fork     string
	selfName string
}

// parentSpellings returns the cases of the parent spelling test.
func parentSpellings() []parentSpelling {
	return []parentSpelling{
		{"four spellings of one parent read it once", []string{"parent", "PARENT", "Parent", "pArEnT"},
			[]string{"parent"}, true, 1, "fork", ""},
		{"spellings in accented capitals read one parent once", []string{"\u00e9t\u00e9", "\u00c9T\u00c9", "\u00c9t\u00e9"},
			[]string{"\u00e9t\u00e9"}, true, 1, "fork", ""},
		{"spellings in Greek capitals read one parent once", []string{"\u03b1\u03b2\u03b3", "\u0391\u0392\u0393"},
			[]string{"\u03b1\u03b2\u03b3"}, true, 1, "fork", ""},
		{"a fork naming itself in Greek capitals keeps its entries", []string{"parent"},
			[]string{"parent"}, true, 0, "\u03c6\u03bf\u03c1\u03ba", "\u03a6\u039f\u03a1\u039a0"},
		{"a spelling with combining accents reads one parent once", []string{"\u00e9t\u00e9", "e\u0301te\u0301"},
			[]string{"\u00e9t\u00e9"}, true, 1, "fork", ""},
		{"two parents are both read", []string{"parent", "other"}, []string{"parent", "other"}, false, 2, "fork", ""},
		{"a fork naming itself in another spelling keeps its entries", []string{"parent"},
			[]string{"parent"}, true, 0, "fork", "FORK0"},
		{"a fork naming itself in accented capitals keeps its entries", []string{"parent"},
			[]string{"parent"}, true, 0, "f\u00e9rk", "F\u00c9RK0"},
		{"a fork naming itself with combining accents keeps its entries", []string{"parent"},
			[]string{"parent"}, true, 0, "f\u00e9rk", "fe\u0301rk0"},
	}
}

// Forks naming one parent by spellings of its file name the disk opens it
// by (another case, or an accent written as a combining mark) read it
// once, and a fork naming itself in another spelling keeps its own
// entries; two parents are both read. A spelling the disk tells apart
// names no file, and the case says so.
func TestEverySpellingOfOneParentSessionReadsItOnce(t *testing.T) {
	const directory = ".pi/agent/sessions/d/"
	tests := parentSpellings()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			write := func(rel, text string) { writeHomeFile(t, home, rel, text) }
			for _, name := range test.files {
				write(directory+name+".jsonl", sessionText(name, "", "e-"+name))
			}
			var rels []string
			for i, parent := range test.parents {
				rel := fmt.Sprintf("%s%s%d.jsonl", directory, test.fork, i)
				if test.selfName != "" {
					parent = test.selfName
				}
				rels = append(rels, rel)
				write(rel, sessionText(fmt.Sprintf("F%d", i), "/home/u/"+directory+parent+".jsonl",
					"e-"+test.parents[i], fmt.Sprintf("own-%d", i)))
			}
			spelled := test.parents[len(test.parents)-1]
			if test.selfName != "" {
				spelled = test.selfName
			}
			if _, err := osfile.Lstat(filepath.Join(home, filepath.FromSlash(directory+spelled+".jsonl"))); test.anyCase &&
				err != nil {
				t.Skip("this disk tells names apart by case, so no two spellings open one file")
			}
			subject := New()
			subject.Work = parserkit.CountingWork()
			collectedHome := &input.Home{DiskPath: home, Original: "/home/u"}
			own := 0
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Original: "/home/u/" + rel, Home: collectedHome}
				if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
					if strings.HasPrefix(row.Text, "entry own-") {
						own++
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if reads := subject.Work.Count("parent files read"); reads != test.reads || own != len(rels) {
				t.Errorf("read the parents %d times and showed %d own entries, want %d and %d", reads, own,
					test.reads, len(rels))
			}
		})
	}
}
