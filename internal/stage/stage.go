package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Stager says where files are staged. Dir is the staging directory's name,
// for messages and staged paths, Tree the open directory every staged write
// goes through, and Root the image root ("" on a live host). From and Into,
// when set, are an in-base symlinked directory a path ran through and that
// directory's physical path in the base: a path at or
// under From is staged under Into, so nothing is staged through a link
// recreated in the staging tree. Opener opens each source, as the run
// decided (whether to ask Windows to hold its access time); nil asks
// wherever the platform can.
type Stager struct {
	Dir    string
	Tree   *Tree
	Root   string
	From   string
	Into   string
	Opener *fsmeta.Opener
	// FoldsCase is set where the staging directory takes two names for one
	// (FoldsCase): a member's spelling is then read from a walk of the tree
	// where the system gives no path for it. Safety does not depend on it.
	FoldsCase bool
	// Places is the run's one record of what it staged (Places).
	Places *Places
	// afterOpen, set by tests, is called with a source's path right after
	// Copy opens it.
	afterOpen func(path string)
	// stagedFS, set by tests, replaces the file system walkStaged asks.
	stagedFS *stagedFS
	// wrapWriter, set by tests, wraps the staged copy Copy writes to.
	wrapWriter func(target io.Writer) io.Writer
}

// Through returns a copy of the stager that stages paths under from at into.
func (stager Stager) Through(from, into string) Stager {
	stager.From, stager.Into = from, into
	return stager
}

// Dest returns the archive member of path, fs/ and the path without the
// root, and where it is staged. The root is removed only as a leading
// string (ignoring ASCII case on Windows), and a path not starting with it
// keeps every byte after fs. From is matched byte for
// byte, or in another ASCII case where fsmeta.SameEntry calls the two one
// entry. A drive or UNC path left is archived by its drive or server (C:/a
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
	return slices.ContainsFunc(strings.Split(path, "/"), func(part string) bool {
		return part == "." || part == ".."
	})
}

// dottedPath is the reason Dest refuses a path with a `.` or `..` component.
type dottedPath struct{}

// Error returns the reason.
func (dottedPath) Error() string {
	return "holds a . or .. component, so where it is staged is not known"
}

// MakeParent creates the directory path is staged in, with its parents, as
// mkdir -p does. It is made for every file given, skipped files included,
// so the archive holds those directories too.
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
	exists, err := stager.findInside("mkdir", dir)
	if err != nil || exists {
		return err
	}
	return stager.Tree.MkdirAll(stager.name(dir))
}

// name returns the staged path dest, a path Dest built, relative to the
// staging directory: "." for the directory itself.
func (stager Stager) name(dest string) string {
	if dest == stager.Dir {
		return "."
	}
	return strings.TrimPrefix(dest, stager.Dir+"/")
}

// requireInside returns an *fs.PathError of operation on dir unless dir, or
// its nearest ancestor that exists when dir does not, resolves with every
// symlink to a directory inside the staging directory (Tree.lookDir); an
// error other than a missing path is returned as it is. Every write into
// the staging tree passes it first: a link recreated there can lead to the
// source, and a write through it would change the evidence.
func (stager Stager) requireInside(operation, dir string) error {
	_, err := stager.findInside(operation, dir)
	return err
}

// findInside is requireInside, also reporting whether dir itself is the
// directory found, so nothing is left to make.
func (stager Stager) findInside(operation, dir string) (bool, error) {
	outside := &fs.PathError{Op: operation, Path: syspath.ToNative(dir), Err: outsideStage{}}
	existing := dir
	for {
		if existing != stager.Dir && !strings.HasPrefix(existing, stager.Dir+"/") {
			return false, outside
		}
		found, err := stager.Tree.lookDir(stager.name(existing))
		switch {
		case err != nil:
			return false, err
		case found == dirInside:
			return existing == dir, nil
		case found == dirOutside || existing == stager.Dir:
			return false, outside
		}
		existing = parentOf(existing)
	}
}

// dirFound is what Tree.lookDir found at a name in the tree.
type dirFound int

const (
	// dirMissing: nothing is at the name, a dangling link excepted.
	dirMissing dirFound = iota
	// dirInside: the name resolves to a directory inside the tree.
	dirInside
	// dirOutside: something is at the name that does not resolve to a
	// directory inside the tree.
	dirOutside
)

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

// outsideStage is the reason a write into the staging tree is refused: the
// directory written in does not resolve inside the staging directory.
type outsideStage struct{}

