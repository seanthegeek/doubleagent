package analyze

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// onDisk is what a collection holds of one project path: its agents' files,
// its shared files and its repository.
type onDisk struct {
	agents      agentCounts
	sharedFiles int64
	git         projectGit
}

// diskEntry is one project path of the input with what it holds.
type diskEntry struct {
	path string
	onDisk
}

// projectFiles returns the project directories of the input keyed as
// collectedProjects keys them, in the order of their first directory by
// path: directories that come to one path summed, the first repository
// with remotes, and the first with a head, kept.
func projectFiles(projects []projectEntry, summary jsonvalue.Value) []diskEntry {
	var entries firstMet[string, diskEntry]
	for _, project := range projects {
		key := timeline.CanonicalProjectPath(stripImageRoot(project.path, summary))
		mergeOnDisk(&entries.entry(key, diskEntry{path: key}).onDisk, project.sharedFiles, project.git,
			project.agents)
	}
	return entries.values
}

// mergeOnDisk adds one directory's shared files, repository and agents to
// merged: a repository's remotes and head only where merged has none yet.
func mergeOnDisk(merged *onDisk, shared int64, git projectGit, agents []agentFiles) {
	merged.sharedFiles += shared
	if len(merged.git.remotes) == 0 {
		merged.git.remotes = git.remotes
	}
	if merged.git.head == "" {
		merged.git.head = git.head
	}
	for _, agent := range agents {
		entry := merged.agents.entry(agent.agent, fileCounts{agent: agent.agent})
		entry.files += agent.files
		entry.bytes = addBytes(entry.bytes, agent.bytes)
	}
}

