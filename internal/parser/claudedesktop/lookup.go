package claudedesktop

import (
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// A session dir's name: local_<uuid>, or the UUID's first eight hex
// digits.
var (
	longName  = regexp.MustCompile(`^local_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z`)
	shortName = regexp.MustCompile(`^[0-9a-f]{8}\z`)
)

// orgDir returns the org dir that holds a session record's directory dir,
// the artifact's own: dir itself, or the dir above it when it is an agent/
// dir. It is false when that dir lies outside the artifact's home.
func orgDir(artifact *input.Artifact, dir string) (string, bool) {
	if filepath.Base(dir) == "agent" {
		return parser.Above(artifact, 1)
	}
	return dir, true
}

// readJSON returns the file decoded as JSON, or false when it cannot be
// read (its problem added) or is not JSON.
func readJSON(path string, problems *parser.Problems) (jsonvalue.Value, bool) {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.Decode(data)
	return value, err == nil
}

// recordKey names a session dir looked up: its home and its path
// relative to the home.
type recordKey struct {
	home, session string
}

// foundRecord is a session record looked up, and whether one was found.
type foundRecord struct {
	record jsonvalue.Value
	found  bool
}

// sessionRecord returns findRecord's answer for the session dir, looked up
// once per run, since every transcript and audit log of the session dir
// needs it and the lookup reads the record (and, for a short name, lists
// the org dir).
func (p *Parser) sessionRecord(home *input.Home, session string) (jsonvalue.Value, bool) {
	key := recordKey{session: session}
	if home != nil {
		key.home = home.DiskPath
	}
	if cached, ok := p.records[key]; ok {
		return cached.record, cached.found
	}
	p.Work.Add("record lookups", 1)
	record, found := findRecord(home, session)
	if p.records == nil {
		p.records = map[recordKey]foundRecord{}
	}
	p.records[key] = foundRecord{record: record, found: found}
	return record, found
}

// findRecord returns the session record of the session dir at session, a
// path relative to the home, or false. The dir and its parents are found
// where the collection holds them (Home.PathOf follows the collector's
// staged paths). The record is in <org>/ or <org>/agent/, found by the
// dir's full or short name, never through a link. A record that cannot be
// read is skipped here: it is an artifact of its own, reported when it is
// parsed.
func findRecord(home *input.Home, session string) (jsonvalue.Value, bool) {
	if home == nil {
		return jsonvalue.NewNull(), false
	}
	parentRel := path.Dir(session)
	parent, parentFound := home.PathOf(parentRel)
	org, orgFound := parent, parentFound
	if filepath.Base(parent) == "agent" {
		org, orgFound = home.PathOf(path.Dir(parentRel))
	}
	sessionPath, sessionFound := home.PathOf(session)
	name := filepath.Base(sessionPath)
	long := longName.MatchString(name)
	if !parentFound || !orgFound || !sessionFound || !long && !shortName.MatchString(name) ||
		parser.IsLink(org) || parser.IsLink(parent) {
		return jsonvalue.NewNull(), false
	}
	paths := []string{org}
	if agent, ok := parser.Under(org, "agent"); ok {
		paths = append(paths, agent)
	}
	for _, path := range paths {
		directory, ok := parser.DirectoryAt(path)
		if !ok {
			continue
		}
		for _, candidate := range recordCandidates(directory, name, long) {
			if !longName.MatchString(strings.TrimSuffix(filepath.Base(candidate), ".json")) ||
				!parser.RegularFile(candidate) {
				continue
			}
			var ignored parser.Problems
			if record, ok := readJSON(candidate, &ignored); ok && record.IsObject() {
				return record, true
			}
		}
	}
	return jsonvalue.NewNull(), false
}

// recordCandidates returns the paths that may hold the record of the
// session dir called name: <name>.json for a full name, else every
// local_<name>-*.json, sorted by name.
func recordCandidates(directory parser.Directory, name string, long bool) []string {
	if long {
		candidate, ok := directory.Child(name + ".json")
		if !ok {
			return nil
		}
		return []string{candidate}
	}
	entries, err := parser.ReadDir(directory.Path())
	if err != nil {
		return nil
	}
	prefix := "local_" + name + "-"
	var names []string
	for _, entry := range entries {
		entryName := entry.Name()
		if strings.HasPrefix(entryName, prefix) && strings.HasSuffix(entryName[len(prefix):], ".json") {
			names = append(names, entryName)
		}
	}
	slices.Sort(names)
	candidates := make([]string, 0, len(names))
	for _, entryName := range names {
		if candidate, ok := directory.Child(entryName); ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// hasTranscriptFile reports whether a plain directory directly below the
// plain directory projects holds a regular file named name, or any regular
// .jsonl file when name is "", through no link.
func hasTranscriptFile(projects, name string) bool {
	directory, ok := parser.DirectoryAt(projects)
	if !ok {
		return false
	}
	slugs, err := parser.ReadDir(projects)
	if err != nil {
		return false
	}
	for _, slug := range slugs {
		slugPath, ok := directory.Child(slug.Name())
		if !ok {
			continue
		}
		if slugDirectory, ok := parser.DirectoryAt(slugPath); ok && slugHolds(slugDirectory, name) {
			return true
		}
	}
	return false
}

// slugHolds reports whether the plain directory slug holds a regular file
// named name, or any regular .jsonl file when name is "".
func slugHolds(slug parser.Directory, name string) bool {
	if name != "" {
		path, ok := slug.Child(name)
		return ok && parser.RegularFile(path)
	}
	files, err := parser.ReadDir(slug.Path())
	if err != nil {
		return false
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".jsonl") {
			continue
		}
		if path, ok := slug.Child(file.Name()); ok && parser.RegularFile(path) {
			return true
		}
	}
	return false
}

// hasTranscript reports whether a session dir holds a transcript under
// .claude/projects/.
func hasTranscript(sessionDir parser.Directory) bool {
	claude, ok := sessionDir.Child(".claude")
	if !ok {
		return false
	}
	claudeDirectory, ok := parser.DirectoryAt(claude)
	if !ok {
		return false
	}
	projects, ok := claudeDirectory.Child("projects")
	return ok && hasTranscriptFile(projects, "")
}

// folder returns the first folder the person chose for a session or task,
// or "".
func folder(record jsonvalue.Value) string {
	return parser.Text(record.Get("userSelectedFolders").Index(0))
}

// sessionCollected reports whether a transcript or audit log of a record's
// session is in the input: in the session dir (full or short name, beside
// the record or in its org dir), in the org's imported-staging/, or for the
// Code tab in ~/.claude/projects/*/. An id from the record that is not a
// plain file name names nothing collected.
func (p *Parser) sessionCollected(artifact *input.Artifact, record jsonvalue.Value) bool {
	recordDir, found := parser.Beside(artifact)
	if !found {
		return false
	}
	org, found := orgDir(artifact, recordDir)
	if !found {
		return false
	}
	sessionID := record.Get("sessionId").Text()
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(artifact.Rel), ".json")
	}
	var names []string
	if _, named := parser.Component(sessionID); named {
		names = append(names, sessionID)
	}
	if longName.MatchString(sessionID) {
		names = append(names, sessionID[len("local_"):len("local_")+8])
	}
	for _, path := range slices.Compact([]string{recordDir, org}) {
		directory, ok := parser.DirectoryAt(path)
		if !ok {
			continue
		}
		for _, name := range names {
			if sessionDirHolds(directory, name) {
				return true
			}
		}
	}
	cliID, named := parser.Component(record.Get("cliSessionId").Text())
	if !named {
		return false
	}
	if staged, ok := parser.Under(org, "imported-staging", cliID+".jsonl"); ok && parser.RegularFile(staged) {
		return true
	}
	claude, claudeFound := artifact.Home.PathOf(".claude")
	projects, projectsFound := artifact.Home.PathOf(".claude/projects")
	return claudeFound && projectsFound && parser.IsDirectory(claude) &&
		p.transcriptIndexOf(projects).holds(cliID+".jsonl")
}

