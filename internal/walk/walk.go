package walk

import (
	"io/fs"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// sharedAgent is the agent of the catalog lines whose files are collected
// for the relocation step and not counted by the inventory.
const sharedAgent = "shared"

// Logger takes the run's log lines; *runlog.Log is one.
type Logger interface {
	Line(text string) error
}

// Records takes what an --inventory walk finds instead of staging it: each
// catalog match of an agent other than shared, and each regular file's size
// and modification time.
type Records interface {
	Match(agent, glob string)
	File(agent string, size, mtime int64)
}

// Collector holds what every walk of a run reads. Excludes (empty under
// --full) and SecretGlobs are matched as Excluded and IsSecret say;
// VolumeExcludes and VolumeSecretGlobs, the `volume|` lines written for a
// volume's root, are added only under a volume's _data (CollectVolume).
// MaxSize is the size cap in bytes, 0 for none. Under Inventory nothing is
// staged: matches and files go to Records. Owners names the owners Windows
// rows record (nil: none). RootCheck is the image root's check, zero on a
// live run, so the expansion looks at nothing behind a link out of the root.
// Output names the run's own outputs, passed over without a row. It is not
// safe for concurrent use, which the sequential walk never needs.
type Collector struct {
	Tables            *catalog.Tables
	Excludes          []string
	SecretGlobs       []string
	VolumeExcludes    []string
	VolumeSecretGlobs []string
	Log               Logger
	Manifest          *manifest.Writer
	Stager            stage.Stager
	Root              string
	MaxSize           int64
	NoSecrets         bool
	Inventory         bool
	Records           Records
	Owners            *fsmeta.Owners
	RootCheck         RootCheck
	Output            stage.RunOutput
	// listDirectory, set by tests, replaces the open and listing of a
	// directory the walk visits (readEntries).
	listDirectory func(directory string) ([]fs.DirEntry, error)
}

// check is the root check the follow functions take: the image root's, nil
// on a live run.
func (collector *Collector) check() follow.Check {
	return collector.RootCheck.Expansion()
}

// pathSet returns an empty set of paths, one per file system entry, that
// asks the file system, under the root check, about two spellings of one
// path that differ only in ASCII case.
func (collector *Collector) pathSet() *syspath.Entries[struct{}] {
	return syspath.NewEntries[struct{}](follow.SameEntry(collector.check()))
}

// ownerOf returns the owner of path itself for its row, or "" without Owners.
func (collector *Collector) ownerOf(path string) string {
	if collector.Owners == nil {
		return ""
	}
	return collector.Owners.Name(path)
}

// matchesAny reports whether path matches one of globs (glob.MatchPath).
func matchesAny(globs []string, path string) bool {
	return slices.ContainsFunc(globs, func(pattern string) bool {
		return glob.MatchPath(pattern, path)
	})
}

// excluded is Excluded under the base of owner, a volume's lines included
// when it is a volume. A path not under the base matches nothing.
func (collector *Collector) excluded(owner scope, path string) bool {
	relative, under := syspath.CutPrefix(path, owner.home+"/")
	return under && (matchesAny(collector.Excludes, relative) ||
		owner.volume && matchesAny(collector.VolumeExcludes, relative))
}

// Excluded reports whether path, under home, a home or project, matches an
// active exclusion.
func (collector *Collector) Excluded(home, path string) bool {
	return collector.excluded(scope{home: home}, path)
}

// isSecret is IsSecret for a full path under the base of owner, a volume's
// lines included when it is a volume; a path not under the base is matched
// whole.
func (collector *Collector) isSecret(owner scope, path string) bool {
	relative, _ := syspath.CutPrefix(path, owner.home+"/")
	return matchesAny(collector.SecretGlobs, relative) ||
		owner.volume && matchesAny(collector.VolumeSecretGlobs, relative)
}

// IsSecret reports whether path, relative to home, a home or project,
// matches a credential glob.
func (collector *Collector) IsSecret(home, relative string) bool {
	return collector.isSecret(scope{home: home}, home+"/"+relative)
}

// scope is what one CollectPath call carries: whose rows it writes, where
// its files are staged, whether its base is a volume's _data, which paths
// other catalog lines claim, one per file system entry (syspath.Entries),
// and the lstat of a path a caller took before reading it.
type scope struct {
	user   string
	home   string
	agent  string
	volume bool
	stager stage.Stager
	claims *syspath.Entries[struct{}]
	before map[string]fsmeta.Meta
}
