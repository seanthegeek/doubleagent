// Package stage puts collected files into the staging directory, the tree that
// becomes the archive: the staged path of each file, the copy and its hash,
// recreated symlinks, and the removal of a failed partial copy. It reproduces
// the set_dest_var, copy_run and test_inside_stage steps of the sh
// collector's STAGE_FILES_WORKER.
//
// A recreated link in the staging tree may lead to the source or to the
// analyst's own files, so the stager makes no call on a staged path below a
// link: a write, an open or a link's creation first passes requireInside
// for its directory (which resolves it and checks that it lies inside the
// staging directory, listing nothing), and every respelling walks the tree
// (walkStaged on Unix, fsmeta.OnDiskPath on Windows), which stops at a
// link. The one exception is RemoveFailedCopy's lstat of the staged path,
// copy_run's test of whether a failed copy left anything, which reads the
// entry's metadata and lists nothing.
package stage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Stager says where files are staged. Dir is the staging directory, Physical
// its path with every symlink resolved, and Root the image root ("" on a live
// host). From and Into, when set, are an in-base symlinked directory a path
// ran through and that directory's physical path in the base: a path at or
// under From is staged under Into, so nothing is staged through a link
// recreated in the staging tree. Opener opens each source, as the run
// decided (whether to ask Windows to hold its access time); nil asks
// wherever the platform can.
type Stager struct {
	Dir      string
	Physical string
	Root     string
	From     string
	Into     string
	Opener   *fsmeta.Opener
	// FoldsCase is set where the staging directory takes names differing
	// only in case for one (FoldsCase): every Unix copy and link is then
	// checked against where it landed; Windows always checks.
	FoldsCase bool
	// Places is the run's one record of staged places (Places).
	Places *Places
	// afterOpen, set by tests, is called with a source's path right after
	// CopyFile opens it.
	afterOpen func(path string)
	// stagedFS, set by tests, replaces the file system walkStaged asks.
	stagedFS *stagedFS
}

// Through returns a copy of the stager that stages paths under from at into.
func (stager Stager) Through(from, into string) Stager {
	stager.From, stager.Into = from, into
	return stager
}

// Dest returns the archive member of path, fs/ and the path without the
// root, and where it is staged. As set_dest_var does, the root is removed
// only as a leading string (ignoring ASCII case on Windows), and a path not
// starting with it keeps every byte after fs. From is matched byte for
// byte, or in another ASCII case where fsmeta.SameEntry calls the two one
// entry. A drive or UNC path left is archived as Get-ArchivePath does (C:/a
// under fs/C/a, //server/share/a under fs/server/share/a). A path with a `.`
// or `..` component, which every entry point replaces by the path the system
// reaches (walk.RootCheck.Reached), is refused with an error: kept, it would
// stage the copy, and name its member, somewhere else.
func (stager Stager) Dest(path string) (archivePath, destPath string, err error) {
	if hasDotComponent(path) {
		return "", "", &fs.PathError{Op: "stage", Path: syspath.ToNative(path), Err: dottedPath{}}
	}
	relative := path
	if stager.From != "" {
		// SameEntry is asked without the root check: path is a path the walk
		// stages, under a base the check let through, and From an in-base
		// link already judged; the two lstats follow no last component.
		if syspath.Equal(path, stager.From) &&
			(path == stager.From || fsmeta.SameEntry(path, stager.From)) {
			relative = stager.Into
		} else if below, found := syspath.CutUnder(path, stager.From, fsmeta.SameEntry); found {
			relative = stager.Into + "/" + below
		}
	}
	rest, _ := syspath.CutPrefix(relative, stager.Root)
	archivePath = "fs" + syspath.ArchiveRel(rest)
	return archivePath, stager.Dir + "/" + archivePath, nil
}

