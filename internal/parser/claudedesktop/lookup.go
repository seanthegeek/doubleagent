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
	for _, directory := range []string{org, filepath.Join(org, "agent")} {
		if !parser.IsDirectory(directory) {
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
func recordCandidates(directory, name string, long bool) []string {
	if long {
		return []string{filepath.Join(directory, name+".json")}
	}
	entries, err := parser.ReadDir(directory)
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
		candidates = append(candidates, filepath.Join(directory, entryName))
	}
	return candidates
}

// hasTranscriptFile reports whether a plain directory directly below the
// plain directory projects holds a regular file named name, or any regular
// .jsonl file when name is "", through no link.
func hasTranscriptFile(projects, name string) bool {
	if !parser.IsDirectory(projects) {
		return false
	}
	slugs, err := parser.ReadDir(projects)
	if err != nil {
		return false
	}
	for _, slug := range slugs {
		slugPath := filepath.Join(projects, slug.Name())
		if !parser.IsDirectory(slugPath) {
			continue
		}
		if name != "" {
			if parser.RegularFile(filepath.Join(slugPath, name)) {
				return true
			}
			continue
		}
		files, err := parser.ReadDir(slugPath)
		if err != nil {
			continue
		}
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".jsonl") && parser.RegularFile(filepath.Join(slugPath, file.Name())) {
				return true
			}
		}
	}
	return false
}

// hasTranscript reports whether a session dir holds a transcript under
// .claude/projects/.
func hasTranscript(sessionDir string) bool {
	claude := filepath.Join(sessionDir, ".claude")
	return parser.IsDirectory(sessionDir) && parser.IsDirectory(claude) && hasTranscriptFile(filepath.Join(claude, "projects"), "")
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
func sessionCollected(artifact *input.Artifact, record jsonvalue.Value) bool {
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
	for _, directory := range slices.Compact([]string{recordDir, org}) {
		for _, name := range names {
			candidate := filepath.Join(directory, name)
			if parser.IsDirectory(candidate) && (parser.RegularFile(filepath.Join(candidate, "audit.jsonl")) || hasTranscript(candidate)) {
				return true
			}
		}
	}
	cliID, named := parser.Component(record.Get("cliSessionId").Text())
	if !named {
		return false
	}
	staging := filepath.Join(org, "imported-staging")
	if parser.IsDirectory(staging) && parser.RegularFile(filepath.Join(staging, cliID+".jsonl")) {
		return true
	}
	claude, claudeFound := artifact.Home.PathOf(".claude")
	projects, projectsFound := artifact.Home.PathOf(".claude/projects")
	return claudeFound && projectsFound && parser.IsDirectory(claude) &&
		hasTranscriptFile(projects, cliID+".jsonl")
}
