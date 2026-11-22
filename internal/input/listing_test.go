package input

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// namedEntry is a directory entry that has only a name.
type namedEntry string

// Name returns the entry's name.
func (e namedEntry) Name() string { return string(e) }

// IsDir reports that the entry is not a directory.
func (namedEntry) IsDir() bool { return false }

// Type returns the mode of a regular file.
func (namedEntry) Type() fs.FileMode { return 0 }

// Info returns no description.
func (namedEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

func TestALooseListingIsInNameOrderAsBytes(t *testing.T) {
	tests := []struct {
		name  string
		given []string
		want  []string
	}{
		{"capitals sort before small letters and both are kept",
			[]string{"b", "a", "B", "A"}, []string{"A", "B", "a", "b"}},
		{"punctuation sorts by its byte value", []string{"a", "_x", ".x", "Z"},
			[]string{".x", "Z", "_x", "a"}},
		{"a name that is not valid UTF-8 sorts by its bytes",
			[]string{"\xff", "é", "z", "\x80a"}, []string{"z", "\x80a", "é", "\xff"}},
		{"a prefix sorts before the longer name", []string{"ab", "a", "a.b"},
			[]string{"a", "a.b", "ab"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collection := &Collection{readDir: func(string) ([]fs.DirEntry, error) {
				entries := make([]fs.DirEntry, len(test.given))
				for i, name := range test.given {
					entries[i] = namedEntry(name)
				}
				return entries, nil
			}}
			entries, err := collection.listDirectory("anywhere")
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(entries))
			for i, entry := range entries {
				got[i] = entry.Name()
			}
			wantSlice(t, "names", got, test.want)
		})
	}
}

// reversedListing returns a lister that gives a directory's entries in
// the reverse of name order, or in name order.
func reversedListing(reverse bool) func(string) ([]fs.DirEntry, error) {
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

// openedOrder returns the homes and the artifacts with their agents of a
// collection, in order, as text.
func openedOrder(collection *Collection) []string {
	var order []string
	for _, home := range collection.Homes {
		order = append(order, "home "+home.DiskPath)
	}
	for _, artifact := range collection.Artifacts {
		order = append(order, artifact.Agent+" "+artifact.DiskPath)
	}
	return order
}

func TestALooseTreeOpensInOneOrderWhateverTheListingOrder(t *testing.T) {
	dir := t.TempDir()
	files := looseHome()
	files["home/bob/.codex/history.jsonl"] = "{}\n"
	files["home/bob/.codex/sessions/b.jsonl"] = "{}\n"
	files["home/bob/.codex/sessions/a.jsonl"] = "{}\n"
	writeTree(t, dir, files)
	var orders [2][]string
	for i, reverse := range []bool{false, true} {
		collection, err := Open(t.Context(), filepath.Join(dir, "home"),
			Options{Matcher: matcherOf(t, ""), ReadDir: reversedListing(reverse)})
		opened(t, err)
		orders[i] = openedOrder(collection)
	}
	wantSlice(t, "order with the listing reversed", orders[1], orders[0])
	home := filepath.Join(dir, "home")
	wantSlice(t, "homes", orders[0][:2], []string{"home " + filepath.Join(home, "alice"),
		"home " + filepath.Join(home, "bob")})
	sessions := filepath.Join(home, "bob", ".codex", "sessions")
	first := slices.Index(orders[0], "codex-cli "+filepath.Join(sessions, "a.jsonl"))
	second := slices.Index(orders[0], "codex-cli "+filepath.Join(sessions, "b.jsonl"))
	if first < 0 || second < first {
		t.Errorf("bob's sessions are not in name order: %q", orders[0])
	}
}

// redirectedEntry is a directory entry the system marks irregular, as
// Windows marks a reparse point that is not a link.
type redirectedEntry struct{ fs.DirEntry }

// IsDir reports that the entry is a directory.
func (redirectedEntry) IsDir() bool { return true }

// Type returns a directory marked irregular.
func (redirectedEntry) Type() fs.FileMode { return fs.ModeDir | fs.ModeIrregular }

func TestTheWalkNeverEntersADirectoryTheSystemRedirects(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alice", ".claude", "history.jsonl"), "{}")
	writeFile(t, filepath.Join(dir, "alice", ".claude", "projects", "p", "s.jsonl"), "{}")
	redirected := filepath.Join(dir, "alice", ".claude", "projects")
	readDir := func(path string) ([]fs.DirEntry, error) {
		entries, err := osfile.ReadDir(path)
		for i, entry := range entries {
			if filepath.Join(path, entry.Name()) == redirected {
				entries[i] = redirectedEntry{entry}
			}
		}
		return entries, err
	}
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, ""), ReadDir: readDir})
	opened(t, err)
	wantSlice(t, "artifacts", rels(collection), []string{".claude/history.jsonl"})
	wantSlice(t, "problems", collection.Problems, []string{redirected + ": " + errRedirected.Error()})
}

func TestRealDirectoryTakesOnlyAPlainDirectory(t *testing.T) {
	for _, test := range []struct {
		name string
		mode fs.FileMode
		want bool
	}{
		{"a directory", fs.ModeDir, true},
		{"a link to a directory", fs.ModeDir | fs.ModeSymlink, false},
		{"a directory marked irregular", fs.ModeDir | fs.ModeIrregular, false},
		{"a file", 0, false},
	} {
		if got := realDirectory(test.mode); got != test.want {
			t.Errorf("%s: realDirectory = %v, want %v", test.name, got, test.want)
		}
	}
}
