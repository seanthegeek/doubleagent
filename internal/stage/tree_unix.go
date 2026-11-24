//go:build unix

package stage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"golang.org/x/sys/unix"
)

// errPathEscapes is the text os.Root gives a path that leads out of the
// root, by a `..` or a link (os/file.go, errPathEscapes); the standard
// library exports no value to compare with.
const errPathEscapes = "path escapes from parent"

// heldDirectory is what a tree holds open: an os.Root on the staging
// directory, whose methods walk each name from its descriptor and refuse a
// `..` or a link leading out of it, and the chain of directories from it
// down to the parent of the last name used (descend), each opened as one
// real directory from the level above, never through a link. The chain
// assumes only the run, or a writer inside the tree, replaces a directory
// in the tree while it is open. It is kept for its cost: staging that walks
// each name from the root measured several times slower per file. A run
// stages from one goroutine, so it takes no lock.
type heldDirectory struct {
	root   *os.Root
	levels []heldLevel
}

// heldLevel is one open directory of the chain: its name in the directory
// above, the directory, and, once SetTimes needs it, the same directory
// open as a file for its descriptor.
type heldLevel struct {
	name      string
	directory *os.Root
	file      *os.File
}

// CreateTree makes the staging directory at path, new (osfile.Mkdir), and
// opens it. The directory opened must then be the one just made
// (madeByThisRun), so a directory or link swapped in between the two calls
// is refused. When the open or that check fails, the directory made is
// left at its name, which may no longer be the run's, and the error says
// so (madeButRefused).
func CreateTree(path string) (*Tree, error) {
	if err := osfile.Mkdir(path, 0o777); err != nil {
		return nil, err
	}
	tree, err := OpenTree(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: madeButRefused{reason: innerError(err)}}
	}
	if !tree.madeByThisRun() {
		_ = tree.Close()
		return nil, &fs.PathError{Op: "open", Path: path,
			Err: madeButRefused{reason: notOwnDirectory{}}}
	}
	return tree, nil
}

// madeByThisRun reports whether the open directory is what the tree's name
// holds, unfollowed, is empty, and has the owner the file system gives a
// probe file this process creates in it now (sameOwnerAsNew). On an
// ordinary file system that is the effective user, so another account's
// directory is refused; on a volume that keeps no owners (FAT, exFAT or
// NTFS on Linux, NFS with root squashed, a macOS volume ignoring ownership)
// every entry gets the same owner, so nothing is refused there.
func (tree *Tree) madeByThisRun() bool {
	info, err := tree.held.root.Stat(".")
	if err != nil || !tree.stillNamed() {
		return false
	}
	directory, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	probe := ".owner-probe-" + hex.EncodeToString(random)
	file, err := tree.CreateNew(probe, 0o600)
	if err != nil {
		return false
	}
	created, statErr := file.Stat()
	_ = file.Close()
	if err := tree.Remove(probe); err != nil || statErr != nil {
		return false
	}
	createdStatus, ok := created.Sys().(*syscall.Stat_t)
	if !ok || !sameOwnerAsNew(directory.Uid, createdStatus.Uid) {
		return false
	}
	entries, err := tree.ReadDir(".")
	return err == nil && len(entries) == 0
}

// sameOwnerAsNew reports whether a directory owned by directory is one this
// process could have made, given created, the owner of an entry it just
// made beside it: the two must be the same.
func sameOwnerAsNew(directory, created uint32) bool {
	return directory == created
}

// OpenTree opens the existing staging directory at path, following a link
// in its name as os.OpenRoot does.
func OpenTree(path string) (*Tree, error) {
	root, err := osfile.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &Tree{path: path, held: heldDirectory{root: root}}, nil
}

// OpenDir opens the directory name in the tree as a tree of its own, so a
// walk down the tree takes each name one step from an open directory. A
// link at name is followed only to a directory inside the tree.
func (tree *Tree) OpenDir(name string) (*Tree, error) {
	root, err := tree.held.root.OpenRoot(name)
	if err != nil {
		return nil, tree.pathError("open", name, err)
	}
	return &Tree{path: tree.full(name), held: heldDirectory{root: root}}, nil
}

// Close closes the tree and the parent it holds; its methods fail
// afterwards.
func (tree *Tree) Close() error {
	tree.releaseParent()
	return tree.held.root.Close()
}

// releaseParent closes every level of the chain the tree holds.
func (tree *Tree) releaseParent() {
	tree.keepLevels(0)
}

// keepLevels closes the levels of the chain below the first count, deepest
// first.
func (tree *Tree) keepLevels(count int) {
	held := &tree.held
	for i := len(held.levels) - 1; i >= count; i-- {
		if held.levels[i].file != nil {
			_ = held.levels[i].file.Close()
		}
		_ = held.levels[i].directory.Close()
	}
	held.levels = held.levels[:min(count, len(held.levels))]
}

// descentEnd is how descend ended.
type descentEnd int

const (
	// descentHeld: every component is held, the deepest level last.
	descentHeld descentEnd = iota
	// descentMissing: a component does not exist (an lstat from the level
	// above found nothing), so neither does the name.
	descentMissing
	// descentOther: a component is a link or not a real directory, or a
	// call failed; the caller walks from the root instead.
	descentOther
)

