package walk

import (
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

// Collector holds what every walk of a run reads. Excludes is the active
// exclusion table (empty under --full) and SecretGlobs the credential globs,
// matched as Excluded and IsSecret say; MaxSize is the size cap in bytes, 0
// for none. Under Inventory nothing is staged or written: matches and files go to
// Records. Owners names the owners the Windows rows record; nil records none.
// RootCheck is the image root's check (NewRootCheck), the zero RootCheck on
// a live run: the catalog expansion looks at nothing behind a link out of
// the image root.
// It is not safe for concurrent use, which the sequential walk never needs.
type Collector struct {
	Tables      *catalog.Tables
	Excludes    []string
	SecretGlobs []string
	Log         Logger
	Manifest    *manifest.Writer
	Stager      stage.Stager
	Root        string
	MaxSize     int64
	NoSecrets   bool
	Inventory   bool
	Records     Records
	Owners      *fsmeta.Owners
	RootCheck   RootCheck
	// folding, secretPatterns and the last home's exclusion patterns are
	// made on first use (caseFolding, isSecret, homeExcludes).
	folding         *caseFolding
	secretPatterns  []pathPattern
	excludedHome    string
	excludedPattern []pathPattern
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

// Excluded reports whether path, under home, matches an active exclusion as
// a shell's case statement matches it: on Windows ignoring ASCII case, on
// Linux and macOS ignoring it only in the components below home whose
// directory folds case (docs/internals.md, "Glob matching").
func (collector *Collector) Excluded(home, path string) bool {
	return collector.matchesUnder(collector.homeExcludes(home), home, path)
}

// ExcludedByBytes reports whether path, under home, matches an active
// exclusion by glob.MatchPath alone: byte for byte on Linux and macOS,
// ignoring ASCII case on Windows. It never looks at the disk.
func (collector *Collector) ExcludedByBytes(home, path string) bool {
	return slices.ContainsFunc(collector.homeExcludes(home), func(pattern pathPattern) bool {
		return glob.MatchPath(pattern.text, path)
	})
}

// homeExcludes returns the exclusion patterns under home, kept for the
// last home asked about.
func (collector *Collector) homeExcludes(home string) []pathPattern {
	if collector.excludedPattern == nil || collector.excludedHome != home {
		collector.excludedHome, collector.excludedPattern = home, collector.excludePatterns(home)
	}
	return collector.excludedPattern
}

// IsSecret reports whether path, relative to home, matches a credential
// glob, ASCII case counting as Excluded says.
func (collector *Collector) IsSecret(home, relative string) bool {
	return collector.isSecret(home, home+"/"+relative)
}

// isSecret is IsSecret for a full path; a path not under home is matched
// whole, byte for byte on Linux and macOS.
func (collector *Collector) isSecret(home, path string) bool {
	if collector.secretPatterns == nil {
		collector.secretPatterns = newPatterns(collector.SecretGlobs)
	}
	relative, under := syspath.CutPrefix(path, home+"/")
	from, foldFrom := len(path)-len(relative), len(path)
	if under {
		foldFrom = from
	}
	return collector.caseFolding().matchUnder(collector.secretPatterns, path, from, foldFrom)
}

// scope is what one CollectPath call carries: whose rows it writes, where
// its files are staged and which paths other catalog lines claim, one per
// file system entry (syspath.Entries), and the lstat of a path a caller took
// before reading it.
type scope struct {
	user   string
	home   string
	agent  string
	stager stage.Stager
	claims *syspath.Entries[struct{}]
	before map[string]fsmeta.Meta
}

// excludePatterns returns the active exclusions as patterns for full paths
// under home, the base escaped so only the globs are patterns.
func (collector *Collector) excludePatterns(home string) []pathPattern {
	base := glob.QuoteMeta(home)
	globs := make([]string, 0, len(collector.Excludes))
	for _, exclude := range collector.Excludes {
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
