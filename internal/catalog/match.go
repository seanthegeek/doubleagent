package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// Pattern is one `agent|glob` line compiled for matching a path against
// it: the glob split at slashes into segments, empty ones
// dropped, each segment a regular expression matched against one whole
// directory entry name. A `*` or `?` never matches a slash, takes a leading
// dot as any other character, and counts characters (code points), not
// bytes; a `[...]` class passes through, `!` negating it.
type Pattern struct {
	Agent    string
	Glob     string
	Segments []string
	exact    []*regexp.Regexp
	folded   []*regexp.Regexp
}

// Matcher holds the home and project tables of a catalog compiled for
// matching paths, in table order.
type Matcher struct {
	Home    []Pattern
	Project []Pattern
	// firstSegments holds one expression per distinct first segment of the
	// home table, in the order of first appearance.
	firstSegments []*regexp.Regexp
}

// NewMatcher compiles the home and project tables of tables. A glob with no
// segment, or a segment the regular expression syntax rejects, is an error
// naming the line.
func NewMatcher(tables *Tables) (*Matcher, error) {
	matcher := &Matcher{}
	seen := map[string]bool{}
	for _, entry := range tables.Catalog {
		pattern, err := compilePattern(entry)
		if err != nil {
			return nil, err
		}
		matcher.Home = append(matcher.Home, pattern)
		if first := pattern.Segments[0]; !seen[first] {
			seen[first] = true
			matcher.firstSegments = append(matcher.firstSegments, pattern.exact[0])
		}
	}
	for _, entry := range tables.ProjectCatalog {
		pattern, err := compilePattern(entry)
		if err != nil {
			return nil, err
		}
		matcher.Project = append(matcher.Project, pattern)
	}
	return matcher, nil
}

// compilePattern compiles the segments of one entry in both case modes.
func compilePattern(entry Entry) (Pattern, error) {
	pattern := Pattern{Agent: entry.Agent, Glob: entry.Glob}
	for _, segment := range strings.Split(entry.Glob, "/") {
		if segment == "" {
			continue
		}
		expression := `\A(?:` + SegmentExpression(segment) + `)\z`
		exact, err := regexp.Compile(expression)
		if err != nil {
			return Pattern{}, fmt.Errorf("catalog glob %q: %w", entry.Glob, err)
		}
		folded, err := regexp.Compile(`(?i)` + expression)
		if err != nil {
			return Pattern{}, fmt.Errorf("catalog glob %q: %w", entry.Glob, err)
		}
		pattern.Segments = append(pattern.Segments, segment)
		pattern.exact = append(pattern.exact, exact)
		pattern.folded = append(pattern.folded, folded)
	}
	if len(pattern.Segments) == 0 {
		return Pattern{}, fmt.Errorf("catalog glob %q has no path segment", entry.Glob)
	}
	return pattern, nil
}

// SegmentExpression returns the regular expression, unanchored, that one
// glob segment stands for: `*` is `[^/]*`, `?` is `[^/]`, a `[` with a
// closing `]` later in the segment opens a class whose leading `!` becomes
// `^` and whose backslashes are taken literally, and every other character,
// an unclosed `[` included, matches itself.
func SegmentExpression(segment string) string {
	var out strings.Builder
	for i := 0; i < len(segment); i++ {
		switch segment[i] {
		case '*':
			out.WriteString("[^/]*")
		case '?':
			out.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(segment[i+1:], ']')
			if end < 0 {
				out.WriteString(regexp.QuoteMeta("["))
				continue
			}
			body := segment[i+1 : i+1+end]
			if strings.HasPrefix(body, "!") {
				body = "^" + body[1:]
			}
			out.WriteString("[" + strings.ReplaceAll(body, `\`, `\\`) + "]")
			i += 1 + end
		default:
			out.WriteString(regexp.QuoteMeta(segment[i : i+1]))
		}
	}
	return out.String()
}

// Depth returns the number of segments of p.
func (p Pattern) Depth() int { return len(p.Segments) }

// MatchSegment reports whether name, one directory entry name, matches
// segment i of p exactly, case included.
func (p Pattern) MatchSegment(i int, name string) bool {
	return p.exact[i].MatchString(name)
}

// matchPrefix reports whether the first segments of parts match every
// segment of p, in the case mode folded selects; parts must be at least as
// many as the segments.
func (p Pattern) matchPrefix(parts []string, folded bool) bool {
	expressions := p.exact
	if folded {
		expressions = p.folded
	}
	for i, expression := range expressions {
		if !expression.MatchString(parts[i]) {
			return false
		}
	}
	return true
}

// MatchesFirstSegment reports whether name matches the first segment of any
// home entry, the test that a directory holding name may be a home.
func (m *Matcher) MatchesFirstSegment(name string) bool {
	for _, expression := range m.firstSegments {
		if expression.MatchString(name) {
			return true
		}
	}
	return false
}

// ProjectAgentFor returns the agent of the project entry that claims rel, a
// path relative to a project directory (slashes or backslashes), or "".
// The entry with the most segments wins, the first in table order among
// equals, as collect attributes a file.
func (m *Matcher) ProjectAgentFor(rel string) string {
	return bestClaim(m.Project, splitNonEmpty(strings.ReplaceAll(rel, `\`, "/"), "/"), false)
}

// ClaimsHomePath returns the agent of the home entry that claims rel, a
// path relative to a home, or "": the entry whose every segment matches the
// path's segment at the same place, the most specific one winning. With
// windows set, rel is split at backslashes and matched without regard to
// case.
func (m *Matcher) ClaimsHomePath(rel string, windows bool) string {
	separator := "/"
	if windows {
		separator = `\`
	}
	return bestClaim(m.Home, splitNonEmpty(rel, separator), windows)
}

// bestClaim returns the agent of the deepest pattern matching the leading
// parts, the first among equals, or "".
func bestClaim(patterns []Pattern, parts []string, folded bool) string {
	best := -1
	for i, pattern := range patterns {
		depth := pattern.Depth()
		if depth > len(parts) || (best >= 0 && depth <= patterns[best].Depth()) {
			continue
		}
		if pattern.matchPrefix(parts, folded) {
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
// once.
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