// hasDotComponent reports whether path, canonical, has a `.` or `..`
// component.
func hasDotComponent(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

// dottedPath is the reason Dest refuses a path with a `.` or `..` component.
type dottedPath struct{}

// Error returns the reason.
func (dottedPath) Error() string {
	return "holds a . or .. component, so where it is staged is not known"
}

// MakeParent creates the directory path is staged in, with its parents, as
// mkdir -p does; the sh collector makes it for every file it is given,
// skipped files included, so the archive holds those directories too.
func (stager Stager) MakeParent(path string) error {
	_, dest, err := stager.Dest(path)
	if err != nil {
		return err
	}
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

// Under reports whether the physical path lies at or under root and returns
// the rest after root's slash ("" for root itself). Both must be PhysicalDir
// results, which spell every name as the disk holds it, so they are compared
// byte for byte: in a case-sensitive directory HOME is not home. A path
// that is text someone wrote (a link target, an argument) is compared with
// syspath instead. This relies on GetFinalPathNameByHandle giving the disk's
// spelling, as local NTFS does; on a share or file system that does not, a
// link inside the root whose spelling differs in case counts as leading out,
// and the walk does not go through it.
func Under(physical, root string) (rest string, inside bool) {
	if physical == root {
		return "", true
	}
	rest, inside = strings.CutPrefix(physical, strings.TrimSuffix(root, "/")+"/")
	if !inside {
		return "", false
	}
	return rest, true
}

// holds reports whether the physical directory lies at or under the
// staging directory. Physical is compared first; when it does not match, or
// is empty, the staging directory is resolved again, so a stager whose
// Physical was given unresolved still stages; physical and the resolved
// staging directory are PhysicalDir results, compared through Under.
func (stager Stager) holds(physical string) bool {
	under := func(root string) bool {
		_, inside := Under(physical, root)
		return root != "" && inside
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
	_, dest, err := stager.Dest(path)
	if err != nil {
		return "", 0, err
	}
	collision := &fs.PathError{Op: "stage", Path: syspath.ToNative(path), Err: caseCollision{}}
	if err := stager.makeDir(parentOf(dest)); err != nil {
		if stager.takenByAnother(parentOf(dest)) {
			return "", 0, collision
		}
		return "", 0, err
	}
	source, err := stager.Opener.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	if stager.afterOpen != nil {
		stager.afterOpen(path)
	}
	info, err := source.Stat()
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: notRegular{}}
	}
	target, err := stager.createCopy(handleIdentity(source, info), dest)
	if errors.As(err, &caseCollision{}) || (err != nil && stager.takenByAnother(dest)) {
		return "", 0, collision
	}
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

// caseCollision is the reason a file or link is not staged: its staged path
// is already another entry's file, link or directory, whose name differs
// from its own only in letter case. A case-sensitive source directory can
// hold Alice and alice; a staging directory that folds case cannot, and
// staging one over the other would leave a row pointing at the other
// entry's bytes. Whichever was staged first keeps the place, a directory
// made for a later path included.
type caseCollision struct{}

// IsCaseCollision reports whether err says a file or link was not staged
// because its staged place holds another entry's copy (caseCollision), the
// one link failure every platform records in the link's row.
func IsCaseCollision(err error) bool {
	return errors.As(err, &caseCollision{})
}

// Error returns the reason.
func (caseCollision) Error() string {
	return "its staged path is already another entry's whose name differs only in case; " +
		"not staged"
}

// Symlink recreates the symlink path, with its stored target, at its staged
// path. A staged path that already exists is left as it is: the same entry
// was staged there before, and a link made onto a link to a directory would
// land inside that directory, on the source. Where staged places are checked
// (checksPlaces), a path that holds another entry's copy or link is refused
// with caseCollision. No link is created in a directory that does not
// resolve inside the staging directory. directory makes a Windows directory
// link, as a junction or directory symlink is recreated; Unix ignores it.
func (stager Stager) Symlink(path, target string, directory bool) error {
	_, dest, err := stager.Dest(path)
	if err != nil {
		return err
	}
	collision := &fs.PathError{Op: "stage", Path: syspath.ToNative(path), Err: caseCollision{}}
	if err := stager.requireInside("symlink", parentOf(dest)); err != nil {
		if stager.takenByAnother(parentOf(dest)) {
			return collision
		}
		return err
	}
	if _, err := osfile.Lstat(dest); err == nil {
		if stager.checksPlaces() &&
			!stager.mayFill(linkIdentity(path), dest, stager.placeOf(dest), true) {
			return collision
		}
		return nil
	}
	if err := createLink(target, dest, directory); err != nil {
		return err
	}
	if stager.checksPlaces() {
		stager.mayFill(linkIdentity(path), dest, stager.placeOf(dest), false)
	}
	return nil
}

// takenByAnother reports whether path, a staged path the stager could not
// make or open, is, where staged places are checked, another entry's place
// (stagedSpelling.collides): the component the failure is about is spelled
// otherwise on disk, a link, file or directory of another entry, which a
// failed open or mkdir would word as a link loop or a file in the way.
func (stager Stager) takenByAnother(path string) bool {
	if !stager.checksPlaces() {
		return false
	}
	return stager.spelling(path).collides()
}

// placeOf returns the place dest, a staged path that exists, resolves to,
// as the staging directory spells it (Stager.spelling), or dest itself.
func (stager Stager) placeOf(dest string) string {
	if spelling := stager.spelling(dest); spelling.stop == stagedEnd && spelling.respelled {
		return spelling.path
	}
	return dest
}

// RemoveFailedCopy removes what a failed copy left at dest and reports true,
// or reports true when it left nothing, or when what is at dest is an entry
// staged under another spelling (Stager.spelling), which it leaves. It
// reports false, removing nothing,
// when dest's directory does not resolve inside the physical staging
// directory: the staged path then leads through a recreated link out of the
// staging tree, and a file there may be a source.
func (stager Stager) RemoveFailedCopy(dest string) bool {
	// The lstat is copy_run's test of whether the copy left anything; it
	// lists nothing. The staged path is respelled only once its directory is
	// known to lie inside the staging directory.
	info, err := osfile.Lstat(dest)
	if err != nil {
		return true
	}
	physical, err := PhysicalDir(parentOf(dest))
	if stager.Physical == "" || err != nil || !stager.holds(physical) {
		return false
	}
	if stager.spelling(dest).respelled {
		return true
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

// FoldsCase reports whether dir, a staging directory, takes two names that
// differ only in letter case for one: it creates a probe file with a random
// lower-case name, looks it up in upper case and removes it. The run asks
// once; where it folds, every copy and link is checked against where it
// landed before anything is written into it. A probe that cannot be made
// counts as folding, the side that checks.
func FoldsCase(dir string) bool {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	name := ".case-probe-" + hex.EncodeToString(random)
	probe, err := osfile.OpenFile(dir+"/"+name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return true
	}
	_ = probe.Close()
	_, err = osfile.Lstat(dir + "/" + strings.ToUpper(name))
	_ = osfile.Remove(dir + "/" + name)
	return err == nil
}

// Places records, for one run, the source entry each staged copy and link
// was made from, keyed by its staged path as the staging directory spells
// it, so the stager can tell an entry staged again (one source file reached
// through two spellings on a volume that folds case) from another entry
// whose staged path the directory resolves to the same place. The Stager
// values a run derives (Through) share one; a run stages from one
// goroutine, so it takes no lock.
type Places struct {
	sources map[string][2]uint64
}

// NewPlaces returns an empty record of staged places.
func NewPlaces() *Places {
	return &Places{sources: map[string][2]uint64{}}
}

// fill reports whether the entry of identity may be staged at place, and
// records it: yes when nothing was there before (existed false) or what is
// there was staged from the same non-zero identity; no for another entry's,
// or for anything not recorded.
func (places *Places) fill(place string, existed bool, identity [2]uint64) bool {
	recorded, found := places.sources[place]
	if existed && (!found || recorded != identity || identity == [2]uint64{}) {
		return false
	}
	places.sources[place] = identity
	return true
}

// mayFill reports whether the entry of identity (device and inode, or
// volume serial number and file index) may be staged at dest, which the
// staging directory resolved to place (dest itself, or another entry's
// spelling); existed says whether something was there before. Without
// Places, only dest's own place may be written over.
func (stager Stager) mayFill(identity [2]uint64, dest, place string, existed bool) bool {
	if stager.Places == nil {
		return !existed || place == dest
	}
	return stager.Places.fill(place, existed, identity)
}

// linkIdentity returns the identity of the link path itself from its lstat,
// or zero. Symlink is handed no lstat of its own, so a link swapped after
// the walk's lstat is judged by what is there now.
func linkIdentity(path string) [2]uint64 {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return [2]uint64{}
	}
	return [2]uint64{meta.Dev, meta.Ino}
}
