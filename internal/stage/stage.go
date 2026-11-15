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
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Stager says where files are staged. Dir is the staging directory, Physical
// its path with every symlink resolved, and Root the image root ("" on a live
// host). From and Into, when set, are an in-base symlinked directory a path
// ran through and that directory's physical path in the base: a path at or
// under From is staged under Into, so nothing is staged through a link
// recreated in the staging tree.
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
// every byte after fs. On Windows the root and From are matched ignoring
// case, and a drive or UNC path left after the root is archived as
// Get-ArchivePath does (C:/a under fs/C/a, //server/share/a under
// fs/server/share/a).
func (stager Stager) Dest(path string) (archivePath, destPath string) {
	relative := path
	if stager.From != "" {
		if syspath.Equal(path, stager.From) {
			relative = stager.Into
		} else if below, found := syspath.CutPrefix(path, stager.From+"/"); found {
			relative = stager.Into + "/" + below
		}
	}
	rest, _ := syspath.CutPrefix(relative, stager.Root)
	archivePath = "fs" + syspath.ArchiveRel(rest)
	return archivePath, stager.Dir + "/" + archivePath
}

// MakeParent creates the directory path is staged in, with its parents, as
// mkdir -p does; the sh collector makes it for every file it is given,
// skipped files included, so the archive holds those directories too.
func (stager Stager) MakeParent(path string) error {
	_, dest := stager.Dest(path)
	if err := stager.makeDir(parentOf(dest)); err != nil {
		return fmt.Errorf("stage %s: %w", syspath.ToNative(path), err)
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
	return osfile.MkdirAll(dir, 0o777)
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
		_, err := osfile.Lstat(existing)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := parentOf(existing)
		if parent == existing || parent == "" {
			return &fs.PathError{Op: operation, Path: syspath.ToNative(dir), Err: outsideStage{}}
		}
		existing = parent
	}
	physical, err := PhysicalDir(existing)
	if err != nil || !stager.holds(physical) {
		return &fs.PathError{Op: operation, Path: syspath.ToNative(dir), Err: outsideStage{}}
	}
	return nil
}

// holds reports whether the physical directory lies at or under the
// staging directory. Physical is compared first; when it does not match, or
// is empty, the staging directory is resolved again, so a stager whose
// Physical was given unresolved still stages.
func (stager Stager) holds(physical string) bool {
	under := func(root string) bool {
		return root != "" && syspath.HasPrefix(physical+"/", strings.TrimSuffix(root, "/")+"/")
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
// SHA-256 of the bytes written, in lower-case hex, and their count, which
// the Windows rows give as the size, as Copy-FileShared does. Nothing is
// written unless the staged path's directory resolves inside the staging
// directory and neither end is opened through a final link; the copy gets
// the source's times, and on Unix its mode and, as root, its owner, as cp -p
// gives them. The error is the innermost *fs.PathError the copy met.
func (stager Stager) CopyFile(path string) (string, int64, error) {
	_, dest := stager.Dest(path)
	if err := stager.makeDir(parentOf(dest)); err != nil {
		return "", 0, err
	}
	source, err := openSource(path)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: notRegular{}}
	}
	target, err := createDest(dest)
	if err != nil {
		return "", 0, syspath.NativeErrors(err)
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(target, hash), source)
	if err == nil {
		err = preserve(target, dest, info)
	}
	if closeError := target.Close(); err == nil {
		err = closeError
	}
	if err != nil {
		// The staged copy was opened through osfile, so its write and close
		// errors name the \\?\ form.
		return "", 0, syspath.NativeErrors(err)
	}
	return hex.EncodeToString(hash.Sum(nil)), written, nil
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
// directory makes a Windows directory link, as a junction or a directory
// symlink is recreated; Unix links have no kind and ignore it.
func (stager Stager) Symlink(path, target string, directory bool) error {
	_, dest := stager.Dest(path)
	if _, err := osfile.Lstat(dest); err == nil {
		return nil
	}
	if err := stager.requireInside("symlink", parentOf(dest)); err != nil {
		return err
	}
	return createLink(target, dest, directory)
}

// RemoveFailedCopy removes what a failed copy left at dest and reports true,
// or reports true when it left nothing. It reports false, removing nothing,
// when dest's directory does not resolve inside the physical staging
// directory: the staged path then leads through a recreated link out of the
// staging tree, and a file there may be a source.
func (stager Stager) RemoveFailedCopy(dest string) bool {
	info, err := osfile.Lstat(dest)
	if err != nil {
		return true
	}
	physical, err := PhysicalDir(parentOf(dest))
	if stager.Physical == "" || err != nil ||
		!syspath.HasPrefix(physical+"/", stager.Physical+"/") {
		return false
	}
	// rm -f removes no directory, and os.Remove would remove an empty one.
	if !info.IsDir() {
		_ = osfile.Remove(dest)
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
