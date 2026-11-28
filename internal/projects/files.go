package projects

import (
	"bufio"
	"bytes"
	"io"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// scanLines calls each with every line of the regular file at path, less its
// newline; a read error keeps the lines before it.
func (s *homeScan) scanLines(path string, each func(line []byte)) {
	if s.leaves(path) {
		return
	}
	handle, err := s.opener.OpenRegular(path)
	if err != nil {
		return
	}
	defer handle.Close()
	reader := bufio.NewReaderSize(handle, 64*1024)
	for {
		line, readError := reader.ReadBytes('\n')
		if len(line) > 0 {
			each(bytes.TrimSuffix(line, []byte{'\n'}))
		}
		if readError != nil {
			return
		}
	}
}

// valuesFromLines returns what extract finds in each line of each file, in
// file order.
func (s *homeScan) valuesFromLines(files []string, extract func(line []byte) []string) []string {
	var values []string
	for _, file := range files {
		s.scanLines(file, func(line []byte) { values = append(values, extract(line)...) })
	}
	return values
}

// readWhole returns the bytes of the regular file at path, or nil when it
// cannot be opened.
func (s *homeScan) readWhole(path string) []byte {
	if s.leaves(path) {
		return nil
	}
	handle, err := s.opener.OpenRegular(path)
	if err != nil {
		return nil
	}
	defer handle.Close()
	content, _ := io.ReadAll(handle)
	return content
}

// refusedBy records the link of err when it says a path leads out of the
// image root, and reports whether it did.
func (s *homeScan) refusedBy(err error) bool {
	refusal, out := follow.OutOfRoot(err)
	if out {
		s.refuse(refusal)
	}
	return out
}

// isDirectory reports whether path is a directory or a link to one, and
// refuses, without a stat, a path leading out of the image root.
func (s *homeScan) isDirectory(path string) bool {
	isDir, err := follow.IsDirectory(s.outOfScope, path)
	return !s.refusedBy(err) && isDir
}

// isRegularFile is isDirectory for a regular file.
func (s *homeScan) isRegularFile(path string) bool {
	isFile, err := follow.IsRegularFile(s.outOfScope, path)
	return !s.refusedBy(err) && isFile
}

// expandIn returns the matches of each pattern inside base, in pattern order,
// each pattern's sorted. In image mode nothing behind a link out of the image
// root is listed or looked up; each such link becomes a Refusal.
func (s *homeScan) expandIn(base string, patterns ...string) []string {
	var paths []string
	for _, pattern := range patterns {
		var check glob.Check
		if s.outOfScope != nil {
			check = glob.Check(s.outOfScope)
		}
		matches, refused := glob.Expand(base, pattern, check, s.opener)
		paths = append(paths, slices.DeleteFunc(matches, s.ownOutput)...)
		for _, refusal := range refused {
			s.refuse(refusal.Refusal)
		}
	}
	return paths
}

// realDirectory reports whether path's own lstat types it a directory: not a
// symlink, a junction or any other reparse point, which a listing on Windows
// may still call a directory (a cloud-file placeholder directory).
func realDirectory(path string) bool {
	meta, err := fsmeta.Lstat(path)
	return err == nil && meta.Kind == fsmeta.Dir
}

// walkNamed appends to found the matching paths below dir, for findNamed,
// passing over the run's own outputs. An unreadable directory gives the
// entries read before the error; a cancelled context stops at the next one.
func (s *homeScan) walkNamed(dir, pattern string, found *[]string) {
	if s.ctx.Err() != nil {
		return
	}
	handle, err := fsmeta.OpenDir(s.opener, dir)
	if err != nil {
		return
	}
	entries, _ := handle.ReadDir(-1)
	handle.Close()
	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		if s.ownOutput(path) {
			continue
		}
		// A `*` also matches a name starting with a dot.
		if glob.MatchPath(pattern, entry.Name()) {
			*found = append(*found, path)
		}
		if entry.IsDir() && realDirectory(path) {
			s.walkNamed(path, pattern, found)
		}
	}
}

// findNamed returns the paths below dir whose name matches pattern, depth
// first, each directory's entries in listing order, never descending a link.
// A dir that is a link, or whose path leads out of the image root, gives
// nothing.
func (s *homeScan) findNamed(dir, pattern string) []string {
	if s.leaves(dir) {
		return nil
	}
	if !realDirectory(dir) {
		return nil
	}
	var found []string
	s.walkNamed(dir, pattern, &found)
	return found
}
