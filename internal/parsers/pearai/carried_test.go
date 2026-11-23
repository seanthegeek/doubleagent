package pearai

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	write(".pearai/sessions/sessions.json", index)
	each, texts := New(), []string{}
	for i := range n {
		if err := each.Parse(write(fmt.Sprintf(".pearai/sessions/f%d.json", i), session), parserkit.Options{},
			collect(&texts)); err != nil {
			t.Fatal(err)
		}
	}
	return texts
}

// The title and integrationType of a sessions.json entry go into the
// start row of every session file naming the entry, so values over the
// limit are cut there, with one problem each for the file: no row holds
// them whole, and twice the files cost twice the bytes, never the values'
// length again. A file's own title is its own and stays whole.
func TestAnIndexEntrysTitleAndIntegrationAreCutInEverySessionNamingIt(t *testing.T) {
	long := strings.Repeat("a", 50_000)
	title, integration := "t"+long, "i"+long
	tests := []struct {
		name, session string
		whole         int
		problems      []string
	}{
		{"a file without a title takes the entry's, cut", `{"sessionId": "s"}`, 0,
			[]string{"index title", "integration type"}},
		{"a file with its own title keeps it whole", `{"sessionId": "s", "title": "` + title + `"}`, 1,
			[]string{"integration type"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sizes []int
			for _, n := range []int{20, 40} {
				texts := indexedSessions(t, `[{"sessionId": "s", "title": "`+title+`", "integrationType": "`+
					integration+`"}]`, test.session, n)
				size, whole, problems := 0, 0, map[string]int{}
				for _, text := range texts {
					size += len(text)
					if strings.Contains(text, long) {
						whole++
					}
					for _, name := range []string{"index title", "integration type"} {
						if text == parserkit.ProblemPrefix+parserkit.CarriedProblem(name, 1) {
							problems[name]++
						}
					}
				}
				sizes = append(sizes, size)
				if whole != test.whole*n || len(problems) != len(test.problems) {
					t.Errorf("%d files gave %d whole values and problems %v, want %d and one each of %q", n, whole,
						problems, test.whole*n, test.problems)
				}
				for _, name := range test.problems {
					if problems[name] != n {
						t.Errorf("%d files gave %d problems naming %s, want one each", n, problems[name], name)
					}
				}
			}
			if growth := sizes[1] - sizes[0]; growth > 20*(2*parserkit.CarriedLimit+1024)+test.whole*20*len(title) {
				t.Errorf("20 more files cost %d bytes", growth)
			}
		})
	}
}
