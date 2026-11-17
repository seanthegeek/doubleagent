package input

import (
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// listingCache holds each directory's listing for one expansion, so a
// directory many patterns pass through is listed, and its error recorded,
// once.
type listingCache map[string][]fs.DirEntry

// matchesInHome returns every catalog hit under home, pattern by pattern in
// table order, each pattern's hits in name order. Expansion goes segment
// by segment through listings, entering only real directories.
func (c *Collection) matchesInHome(matcher *catalog.Matcher, home string) []hit {
	cache := listingCache{}
	var hits []hit
	for i := range matcher.Home {
		hits = c.expand(&matcher.Home[i], 0, home, nil, cache, hits)
	}
	return hits
}

// nestedMatches returns the hits of the entries with two or more segments
// whose first segment names the agent directory tree, such as
// .gemini/antigravity-cli inside a .gemini given as the root.
func (c *Collection) nestedMatches(matcher *catalog.Matcher, tree string) []hit {
	cache := listingCache{}
	var hits []hit
	name := filepath.Base(tree)
	for i := range matcher.Home {
		pattern := &matcher.Home[i]
		if pattern.Depth() < 2 || !pattern.MatchSegment(0, name) {
			continue
		}
		hits = c.expand(pattern, 1, tree, []string{name}, cache, hits)
	}
	return hits
}

// expand appends to hits each path below base that segments index onward
// of pattern match. The last segment matches any entry, a link included;
// an earlier one only a real directory.
func (c *Collection) expand(pattern *catalog.Pattern, index int, base string, rel []string,
	cache listingCache, hits []hit) []hit {
	if index == pattern.Depth() {
		return append(hits, hit{pattern: pattern, path: base, rel: rel})
	}
	key := base
	entries, listed := cache[key]
	if !listed {
		var err error
		if entries, err = c.listDirectory(key); err != nil {
			entries = nil
			c.recordProblem(describeError(key, err))
		}
		cache[key] = entries
	}
	last := index+1 == pattern.Depth()
	for _, entry := range entries {
		name := entry.Name()
		if !pattern.MatchSegment(index, name) || (!last && !entry.IsDir()) {
			continue
		}
		hits = c.expand(pattern, index+1, joinPath(base, name), appendPart(rel, name), cache, hits)
	}
	return hits
}

// addHits makes the files under each hit artifacts of home. The most
// specific pattern claims a file first (.gemini/antigravity-cli before the
// enclosing .gemini), as the collectors' nested-claim rule does, and each
// file is attributed once.
func (c *Collection) addHits(home *Home, hits []hit) {
	slices.SortStableFunc(hits, func(first, second hit) int {
		return second.pattern.Depth() - first.pattern.Depth()
	})
	seen := map[string]bool{}
	for _, found := range hits {
		c.walkFiles(found.path, found.rel, func(diskPath string, rel []string) {
			if seen[diskPath] {
				return
			}
			seen[diskPath] = true
			relText := joinSlash(rel)
			original := relText
			if home.Original != "" {
				original = trimRightSlash(home.Original) + "/" + relText
			}
			c.Artifacts = append(c.Artifacts, &Artifact{DiskPath: diskPath, Original: original,
				Rel: relText, Agent: found.pattern.Agent, Home: home})
		})
	}
}

// walkFiles calls yield for start when it is a regular file, or for every
// file below it when it is a directory, top-down in name order; links
// are never followed or yielded. A start that is neither, or a directory
// that cannot be listed, is a problem.
func (c *Collection) walkFiles(start string, rel []string, yield func(string, []string)) {
	info, err := osfile.Lstat(start)
	switch {
	case err != nil:
		c.recordProblem(describeError(start, err))
		return
	case info.Mode()&fs.ModeSymlink != 0:
		return
	case info.Mode().IsRegular():
		yield(start, rel)
		return
	case !info.IsDir():
		c.recordProblem(describeError(start, errNotDirectory))
		return
	}
	stack := []directory{{path: start, rel: rel}}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		entries, err := c.listDirectory(current.path)
		if err != nil {
			c.recordProblem(describeError(current.path, err))
			continue
		}
		for _, entry := range entries {
			if entry.Type()&(fs.ModeDir|fs.ModeSymlink|fs.ModeIrregular) == 0 {
				yield(joinPath(current.path, entry.Name()), appendPart(current.rel, entry.Name()))
			}
		}
		stack = append(stack, subdirectories(current, entries)...)
	}
}
