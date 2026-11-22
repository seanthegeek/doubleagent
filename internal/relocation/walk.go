package relocation

import (
	"context"
	"io/fs"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// sweepWalk lists the files of one sweep in the platform's order
// (sweepOrder): depth first, never following a symlink or any other reparse
// point (a Windows junction included), a pruned path left out with everything under it.
type sweepWalk struct {
	ctx     context.Context
	opener  *fsmeta.Opener
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
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return nil
	}
	if meta.Kind == fsmeta.File {
		walk.addFile(path, meta.Size)
		return nil
	}
	if meta.Kind != fsmeta.Dir {
		return nil
	}
	if err := walk.ctx.Err(); err != nil {
		return err
	}
	for _, name := range sweepOrder(readEntries(walk.opener, path)) {
		if err := walk.visit(path + "/" + name); err != nil {
			return err
		}
	}
	return nil
}

// addFile lists a regular file or counts it as over the size limit; a file
// whose name holds a newline is neither, since a line-per-path list could
// not tell it from two paths.
func (walk *sweepWalk) addFile(path string, size int64) {
	name := path[strings.LastIndexByte(path, '/')+1:]
	if strings.Contains(name, "\n") {
		return
	}
	if walk.limit == 0 || size <= walk.limit {
		walk.files = append(walk.files, path)
	} else {
		walk.skipped++
	}
}

// readEntries returns the entries of directory in directory order, listed
// through fsmeta.OpenDir with opener.
func readEntries(opener *fsmeta.Opener, directory string) []fs.DirEntry {
	handle, err := fsmeta.OpenDir(opener, directory)
	if err != nil {
		return nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	return entries
}

// stackOrder returns the names of a directory's entries in the order the
// Windows sweep reaches them: every entry that is not a directory in
// listing order, then the directories in reverse listing order. The function
// pops a directory off a stack, adds its files as it lists them and pushes
// its subdirectories, so the subdirectory listed last is popped next and
// swept whole before the one listed before it. Visiting each name in this
// order depth first yields the same files in the same order. A junction or a
// directory symlink, which Go does not type a directory, lands among the
// files; the sweep skips it wherever it lies.
func stackOrder(entries []fs.DirEntry) []string {
	names := make([]string, 0, len(entries))
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		} else {
			names = append(names, entry.Name())
		}
	}
	for i := len(directories) - 1; i >= 0; i-- {
		names = append(names, directories[i])
	}
	return names
}

// claimLines returns the claim paths of one catalog match. A path holding a
// newline is read as one line per line of it, and a later line is kept only
// when it holds two `|` (its text after the second is then read as a path),
// so the first line of the path stands for it; a kept line starting `#|` is
// dropped as a log message.
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