// descend makes the chain the tree holds end at the directory name: the
// levels it shares with name are kept, the rest closed, and each further
// component opened from the level above, made first when create is set
// and the lstat finds nothing. A component that is a link, `.`, `..` or
// anything but a real directory is never stepped through.
func (tree *Tree) descend(name string, create bool) descentEnd {
	components := strings.Split(name, "/")
	held := &tree.held
	common := 0
	for common < len(held.levels) && common < len(components) &&
		held.levels[common].name == components[common] {
		common++
	}
	tree.keepLevels(common)
	for _, component := range components[common:] {
		above := held.root
		if len(held.levels) > 0 {
			above = held.levels[len(held.levels)-1].directory
		}
		if component == "" || component == "." || component == ".." {
			return descentOther
		}
		info, err := above.Lstat(component)
		if errors.Is(err, fs.ErrNotExist) && create {
			err = above.Mkdir(component, 0o777)
			if err == nil {
				info, err = above.Lstat(component)
			}
		}
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return descentMissing
		case err != nil || !info.IsDir():
			return descentOther
		}
		directory, err := above.OpenRoot(component)
		if err != nil {
			return descentOther
		}
		held.levels = append(held.levels, heldLevel{name: component, directory: directory})
	}
	return descentHeld
}

// inParent returns the open directory holding name and name's last
// component: the tree's root for a name of one component, else the deepest
// level of the chain once it ends at name's parent (descend). ok is false
// when the chain cannot reach that parent; the caller then walks from the
// root, whose error says why.
func (tree *Tree) inParent(name string) (directory *os.Root, last string, ok bool) {
	slash := strings.LastIndexByte(name, '/')
	if slash < 0 {
		return tree.held.root, name, true
	}
	if tree.descend(name[:slash], false) != descentHeld {
		return nil, "", false
	}
	levels := tree.held.levels
	return levels[len(levels)-1].directory, name[slash+1:], true
}

// stillNamed reports whether the tree's name, unfollowed, holds the
// directory the tree has open.
func (tree *Tree) stillNamed() bool {
	opened, err := tree.held.root.Stat(".")
	if err != nil {
		return false
	}
	named, err := osfile.Lstat(tree.path)
	return err == nil && named.IsDir() && os.SameFile(opened, named)
}

// MkdirAll creates the directory name in the tree and its missing parents,
// as mkdir -p does: one mkdir and one open per missing component from the
// deepest level the chain shares with name (descend), else by a walk from
// the root.
func (tree *Tree) MkdirAll(name string) error {
	if tree.descend(name, true) == descentHeld {
		return nil
	}
	return tree.pathError("mkdir", name, tree.held.root.MkdirAll(name, 0o777))
}

// openFile opens name in the tree with flag and perm, one step from its
// parent (inParent), never following a final link (os.Root adds
// O_NOFOLLOW).
func (tree *Tree) openFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if directory, last, ok := tree.inParent(name); ok {
		file, err := directory.OpenFile(last, flag, perm)
		return file, tree.stepError("open", name, err)
	}
	file, err := tree.held.root.OpenFile(name, flag, perm)
	return file, tree.pathError("open", name, err)
}

// Open opens the file name in the tree for reading, by a walk from the
// root, which listings use, so they leave the parent the stager holds.
func (tree *Tree) Open(name string) (*os.File, error) {
	file, err := tree.held.root.OpenFile(name, os.O_RDONLY, 0)
	return file, tree.pathError("open", name, err)
}

// Lstat describes name in the tree itself, a link included, one step from
// its parent (inParent).
func (tree *Tree) Lstat(name string) (fs.FileInfo, error) {
	if directory, last, ok := tree.inParent(name); ok {
		info, err := directory.Lstat(last)
		return info, tree.stepError("lstat", name, err)
	}
	info, err := tree.held.root.Lstat(name)
	return info, tree.pathError("lstat", name, err)
}

// meta is Lstat as the identity and kind the stager compares: device,
// inode and whether name is a link.
func (tree *Tree) meta(name string) (fsmeta.Meta, error) {
	info, err := tree.Lstat(name)
	if err != nil {
		return fsmeta.Meta{}, err
	}
	identity := fsmeta.StatIdentity(info)
	meta := fsmeta.Meta{Dev: identity[0], Ino: identity[1], Kind: fsmeta.Other}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		meta.Kind = fsmeta.Symlink
	case info.IsDir():
		meta.Kind = fsmeta.Dir
	case info.Mode().IsRegular():
		meta.Kind = fsmeta.File
	}
	return meta, nil
}

// ReadDir lists the directory name in the tree in directory order.
func (tree *Tree) ReadDir(name string) ([]fs.DirEntry, error) {
	directory, err := tree.Open(name)
	if err != nil {
		return nil, err
	}
	entries, err := directory.ReadDir(-1)
	if closeErr := directory.Close(); err == nil {
		err = closeErr
	}
	return entries, err
}

