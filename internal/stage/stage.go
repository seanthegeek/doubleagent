// Package stage puts collected files into the staging directory, the tree that
// becomes the archive: the staged path of each file, the copy and its hash,
// recreated symlinks, and the removal of a failed partial copy. It reproduces
// the set_dest_var, copy_run and test_inside_stage steps of the sh
// collector's STAGE_FILES_WORKER.
package stage

import (
	"crypto/sha256"
	"encoding/hex"
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
	if err := os.MkdirAll(parentOf(dest), 0o777); err != nil {
		return fmt.Errorf("stage %s: %w", path, err)
	}
	return nil
}

// CopyFile copies the regular file path to its staged path and returns the
// SHA-256 of the bytes written, in lower-case hex. Neither end is opened
// through a final symlink, and the copy gets the source's mode and times, and
// its owner when running as root, as cp -p gives it. The error is the
// innermost *fs.PathError the copy met.
func (stager Stager) CopyFile(path string) (string, error) {
	_, dest := stager.Dest(path)
	if err := os.MkdirAll(parentOf(dest), 0o777); err != nil {
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
// directory would create it inside that directory, on the source.
func (stager Stager) Symlink(path, target string) error {
	_, dest := stager.Dest(path)
	if _, err := os.Lstat(dest); err == nil {
		return nil
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
