package analyze

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// listedInOrder returns a lister that gives a directory's entries sorted
// by name, or in the reverse of that order, whatever the file system
// gives.
func listedInOrder(reverse bool) func(path string) ([]fs.DirEntry, error) {
	return func(path string) ([]fs.DirEntry, error) {
		entries, err := osfile.ReadDir(path)
		slices.SortFunc(entries, func(first, second fs.DirEntry) int {
			return strings.Compare(first.Name(), second.Name())
		})
		if reverse {
			slices.Reverse(entries)
		}
		return entries, err
	}
}

// The two ShellGPT cache files hold the same answer: each is a secondary
// source with one undated row, and deduplication keeps one of the two
// equal rows, which before the listing was sorted was the one listed
// first.
func TestTimelineOfALooseTreeDoesNotDependOnTheListingOrder(t *testing.T) {
	work := t.TempDir()
	cache := filepath.Join(work, "in", "home", "alice", ".config", "shell_gpt", "cache")
	for _, name := range []string{strings.Repeat("a", 32), strings.Repeat("b", 32)} {
		writeFile(t, filepath.Join(cache, name), "the same answer\n")
	}
	outputs := map[string]map[string]string{}
	for _, order := range []struct {
		name    string
		reverse bool
	}{{"name order", false}, {"reverse name order", true}} {
		env := testEnvironment()
		env.Parsers = parsers.New
		env.ReadDir = listedInOrder(order.reverse)
		output := filepath.Join(work, "out-"+strings.ReplaceAll(order.name, " ", "-"))
		code, stdout, stderr := runCommand(env, "timeline", filepath.Join(work, "in"), "-o", output)
		if code != ExitFound {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q", order.name, code, stdout, stderr)
		}
		outputs[order.name] = map[string]string{}
		for _, name := range []string{TimelineFile, SessionsFile} {
			data, err := osfile.ReadFile(filepath.Join(output, name))
			if err != nil {
				t.Fatal(err)
			}
			outputs[order.name][name] = string(data)
		}
	}
	for _, name := range []string{TimelineFile, SessionsFile} {
		forward, reverse := outputs["name order"][name], outputs["reverse name order"][name]
		if forward != reverse {
			t.Errorf("%s differs with the listing order:\n name order    %q\n reverse order %q",
				name, forward, reverse)
		}
	}
	if rows := outputs["name order"][TimelineFile]; strings.Count(rows, "\n") != 1 ||
		!strings.Contains(rows, "/cache/"+strings.Repeat("a", 32)+`"`) {
		t.Errorf("want the one row of the first cache file by name, got %q", rows)
	}
}