// Readlink returns the stored target of the link name in the tree.
func (tree *Tree) Readlink(name string) (string, error) {
	target, err := tree.held.root.Readlink(name)
	return target, tree.pathError("readlink", name, err)
}

// Remove removes the file, link or empty directory name in the tree, one
// step from its parent (inParent).
func (tree *Tree) Remove(name string) error {
	if directory, last, ok := tree.inParent(name); ok {
		return tree.stepError("remove", name, directory.Remove(last))
	}
	return tree.pathError("remove", name, tree.held.root.Remove(name))
}

// removeAll removes name in the tree and everything under it, following
// no link.
func (tree *Tree) removeAll(name string) error {
	return tree.pathError("remove", name, tree.held.root.RemoveAll(name))
}

// Symlink makes name in the tree a link to target, stored as given, one
// step from its parent (inParent). A Unix link has no file or directory
// kind, so directory is not needed.
func (tree *Tree) Symlink(target, name string, _ bool) error {
	var err error
	if directory, last, ok := tree.inParent(name); ok {
		err = directory.Symlink(target, last)
	} else {
		err = tree.held.root.Symlink(target, name)
	}
	var linkErr *os.LinkError
	if err == nil || !errors.As(err, &linkErr) {
		return tree.pathError("symlink", name, err)
	}
	if linkErr.Err.Error() == errPathEscapes {
		return &fs.PathError{Op: "symlink", Path: parentOf(tree.full(name)), Err: outsideStage{}}
	}
	return &os.LinkError{Op: "symlink", Old: target, New: tree.full(name), Err: linkErr.Err}
}

// SetTimes sets the access and modification times of name in the tree, to
// the nanosecond, never following a final link: utimensat with
// AT_SYMLINK_NOFOLLOW on the descriptor of the deepest level of the chain
// (inParent); a name of one component goes through the root, whose
// Chtimes follows a final link only inside the tree. It goes by name, not
// by the open copy, since no one call sets nanosecond times on an open file
// on both Linux and macOS.
func (tree *Tree) SetTimes(name string, accessed, modified time.Time) error {
	directory, last, ok := tree.inParent(name)
	if !ok || directory == tree.held.root {
		return tree.pathError("utimes", name, tree.held.root.Chtimes(name, accessed, modified))
	}
	level := &tree.held.levels[len(tree.held.levels)-1]
	if level.file == nil {
		file, err := directory.Open(".")
		if err != nil {
			return tree.stepError("utimes", name, err)
		}
		level.file = file
	}
	times := []unix.Timespec{unix.NsecToTimespec(accessed.UnixNano()),
		unix.NsecToTimespec(modified.UnixNano())}
	connection, err := level.file.SyscallConn()
	if err != nil {
		return tree.stepError("utimes", name, err)
	}
	var callErr error
	err = connection.Control(func(descriptor uintptr) {
		callErr = unix.UtimesNanoAt(int(descriptor), last, times, unix.AT_SYMLINK_NOFOLLOW)
	})
	if err == nil {
		err = callErr
	}
	return tree.stepError("utimes", name, err)
}

// lookDir reports what is at name in the tree, for findInside: a directory
// reached without leaving the tree, something else (a file, a link leading
// out or dangling), or nothing; err is a failure of another kind. A name the
// chain reaches (descend) is a directory inside the tree, and one it finds
// missing is missing; any other is asked of the root.
func (tree *Tree) lookDir(name string) (dirFound, error) {
	if name == "." {
		return dirInside, nil
	}
	switch tree.descend(name, false) {
	case descentHeld:
		return dirInside, nil
	case descentMissing:
		return dirMissing, nil
	}
	info, err := tree.held.root.Stat(name)
	switch {
	case err == nil && info.IsDir():
		return dirInside, nil
	case err == nil || leavesTree(tree.pathError("stat", name, err)):
		return dirOutside, nil
	}
	_, err = tree.Lstat(name)
	switch {
	case err == nil || leavesTree(err):
		return dirOutside, nil
	case errors.Is(err, fs.ErrNotExist):
		return dirMissing, nil
	}
	return dirMissing, err
}

// stepError is pathError for err, an error of an os.Root method given
// name's last component one step from its parent: the path named is name.
func (tree *Tree) stepError(operation, name string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = &fs.PathError{Op: pathErr.Op, Path: name, Err: pathErr.Err}
	}
	return tree.pathError(operation, name, err)
}

// pathError returns err, an error of an os.Root method on name, as the
// path functions word it: the operation and the path below the tree's
// name; a path the root refused because it leads out of the tree has the
// reason outsideStage.
func (tree *Tree) pathError(operation, name string, err error) error {
	if err == nil {
		return nil
	}
	inner, relative := err, name
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	if errors.As(err, &pathErr) {
		inner, relative = pathErr.Err, pathErr.Path
	} else if errors.As(err, &linkErr) {
		inner = linkErr.Err
	}
	if inner.Error() == errPathEscapes {
		return &fs.PathError{Op: operation, Path: tree.full(name), Err: outsideStage{}}
	}
	return &fs.PathError{Op: operation, Path: tree.full(relative), Err: inner}
}
