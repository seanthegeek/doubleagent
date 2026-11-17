package input

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// homeParents returns the parent directories whose children are user
// homes, as the collectors enumerate them.
func homeParents() [][]string {
	return [][]string{{"home"}, {"Users"}, {"usr", "home"}, {"export", "home"}}
}

// directory is one directory of a walk: its path, joined to the walk's
// base by filepath.Join, and its components below the walk's base.
type directory struct {
	path string
	rel  []string
}

// hit is a path a catalog pattern matched, with its components below the
// home it was found in.
type hit struct {
	pattern *catalog.Pattern
	path    string
	rel     []string
}

// discoverLoose finds the homes of a loose tree by catalog hits and makes
// their files artifacts. fs/ without a manifest is taken as the root; a
// root that is itself an agent directory makes its parent the one home.
func (c *Collection) discoverLoose(matcher *catalog.Matcher) error {
	tree := c.Root
	if lstatMode(joinPath(c.Root, "fs")).IsDir() {
		if _, err := osfile.Lstat(joinPath(c.Root, "manifest.jsonl")); err != nil {
			tree = joinPath(tree, "fs")
			c.Notes = append(c.Notes, "fs/ found without manifest.jsonl; treating fs/ as the root")
		}
	}
	if c.agentDirectoryRoot(matcher, tree) {
		return nil
	}
	c.walkForHomes(matcher, directory{path: tree})
	return nil
}

// agentDirectoryRoot handles a tree that is an agent directory (~/.claude
// copied on its own): its parent is the home, restricted to the tree, and
// nothing inside it is a home. It reports whether the tree was one.
func (c *Collection) agentDirectoryRoot(matcher *catalog.Matcher, tree string) bool {
	name := filepath.Base(tree)
	for i := range matcher.Home {
		pattern := &matcher.Home[i]
		if pattern.Depth() != 1 || pattern.Agent == sharedAgent || !pattern.MatchSegment(0, name) {
			continue
		}
		home := &Home{DiskPath: parentPath(tree), User: userFromParts(resolvedParent(tree)),
			Host: c.Host, Inferred: true}
		c.Notes = append(c.Notes, "root is a "+pattern.Agent+" directory; treating its parent as the home")
		c.Homes = append(c.Homes, home)
		hits := []hit{{pattern: pattern, path: tree, rel: []string{name}}}
		hits = append(hits, c.nestedMatches(matcher, tree)...)
		c.addHits(home, hits)
		return true
	}
	return false
}

// resolvedParent returns the elements of the tree's parent once its links
// are resolved, or of its parent as written when it cannot be resolved.
func resolvedParent(tree string) []string {
	if physical, err := stage.PhysicalDir(tree); err == nil {
		return pathElements(parentPath(physical))
	}
	return pathElements(parentPath(tree))
}

// walkForHomes walks the tree from start top-down, each directory's
// entries in name order: a directory holding a name that may begin a
// catalog glob and a real agent's hit is a home and is not entered
// further; links are never entered.
func (c *Collection) walkForHomes(matcher *catalog.Matcher, start directory) {
	stack := []directory{start}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := c.listDirectory(current.path)
		if err != nil {
			c.recordProblem(describeError(current.path, err))
			continue
		}
		if holdsFirstSegment(matcher, entries) {
			hits := c.matchesInHome(matcher, current.path)
			if hasAgentHit(hits) {
				home := makeHome(current, c.Host)
				c.Homes = append(c.Homes, home)
				c.addHits(home, hits)
				continue
			}
		}
		stack = append(stack, subdirectories(current, entries)...)
	}
}

// holdsFirstSegment reports whether any entry's name matches the first
// segment of a home entry.
func holdsFirstSegment(matcher *catalog.Matcher, entries []fs.DirEntry) bool {
	for _, entry := range entries {
		if matcher.MatchesFirstSegment(entry.Name()) {
			return true
		}
	}
	return false
}

// hasAgentHit reports whether a hit belongs to an agent other than shared:
// a directory holding only shared files (AGENTS.md) is a project, not a
// home.
func hasAgentHit(hits []hit) bool {
	for _, found := range hits {
		if found.pattern.Agent != sharedAgent {
			return true
		}
	}
	return false
}

// subdirectories returns the real directories among entries, links left
// out, in reverse name order, for a stack that pops them in order.
func subdirectories(parent directory, entries []fs.DirEntry) []directory {
	var children []directory
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].IsDir() {
			name := entries[i].Name()
			children = append(children, directory{path: joinPath(parent.path, name),
				rel: appendPart(parent.rel, name)})
		}
	}
	return children
}

// appendPart returns parts with part appended, never sharing parts' array.
func appendPart(parts []string, part string) []string {
	return append(slices.Clip(parts), part)
}

// makeHome returns the inferred home at a walked directory: its user from
// the home-parent conventions, else the directory's name (a directory that
// holds agent state is usually named after its owner) unless it is fs.
func makeHome(place directory, host string) *Home {
	user := userFromParts(place.rel)
	if user == "" && len(place.rel) > 0 && place.rel[len(place.rel)-1] != "fs" {
		user = place.rel[len(place.rel)-1]
	}
	original := ""
	if len(place.rel) > 0 {
		original = "/" + joinSlash(place.rel)
	}
	return &Home{DiskPath: place.path, Original: original, User: user, Host: host, Inferred: true}
}

// userFromParts returns the owner of a home from its path components, or
// "": the last component below home, Users, usr/home or export/home, or
// root for root and var/root.
func userFromParts(parts []string) string {
	parts = slices.DeleteFunc(slices.Clone(parts), func(part string) bool { return part == "" })
	for _, parent := range homeParents() {
		if len(parts) >= len(parent)+1 && slices.Equal(parts[len(parts)-len(parent)-1:len(parts)-1], parent) {
			return parts[len(parts)-1]
		}
	}
	if joined := joinSlash(parts); joined == "root" || joined == "var/root" {
		return "root"
	}
	return ""
}

// recordProblem appends line to the problems unless it is there already.
func (c *Collection) recordProblem(line string) {
	if !slices.Contains(c.Problems, line) {
		c.Problems = append(c.Problems, line)
	}
}

// listDirectory lists a directory of a loose tree in name order, not the
// file system's: the order artifacts reach the parsers decides ties in the
// timeline and which of two equal secondary rows is kept, and two file
// systems list one tree differently. Names compare as bytes, with no
// locale or case folding, so "B" sorts before "a" and both are kept, and a
// byte that is not valid UTF-8 sorts by its value.
func (c *Collection) listDirectory(path string) ([]fs.DirEntry, error) {
	entries, err := c.readDir(path)
	slices.SortFunc(entries, func(first, second fs.DirEntry) int {
		return strings.Compare(first.Name(), second.Name())
	})
	return entries, err
}
