package continuedev

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

// indexedSessions writes index as the sessions.json of a new home and n
// session files holding session beside it, and returns the text of every
// row one parser gives for the files.
func indexedSessions(t *testing.T, index, session string, n int) []string {
	t.Helper()
	home := t.TempDir()
	write := func(rel, text string) *input.Artifact {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	}
	write(".continue/sessions/"+IndexName, index)
	each, texts := New(), []string{}
	for i := range n {
		if err := each.Parse(write(fmt.Sprintf(".continue/sessions/f%d.json", i), session), parser.Options{},
			func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
			t.Fatal(err)
		}
	}
	return texts
}

// The title of a sessions.json entry goes into the start row of every
// session file naming the entry that has no title of its own, so a title
// over the limit is cut there, with one problem for the file: no row holds
// it whole, and twice the files cost twice the bytes, never the title's
// length again. A file's own title is its own and stays whole.
func TestAnIndexEntrysTitleIsCutInEverySessionNamingIt(t *testing.T) {
	title := "t" + strings.Repeat("a", 50_000)
	problem := parser.ProblemPrefix + parser.CarriedProblem("index title", 1)
	tests := []struct {
		name, session   string
		whole, problems int
	}{
		{"a file without a title takes the entry's, cut", `{"sessionId": "s", "history": []}`, 0, 1},
		{"a file with its own title keeps it whole", `{"sessionId": "s", "title": "` + title +
			`", "history": []}`, 1, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sizes []int
			for _, n := range []int{20, 40} {
				texts := indexedSessions(t, `[{"sessionId": "s", "title": "`+title+`"}]`, test.session, n)
				size, whole, problems := 0, 0, 0
				for _, text := range texts {
					size += len(text)
					if strings.Contains(text, title) {
						whole++
					}
					if text == problem {
						problems++
					}
				}
				sizes = append(sizes, size)
				if whole != test.whole*n || problems != test.problems*n {
					t.Errorf("%d files gave %d whole titles and %d problems, want %d and %d", n, whole,
						problems, test.whole*n, test.problems*n)
				}
			}
			if growth := sizes[1] - sizes[0]; growth > 20*(parser.CarriedLimit+1024)+test.whole*20*len(title) {
				t.Errorf("20 more files cost %d bytes", growth)
			}
		})
	}
}