// transcriptIndex is a .claude/projects directory read once: the slug
// directories whose listing holds each name as a regular file, and the
// slug directories that could not be listed.
type transcriptIndex struct {
	holders  map[string][]parser.Directory
	unlisted []parser.Directory
}

// transcriptIndexOf returns the index of the projects directory, made on
// first use for the run, since every Code tab record looks for its
// transcript in each slug directory.
func (p *Parser) transcriptIndexOf(projects string) *transcriptIndex {
	if index, ok := p.transcripts[projects]; ok {
		return index
	}
	index := &transcriptIndex{holders: map[string][]parser.Directory{}}
	if p.transcripts == nil {
		p.transcripts = map[string]*transcriptIndex{}
	}
	p.transcripts[projects] = index
	directory, ok := parser.DirectoryAt(projects)
	slugs, err := parser.ReadDir(projects)
	if !ok || err != nil {
		return index
	}
	for _, slug := range slugs {
		slugPath, ok := directory.Child(slug.Name())
		if !ok {
			continue
		}
		if slugDirectory, ok := parser.DirectoryAt(slugPath); ok {
			p.Work.Add("slug directory listings", 1)
			index.add(slugDirectory)
		}
	}
	return index
}

// add indexes the regular files of one slug directory by name.
func (x *transcriptIndex) add(slug parser.Directory) {
	files, err := parser.ReadDir(slug.Path())
	if err != nil {
		x.unlisted = append(x.unlisted, slug)
		return
	}
	for _, file := range files {
		if file.Type().IsRegular() {
			x.holders[file.Name()] = append(x.holders[file.Name()], slug)
		}
	}
}

// holds reports whether a slug directory holds a regular file named name,
// as hasTranscriptFile does: by its exact name in each slug's listing,
// since the record names the transcript by the session id the CLI wrote
// it under, the file then looked at once more as a look per slug would.
func (x *transcriptIndex) holds(name string) bool {
	for _, slug := range slices.Concat(x.holders[name], x.unlisted) {
		if slugHolds(slug, name) {
			return true
		}
	}
	return false
}

// sessionDirHolds reports whether the session dir called name in directory
// is a plain directory holding an audit log or a transcript.
func sessionDirHolds(directory parser.Directory, name string) bool {
	path, ok := directory.Child(name)
	if !ok {
		return false
	}
	session, ok := parser.DirectoryAt(path)
	if !ok {
		return false
	}
	audit, ok := session.Child("audit.jsonl")
	return ok && parser.RegularFile(audit) || hasTranscript(session)
}
