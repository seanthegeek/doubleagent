// Package walk is the catalog walk: it expands the catalog tables under a
// base (a home, a project or a volume), folds case variants of one path,
// claims nested matches, walks each match depth first and stages every file
// with its manifest row. collectors/docs/internals.md, "Glob matching" and
// "Claimed paths and case variants", has the rules.
//
// filepath.WalkDir and os.ReadDir return a directory's entries sorted by name
// and know nothing of the image root; this walk lists each directory in the
// order the system gives it and asks the root check before it enters one.
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

// fileLogger is a Logger that can also write a line to the log file alone,
// as *runlog.Log does, for the lines a program writes into the log.
type fileLogger interface {
	FileLine(text string) error
}

// Records takes what an --inventory walk finds instead of staging it: each
// catalog match of an agent other than shared, and each regular file's size
// and modification time.
type Records interface {
	Match(agent, glob string)
	File(agent string, size, mtime int64)
}

// Collector holds what every walk of a run reads. Excludes is the active
// exclusion table (empty under --full); Secret tells whether a path, relative
// to its base, is a credential file; MaxSize is the size cap in bytes, 0 for
// none. Under Inventory nothing is staged or written: matches and files go to
// Records. Owners names the owners the Windows rows record; nil records none.
// RootCheck is the image root's check (NewRootCheck), the zero RootCheck on
// a live run: the catalog expansion looks at nothing behind a link out of
// the image root.
// It is not safe for concurrent use, which the sequential walk never needs.
type Collector struct {
	Tables    *catalog.Tables
	Excludes  []string
	Secret    func(relative string) bool
	Log       Logger
	Manifest  *manifest.Writer
	Stager    stage.Stager
	Root      string
	MaxSize   int64
	NoSecrets bool
	Inventory bool
	Records   Records
	Owners    *fsmeta.Owners
	RootCheck RootCheck
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

// MatchAny returns a test that is true when a path matches any of globs as
// a shell's case statement matches them (glob.MatchPath).
func MatchAny(globs []string) func(string) bool {
	patterns := append([]string(nil), globs...)
	return func(text string) bool {
		return slices.ContainsFunc(patterns, func(pattern string) bool {
			return glob.MatchPath(pattern, text)
		})
	}
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
func (collector *Collector) excludePatterns(home string) []string {
	base := glob.QuoteMeta(home)
	patterns := make([]string, 0, len(collector.Excludes))
	for _, exclude := range collector.Excludes {
		if exclude != "" {
			patterns = append(patterns, base+"/"+exclude)
		}
	}
	return patterns
}

// matchesAny reports whether path matches one of patterns.
func matchesAny(patterns []string, path string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		return glob.MatchPath(pattern, path)
	})
}
