// Package walk is the catalog walk: it expands the catalog tables under a
// base (a home, a project or a volume), folds case variants of one path,
// claims nested matches, walks each match depth first and stages every file
// with its manifest row, reproducing the sh collector's "Catalog walk"
// section and its worker programs. collectors/docs/internals.md, "Glob
// matching" and "Claimed paths and case variants", has the rules.
package walk

import (
	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
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
// exclusion table (empty under --full); Secret tells whether a path, relative
// to its base, is a credential file; MaxSize is the size cap in bytes, 0 for
// none. Under Inventory nothing is staged or written: matches and files go to
// Records.
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
}

// MatchAny returns a test that is true when a path matches any of globs as
// the shell's case does, the test the sh collector compiles SECRET_GLOBS into.
func MatchAny(globs []string) func(string) bool {
	patterns := append([]string(nil), globs...)
	return func(text string) bool {
		for _, pattern := range patterns {
			if glob.MatchPath(pattern, text) {
				return true
			}
		}
		return false
	}
}

// scope is what one collect_path call carries: whose rows it writes, where
// its files are staged and which paths other catalog lines claim.
type scope struct {
	user   string
	home   string
	agent  string
	stager stage.Stager
	claims map[string]struct{}
}

// excludePatterns returns the active exclusions as patterns for full paths
// under home, the base escaped so only the globs are patterns, as find -path
// takes them.
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
	for _, pattern := range patterns {
		if glob.MatchPath(pattern, path) {
			return true
		}
	}
	return false
}
