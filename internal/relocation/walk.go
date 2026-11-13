package relocation

import (
	"context"
	"io/fs"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// sweepWalk lists the files of one sweep as the sh collector's find(1) does:
// depth first, in directory order, never following a symlink, a pruned path
// left out with everything under it.
type sweepWalk struct {
	ctx     context.Context
	pruned  func(path string) bool
	limit   int64
	files   []string
	skipped int
}

// visit handles path: a regular file whose name holds no newline is listed
// when it is within the limit (0 for none) and counted as skipped otherwise;
// a directory is read and its entries visited. A directory that cannot be
// read gives the entries read before the error, as find goes on past it.
func (walk *sweepWalk) visit(path string) error {
	if walk.pruned(path) {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode().IsRegular() {
		walk.addFile(path, info)
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	if err := walk.ctx.Err(); err != nil {
		return err
	}
	for _, name := range readEntryNames(path) {
		if err := walk.visit(path + "/" + name); err != nil {
			return err
		}
	}
	return nil
}

// addFile lists a regular file or counts it as over the size limit; a file
// whose name holds a newline is neither, since it would forge a line of the
// sh collector's search output.
func (walk *sweepWalk) addFile(path string, info fs.FileInfo) {
	name := path[strings.LastIndexByte(path, '/')+1:]
	if strings.Contains(name, "\n") {
		return
	}
	if walk.limit == 0 || info.Size() <= walk.limit {
		walk.files = append(walk.files, path)
	} else {
		walk.skipped++
	}
}

// readEntryNames returns the names in directory in directory order.
func readEntryNames(directory string) []string {
	handle, err := os.Open(directory)
	if err != nil {
		return nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

// claimLines returns the find -path operands the sh collector makes of one
// catalog match. expand_catalog prints a path holding a newline as one line
// per line of it, and the case classification after it keeps a later line
// only when it holds two `|` (its text after the second is then read as a
// path), so the first line of the path stands for it; a kept line starting
// `#|` is dropped as a log message.
func claimLines(path string) []string {
	pieces := strings.Split(path, "\n")
	lines := []string{pieces[0]}
	for _, piece := range pieces[1:] {
		if strings.Count(piece, "|") >= 2 && !strings.HasPrefix(piece, "#|") {
			lines = append(lines, strings.SplitN(piece, "|", 3)[2])
		}
	}
	return lines
}

// pathsOf returns the paths of a user or project list.
func pathsOf(bases []users.Home) []string {
	paths := make([]string, len(bases))
	for i, base := range bases {
		paths[i] = base.Path
	}
	return paths
}