// stripImageRoot returns a collected path as the imaged host saw it: in
// image mode the collector records <root>/srv/proj where the transcripts
// say /srv/proj. A path outside the root is returned unchanged.
func stripImageRoot(path string, summary jsonvalue.Value) string {
	root := ""
	if value := summary.Get("root"); !value.IsEmpty() {
		root = value.Text()
	}
	mode := summary.Get("mode")
	if !mode.IsString() || mode.StringValue() != "image" || root == "" || root == "/" || root == `\` {
		return path
	}
	root = strings.TrimRight(root, `/\`)
	if rest, ok := strings.CutPrefix(path, root); ok && (strings.HasPrefix(rest, "/") ||
		strings.HasPrefix(rest, `\`)) {
		return rest
	}
	return path
}

// collectedProjects returns the project directories the input holds, as
// paths on the source host: collection.json's projects and the homes that
// are project directories, each in its one spelling, sorted and each once.
func collectedProjects(collection *input.Collection) []string {
	unique := map[string]bool{}
	for _, path := range collectionProjectTexts(collection) {
		unique[timeline.CanonicalProjectPath(stripImageRoot(path, collection.Summary))] = true
	}
	return sortedKeys(unique)
}

// collectionProjectTexts returns collection.json's projects and the homes
// that are project directories as the input spells them.
func collectionProjectTexts(collection *input.Collection) []string {
	var paths []string
	for _, listed := range collection.Summary.Get("projects").Items() {
		if !listed.IsEmpty() {
			paths = append(paths, listed.Text())
		}
	}
	for _, home := range collection.Homes {
		if home.Project && home.Original != "" {
			paths = append(paths, home.Original)
		}
	}
	return paths
}

// scratchFinder tells a project path that is a scratch directory, one that
// belongs to an agent rather than to the user, from the catalog: the
// project entries' segments compiled exact and without regard to case, and
// the home claims. tests counts the segment expressions tried, so a test
// can see the work a run does.
type scratchFinder struct {
	found   *catalog.Matcher
	entries [2]entryIndex
	tests   int
}

// entryIndex holds the project entries of one case mode by their first
// segment: a literal one (no `*`, `?` or `[`) under its segmentKey, so a
// path component finds the entries it can start in one lookup; the others
// in globbed, with globFirst matching any of their first segments, nil
// when there are none.
type entryIndex struct {
	literal   map[string][][]*regexp.Regexp
	globbed   [][]*regexp.Regexp
	globFirst *regexp.Regexp
}

// newScratchFinder compiles the project entries of the catalog that name
// an agent's directory, index 0 of entries exact and 1 without regard to
// case.
func newScratchFinder(found *catalog.Matcher) *scratchFinder {
	finder := &scratchFinder{found: found}
	var globs [2][]string
	for mode, flags := range []string{"", "(?i)"} {
		finder.entries[mode].literal = map[string][][]*regexp.Regexp{}
		for _, pattern := range found.Project {
			if pattern.Agent == input.ProjectAgent {
				continue
			}
			var segments []*regexp.Regexp
			for _, segment := range pattern.Segments {
				segments = append(segments, regexp.MustCompile(flags+`\A(?:`+catalog.SegmentExpression(segment)+`)\z`))
			}
			index, first := &finder.entries[mode], pattern.Segments[0]
			if strings.ContainsAny(first, "*?[") {
				index.globbed = append(index.globbed, segments)
				globs[mode] = append(globs[mode], catalog.SegmentExpression(first))
				continue
			}
			key := segmentKey(first, mode == 1)
			index.literal[key] = append(index.literal[key], segments)
		}
		if len(globs[mode]) > 0 {
			finder.entries[mode].globFirst = regexp.MustCompile(flags + `\A(?:` + strings.Join(globs[mode], "|") + `)\z`)
		}
	}
	return finder
}

// segmentKey returns text as a literal first segment is looked up: each
// character as the expression reads it (a byte that is not UTF-8 as
// U+FFFD) and, without regard to case, the least character that simple
// case folding gives the same, so two texts have one key exactly when the
// literal's expression matching one would match the other.
func segmentKey(text string, folded bool) string {
	var key strings.Builder
	for _, char := range text {
		least := char
		for other := unicode.SimpleFold(char); folded && other != char; other = unicode.SimpleFold(other) {
			least = min(least, other)
		}
		key.WriteRune(least)
	}
	return key.String()
}

// scratchKeys returns the project key of every project path that is a
// scratch directory: one a parser's scratch directory: row names, or one a
// home entry claims in a home of the input, or one under a directory a
// project entry names inside a project. Each distinct path is answered
// once, however many rows name it.
func (f *scratchFinder) scratchKeys(collection *input.Collection, rows []timeline.Row) map[string]bool {
	keys := map[string]bool{}
	for _, row := range rows {
		if row.TurnType == timeline.TurnSystem && row.ProjectPath != "" &&
			strings.HasPrefix(row.Text, timeline.ScratchPrefix) {
			keys[timeline.ProjectKey(row.ProjectPath)] = true
		}
	}
	var homes []string
	for _, home := range collection.Homes {
		if home.Original != "" && !home.Project {
			homes = append(homes, stripImageRoot(home.Original, collection.Summary))
		}
	}
	paths := collectedProjects(collection)
	for _, row := range rows {
		paths = append(paths, row.ProjectPath)
	}
	answered := map[string]bool{"": true}
	for _, path := range paths {
		if answered[path] {
			continue
		}
		answered[path] = true
		key := timeline.ProjectKey(path)
		if !keys[key] && (f.underProjectEntry(key) || f.underHomeClaim(key, homes)) {
			keys[key] = true
		}
	}
	return keys
}

// underProjectEntry reports whether a project key lies under a directory a
// project entry names inside a project: with at least one segment before
// the entry and one after it. The key is split by the evidence's rules
// (evidencepath), "." elements dropped; a Windows path is matched without
// regard to case. Each component is looked up once, and only the entries
// whose first segment it matches are tried there.
func (f *scratchFinder) underProjectEntry(key string) bool {
	windows := timeline.IsWindowsPath(key)
	index := &f.entries[0]
	if windows {
		index = &f.entries[1]
	}
	parts := slices.DeleteFunc(evidencepath.Elements(key, windows), func(part string) bool { return part == "." })
	for start := 1; start < len(parts)-1; start++ {
		candidates := index.literal[segmentKey(parts[start], windows)]
		if index.globFirst != nil {
			f.tests++
			if index.globFirst.MatchString(parts[start]) {
				candidates = slices.Concat(candidates, index.globbed)
			}
		}
		for _, segments := range candidates {
			if start < len(parts)-len(segments) && f.matchesAll(segments, parts[start:start+len(segments)]) {
				return true
			}
		}
	}
	return false
}

// matchesAll reports whether each part matches the segment at its place.
func (f *scratchFinder) matchesAll(segments []*regexp.Regexp, parts []string) bool {
	for i, segment := range segments {
		f.tests++
		if !segment.MatchString(parts[i]) {
			return false
		}
	}
	return true
}

// underHomeClaim reports whether a project key lies in or under a
// directory a home entry claims in one of the homes, paths on the source
// host; a Windows home is compared in its key form and without regard to
// case.
func (f *scratchFinder) underHomeClaim(key string, homes []string) bool {
	for _, home := range homes {
		windows := timeline.IsWindowsPath(home)
		separator := "/"
		if windows {
			separator = `\`
		}
		homeKey := timeline.ProjectKey(home)
		prefix := homeKey
		if !strings.HasSuffix(prefix, separator) {
			prefix += separator
		}
		if key != homeKey && !strings.HasPrefix(key, prefix) {
			continue
		}
		if f.found.ClaimsHomePath(key[len(homeKey):], windows) != "" {
			return true
		}
	}
	return false
}

// recordedRemotes returns the URLs of the git remote: rows by project key:
// a row with a session counted under its session's project, any other row
// under its own project path.
func recordedRemotes(rows []timeline.Row, sessions []timeline.SessionSummary) map[string]map[string]bool {
	type sessionKey struct{ host, user, agent, id string }
	projects := map[sessionKey]string{}
	for _, session := range sessions {
		projects[sessionKey{session.Host, session.User, session.Agent, session.SessionID}] =
			session.ProjectPath
	}
	remotes := map[string]map[string]bool{}
	for _, row := range rows {
		url, ok := strings.CutPrefix(row.Text, timeline.GitRemotePrefix)
		if row.TurnType != timeline.TurnSystem || !ok || url == "" {
			continue
		}
		path, ok := projects[sessionKey{row.Host, row.User, row.Agent, row.SessionID}]
		if !ok {
			path = row.ProjectPath
		}
		key := timeline.ProjectKey(path)
		if remotes[key] == nil {
			remotes[key] = map[string]bool{}
		}
		remotes[key][url] = true
	}
	return remotes
}

// emptyGit is a project's repository when the input holds none of it.
func emptyGit() projectGit {
	return projectGit{remotes: []gitmeta.Remote{}}
}
