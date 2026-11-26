package analyze

import (
	"hash/maphash"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
// image mode collect records <root>/srv/proj where the transcripts
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
// project entries matched by glob.MatchSegment, and the home claims. tests
// counts the segments tried, homeLookups the home keys looked up and
// homeBytes the bytes of project keys hashed or compared to find them, so a
// test can see the work a run does.
type scratchFinder struct {
	found       *catalog.Matcher
	literal     map[string][][]string
	globbed     [][]string
	tests       int
	homeLookups int
	homeBytes   int
}

// newScratchFinder indexes the segments of the project entries that name
// an agent's directory by their first segment: a literal one (no `*`, `?`
// or `[`) in literal under its ASCII lower case, the one fold
// glob.MatchSegment makes, so a path component finds the entries it can
// start in one lookup; the others in globbed.
func newScratchFinder(found *catalog.Matcher) *scratchFinder {
	finder := &scratchFinder{found: found, literal: map[string][][]string{}}
	for _, pattern := range found.Project {
		if pattern.Agent == input.ProjectAgent {
			continue
		}
		if first := pattern.Segments[0]; strings.ContainsAny(first, "*?[") {
			finder.globbed = append(finder.globbed, pattern.Segments)
		} else {
			key := textlines.LowerASCII(first)
			finder.literal[key] = append(finder.literal[key], pattern.Segments)
		}
	}
	return finder
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
	homes := newHomeKeys()
	for _, home := range collection.Homes {
		if home.Original != "" && !home.Project {
			homes.add(stripImageRoot(home.Original, collection.Summary))
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
// (evidencepath), "." elements dropped. At each component only the literal
// entries it can start, found in one lookup, and the globbed ones are tried.
func (f *scratchFinder) underProjectEntry(key string) bool {
	windows := timeline.IsWindowsPath(key)
	parts := slices.DeleteFunc(evidencepath.Elements(key, windows), func(part string) bool { return part == "." })
	for start := 1; start < len(parts)-1; start++ {
		for _, candidates := range [][][]string{f.literal[textlines.LowerASCII(parts[start])], f.globbed} {
			for _, segments := range candidates {
				if start < len(parts)-len(segments) && f.matchesAll(segments, parts[start:start+len(segments)]) {
					return true
				}
			}
		}
	}
	return false
}

// matchesAll reports whether each part matches the segment at its place.
func (f *scratchFinder) matchesAll(segments, parts []string) bool {
	for i, segment := range segments {
		f.tests++
		if !glob.MatchSegment(segment, parts[i]) {
			return false
		}
	}
	return true
}

// homeKeys holds the homes of a run, paths on the source host, by their
// project key: for each key whether a home of that key is a Windows path,
// each answer once, and the hash of each key, so a prefix of a project key
// can be looked up from a hash taken as the key is read.
type homeKeys struct {
	keys   map[string][]bool
	seed   maphash.Seed
	hashes map[uint64]bool
}

// newHomeKeys returns an empty set of homes.
func newHomeKeys() *homeKeys {
	return &homeKeys{keys: map[string][]bool{}, seed: maphash.MakeSeed(), hashes: map[uint64]bool{}}
}

// add records a home.
func (h *homeKeys) add(home string) {
	key, windows := timeline.ProjectKey(home), timeline.IsWindowsPath(home)
	if !slices.Contains(h.keys[key], windows) {
		h.keys[key] = append(h.keys[key], windows)
	}
	h.hashes[maphash.String(h.seed, key)] = true
}

// underHomeClaim reports whether a project key lies in or under a
// directory a home entry claims in one of the homes; a Windows home is
// compared in its key form. A home holding the
// key ends just before or just after one of its separators, or is the key
// itself. The key is hashed once, byte by byte, and only a prefix whose
// hash is a home's is looked up and compared, so the work grows with the
// key's length, not with its square or with the number of homes.
func (f *scratchFinder) underHomeClaim(key string, homes *homeKeys) bool {
	var hash maphash.Hash
	hash.SetSeed(homes.seed)
	for i := range len(key) {
		if key[i] == '/' || key[i] == '\\' {
			if f.claimedUnder(key, key[:i], homes, hash.Sum64()) {
				return true
			}
			f.homeBytes++
			_ = hash.WriteByte(key[i])
			if f.claimedUnder(key, key[:i+1], homes, hash.Sum64()) {
				return true
			}
			continue
		}
		f.homeBytes++
		_ = hash.WriteByte(key[i])
	}
	return f.claimedUnder(key, key, homes, hash.Sum64())
}

// claimedUnder reports whether homeKey, a prefix of key whose hash is
// sum, is the key of a home of homes that holds key, the key itself or a
// path under it by the home's separator, and a home entry claims the rest
// of key in that home.
func (f *scratchFinder) claimedUnder(key, homeKey string, homes *homeKeys, sum uint64) bool {
	if !homes.hashes[sum] {
		return false
	}
	f.homeLookups++
	f.homeBytes += len(homeKey)
	for _, windows := range homes.keys[homeKey] {
		separator := "/"
		if windows {
			separator = `\`
		}
		prefix := homeKey
		if !strings.HasSuffix(prefix, separator) {
			prefix += separator
		}
		if (key == homeKey || strings.HasPrefix(key, prefix)) &&
			f.found.ClaimsHomePath(key[len(homeKey):], separator) != "" {
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
