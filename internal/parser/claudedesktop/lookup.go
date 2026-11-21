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
func readJSON(artifact *input.Artifact, path string, problems *parser.Problems) (jsonvalue.Value, bool) {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	value, err := parser.DecodeJSONBytes(artifact, data)
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
	record, found := p.findRecord(home, session)
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
func (p *Parser) findRecord(home *input.Home, session string) (jsonvalue.Value, bool) {
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
		for _, candidate := range p.recordCandidates(directory, name, long) {
			if !longName.MatchString(strings.TrimSuffix(filepath.Base(candidate), ".json")) ||
				!parser.RegularFile(candidate) {
				continue
			}
			var ignored parser.Problems
			if record, ok := readJSON(nil, candidate, &ignored); ok && record.IsObject() {
				return record, true
			}
		}
	}
	return jsonvalue.NewNull(), false
}

// recordCandidates returns the paths that may hold the record of the
// session dir called name: <name>.json for a full name, else every
// local_<name>-*.json, sorted by name, from the directory's index.
func (p *Parser) recordCandidates(directory parser.Directory, name string, long bool) []string {
	if long {
		candidate, ok := directory.Child(name + ".json")
		if !ok {
			return nil
		}
		return []string{candidate}
	}
	names := p.shortNamesOf(directory)[name]
	candidates := make([]string, 0, len(names))
	for _, entryName := range names {
		if candidate, ok := directory.Child(entryName); ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// shortNamesOf returns the names of the directory's entries
// local_<eight characters>-*.json by those eight characters, each list
// sorted, made on first use for the run, since every session dir of an
// org with a short name looks its record up there. A directory that cannot
// be listed has none.
func (p *Parser) shortNamesOf(directory parser.Directory) map[string][]string {
	if names, ok := p.shortNames[directory.Path()]; ok {
		return names
	}
	if p.shortNames == nil {
		p.shortNames = map[string]map[string][]string{}
	}
	names := map[string][]string{}
	p.shortNames[directory.Path()] = names
	p.Work.Add("org directory listings", 1)
	entries, err := parser.ReadDir(directory.Path())
	if err != nil {
		return names
	}
	const prefix, short = "local_", 8
	for _, entry := range entries {
		entryName := entry.Name()
		rest, found := strings.CutPrefix(entryName, prefix)
		if found && len(rest) > short && rest[short] == '-' && strings.HasSuffix(rest[short+1:], ".json") {
			names[rest[:short]] = append(names[rest[:short]], entryName)
		}
	}
	for _, list := range names {
		slices.Sort(list)
	}
	return names
}

// hasTranscriptFile reports whether a plain directory directly below the
// plain directory projects holds any regular .jsonl file, through no link,
// counting each directory it lists in work.
func hasTranscriptFile(projects string, work parser.Work) bool {
	directory, ok := parser.DirectoryAt(projects)
	if !ok {
		return false
	}
	work.Add("session directory listings", 1)
	slugs, err := parser.ReadDir(projects)
	if err != nil {
		return false
	}
	for _, slug := range slugs {
		slugPath, ok := directory.Child(slug.Name())
		if !ok {
			continue
		}
		if slugDirectory, ok := parser.DirectoryAt(slugPath); ok && slugHolds(slugDirectory, "", work) {
			return true
		}
	}
	return false
}

// slugHolds reports whether the plain directory slug holds a regular file
// named name, or any regular .jsonl file when name is "", which lists it
// (counted in work).
func slugHolds(slug parser.Directory, name string, work parser.Work) bool {
	if name != "" {
		path, ok := slug.Child(name)
		return ok && parser.RegularFile(path)
	}
	work.Add("session directory listings", 1)
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
// .claude/projects/, counting the directories it lists in work.
func hasTranscript(sessionDir parser.Directory, work parser.Work) bool {
	claude, ok := sessionDir.Child(".claude")
	if !ok {
		return false
	}
	claudeDirectory, ok := parser.DirectoryAt(claude)
	if !ok {
		return false
	}
	projects, ok := claudeDirectory.Child("projects")
	return ok && hasTranscriptFile(projects, work)
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
			if p.sessionDirHolds(artifact.Home, directory, name) {
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
		p.transcriptHeld(projects, cliID+".jsonl")
}

// transcriptIndexOf returns the slug directories of the projects
// directory, each listed once, made on first use for the run, since every
// Code tab record looks for its transcript in each of them. A slug that
// could not be listed holds no transcript: a lookup stays quiet, as the
// transcript's own problems are reported where it is parsed.
func (p *Parser) transcriptIndexOf(projects string) *parser.ChildListings {
	if index, ok := p.transcripts[projects]; ok {
		return index
	}
	if p.transcripts == nil {
		p.transcripts = map[string]*parser.ChildListings{}
	}
	index := &parser.ChildListings{}
	directory, ok := parser.DirectoryAt(projects)
	slugs, err := parser.ReadDir(projects)
	if ok && err == nil {
		names := make([]string, 0, len(slugs))
		for _, slug := range slugs {
			names = append(names, slug.Name())
		}
		index = parser.ListChildren(directory, names, nil, p.Work, "slug directory listings")
	}
	p.transcripts[projects] = index
	return index
}

// transcriptHeld reports whether a slug directory holds a regular file
// named name, looking only in the slugs whose listing holds it by its
// exact name, since the record names the transcript by the session id the
// CLI wrote it under; the file is then looked at once more, never through
// a link.
func (p *Parser) transcriptHeld(projects, name string) bool {
	index := p.transcriptIndexOf(projects)
	for _, place := range index.Holders(name) {
		p.Work.Add("transcript candidates", 1)
		if slugHolds(index.Directory(place), name, p.Work) {
			return true
		}
	}
	return false
}

// sessionDirHolds reports whether the session dir called name in directory
// is a plain directory holding an audit log or a transcript. The answer is
// found once per session dir for the run, by the entry its path opens in
// the home, since every record naming the dir asks, by any spelling, and
// finding it lists the dir's .claude/projects and each slug below it.
func (p *Parser) sessionDirHolds(home *input.Home, directory parser.Directory, name string) bool {
	path, ok := directory.Child(name)
	if !ok {
		return false
	}
	answers := p.sessionDirs.For(home)
	key := parser.KeyOf(path)
	if holds, ok := answers[key]; ok {
		return holds
	}
	holds := false
	if session, ok := parser.DirectoryAt(path); ok {
		audit, ok := session.Child("audit.jsonl")
		holds = ok && parser.RegularFile(audit) || hasTranscript(session, p.Work)
	}
	answers[key] = holds
	return holds
}
