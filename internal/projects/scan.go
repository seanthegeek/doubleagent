package projects

import (
	"bytes"
	"context"
	"strings"
)

// homeScan gathers one home's discovery output. stream is the byte stream
// the sh collector's six groups write into one pipe: each grep and sed value
// ends in a newline, but a file the group copies whole (`cat`) is written as
// it is, so a file not ending in a newline runs into the next value's line.
type homeScan struct {
	ctx        context.Context
	root       string
	home       string
	dropSecret func(relative string) bool
	stream     bytes.Buffer
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
	s.emit(grepFiles(s.readable(files), jsonValueMatcher(key)))
}

// jsonPathKeys is json_path_keys over one file: next follows the colon.
func (s *homeScan) jsonPathKeys(file, next string) {
	if isRegularFile(file) && len(s.readable([]string{file})) == 1 {
		s.emit(grepFiles([]string{file}, jsonPathKeyMatcher(next)))
	}
}

// absStrings returns abs_strings over files.
func (s *homeScan) absStrings(files []string) []string {
	return grepFiles(s.readable(files), absStringMatcher())
}

// sessionCwds is session_cwds: the cwd values of the files named name below
// dir, unfiltered and not read through readable, as find feeds grep directly.
func (s *homeScan) sessionCwds(dir, name string) []string {
	if !isDirectory(dir) {
		return nil
	}
	return grepFiles(findNamed(s.ctx, dir, name), sessionCwdMatcher())
}

// emitOutside emits the values for which the root-prefixed value does not
// lie at or under any of the excluded paths (a `case` with "$1/x"*
// patterns, so a prefix match, with exact naming the bare path).
func (s *homeScan) emitOutside(values []string, exact []string, under []string) {
	for _, value := range values {
		path := s.root + value
		excluded := false
		for _, name := range exact {
			excluded = excluded || path == name
		}
		for _, prefix := range under {
			excluded = excluded || strings.HasPrefix(path, prefix)
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
		if content, ok := readWhole(file); ok {
			s.stream.Write(content)
		}
	}
}

// lines returns the lines of a regular file, or nil when scanLines rejects it.
func lines(file string) []string {
	var found []string
	if !scanLines(file, func(line []byte) { found = append(found, string(line)) }) {
		return nil
	}
	return found
}

// values returns the stream's lines that start with `/`, then with every `\/`
// made `/`: a JSON-escaped path is filtered before it is unescaped, so one
// that starts `\/` is dropped. A last line with no newline counts.
func (s *homeScan) values() []string {
	text := strings.TrimSuffix(s.stream.String(), "\n")
	if text == "" {
		return nil
	}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "/") {
			kept = append(kept, strings.ReplaceAll(line, `\/`, "/"))
		}
	}
	return kept
}
