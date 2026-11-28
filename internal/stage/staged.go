package stage

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// stagedStop is how a walk of a staged path ended (Stager.spelling).
type stagedStop int

const (
	// stagedEnd: every component was found; the last may be anything.
	stagedEnd stagedStop = iota
	// stagedNotDirectory: a component before the last is a link, a file or
	// anything else that is not a real directory; nothing below it was
	// looked at.
	stagedNotDirectory
	// stagedMissing: a component does not exist, nor any other spelling of
	// it, so neither does the rest.
	stagedMissing
	// stagedFailed: a listing or lstat failed for another reason, or the
	// path is not below the staging directory.
	stagedFailed
)

// stagedSpelling is what a walk of a staged path found: the path as the
// staging tree spells it as far as the walk got, whether any component
// was respelled, how the walk ended, and whether the component it ended or
// stopped at was itself respelled.
type stagedSpelling struct {
	path          string
	respelled     bool
	stop          stagedStop
	stopRespelled bool
}

// collides reports whether the walk shows path's place taken by another
// entry: some component is spelled otherwise on disk and the walk either
// reached the end or stopped at a link or file where a directory was
// needed. A walk that found a component missing, or failed, proves nothing,
// and the caller keeps its own error.
func (spelling stagedSpelling) collides() bool {
	return spelling.respelled &&
		(spelling.stop == stagedEnd || spelling.stop == stagedNotDirectory)
}

// stagedFS is what walkStaged asks of the file system: a directory's
// listing and an lstat. Tests count the calls and fake their answers.
type stagedFS struct {
	list  func(dir string) ([]fs.DirEntry, error)
	lstat func(path string) (fsmeta.Meta, error)
}

// files returns the file system the stager's walks and lstats of staged
// paths ask: the staging tree's, or a test's.
func (stager Stager) files() stagedFS {
	if stager.stagedFS != nil {
		return *stager.stagedFS
	}
	return stagedFS{
		list:  func(dir string) ([]fs.DirEntry, error) { return stager.Tree.ReadDir(stager.name(dir)) },
		lstat: func(path string) (fsmeta.Meta, error) { return stager.Tree.meta(stager.name(path)) },
	}
}

// entryNamed returns the entry of entries named name byte for byte.
func entryNamed(entries []fs.DirEntry, name string) (fs.DirEntry, bool) {
	for _, entry := range entries {
		if entry.Name() == name {
			return entry, true
		}
	}
	return nil, false
}

// entryResolved returns the entry of entries, the listing of the real
// directory dir, that an lstat of dir/name reaches when no entry has
// name's bytes, matched by device and inode; else nil and how the walk
// stops: missing when the lstat finds nothing, failed for any other error
// or when no entry has that identity.
func entryResolved(fsys stagedFS, dir, name string, entries []fs.DirEntry,
) (fs.DirEntry, stagedStop) {
	meta, err := fsys.lstat(dir + "/" + name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, stagedMissing
	}
	if err != nil || meta.Ino == 0 {
		return nil, stagedFailed
	}
	for _, entry := range entries {
		other, err := fsys.lstat(dir + "/" + entry.Name())
		if err == nil && other.Dev == meta.Dev && other.Ino == meta.Ino {
			return entry, stagedEnd
		}
	}
	return nil, stagedFailed
}

// walkStaged spells path, below base, as the staging tree holds it. It
// lists only base and directories its parent's listing types as real, so
// it never opens, lists or stats anything through a link. A component
// listed with its own bytes is kept; one an lstat in that directory still
// finds was resolved by the volume to another spelling, found by device
// and inode at one lstat per listed entry until one matches. Only a writer
// inside the run's own staging directory could swap a listed directory for
// a link meanwhile.
func walkStaged(fsys stagedFS, base, path string) stagedSpelling {
	rest, found := strings.CutPrefix(path, base+"/")
	if !found {
		return stagedSpelling{path: path, stop: stagedFailed}
	}
	components := strings.Split(rest, "/")
	result := stagedSpelling{path: base}
	for i, component := range components {
		entries, err := fsys.list(result.path)
		if err != nil {
			result.stop = stagedFailed
			return result
		}
		entry, exact := entryNamed(entries, component)
		stepRespelled := false
		if !exact {
			var stop stagedStop
			if entry, stop = entryResolved(fsys, result.path, component, entries); entry == nil {
				result.stop = stop
				return result
			}
			stepRespelled, result.respelled = true, true
		}
		result.path += "/" + entry.Name()
		result.stopRespelled = stepRespelled
		if i == len(components)-1 {
			return result
		}
		if entry.Type()&fs.ModeType != fs.ModeDir {
			result.stop = stagedNotDirectory
			return result
		}
	}
	return result
}

// spelling returns path, a staged path, as the staging tree spells it
// (walkStaged) where the staging directory folds case; where it tells case
// apart, a path is spelled as named and nothing is asked.
func (stager Stager) spelling(path string) stagedSpelling {
	if !stager.FoldsCase {
		return stagedSpelling{path: path}
	}
	return walkStaged(stager.files(), stager.Dir, path)
}
