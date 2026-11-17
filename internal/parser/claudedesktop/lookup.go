package claudedesktop

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// A session dir's name: local_<uuid>, or the UUID's first eight hex
// digits.
var (
	longName  = regexp.MustCompile(`^local_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z`)
	shortName = regexp.MustCompile(`^[0-9a-f]{8}\z`)
)

// isLink reports whether path's own entry is a symbolic link.
func isLink(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// isPlainDir reports whether path's own entry is a directory, not a link.
func isPlainDir(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.IsDir()
}

// orgDir returns the org dir that holds the record's directory dir, which
// lies levels above the artifact's own directory: dir itself, or the dir
// above it when it is an agent/ dir. It is false when that dir lies outside
// the artifact's home.
func orgDir(artifact *input.Artifact, dir string, levels int) (string, bool) {
	if filepath.Base(dir) == "agent" {
		return parser.Above(artifact, levels+1)
	}
	return dir, true
}

// readJSON returns the file decoded as JSON, or false when it cannot be
// read (its problem added) or is not JSON.
func readJSON(path string, problems *parser.Problems) (pyjson.Value, bool) {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return pyjson.NewNull(), false
	}
	value, err := pyjson.Decode(data)
	return value, err == nil
}

// findRecord returns the session record of the session dir that lies
// levels above the artifact's own directory, or false. The record is in
// <org>/ or <org>/agent/, found by the dir's full or short name, never
// through a link. A record that cannot be read is skipped here: it is an
// artifact of its own, reported when it is parsed.
func findRecord(artifact *input.Artifact, levels int) (pyjson.Value, bool) {
	session, found := parser.Above(artifact, levels)
	parent, foundParent := parser.Above(artifact, levels+1)
	if !found || !foundParent {
		return pyjson.NewNull(), false
	}
	org, found := orgDir(artifact, parent, levels+1)
	name := filepath.Base(session)
	long := longName.MatchString(name)
	if !found || !long && !shortName.MatchString(name) || isLink(org) || isLink(parent) {
		return pyjson.NewNull(), false
	}
	for _, directory := range []string{org, filepath.Join(org, "agent")} {
		if !isPlainDir(directory) {
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
	return pyjson.NewNull(), false
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
	if !isPlainDir(projects) {
		return false
	}
	slugs, err := parser.ReadDir(projects)
	if err != nil {
		return false
	}
	for _, slug := range slugs {
		slugPath := filepath.Join(projects, slug.Name())
		if !isPlainDir(slugPath) {
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
	return isPlainDir(sessionDir) && isPlainDir(claude) && hasTranscriptFile(filepath.Join(claude, "projects"), "")
}

// folder returns the first folder the person chose for a session or task,
// or "".
func folder(record pyjson.Value) string {
	return parser.Str(record.Get("userSelectedFolders").Index(0))
}

// sessionCollected reports whether a transcript or audit log of a record's
// session is in the input: in the session dir (full or short name, beside
// the record or in its org dir), in the org's imported-staging/, or for the
// Code tab in ~/.claude/projects/*/.
func sessionCollected(artifact *input.Artifact, record pyjson.Value) bool {
	recordDir := parser.Beside(artifact)
	org, found := orgDir(artifact, recordDir, 0)
	if !found {
		return false
	}
	sessionID := text(record.Get("sessionId"))
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(artifact.Rel), ".json")
	}
	names := []string{sessionID}
	if longName.MatchString(sessionID) {
		names = append(names, sessionID[len("local_"):len("local_")+8])
	}
	for _, directory := range slices.Compact([]string{recordDir, org}) {
		for _, name := range names {
			candidate := filepath.Join(directory, name)
			if isPlainDir(candidate) && (parser.RegularFile(filepath.Join(candidate, "audit.jsonl")) || hasTranscript(candidate)) {
				return true
			}
		}
	}
	cliID := text(record.Get("cliSessionId"))
	if cliID == "" {
		return false
	}
	staging := filepath.Join(org, "imported-staging")
	if isPlainDir(staging) && parser.RegularFile(filepath.Join(staging, cliID+".jsonl")) {
		return true
	}
	return isPlainDir(artifact.Home.PathOf(".claude")) &&
		hasTranscriptFile(artifact.Home.PathOf(".claude/projects"), cliID+".jsonl")
}
