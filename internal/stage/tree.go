package stage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Tree is the open staging directory. Every method takes a slash path
// relative to it, as Stager.Dest names a member (fs/..., collection.json),
// "." for the directory itself, and reaches it through the directory the
// tree opened, not through what the directory's name points at now: on
// Linux and macOS through an os.Root, which no rename of the directory
// changes and which refuses a link leading out of the tree; on Windows the
// open handle (held) keeps the name from being renamed or deleted, and the
// path is used through osfile so every name stays exact. Path is the name
// the run gave the directory, kept for messages; physical is that name
// with every link resolved when the tree was opened.
type Tree struct {
	path     string
	physical string
	held     heldDirectory
}

// Path returns the name the staging directory was opened by, in canonical
// form.
func (tree *Tree) Path() string { return tree.path }

// full returns the path of name below the tree as the run named the tree,
// for messages and, on Windows, for the calls themselves.
func (tree *Tree) full(name string) string {
	if name == "." {
		return tree.path
	}
	return tree.path + "/" + name
}

// errPathEscapes is the text os.Root gives a path that leads out of the
// root, by a `..` or a link (os/file.go, errPathEscapes); the standard
// library exports no value to compare with.
const errPathEscapes = "path escapes from parent"

// leavesTree reports whether err says a path left the tree: the reason
// outsideStage, which the tree's methods put in place of the root's own.
func leavesTree(err error) bool {
	return errors.As(err, &outsideStage{})
}

// CreateNew creates the file name in the tree for writing, new: anything
// already there, a link or a dangling link included, fails the create and
// is neither followed nor changed.
func (tree *Tree) CreateNew(name string, perm fs.FileMode) (*os.File, error) {
	return tree.openFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
}

// FoldsCase reports whether the directory name in the tree, a staging
// directory, takes two names for one: two differing in ASCII letter case,
// or the two Unicode forms of an accented letter (precomposed é, and e
// with a combining acute accent), made as the first and looked up as the
// second. Safety does not depend on it: no staged file is ever written
// over (Copy). It decides the note, whether a member's spelling is read
// from the tree (Linux, links) and whether a refusal is worded as a
// collision (again, takenByAnother); a rule it misses, such as a Windows
// short name, costs an archive path spelled as named or a plain reason.
func (tree *Tree) FoldsCase(name string) bool {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	stem := ".case-probe-" + hex.EncodeToString(random)
	for _, pair := range [][2]string{{stem, strings.ToUpper(stem)},
		{stem + "-\u00e9", stem + "-e\u0301"}} {
		probe, err := tree.CreateNew(join(name, pair[0]), 0o600)
		if err != nil {
			return true
		}
		_ = probe.Close()
		_, err = tree.Lstat(join(name, pair[1]))
		_ = tree.Remove(join(name, pair[0]))
		if err == nil {
			return true
		}
	}
	return false
}

// join returns the tree path of child in the tree directory name, "." for
// the tree itself.
func join(name, child string) string {
	if name == "." {
		return child
	}
	return name + "/" + child
}

// RemoveContents removes everything in the tree, leaving the directory
// itself, and returns the first error met.
func (tree *Tree) RemoveContents() error {
	tree.releaseParent()
	entries, err := tree.ReadDir(".")
	for _, entry := range entries {
		if removeErr := tree.removeAll(entry.Name()); err == nil {
			err = removeErr
		}
	}
	return err
}

// Discard removes everything in the tree, closes it, then removes the
// directory by the name the run gave it when, just before the close, that
// name still held the tree's own directory; a name that holds anything
// else (the directory moved aside and another entry put in its place) is
// left as it is, with an error that says so. The first error is returned.
func (tree *Tree) Discard() error {
	err := tree.RemoveContents()
	named := tree.stillNamed()
	if closeErr := tree.Close(); err == nil {
		err = closeErr
	}
	if !named {
		return &fs.PathError{Op: "remove", Path: syspath.ToNative(tree.path), Err: notThisTree{}}
	}
	if removeErr := osfile.Remove(tree.path); err == nil {
		err = removeErr
	}
	return err
}

// notThisTree is the reason the staging directory's name is not removed:
// it no longer names the directory the run staged in.
type notThisTree struct{}

// Error returns the reason.
func (notThisTree) Error() string { return "no longer names the staging directory this run made" }

// madeButRefused is the reason CreateTree gives up on a directory it made
// when the open or the check of it fails: the directory is left at its
// name, which may no longer be the run's.
type madeButRefused struct {
	reason error
}

// Error returns the reason.
func (refused madeButRefused) Error() string {
	return refused.reason.Error() + "; the directory made there is left as it is"
}

// Unwrap returns what failed.
func (refused madeButRefused) Unwrap() error { return refused.reason }

// innerError returns the reason of a *fs.PathError, so a message that names
// the path itself does not name it twice, or err as it is.
func innerError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// swappedDirectory is the reason CreateTree refuses the directory it opened:
// it is not the empty directory the run just made under that name.
type swappedDirectory struct{}

// Error returns the reason.
func (swappedDirectory) Error() string {
	return "is not the empty directory this run just made; another entry took its name"
}
