//go:build unix

package walk

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// fakeDisk is a file system the probe reads instead of the real one: the
// metadata of each path, the names each directory lists, the links the
// root check refuses with everything under them, and each call made.
type fakeDisk struct {
	entries  map[string]fsmeta.Meta
	listings map[string][]string
	refused  map[string]bool
	lstats   []string
	listed   []string
	checked  []string
}

// folding returns a cache over the fake disk.
func (disk *fakeDisk) folding() *caseFolding {
	return &caseFolding{
		check: func(path string) (follow.Refusal, bool) {
			disk.checked = append(disk.checked, path)
			for link := range disk.refused {
				if path == link || strings.HasPrefix(path, link+"/") {
					return follow.Refusal{Link: link}, true
				}
			}
			return follow.Refusal{}, false
		},
		lstat: func(path string) (fsmeta.Meta, error) {
			disk.lstats = append(disk.lstats, path)
			meta, found := disk.entries[path]
			if !found {
				return fsmeta.Meta{}, fs.ErrNotExist
			}
			return meta, nil
		},
		listing: func(directory string) []string {
			disk.listed = append(disk.listed, directory)
			return disk.listings[directory]
		},
		folds: map[string]bool{},
	}
}

// directory and file are fake lstat results with an identity.
func directory(inode uint64) fsmeta.Meta {
	return fsmeta.Meta{Kind: fsmeta.Dir, Dev: 1, Ino: inode, Nlink: 3}
}

// file is the lstat of a regular file with links names.
func file(inode, links uint64) fsmeta.Meta {
	return fsmeta.Meta{Kind: fsmeta.File, Dev: 1, Ino: inode, Nlink: links}
}

func TestTheProbeTellsAFoldingDirectoryFromOneThatKeepsCase(t *testing.T) {
	cases := []struct {
		name     string
		disk     fakeDisk
		want     bool
		listings int
	}{
		{"two spellings of one directory fold", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": directory(7), "/h/.config/gOOSE": directory(7)}}, true, 0},
		{"two directories are two entries", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": directory(7), "/h/.config/gOOSE": directory(8)}}, false, 0},
		{"a missing other spelling keeps case", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": directory(7)}}, false, 0},
		{"a file with one link folds", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": file(7, 1), "/h/.config/gOOSE": file(7, 1)}}, true, 0},
		{"linked twins listed apart keep case", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": file(7, 2), "/h/.config/gOOSE": file(7, 2)},
			listings: map[string][]string{"/h/.config": {"Goose", "gOOSE"}}}, false, 1},
		{"a linked file listed once folds", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": file(7, 2), "/h/.config/gOOSE": file(7, 2)},
			listings: map[string][]string{"/h/.config": {"Goose", "other"}}}, true, 1},
		{"no identity proves nothing", fakeDisk{entries: map[string]fsmeta.Meta{
			"/h/.config/Goose": {Kind: fsmeta.Dir}, "/h/.config/gOOSE": {Kind: fsmeta.Dir}}}, false, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			disk := test.disk
			if got := disk.folding().probe("/h/.config", "Goose"); got != test.want {
				t.Errorf("probe = %v, want %v", got, test.want)
			}
			if len(disk.listed) != test.listings {
				t.Errorf("listed %q; want %d listings", disk.listed, test.listings)
			}
		})
	}
}

func TestARefusedRootCheckGivesByteMatchingAndLooksAtNothing(t *testing.T) {
	disk := fakeDisk{entries: map[string]fsmeta.Meta{
		"/h/.config": directory(2), "/h/.CONFIG": directory(2),
		"/h/.config/Goose": directory(7), "/h/.config/gOOSE": directory(7)},
		refused: map[string]bool{"/h/.config": true}}
	folding := disk.folding()
	patterns := newPatterns([]string{"/h/.config/goose/secrets.yaml"})
	if folding.matchUnder(patterns, "/h/.config/Goose/secrets.yaml", 0, len("/h/")) {
		t.Error("a path behind a refused name matched as if its directory folded")
	}
	if len(disk.lstats) != 0 || len(disk.listed) != 0 {
		t.Errorf("looked at %q and listed %q behind a refused check", disk.lstats, disk.listed)
	}
}

func TestOnlyTheDirectoriesThatFoldIgnoreCase(t *testing.T) {
	// The home keeps case, .config folds, Goose keeps case: a mixed tree, as
	// a folding directory on a case-sensitive Linux volume gives.
	disk := fakeDisk{entries: map[string]fsmeta.Meta{
		"/h/.config":       directory(2),
		"/h/.config/Goose": directory(7), "/h/.config/gOOSE": directory(7),
		"/h/.config/Goose/secrets.yaml": file(9, 1)}}
	folding := disk.folding()
	cases := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"a name in the folding directory ignores case", ".config/goose/secrets.yaml", true},
		{"a name in a directory that keeps case keeps it", ".config/Goose/Secrets.yaml", false},
		{"a name in the home, which keeps case, keeps it", ".CONFIG/Goose/secrets.yaml", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := folding.matchUnder(newPatterns([]string{test.pattern}),
				"/h/.config/Goose/secrets.yaml", len("/h/"), len("/h/"))
			if got != test.want {
				t.Errorf("%s matched %v, want %v", test.pattern, got, test.want)
			}
		})
	}
}

func TestTheProbeCostsNothingForAByteHitOrAFoldedMissAndOnceADirectory(t *testing.T) {
	entries := map[string]fsmeta.Meta{
		"/h/.config": directory(2), "/h/.CONFIG": directory(2),
		"/h/.config/Goose": directory(7), "/h/.config/gOOSE": directory(7),
		"/h/.config/Goose/a": file(8, 1), "/h/.config/Goose/A": file(8, 1),
		"/h/.config/Goose/b": file(9, 1), "/h/.config/Goose/B": file(9, 1)}
	disk := fakeDisk{entries: entries}
	folding := disk.folding()
	matches := func(pattern, path string) bool {
		return folding.matchUnder(newPatterns([]string{pattern}), path, 0, len("/h/"))
	}
	if !matches("/h/.config/Goose/*", "/h/.config/Goose/a") ||
		matches("/h/.config/cursor/*", "/h/.config/Goose/a") {
		t.Fatal("a byte hit or a folded miss answered wrongly")
	}
	if len(disk.lstats) != 0 || len(disk.listed) != 0 || len(disk.checked) != 0 {
		t.Errorf("a byte hit and a folded miss looked at %q and listed %q", disk.lstats, disk.listed)
	}
	for _, path := range []string{"/h/.config/Goose/a", "/h/.config/Goose/b",
		"/h/.config/Goose/a"} {
		if !matches("/h/.config/goose/*", path) {
			t.Errorf("%s does not match through its folding directory", path)
		}
	}
	// One probe each for /h, /h/.config and /h/.config/Goose: two lstats.
	if len(disk.lstats) != 6 || len(disk.listed) != 0 {
		t.Errorf("three paths under three directories took lstats %q and listings %q; want 6 and 0",
			disk.lstats, disk.listed)
	}
	if !slices.Contains(disk.lstats, "/h/.config/gOOSE") {
		t.Errorf("the probe of .config did not look up the other spelling of Goose: %q", disk.lstats)
	}
}
