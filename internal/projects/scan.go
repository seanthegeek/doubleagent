package projects

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// homeScan gathers one home's discovery output. stream is the byte stream
// the sh collector's six groups write into one pipe: each grep and sed value
// ends in a newline, but a file the group copies whole (`cat`) is written as
// it is, so a file not ending in a newline runs into the next value's line.
type homeScan struct {
	ctx        context.Context
	root       string
	user       string
	home       string
	rules      pathRules
	dropSecret func(relative string) bool
	outOfScope RootExit
	opener     *fsmeta.Opener
	stream     bytes.Buffer
	refused    []Refusal
}

// leaves reports whether path, a source or a directory of sources, leads out
// of the image root through a link, and records each such path once as a
// Refusal. A live run (no outOfScope) has no image root to leave, so nothing
// is refused there and every source is read as the host holds it.
func (s *homeScan) leaves(path string) bool {
	if s.outOfScope == nil {
		return false
	}
	link, leaves := s.outOfScope(path)
	if leaves && !slices.ContainsFunc(s.refused, func(refusal Refusal) bool {
		return refusal.Source == path
	}) {
		s.refused = append(s.refused, Refusal{User: s.user, Source: path, Link: link})
	}
	return leaves
}

// emit writes each value to the stream as one line.
func (s *homeScan) emit(values []string) {
	for _, value := range values {
		s.stream.WriteString(value)
		s.stream.WriteByte('\n')
	}
}

// readable drops the files dropSecret rejects, by their path relative to
// the home (INVENTORY_DROP_SECRETS); with no dropSecret every file is kept.
func (s *homeScan) readable(files []string) []string {
	if s.dropSecret == nil {
		return files
	}
	var kept []string
	for _, file := range files {
		if !s.dropSecret(strings.TrimPrefix(file, s.home+"/")) {
			kept = append(kept, file)
		}
	}
	return kept
}

// expand expands patterns inside the home.
func (s *homeScan) expand(patterns ...string) []string { return expandIn(s.home, patterns...) }

// jsonValues is json_vals KEY over files.
func (s *homeScan) jsonValues(key string, files []string) {
	s.emit(s.grepFiles(s.readable(files), jsonValueMatcher(key)))
}

// jsonPathKeys is json_path_keys over one file: next follows the colon.
func (s *homeScan) jsonPathKeys(file, next string) {
	if s.isRegularFile(file) && len(s.readable([]string{file})) == 1 {
		s.emit(s.grepFiles([]string{file}, jsonPathKeyMatcher(next, s.rules)))
	}
}

// absStrings returns abs_strings over files.
func (s *homeScan) absStrings(files []string) []string {
	return s.grepFiles(s.readable(files), absStringMatcher(s.rules))
}

// sessionCwds is session_cwds: the cwd values of the files named name below
// dir, unfiltered and not read through readable, as find feeds grep directly.
func (s *homeScan) sessionCwds(dir, name string) []string {
	if !s.isDirectory(dir) {
		return nil
	}
	return s.grepFiles(s.findNamed(dir, name), sessionCwdMatcher())
}

// emitOutside emits the values whose path does not lie at or under any of
// the excluded paths (a `case` with "$1/x"* patterns, so a prefix match,
// with exact naming the bare path). Under the sh rules the path is the raw
// value after root, as sh tests it; under the drive-letter rules it is the
// value's local path, as the PowerShell collector tests it, ignoring case.
func (s *homeScan) emitOutside(values []string, exact []string, under []string) {
	for _, value := range values {
		path := s.root + value
		if s.rules.driveLetters {
			path, _ = localProjectPath(value, s.root, s.rules)
		}
		excluded := false
		for _, name := range exact {
			excluded = excluded || syspath.Equal(path, name)
		}
		for _, prefix := range under {
			excluded = excluded || syspath.HasPrefix(path, prefix)
		}
		if !excluded {
			s.emit([]string{value})
		}
	}
}

// catFiles writes each file whole to the stream, as `cat` does; nothing is
// read through readable, as the sh code calls cat directly.
func (s *homeScan) catFiles(files []string) {
	for _, file := range files {
		if content, ok := s.readWhole(file); ok {
			s.stream.Write(content)
		}
	}
}

// lines returns the lines of a regular file, or nil when scanLines rejects it.
func (s *homeScan) lines(file string) []string {
	var found []string
	if !s.scanLines(file, func(line []byte) { found = append(found, string(line)) }) {
		return nil
	}
	return found
}

// paths returns the local path of each stream line that names one, in
// stream order (localProjectPath with the scan's rules and root). A last
// line with no newline counts.
func (s *homeScan) paths() []string {
	text := strings.TrimSuffix(s.stream.String(), "\n")
	if text == "" {
		return nil
	}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if path, ok := localProjectPath(line, s.root, s.rules); ok {
			kept = append(kept, path)
		}
	}
	return kept
}
