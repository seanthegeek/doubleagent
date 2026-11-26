package catalog

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/glob"
)

// Pattern is one `agent|glob` line split for matching a path against it:
// the glob's segments, empty ones dropped, each matched against one whole
// directory entry name by glob.MatchSegment, the rule collect expands the
// line by.
type Pattern struct {
	Agent    string
	Glob     string
	Segments []string
}

// Matcher holds the home and project tables of a catalog split for
// matching paths, in table order: by agent, then glob, as Parse sorts them.
type Matcher struct {
	Home    []Pattern
	Project []Pattern
	// firstSegments holds each distinct first segment of the home table
	// once, in the order of first appearance.
	firstSegments []string
}

// NewMatcher splits the home and project tables of tables into patterns. A
// glob with no segment is an error naming the line.
func NewMatcher(tables *Tables) (*Matcher, error) {
	matcher := &Matcher{}
	seen := map[string]bool{}
	for _, entry := range tables.Catalog {
		pattern, err := splitPattern(entry)
		if err != nil {
			return nil, err
		}
		matcher.Home = append(matcher.Home, pattern)
		if first := pattern.Segments[0]; !seen[first] {
			seen[first] = true
			matcher.firstSegments = append(matcher.firstSegments, first)
		}
	}
	for _, entry := range tables.ProjectCatalog {
		pattern, err := splitPattern(entry)
		if err != nil {
			return nil, err
		}
		matcher.Project = append(matcher.Project, pattern)
	}
	return matcher, nil
}

// splitPattern splits the glob of one entry into its segments.
func splitPattern(entry Entry) (Pattern, error) {
	pattern := Pattern{Agent: entry.Agent, Glob: entry.Glob, Segments: splitNonEmpty(entry.Glob, "/")}
	if len(pattern.Segments) == 0 {
		return Pattern{}, fmt.Errorf("catalog glob %q has no path segment", entry.Glob)
	}
	return pattern, nil
}

// Depth returns the number of segments of p.
func (p Pattern) Depth() int { return len(p.Segments) }

// MatchSegment reports whether name, one directory entry name, matches
// segment i of p.
func (p Pattern) MatchSegment(i int, name string) bool {
	return glob.MatchSegment(p.Segments[i], name)
}

// matchPrefix reports whether the first segments of parts match every
// segment of p; parts must be at least as many as the segments.
func (p Pattern) matchPrefix(parts []string) bool {
	for i := range p.Segments {
		if !p.MatchSegment(i, parts[i]) {
			return false
		}
	}
	return true
}

// MatchesFirstSegment reports whether name matches the first segment of any
// home entry, the test that a directory holding name may be a home.
func (m *Matcher) MatchesFirstSegment(name string) bool {
	for _, segment := range m.firstSegments {
		if glob.MatchSegment(segment, name) {
			return true
		}
	}
	return false
}

// ProjectAgentFor returns the agent of the project entry that claims rel, a
// path relative to a project directory (slashes or backslashes), or "".
// The entry with the most segments wins, and among equals the first in table
// order (by agent, then glob), as collect attributes a file.
func (m *Matcher) ProjectAgentFor(rel string) string {
	return bestClaim(m.Project, splitNonEmpty(strings.ReplaceAll(rel, `\`, "/"), "/"))
}

// ClaimsHomePath returns the agent of the home entry that claims rel, a
// path relative to a home whose parts separator divides, or "", as
// ProjectAgentFor picks among the project entries.
func (m *Matcher) ClaimsHomePath(rel, separator string) string {
	return bestClaim(m.Home, splitNonEmpty(rel, separator))
}

// bestClaim returns the agent of the deepest pattern matching the leading
// parts, the first among equals, or "".
func bestClaim(patterns []Pattern, parts []string) string {
	best := -1
	for i, pattern := range patterns {
		depth := pattern.Depth()
		if depth > len(parts) || (best >= 0 && depth <= patterns[best].Depth()) {
			continue
		}
		if pattern.matchPrefix(parts) {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return patterns[best].Agent
}

// splitNonEmpty splits text at separator, dropping empty parts.
func splitNonEmpty(text, separator string) []string {
	var parts []string
	for _, part := range strings.Split(text, separator) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// Agents returns the agent names of the home entries in table order, each
// once: sorted by name for a table Parse read.
func (m *Matcher) Agents() []string {
	var agents []string
	seen := map[string]bool{}
	for _, pattern := range m.Home {
		if !seen[pattern.Agent] {
			seen[pattern.Agent] = true
			agents = append(agents, pattern.Agent)
		}
	}
	return agents
}
