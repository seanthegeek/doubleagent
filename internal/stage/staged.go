package stage

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
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

// collides reports whether the walk shows path's own place taken by another
// entry: the component it ended or stopped at is spelled otherwise on disk
// and is the last one, or is a link or file where a directory was needed.
func (spelling stagedSpelling) collides() bool {
	return spelling.stopRespelled &&
		(spelling.stop == stagedEnd || spelling.stop == stagedNotDirectory)
}

// stagedFS is what walkStaged asks of the file system: a directory's
// listing and an lstat. Tests count the calls and fake their answers.
type stagedFS struct {
	list  func(dir string) ([]fs.DirEntry, error)
	lstat func(path string) (fsmeta.Meta, error)
}

// spelling returns path, a staged path, as the staging tree spells it
// (walkStaged) where the staging directory folds case; where it tells case
// apart, a path is spelled as named and nothing is asked.
func (stager Stager) spelling(path string) stagedSpelling {
	if !stager.FoldsCase {
		return stagedSpelling{path: path}
	}
	fsys := systemFS()
	if stager.stagedFS != nil {
		fsys = *stager.stagedFS
	}
	return walkStaged(fsys, stager.Dir, path)
}

// systemFS is the file system itself.
func systemFS() stagedFS {
	return stagedFS{list: osfile.ReadDir, lstat: fsmeta.Lstat}
}

// walkStaged spells path, below base, as the staging tree holds it. It
// lists only base and directories it found typed as real directories in
// their parent's listing, so it never opens, lists or stats anything
// through a link. In each, a component listed with its own bytes is kept;
// one that is not listed but that an lstat in that directory still finds
// was resolved by the volume to an entry spelled otherwise, which is found
// by its device and inode, whatever the volume's case or Unicode rule. The
// walk goes on only into a real directory (internals.md, Claimed paths and
// case variants).
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
