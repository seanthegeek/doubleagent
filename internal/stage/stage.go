// Package stage puts collected files into the staging directory, the tree that
// becomes the archive: the staged path of each file, the copy and its hash,
// recreated symlinks, and the removal of a failed partial copy. It reproduces
// the set_dest_var, copy_run and test_inside_stage steps of the sh
// collector's STAGE_FILES_WORKER.
package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

// Stager says where files are staged. Dir is the staging directory, Physical
// its path with every symlink resolved, and Root the image root ("" on a live
// host). From and To, when set, are an in-base symlinked directory a path ran
// through and that directory's physical path in the base: a path at or under
// From is staged under To, so nothing is staged through a link recreated in
// the staging tree.
type Stager struct {
	Dir      string
	Physical string
	Root     string
	From     string
	Into     string
}

// Through returns a copy of the stager that stages paths under from at into.
func (stager Stager) Through(from, into string) Stager {
	stager.From, stager.Into = from, into
	return stager
}

// Dest returns the archive member of path, fs/ and the path without the
// root, and where it is staged. As set_dest_var does, the root is removed
// only as a leading string, and a path that does not start with it keeps
// every byte after fs.
func (stager Stager) Dest(path string) (archivePath, destPath string) {
	relative := path
	if stager.From != "" &&
		(path == stager.From || strings.HasPrefix(path, stager.From+"/")) {
		relative = stager.Into + path[len(stager.From):]
	}
	archivePath = "fs" + strings.TrimPrefix(relative, stager.Root)
	return archivePath, stager.Dir + "/" + archivePath
}

// MakeParent creates the directory path is staged in, with its parents, as
// mkdir -p does; the sh collector makes it for every file it is given,
// skipped files included, so the archive holds those directories too.
func (stager Stager) MakeParent(path string) error {
	_, dest := stager.Dest(path)
	if err := stager.makeDir(parentOf(dest)); err != nil {
		return fmt.Errorf("stage %s: %w", path, err)
	}
	return nil
}

// makeDir creates dir with its parents, as mkdir -p does, once the part of
// dir that exists resolves inside the staging directory. The directories it
// then creates are new and real, so nothing is created outside it.
func (stager Stager) makeDir(dir string) error {
	if err := stager.requireInside("mkdir", dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o777)
}

// requireInside returns an *fs.PathError of operation on dir unless dir, or
// its nearest ancestor that exists when dir does not, resolves with every
// symlink under the physical staging directory; an lstat error other than a
// missing path is returned as it is. Every write into the staging tree
// passes it first: a link recreated there can lead to the source, and a
// write through it would change the evidence.
func (stager Stager) requireInside(operation, dir string) error {
	existing := dir
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := parentOf(existing)
		if parent == existing || parent == "" {
			return &fs.PathError{Op: operation, Path: dir, Err: outsideStage{}}
		}
		existing = parent
	}
	physical, err := PhysicalDir(existing)
	if err != nil || !stager.holds(physical) {
		return &fs.PathError{Op: operation, Path: dir, Err: outsideStage{}}
	}
	return nil
}

// holds reports whether the physical directory lies at or under the
// staging directory. Physical is compared first; when it does not match, or
// is empty, the staging directory is resolved again, so a stager whose
// Physical was given unresolved still stages.
func (stager Stager) holds(physical string) bool {
	under := func(root string) bool {
		return root != "" && strings.HasPrefix(physical+"/", strings.TrimSuffix(root, "/")+"/")
	}
	if under(stager.Physical) {
		return true
	}
	root := stager.Physical
	if root == "" {
		root = stager.Dir
	}
	resolved, err := PhysicalDir(root)
	return err == nil && under(resolved)
}

// outsideStage is the reason a write into the staging tree is refused: the
// directory written in does not resolve inside the staging directory.
type outsideStage struct{}

// Error returns the reason.
func (outsideStage) Error() string { return "does not resolve inside the staging directory" }

// CopyFile copies the regular file path to its staged path and returns the
// SHA-256 of the bytes written, in lower-case hex. Nothing is written unless
// the staged path's directory resolves inside the staging directory, neither
// end is opened through a final symlink, and the copy gets the source's mode
// and times, and its owner when running as root, as cp -p gives it. The
// error is the innermost *fs.PathError the copy met.
func (stager Stager) CopyFile(path string) (string, error) {
	_, dest := stager.Dest(path)
	if err := stager.makeDir(parentOf(dest)); err != nil {
		return "", err
	}
	source, err := openSource(path)
	if err != nil {
		return "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", &fs.PathError{Op: "open", Path: path, Err: notRegular{}}
	}
	target, err := createDest(dest)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(target, hash), source)
	if err == nil {
		err = preserve(target, dest, info)
	}
	if closeError := target.Close(); err == nil {
		err = closeError
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// notRegular is the reason a path that stopped being a regular file between
// its lstat and its copy is not copied: reading a FIFO or a device could
// block or read something other than a file's contents.
type notRegular struct{}

// Error returns the reason, in the text of the error it replaces.
func (notRegular) Error() string { return "not a regular file" }

// Symlink recreates the symlink path, with its stored target, at its staged
// path. A staged path that already exists, a link included, is left as it
// is: another entry's walk recreated it, and creating a link onto a link to a
// directory would create it inside that directory, on the source. No link is
// created in a directory that does not resolve inside the staging directory.
func (stager Stager) Symlink(path, target string) error {
	_, dest := stager.Dest(path)
	if _, err := os.Lstat(dest); err == nil {
		return nil
	}
	if err := stager.requireInside("symlink", parentOf(dest)); err != nil {
		return err
	}
	return os.Symlink(target, dest)
}

// RemoveFailedCopy removes what a failed copy left at dest and reports true,
// or reports true when it left nothing. It reports false, removing nothing,
// when dest's directory does not resolve inside the physical staging
// directory: the staged path then leads through a recreated link out of the
// staging tree, and a file there may be a source.
func (stager Stager) RemoveFailedCopy(dest string) bool {
	info, err := os.Lstat(dest)
	if err != nil {
		return true
	}
	physical, err := PhysicalDir(parentOf(dest))
	if stager.Physical == "" || err != nil ||
		!strings.HasPrefix(physical+"/", stager.Physical+"/") {
		return false
	}
	// rm -f removes no directory, and os.Remove would remove an empty one.
	if !info.IsDir() {
		_ = os.Remove(dest)
	}
	return true
}

// parentOf returns path up to its last slash, as the shell's ${path%/*}
// does, so a path with no slash is its own parent.
func parentOf(path string) string {
	if position := strings.LastIndexByte(path, '/'); position >= 0 {
		return path[:position]
	}
	return path
}