// Error returns the reason.
func (outsideStage) Error() string { return "does not resolve inside the staging directory" }

// Copied is what Copy staged: the SHA-256 of the bytes written, in
// lower-case hex, their count, and the archive member that holds them,
// spelled as the staging tree names it.
type Copied struct {
	SHA256      string
	Size        int64
	ArchivePath string
}

// Copy copies the regular file path to its staged path; the count is the
// size the Windows rows give. Nothing is written
// unless the staged path's directory resolves inside the staging directory
// and neither end is opened through a final link; the copy gets the source's
// times, and on Unix its mode and, as root, its owner, as cp -p gives them.
// The staged file is created exclusively, so no file already in the staging
// tree is ever opened for write (again). A copy that fails removes the file
// it created. The error is the innermost *fs.PathError the copy met.
func (stager Stager) Copy(path string) (Copied, error) {
	_, dest, err := stager.Dest(path)
	if err != nil {
		return Copied{}, err
	}
	if err := stager.makeDir(parentOf(dest)); err != nil {
		if stager.takenByAnother(parentOf(dest)) {
			return Copied{}, collisionError(path)
		}
		return Copied{}, err
	}
	source, err := stager.Opener.Open(path)
	if err != nil {
		return Copied{}, err
	}
	defer source.Close()
	if stager.afterOpen != nil {
		stager.afterOpen(path)
	}
	info, err := source.Stat()
	if err != nil {
		return Copied{}, err
	}
	if !info.Mode().IsRegular() {
		return Copied{}, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: notRegular{}}
	}
	identity := handleIdentity(source, info)
	target, err := createDest(stager.Tree, stager.name(dest))
	if err != nil {
		if stager.exists(dest) {
			return stager.again(path, dest, identity)
		}
		return Copied{}, syspath.NativeErrors(err)
	}
	copied, err := stager.fill(target, source, dest, info)
	if err != nil {
		// The file was created exclusively above, so it is this copy's own.
		_ = stager.Tree.Remove(stager.name(dest))
		return Copied{}, err
	}
	stager.Places.record(target.staged, identity, dest, copied)
	return copied, nil
}

// fill writes the source to the staged file just created, gives it the
// source's times and returns what was staged.
func (stager Stager) fill(target createdFile, source io.Reader, dest string, info fs.FileInfo,
) (Copied, error) {
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(stager.writer(target.file), hash), source)
	if err == nil {
		err = preserve(target.file, stager.Tree, stager.name(dest), info)
	}
	if closeError := target.file.Close(); err == nil {
		err = closeError
	}
	if err != nil {
		// The staged copy was opened through osfile, so its write and close
		// errors name the \\?\ form.
		return Copied{}, syspath.NativeErrors(err)
	}
	return Copied{SHA256: hex.EncodeToString(hash.Sum(nil)), Size: written,
		ArchivePath: stager.member(target.place, dest, target.staged)}, nil
}

// createdFile is a staged file Copy created: the open file, its own
// identity (Places), and its path as the system reports it ("" where it
// does not).
type createdFile struct {
	file   *os.File
	staged [2]uint64
	place  string
}

// writer returns where Copy writes the staged copy: target, or what a test
// wraps it in.
func (stager Stager) writer(target io.Writer) io.Writer {
	if stager.wrapWriter != nil {
		return stager.wrapWriter(target)
	}
	return target
}

// exists reports whether something is at dest, a staged path whose create
// failed, its last component not followed.
func (stager Stager) exists(dest string) bool {
	_, err := stager.files().lstat(dest)
	return err == nil
}

// again decides a copy or link of path whose staged path dest the system
// says is taken, touching nothing: the recorded result when the entry there
// is one this run staged from the same source (identity, non-zero); a plain
// reason when it was staged for this very staged path, its source since
// replaced; the collision error when it was staged for another staged path,
// or when the volume took dest for another spelling's entry (a walk finds a
// component respelled); else a plain reason for what is not known to be this
// source's copy. Every refusal names path, the source.
func (stager Stager) again(path, dest string, identity [2]uint64) (Copied, error) {
	meta, err := stager.files().lstat(dest)
	// An inode or file index of 0 identifies nothing.
	if err == nil && meta.Ino != 0 && identity[1] != 0 {
		if recorded, found := stager.Places.lookup([2]uint64{meta.Dev, meta.Ino}); found {
			switch {
			case recorded.source == identity:
				return recorded.copied, nil
			case recorded.dest == dest:
				// The same staged path, its source replaced since (saved
				// through a rename, or reached again later, perhaps through
				// a link): no name was merged.
				return Copied{}, &fs.PathError{Op: "stage", Path: syspath.ToNative(path),
					Err: changedSince{}}
			}
			return Copied{}, collisionError(path)
		}
	}
	if stager.spelling(dest).collides() {
		return Copied{}, collisionError(path)
	}
	reason := error(notStagedHere{})
	if err == nil && meta.IsLink() {
		reason = notRegular{}
	}
	return Copied{}, &fs.PathError{Op: "stage", Path: syspath.ToNative(path), Err: reason}
}

