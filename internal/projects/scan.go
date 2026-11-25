package projects

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// homeScan gathers one home's discovery output. stream is the byte stream
// the six groups write into, in order, one value a line: a file a group
// copies whole adds its own lines, so one written without a final line end
// does not run into the next value.
type homeScan struct {
	ctx        context.Context
	root       string
	user       string
	home       string
	rules      pathRules
	dropSecret func(home, relative string) bool
	outOfScope RootExit
	opener     *fsmeta.Opener
	runOutput  func(path string) bool
	stream     bytes.Buffer
	refused    []Refusal
}

// ownOutput reports whether path is one of the run's own outputs, which
// discovery neither lists nor reads.
func (s *homeScan) ownOutput(path string) bool {
	return s.runOutput != nil && s.runOutput(path)
}

// leaves reports whether path, a source or a directory of sources, leads out
// of the image root through a link, and records the link once as a Refusal.
// A live run (no outOfScope) has no image root to leave, so nothing is
// refused there and every source is read as the host holds it.
func (s *homeScan) leaves(path string) bool {
	if s.outOfScope == nil {
		return false
	}
	refusal, leaves := s.outOfScope(path)
	if leaves {
		s.refuse(refusal)
	}
	return leaves
}

// refuse records refusal unless its link is recorded already.
func (s *homeScan) refuse(refusal follow.Refusal) {
	known := func(earlier Refusal) bool { return earlier.Link == refusal.Link }
	if !slices.ContainsFunc(s.refused, known) {
		s.refused = append(s.refused, Refusal{User: s.user, Refusal: refusal})
	}
}

// emit writes each value to the stream as one line.
func (s *homeScan) emit(values []string) {
	for _, value := range values {
		s.stream.WriteString(value)
		s.stream.WriteByte('\n')
	}
}

// readable drops the files dropSecret rejects, by their path relative to
// the home; with no dropSecret every file is kept.
func (s *homeScan) readable(files []string) []string {
	if s.dropSecret == nil {
		return files
	}
	var kept []string
	for _, file := range files {
		if !s.dropSecret(s.home, strings.TrimPrefix(file, s.home+"/")) {
			kept = append(kept, file)
		}
	}
	return kept
}

// expand expands patterns inside the home.
func (s *homeScan) expand(patterns ...string) []string { return s.expandIn(s.home, patterns...) }

// jsonValues emits the jsonValueMatcher values of key over files.
func (s *homeScan) jsonValues(key string, files []string) {
	s.emit(s.grepFiles(s.readable(files), jsonValueMatcher(key)))
}

// jsonPathKeys emits the jsonPathKeyMatcher keys of one file: next follows
// the colon.
func (s *homeScan) jsonPathKeys(file, next string) {
	if s.isRegularFile(file) && len(s.readable([]string{file})) == 1 {
		s.emit(s.grepFiles([]string{file}, jsonPathKeyMatcher(next, s.rules)))
	}
}

// absStrings returns the absStringMatcher values of files.
func (s *homeScan) absStrings(files []string) []string {
	return s.grepFiles(s.readable(files), absStringMatcher(s.rules))
}

// sessionCwds returns the cwd values of the files named name below dir,
// not read through readable.
func (s *homeScan) sessionCwds(dir, name string) []string {
	if !s.isDirectory(dir) {
		return nil
	}
	return s.grepFiles(s.findNamed(dir, name), sessionCwdMatcher())
}

// emitOutside emits the values whose path does not lie at or under any of
// the excluded paths (a prefix match, with exact naming the bare path).
// Under the POSIX rules the path is the raw value after root; under the
// drive-letter rules it is the value's local path, ignoring ASCII case only
// where the file system says the two spellings are one entry, so a
// directory a case-sensitive directory keeps apart is not dropped. Each
// prefix of under ends with a slash.
func (s *homeScan) emitOutside(values []string, exact []string, under []string) {
	same := follow.SameEntry(s.outOfScope)
	for _, value := range values {
		path := s.root + value
		if s.rules.driveLetters {
			path, _ = localProjectPath(value, s.root, s.rules)
		}
		excluded := false
		for _, name := range exact {
			excluded = excluded || syspath.SameAs(path, name, same)
		}
		for _, prefix := range under {
			_, inside := syspath.CutUnder(path, strings.TrimSuffix(prefix, "/"), same)
			excluded = excluded || inside
		}
		if !excluded {
			s.emit([]string{value})
		}
	}
}

// catFiles writes every line of each file to the stream, each ending at LF
// or CRLF and the last one also at the end of the file, less one CR that
// ends it (textlines.Split), so Gemini's `.project_root`, written without a
// final line end, is a value of its own; nothing is read through readable.
func (s *homeScan) catFiles(files []string) {
	for _, file := range files {
		if content, ok := s.readWhole(file); ok {
			s.emit(textlines.Split(string(content)))
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
