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
	// folding, the secret patterns of a home and of a volume, and the last
	// base's exclusion patterns are made on first use (caseFolding,
	// isSecret, baseExcludes).
	folding         *caseFolding
	secretPatterns  [2][]pathPattern
	excludedHome    string
	excludedVolume  bool
	excludedPattern []pathPattern
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

// pathPattern is one EXCLUDES or SECRET_GLOBS pattern as written and in the
// form that matches it with or without ASCII case.
type pathPattern struct {
	text    string
	folding glob.FoldingPattern
}

// newPatterns returns globs as patterns, each lowered once.
func newPatterns(globs []string) []pathPattern {
	patterns := make([]pathPattern, len(globs))
	for i, text := range globs {
		patterns[i] = pathPattern{text: text, folding: glob.NewFoldingPattern(text)}
	}
	return patterns
}

// caseFolding returns the run's cache of which directories fold case, read
// under the root check and the stager's opener, made on first use.
func (collector *Collector) caseFolding() *caseFolding {
	if collector.folding == nil {
		collector.folding = newCaseFolding(collector.check(), collector.Stager.Opener)
	}
	return collector.folding
}

// Excluded reports whether path, under home, a home or project, matches an
// active exclusion as a shell's case statement matches it: on Windows
// ignoring ASCII case, on Linux and macOS ignoring it only in the components
// below home whose directory folds case (docs/internals.md, "Glob matching").
func (collector *Collector) Excluded(home, path string) bool {
	return collector.excluded(scope{home: home}, path)
}

// excluded is Excluded under the base of owner, a volume's lines included
// when it is a volume.
func (collector *Collector) excluded(owner scope, path string) bool {
	return collector.matchesUnder(collector.baseExcludes(owner), owner.home, path)
}

// ExcludedByBytes reports whether path, under home, a home or project,
// matches an active exclusion by glob.MatchPath alone: byte for byte on
// Linux and macOS, ignoring ASCII case on Windows. It never looks at the
// disk.
func (collector *Collector) ExcludedByBytes(home, path string) bool {
	return slices.ContainsFunc(collector.baseExcludes(scope{home: home}), func(pattern pathPattern) bool {
		return glob.MatchPath(pattern.text, path)
	})
}

// baseExcludes returns the exclusion patterns under the base of owner, kept
// for the last base asked about.
func (collector *Collector) baseExcludes(owner scope) []pathPattern {
	if collector.excludedPattern == nil || collector.excludedHome != owner.home ||
		collector.excludedVolume != owner.volume {
		collector.excludedHome, collector.excludedVolume = owner.home, owner.volume
		collector.excludedPattern = collector.excludePatterns(owner)
	}
	return collector.excludedPattern
}

// IsSecret reports whether path, relative to home, a home or project,
// matches a credential glob, ASCII case counting as Excluded says.
func (collector *Collector) IsSecret(home, relative string) bool {
	return collector.isSecret(scope{home: home}, home+"/"+relative)
}

// isSecret is IsSecret for a full path under the base of owner, a volume's
// lines included when it is a volume; a path not under the base is matched
// whole, byte for byte on Linux and macOS.
func (collector *Collector) isSecret(owner scope, path string) bool {
	patterns := &collector.secretPatterns[0]
	if owner.volume {
		patterns = &collector.secretPatterns[1]
	}
	if *patterns == nil && owner.volume {
		*patterns = newPatterns(slices.Concat(collector.SecretGlobs, collector.VolumeSecretGlobs))
	} else if *patterns == nil {
		*patterns = newPatterns(collector.SecretGlobs)
	}
	relative, under := syspath.CutPrefix(path, owner.home+"/")
	from, foldFrom := len(path)-len(relative), len(path)
	if under {
		foldFrom = from
	}
	return collector.caseFolding().matchUnder(*patterns, path, from, foldFrom)
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

// excludePatterns returns the active exclusions as patterns for full paths
// under the base of owner, a volume's lines included when it is a volume,
// the base escaped so only the globs are patterns.
func (collector *Collector) excludePatterns(owner scope) []pathPattern {
	base := glob.QuoteMeta(owner.home)
	excludes := collector.Excludes
	if owner.volume {
		excludes = slices.Concat(excludes, collector.VolumeExcludes)
	}
	globs := make([]string, 0, len(excludes))
	for _, exclude := range excludes {
		if exclude != "" {
			globs = append(globs, base+"/"+exclude)
		}
	}
	return newPatterns(globs)
}

// matchesUnder reports whether path matches one of patterns, the components
// below home compared ignoring ASCII case where their directory folds it.
func (collector *Collector) matchesUnder(patterns []pathPattern, home, path string) bool {
	if len(patterns) == 0 {
		return false
	}
	relative, under := syspath.CutPrefix(path, home+"/")
	foldFrom := len(path) - len(relative)
	if !under {
		foldFrom = len(path)
	}
	return collector.caseFolding().matchUnder(patterns, path, 0, foldFrom)
}