// notStagedHere is the reason a file or link is not staged when its staged
// path holds something not known to be its copy: a file an earlier run of
// the same name left, or one whose identity could not be read.
type notStagedHere struct{}

// Error returns the reason.
func (notStagedHere) Error() string {
	return "already exists in the staging directory and is not known to be this source's copy"
}

// changedSince is the reason a file or link is not staged when this run
// staged a source at the same staged path before, perhaps through another
// source path, and the source there is now another file.
type changedSince struct{}

// Error returns the reason.
func (changedSince) Error() string {
	return "changed since this run staged it; the earlier row holds its copy"
}

// member returns the archive member of the staged file at dest, which the
// stager created by that name, its identity staged ([2]uint64{} for a
// link), and place the path the system gives for it ("" where it gives
// none). A member's spelling never depends on where the tree is or was:
// the name as created is the member unless the volume respelled a
// component; place is used only for its last components, as many as the
// name has, as a candidate spelling, accepted when it differs from the
// name only if looking it up through the tree, following no link, finds
// the very file created (reaches); otherwise the spelling comes from a
// walk through the tree (spelling), and failing that the name as created.
func (stager Stager) member(place, dest string, staged [2]uint64) string {
	name := stager.name(dest)
	if candidate, found := lastComponents(place, strings.Count(name, "/")+1); found {
		if candidate == name || stager.reaches(candidate, staged) {
			return candidate
		}
	}
	if spelling := stager.spelling(dest); spelling.stop == stagedEnd {
		return strings.TrimPrefix(spelling.path, stager.Dir+"/")
	}
	return name
}

// lastComponents returns the last count slash-separated components of
// path, and whether path has that many.
func lastComponents(path string, count int) (string, bool) {
	if path == "" {
		return "", false
	}
	end := len(path)
	for ; count > 0; count-- {
		slash := strings.LastIndexByte(path[:end], '/')
		if slash < 0 {
			return "", false
		}
		end = slash
	}
	return path[end+1:], true
}

// reaches reports whether candidate, a name in the tree, leads through real
// directories alone, each looked at without following it, to the entry of
// identity staged: one lstat per component, an inode of 0 matching
// nothing.
func (stager Stager) reaches(candidate string, staged [2]uint64) bool {
	if staged[1] == 0 {
		return false
	}
	components := strings.Split(candidate, "/")
	for i := range components {
		meta, err := stager.files().lstat(stager.Dir + "/" + strings.Join(components[:i+1], "/"))
		switch {
		case err != nil || meta.IsLink():
			return false
		case i < len(components)-1 && meta.Kind != fsmeta.Dir:
			return false
		case i == len(components)-1:
			return [2]uint64{meta.Dev, meta.Ino} == staged
		}
	}
	return false
}

// notRegular is the reason a path that stopped being a regular file between
// its lstat and its copy is not copied: reading a FIFO or a device could
// block or read something other than a file's contents.
type notRegular struct{}

// Error returns the reason, in the text of the error it replaces.
func (notRegular) Error() string { return "not a regular file" }

// collision is the reason a file or link is not staged: the output
// directory takes its name for another file's, link's or directory's, by
// whatever rule it has (letter case, Unicode form, an 8.3 short name), which
// the stager does not need to know. Staging one over the other would leave
// a row pointing at the other entry's bytes.
type collision struct{}

// collisionError is collision for path.
func collisionError(path string) error {
	return &fs.PathError{Op: "stage", Path: syspath.ToNative(path), Err: collision{}}
}

// IsCollision reports whether err says a file or link was not staged
// because the output directory takes its name for another entry's
// (collision), the one link failure every platform records in its row.
func IsCollision(err error) bool {
	return errors.As(err, &collision{})
}

