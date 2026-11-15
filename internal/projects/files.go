package projects

import (
	"bufio"
	"bytes"
	"context"
	"io"

	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// scanLines calls each with every line of the regular file at path, without
// its newline, the last line also when no newline ends it, as grep reads a
// file. It reports false when the file cannot be opened or holds a NUL byte:
// grep then takes the file as binary and prints none of its lines, so the
// caller discards what each gathered. A read error keeps the lines before it.
func (s *homeScan) scanLines(path string, each func(line []byte)) bool {
	if s.leaves(path) {
		return false
	}
	handle, err := s.opener.OpenRegular(path)
	if err != nil {
		return false
	}
	defer handle.Close()
	reader := bufio.NewReaderSize(handle, 64*1024)
	for {
		line, readError := reader.ReadBytes('\n')
		if bytes.IndexByte(line, 0) >= 0 {
			return false
		}
		if len(line) > 0 {
			each(bytes.TrimSuffix(line, []byte{'\n'}))
		}
		if readError != nil {
			return true
		}
	}
}

// grepFiles returns what extract finds in each line of each file, in file
// order, leaving out a file scanLines rejects, as `grep -h` over the files
// does with `2>/dev/null`.
func (s *homeScan) grepFiles(files []string, extract func(line []byte) []string) []string {
	var values []string
	for _, file := range files {
		var found []string
		if s.scanLines(file, func(line []byte) { found = append(found, extract(line)...) }) {
			values = append(values, found...)
		}
	}
	return values
}

// readWhole returns the bytes of the regular file at path, or false when it
// cannot be opened or holds a NUL byte (the binary-file rule of scanLines).
func (s *homeScan) readWhole(path string) ([]byte, bool) {
	if s.leaves(path) {
		return nil, false
	}
	handle, err := s.opener.OpenRegular(path)
	if err != nil {
		return nil, false
	}
	defer handle.Close()
	content, _ := io.ReadAll(handle)
	if bytes.IndexByte(content, 0) >= 0 {
		return nil, false
	}
	return content, true
}

// isDirectory is test -d: true for a directory or a link to one.
func isDirectory(path string) bool {
	info, err := osfile.Stat(path)
	return err == nil && info.IsDir()
}

// isDirectory is the scan's test -d: false, without a stat, for a path
// leading out of the image root.
func (s *homeScan) isDirectory(path string) bool {
	return !s.leaves(path) && isDirectory(path)
}

// isRegularFile is test -f: true for a regular file or a link to one, and
// false, without a stat, for a path leading out of the image root.
func (s *homeScan) isRegularFile(path string) bool {
	if s.leaves(path) {
		return false
	}
	info, err := osfile.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// expandIn returns the shell expansion of each pattern inside base, in
// pattern order, each pattern's matches sorted, as an unquoted word list
// gives them; a pattern naming nothing that exists gives nothing.
func expandIn(base string, patterns ...string) []string {
	var paths []string
	for _, pattern := range patterns {
		paths = append(paths, glob.Expand(base, pattern)...)
	}
	return paths
}

// findNamed returns the paths below dir whose name matches pattern, in the
// order `find DIR -name PATTERN` lists them: depth first, each directory's
// entries in the order the directory returns them, never descending a
// symlink. A dir that is itself a symlink gives nothing, as find without -L
// does not enter it, and so does one whose path leads out of the image root.
func (s *homeScan) findNamed(dir, pattern string) []string {
	if s.leaves(dir) {
		return nil
	}
	info, err := osfile.Lstat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	var found []string
	walkNamed(s.ctx, dir, pattern, &found)
	return found
}

// walkNamed appends to found the matching paths below dir, for findNamed. An
// unreadable directory contributes the entries read before the error, and
// a cancelled ctx stops the walk at the next directory.
func walkNamed(ctx context.Context, dir, pattern string, found *[]string) {
	if ctx.Err() != nil {
		return
	}
	handle, err := osfile.Open(dir)
	if err != nil {
		return
	}
	entries, _ := handle.ReadDir(-1)
	handle.Close()
	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		// find -name has no leading-dot rule, so `*` matches a name
		// starting with a dot; a name never holds `/`.
		if glob.MatchPath(pattern, entry.Name()) {
			*found = append(*found, path)
		}
		if entry.IsDir() {
			walkNamed(ctx, path, pattern, found)
		}
	}
}
