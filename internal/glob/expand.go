package glob

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Check reports whether expansion must not look at path, a directory it is
// about to list or look a name up in, and the link that decides it: the
// image root's check, which every function that follows a link also takes.
type Check = follow.Check

// Refusal is a path expansion did not look behind: the pattern being
// expanded, the check's refusal (the link and why), and the directory whose
// listing or lookup it stopped.
type Refusal struct {
	Pattern string
	follow.Refusal
	Directory string
}

// Expand returns base + "/" + match for every match of pattern inside base
// (absolute, or empty for the root), sorted bytewise. Each segment, literal
// or not, is compared with the names its directory lists, so a match is
// spelled as the disk holds it, and a symlinked directory in the middle of
// the pattern is followed. check (nil on a live run) is asked before each
// directory is listed; directories are listed through fsmeta.OpenDir with
// opener.
func Expand(base, pattern string, check Check, opener *fsmeta.Opener) ([]string, []Refusal) {
	return NewExpander(check, opener).Expand(base, pattern)
}

// Expander is Expand for many patterns under one base: it keeps each
// directory's check answer and listing, since every catalog line's literal
// segments read the same few directories, and asking and listing them once
// per line tripled the time of a collection over homes of thousands of
// entries. It serves one base's table at a time, so nothing it keeps is
// older than that expansion.
type Expander struct {
	check    Check
	opener   *fsmeta.Opener
	listings map[string]listing
}

// listing is what an Expander keeps of one directory: the check's refusal,
// if any, and otherwise its entry names and whether all were read.
type listing struct {
	refusal  follow.Refusal
	refused  bool
	names    []string
	complete bool
}

// NewExpander returns an Expander that asks check and lists through opener.
func NewExpander(check Check, opener *fsmeta.Opener) *Expander {
	return &Expander{check: check, opener: opener, listings: map[string]listing{}}
}

// Expand is the package's Expand under the expander's check and opener,
// each directory checked and listed at most once for the expander's life.
func (expander *Expander) Expand(base, pattern string) ([]string, []Refusal) {
	run := &expansion{Expander: expander, pattern: pattern}
	paths := []string{base}
	for _, segment := range strings.Split(pattern, "/") {
		var next []string
		for _, directory := range paths {
			next = append(next, run.matchIn(directory, segment)...)
		}
		paths = next
	}
	slices.Sort(paths)
	return paths, run.refused
}

// expansion is one Expand call: the pattern and the links refused so far,
// each reported once.
type expansion struct {
	*Expander
	pattern string
	refused []Refusal
}

// matchIn returns the paths in directory whose name matches segment, as
// its listing spells them. A directory that can be entered but not listed
// (a home of mode 711) answers a literal segment by lstat, spelled as the
// pattern spells it. Nothing is looked at where check refuses directory.
func (run *expansion) matchIn(directory, segment string) []string {
	if segment == "" {
		return nil
	}
	listed := run.listingOf(directory)
	if listed.refused {
		return nil
	}
	var found []string
	for _, name := range listed.names {
		if MatchSegment(segment, name) {
			found = append(found, directory+"/"+name)
		}
	}
	if !listed.complete && found == nil && !strings.ContainsAny(segment, "*?[") {
		if _, err := osfile.Lstat(directory + "/" + segment); err == nil {
			found = append(found, directory+"/"+segment)
		}
	}
	return found
}

// listingOf returns what the expander keeps of directory, asking check
// before listing it the first time, and records a refused link once for
// this expansion.
func (run *expansion) listingOf(directory string) listing {
	listed, known := run.listings[directory]
	if !known {
		if run.check != nil {
			listed.refusal, listed.refused = run.check(directory)
		}
		if !listed.refused {
			listed = readNames(run.opener, directory+"/")
		}
		run.listings[directory] = listed
	}
	known = slices.ContainsFunc(run.refused, func(earlier Refusal) bool {
		return earlier.Link == listed.refusal.Link
	})
	if listed.refused && !known {
		run.refused = append(run.refused, Refusal{Pattern: run.pattern, Refusal: listed.refusal,
			Directory: directory})
	}
	return listed
}

// readNames returns the entry names of directory in listing order and
// whether the whole listing was read; a listing that fails part way gives
// the names read before the error.
func readNames(opener *fsmeta.Opener, directory string) listing {
	handle, err := fsmeta.OpenDir(opener, directory)
	if err != nil {
		return listing{}
	}
	defer handle.Close()
	entries, err := handle.ReadDir(-1)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return listing{names: names, complete: err == nil}
}
