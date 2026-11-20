package pi

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
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

// Forks naming one parent by case spellings of its file name read it once
// on a disk that matches names in any case, and a fork naming itself in
// another spelling keeps its own entries; two parents are both read.
func TestEverySpellingOfOneParentSessionReadsItOnce(t *testing.T) {
	const directory = ".pi/agent/sessions/d/"
	tests := []struct {
		name     string
		parents  []string
		files    []string
		anyCase  bool
		reads    int
		selfName string
	}{
		{"four spellings of one parent read it once", []string{"parent", "PARENT", "Parent", "pArEnT"},
			[]string{"parent"}, true, 1, ""},
		{"two parents are both read", []string{"parent", "other"}, []string{"parent", "other"}, false, 2, ""},
		{"a fork naming itself in another spelling keeps its entries", []string{"parent"},
			[]string{"parent"}, true, 0, "FORK0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			write := func(rel, text string) { writeHomeFile(t, home, rel, text) }
			for _, name := range test.files {
				write(directory+name+".jsonl", sessionText(name, "", "e-"+name))
			}
			var rels []string
			for i, parent := range test.parents {
				rel := fmt.Sprintf("%sfork%d.jsonl", directory, i)
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
			subject.Work = parser.CountingWork()
			own := 0
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Original: "/home/u/" + rel, Home: &input.Home{DiskPath: home, Original: "/home/u"}}
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
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