// Error returns the reason.
func (collision) Error() string {
	return "the output directory takes its name for another file's, link's or directory's; " +
		"not staged"
}

// Link recreates the symlink path, with its stored target, at its staged
// path, and returns the archive member that holds it, spelled as the
// staging tree names it. When the system says the staged path is taken,
// the entry there is the same source link staged before, which is kept, or
// a collision (again). No link is created in a directory that does not
// resolve inside the staging directory. directory makes a Windows directory
// link, as a junction or directory symlink is recreated; Unix ignores it.
func (stager Stager) Link(path, target string, directory bool) (string, error) {
	_, dest, err := stager.Dest(path)
	if err != nil {
		return "", err
	}
	if err := stager.requireInside("symlink", parentOf(dest)); err != nil {
		if stager.takenByAnother(parentOf(dest)) {
			return "", collisionError(path)
		}
		return "", err
	}
	identity := linkIdentity(path)
	if err := stager.Tree.Symlink(target, stager.name(dest), directory); err != nil {
		if stager.exists(dest) {
			copied, err := stager.again(path, dest, identity)
			return copied.ArchivePath, err
		}
		return "", err
	}
	copied := Copied{ArchivePath: stager.member("", dest, [2]uint64{})}
	if meta, err := stager.files().lstat(dest); err == nil {
		stager.Places.record([2]uint64{meta.Dev, meta.Ino}, identity, dest, copied)
	}
	return copied.ArchivePath, nil
}

// takenByAnother reports whether path, a staged directory the stager could
// not make or check, is another entry's place (stagedSpelling.collides): a
// component the volume takes for another spelling leads to another entry's
// link, file or directory, which a failed mkdir would word as a file in the
// way or a link out of the staging tree.
func (stager Stager) takenByAnother(path string) bool {
	return stager.spelling(path).collides()
}

// FailedCopyInside reports, after a copy to dest failed, whether what is at
// dest lies inside the staging directory: true when nothing is there or its
// directory resolves inside the staging directory, false when the staged
// path leads through a recreated link out of the staging tree, where a
// file may be a source. Copy removes the file it created itself, so
// nothing is removed here; the walk logs the false case.
func (stager Stager) FailedCopyInside(dest string) bool {
	if _, err := stager.Tree.Lstat(stager.name(dest)); err != nil {
		return !leavesTree(err)
	}
	return stager.Tree.resolvesInside(stager.name(parentOf(dest)))
}

// parentOf returns path up to its last slash, as the shell's ${path%/*}
// does, so a path with no slash is its own parent.
func parentOf(path string) string {
	if position := strings.LastIndexByte(path, '/'); position >= 0 {
		return path[:position]
	}
	return path
}

// Places records, for one run, what the stager put in the staging tree:
// for each staged file or link, by its own identity (device and inode, or
// volume serial number and file index, as the file system gives them), the
// identity of its source and what Copy or Link returned. When the system
// refuses to create a staged path because the name is taken, the entry
// there is looked up: the same source staged again keeps the first result;
// anything else is a collision. The record trusts the file system's
// identity (ReFS does not promise its 64-bit file index unique); a zero or
// unreadable identity is refused. The Stager values a run derives share
// one; a run stages from one goroutine, so it takes no lock.
type Places struct {
	staged map[[2]uint64]placed
}

// placed is what one staged file or link was made from and what was
// returned for it.
type placed struct {
	source [2]uint64
	dest   string
	copied Copied
}

// NewPlaces returns an empty record of staged places.
func NewPlaces() *Places {
	return &Places{staged: map[[2]uint64]placed{}}
}

// record notes that the staged entry of identity staged, at the staged path
// dest as Dest names it, was made from the source of identity source, with
// result copied; a nil record notes nothing.
func (places *Places) record(staged, source [2]uint64, dest string, copied Copied) {
	if places != nil {
		places.staged[staged] = placed{source, dest, copied}
	}
}

// lookup returns what the staged entry of identity staged was made from.
func (places *Places) lookup(staged [2]uint64) (placed, bool) {
	if places == nil {
		return placed{}, false
	}
	recorded, found := places.staged[staged]
	return recorded, found
}

// linkIdentity returns the identity of the link path itself from its lstat,
// or zero. Link is handed no lstat of its own, so a link swapped after
// the walk's lstat is judged by what is there now.
func linkIdentity(path string) [2]uint64 {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return [2]uint64{}
	}
	return [2]uint64{meta.Dev, meta.Ino}
}
