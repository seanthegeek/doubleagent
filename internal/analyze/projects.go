package analyze

import (
	"regexp"
	"slices"
	"strings"

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
	agents      []fileCounts
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
	var entries []diskEntry
	for _, project := range projects {
		key := timeline.CanonicalProjectPath(stripImageRoot(project.path, summary))
		index := slices.IndexFunc(entries, func(entry diskEntry) bool { return entry.path == key })
		if index < 0 {
			entries = append(entries, diskEntry{path: key})
			index = len(entries) - 1
		}
		mergeOnDisk(&entries[index].onDisk, project.sharedFiles, project.git, project.agents)
	}
	return entries
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
		index := slices.IndexFunc(merged.agents, func(each fileCounts) bool { return each.agent == agent.agent })
		if index < 0 {
			merged.agents = append(merged.agents, fileCounts{agent: agent.agent})
			index = len(merged.agents) - 1
		}
		merged.agents[index].files += agent.files
		merged.agents[index].bytes += agent.bytes
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
	summary := collection.Summary
	var paths []string
	for _, listed := range summary.Get("projects").Items() {
		if !listed.IsEmpty() {
			paths = append(paths, listed.Text())
		}
	}
	for _, home := range collection.Homes {
		if home.Project && home.Original != "" {
			paths = append(paths, home.Original)
		}
	}
	unique := map[string]bool{}
	for _, path := range paths {
		unique[timeline.CanonicalProjectPath(stripImageRoot(path, summary))] = true
	}
	return sortedKeys(unique)
}

// scratchFinder tells a project path that is a scratch directory, one that
// belongs to an agent rather than to the user, from the catalog: the
// project entries' segments compiled exact and without regard to case, and
// the home claims.
type scratchFinder struct {
	found   *catalog.Matcher
	entries [][2][]*regexp.Regexp
}

// newScratchFinder compiles the project entries of the catalog that name
// an agent's directory.
func newScratchFinder(found *catalog.Matcher) *scratchFinder {
	finder := &scratchFinder{found: found}
	for _, pattern := range found.Project {
		if pattern.Agent == input.ProjectAgent {
			continue
		}
		var exact, folded []*regexp.Regexp
		for _, segment := range pattern.Segments {
			expression := `\A(?:` + catalog.SegmentExpression(segment) + `)\z`
			exact = append(exact, regexp.MustCompile(expression))
			folded = append(folded, regexp.MustCompile(`(?i)`+expression))
		}
		finder.entries = append(finder.entries, [2][]*regexp.Regexp{exact, folded})
	}
	return finder
}

// scratchKeys returns the project key of every project path that is a
// scratch directory: one a parser's scratch directory: row names, or one a
// home entry claims in a home of the input, or one under a directory a
// project entry names inside a project.
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
	for _, path := range paths {
		key := timeline.ProjectKey(path)
		if path != "" && !keys[key] && (f.underProjectEntry(key) || f.underHomeClaim(key, homes)) {
			keys[key] = true
		}
	}
	return keys
}

// underProjectEntry reports whether a project key lies under a directory a
// project entry names inside a project: with at least one segment before
// the entry and one after it. The key is split by the evidence's rules
// (evidencepath), "." elements dropped; a Windows path is matched without
// regard to case.
func (f *scratchFinder) underProjectEntry(key string) bool {
	windows := timeline.IsWindowsPath(key)
	mode := 0
	if windows {
		mode = 1
	}
	parts := slices.DeleteFunc(evidencepath.Elements(key, windows), func(part string) bool { return part == "." })
	for _, entry := range f.entries {
		segments := entry[mode]
		for start := 1; start < len(parts)-len(segments); start++ {
			if matchesAll(segments, parts[start:start+len(segments)]) {
				return true
			}
		}
	}
	return false
}

// matchesAll reports whether each part matches the segment at its place.
func matchesAll(segments []*regexp.Regexp, parts []string) bool {
	for i, segment := range segments {
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
