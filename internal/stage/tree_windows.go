package stage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// heldDirectory is what a tree holds open: a handle on the staging
// directory shared for reading and writing but not deletion (holdShare), so
// while it is open Windows refuses every process a rename or deletion of
// the directory; and the security attributes CreateTree made it with, which
// the owner probe uses too (nil for a tree OpenTree opened, or on a volume
// keeping no access lists).
type heldDirectory struct {
	handle     windows.Handle
	attributes *windows.SecurityAttributes
	// physical is the path the held handle gave when the tree was opened.
	physical string
	// closed is set when the tree is closed, and shared with the trees
	// OpenDir returns, which hold nothing of their own; nil is closed.
	closed *bool
	// subtree is set on a tree OpenDir returned, whose Close leaves the
	// tree it came from open.
	subtree bool
}

// shut returns the error of operation on name once the tree is closed, a
// *fs.PathError wrapping fs.ErrClosed, and nil while it is open, so a
// closed tree does nothing by path.
func (tree *Tree) shut(operation, name string) error {
	if tree.held.closed == nil || *tree.held.closed {
		return &fs.PathError{Op: operation, Path: syspath.ToNative(tree.full(name)),
			Err: fs.ErrClosed}
	}
	return nil
}

// holdAccess is the access the held handle asks for: listing, attributes
// and the security descriptor, which the checks of CreateTree read.
// Listing is FILE_LIST_DIRECTORY, the bit of FILE_READ_DATA, so a later open
// must share reading, as the run's own opens do; access to attributes is
// not affected by share modes (CreateFileW, dwShareMode).
// TestTheRunsOwnOpensOfTheHeldDirectoryStillWork shows the run's later
// opens, and its creations and removals inside, succeed while it is held.
const holdAccess = windows.FILE_LIST_DIRECTORY | windows.FILE_READ_ATTRIBUTES |
	windows.READ_CONTROL | windows.SYNCHRONIZE

// holdShare lets later opens of the directory read and write it, as the
// run's listings, its case mark and PhysicalDir do, and leaves out
// FILE_SHARE_DELETE, so no process can open the directory asking for
// delete access, which allows both deletion and renaming (CreateFileW,
// dwShareMode).
const holdShare = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE

// makeDirectory creates the directory path, new, with attributes, or with
// none (os.Mkdir's plain creation) when attributes is nil.
func makeDirectory(path string, attributes *windows.SecurityAttributes) error {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return err
	}
	return windows.CreateDirectory(name, attributes)
}

// keepsAccessLists reports whether the volume of handle preserves and
// enforces access lists: GetVolumeInformationByHandle answers and its flags
// hold FILE_PERSISTENT_ACLS. Microsoft's page for the call lists SMB 3.0
// shares as not supporting it; a volume that does not answer counts as
// keeping none.
func keepsAccessLists(handle windows.Handle) bool {
	var flags uint32
	err := windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, &flags, nil, 0)
	return err == nil && flags&windows.FILE_PERSISTENT_ACLS != 0
}

// volumeKeepsAccessLists reports whether the volume of the directory dir,
// the output directory, keeps access lists (keepsAccessLists of a handle
// opened on it for its attributes alone).
func volumeKeepsAccessLists(dir string) bool {
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	return keepsAccessLists(handle)
}

// ownAccess returns the security attributes the staging directory is made
// with where the volume keeps access lists: the running account as owner,
// and a protected access list (inheriting nothing from the output
// directory) giving that account and the Administrators group full control,
// inherited by everything made inside. Such a volume is local, where that
// owner is allowed.
func ownAccess() (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	account := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + account +
		"D:P(A;OICI;FA;;;" + account + ")(A;OICI;FA;;;BA)")
	if err != nil {
		return nil, err
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	return attributes, nil
}

// handleOwner returns the owner of the object handle names, or nil when it
// cannot be read.
func handleOwner(handle windows.Handle) *windows.SID {
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return nil
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return nil
	}
	return owner
}

// probeOwner creates a probe file in the held directory, new and with the
// tree's security attributes, and returns the owner its handle reports;
// the file is deleted when the handle closes (FILE_FLAG_DELETE_ON_CLOSE).
func (tree *Tree) probeOwner() (*windows.SID, error) {
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	name, err := windows.UTF16PtrFromString(syspath.Extended(
		tree.full(".owner-probe-" + hex.EncodeToString(random))))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL|windows.DELETE, 0,
		tree.held.attributes, windows.CREATE_NEW,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	return handleOwner(handle), nil
}

// ownerAccepted reports whether a directory whose owner is directory may
// be the one this process just made, given created, the owner of a probe
// it just made inside: always on a volume that keeps no access lists, else
// only when both are known and the same.
func ownerAccepted(directory, created *windows.SID, keepsAccessLists bool) bool {
	return !keepsAccessLists || directory != nil && created != nil && directory.Equals(created)
}

// stillNamed reports whether the tree's name, unfollowed, holds the
// directory the tree has open, by volume serial number and file index.
func (tree *Tree) stillNamed() bool {
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(tree.held.handle, &data); err != nil {
		return false
	}
	named, err := fsmeta.Lstat(tree.path)
	return err == nil && !named.IsLink() && named.Kind == fsmeta.Dir &&
		named.Dev == uint64(data.VolumeSerialNumber) &&
		named.Ino == uint64(data.FileIndexHigh)<<32|uint64(data.FileIndexLow)
}

// ReadDir lists the directory name in the tree in directory order.
func (tree *Tree) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := tree.shut("open", name); err != nil {
		return nil, err
	}
	return osfile.ReadDir(tree.full(name))
}

// madeByThisRun reports whether the held directory is what the tree's name
// holds, unfollowed, is empty, and, on a volume that keeps access lists,
// has the owner the volume gives a probe file this process creates in it
// now (probeOwner, ownerAccepted). The listing comes first, so a probe a
// scanner still holds open cannot make the directory look taken.
func (tree *Tree) madeByThisRun() bool {
	if !tree.stillNamed() {
		return false
	}
	if entries, err := tree.ReadDir("."); err != nil || len(entries) != 0 {
		return false
	}
	keeps := keepsAccessLists(tree.held.handle)
	var directory, created *windows.SID
	if keeps {
		directory = handleOwner(tree.held.handle)
		var err error
		if created, err = tree.probeOwner(); err != nil {
			return false
		}
	}
	return ownerAccepted(directory, created, keeps)
}

// openedPath returns the path Windows gives for the held handle
// (GetFinalPathNameByHandle), in canonical form, or "" when it cannot be
// read.
func (tree *Tree) openedPath() string {
	final, err := finalPath(tree.held.handle)
	if err != nil {
		return ""
	}
	return syspath.FromNative(final)
}

// Close releases the hold, after which every method fails with
// fs.ErrClosed, and the directory can be renamed and removed. A second
// Close does nothing; a tree OpenDir returned holds nothing to release.
func (tree *Tree) Close() error {
	if tree.held.subtree {
		return nil
	}
	if tree.held.closed != nil {
		*tree.held.closed = true
	}
	handle := tree.held.handle
	if handle == 0 || handle == windows.InvalidHandle {
		return nil
	}
	tree.held.handle = windows.InvalidHandle
	return windows.CloseHandle(handle)
}

// OpenTree opens the existing staging directory at path, never following
// a link at its last component, holds it (holdShare), then takes its
// physical path from the held handle (openedPath), never from the name:
// that order, the hold first, is the guarantee that no name decides it.
func OpenTree(path string) (*Tree, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	handle, err := windows.CreateFile(name, holdAccess, holdShare, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	var data windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(handle, &data)
	if err == nil && (data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0) {
		err = errors.New("not a directory")
	}
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	tree := &Tree{path: path, held: heldDirectory{handle: handle, closed: new(bool)}}
	if tree.held.physical = tree.openedPath(); tree.held.physical == "" {
		_ = tree.Close()
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path),
			Err: errors.New("the path of the opened directory cannot be read")}
	}
	return tree, nil
}

// CreateTree makes the staging directory at path, new, and opens it: with
// its own access list (ownAccess) when the output directory's volume keeps
// access lists, else plainly, taking what the output directory passes on.
// The directory opened must be the one just made (madeByThisRun), so one
// swapped in between is refused. When the open or that check fails, the
// directory made is left at its name, which may no longer be the run's,
// and the error says so (madeButRefused).
func CreateTree(path string) (*Tree, error) {
	var attributes *windows.SecurityAttributes
	var err error
	if volumeKeepsAccessLists(parentOf(path)) {
		attributes, err = ownAccess()
	}
	if err == nil {
		err = makeDirectory(path, attributes)
	}
	if err != nil {
		return nil, &fs.PathError{Op: "mkdir", Path: syspath.ToNative(path), Err: err}
	}
	tree, err := OpenTree(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path),
			Err: madeButRefused{reason: innerError(err)}}
	}
	tree.held.attributes = attributes
	if !tree.madeByThisRun() {
		_ = tree.Close()
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path),
			Err: madeButRefused{reason: notOwnDirectory{}}}
	}
	return tree, nil
}

// OpenDir returns the directory name in the tree as a tree of its own, so
// a walk down the tree can name each entry by its directory. The names stay
// paths under the held staging directory; nothing more is held.
func (tree *Tree) OpenDir(name string) (*Tree, error) {
	if err := tree.shut("open", name); err != nil {
		return nil, err
	}
	return &Tree{path: tree.full(name),
		held: heldDirectory{closed: tree.held.closed, subtree: true}}, nil
}

// releaseParent does nothing: a Windows tree holds no parent directory,
// since its names go by path under the held staging directory.
func (tree *Tree) releaseParent() {}

// MkdirAll creates the directory name in the tree and its missing parents,
// as mkdir -p does.
func (tree *Tree) MkdirAll(name string) error {
	if err := tree.shut("mkdir", name); err != nil {
		return err
	}
	return osfile.MkdirAll(tree.full(name), 0o777)
}

// openFile opens name in the tree with flag and perm; with O_CREATE and
// O_EXCL Windows creates it with CREATE_NEW and FILE_FLAG_OPEN_REPARSE_POINT
// (syscall/syscall_windows.go), so a final link is never followed.
func (tree *Tree) openFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if err := tree.shut("open", name); err != nil {
		return nil, err
	}
	return osfile.OpenFile(tree.full(name), flag, perm)
}

// Open opens the file name in the tree for reading.
func (tree *Tree) Open(name string) (*os.File, error) {
	if err := tree.shut("open", name); err != nil {
		return nil, err
	}
	return osfile.Open(tree.full(name))
}

// Lstat describes name in the tree itself, a link included.
func (tree *Tree) Lstat(name string) (fs.FileInfo, error) {
	if err := tree.shut("lstat", name); err != nil {
		return nil, err
	}
	return osfile.Lstat(tree.full(name))
}

// meta is Lstat as the identity and kind the stager compares: volume
// serial number, file index and whether name is a link.
func (tree *Tree) meta(name string) (fsmeta.Meta, error) {
	if err := tree.shut("lstat", name); err != nil {
		return fsmeta.Meta{}, err
	}
	return fsmeta.Lstat(tree.full(name))
}

// Readlink returns the stored target of the link name in the tree.
func (tree *Tree) Readlink(name string) (string, error) {
	if err := tree.shut("readlink", name); err != nil {
		return "", err
	}
	return osfile.Readlink(tree.full(name))
}

// Remove removes the file, link or empty directory name in the tree.
func (tree *Tree) Remove(name string) error {
	if err := tree.shut("remove", name); err != nil {
		return err
	}
	return osfile.Remove(tree.full(name))
}

// removeAll removes name in the tree and everything under it.
func (tree *Tree) removeAll(name string) error {
	if err := tree.shut("remove", name); err != nil {
		return err
	}
	return osfile.RemoveAll(tree.full(name))
}

// Symlink makes name in the tree a link to target, stored as given, a
// directory link when directory is set (createLink).
func (tree *Tree) Symlink(target, name string, directory bool) error {
	if err := tree.shut("symlink", name); err != nil {
		return err
	}
	return createLink(target, tree.full(name), directory)
}

// holds reports whether physical, a PhysicalDir result, lies at or under
// the staging directory as the held handle places it, when opened or now
// (openedPath), never by its name; the paths are compared through Under.
func (tree *Tree) holds(physical string) bool {
	for _, root := range []string{tree.held.physical, tree.openedPath()} {
		if _, inside := Under(physical, root); root != "" && inside {
			return true
		}
	}
	return false
}

// resolvesInside reports whether name, every link and junction on its way
// followed, resolves to a directory under the physical staging directory.
func (tree *Tree) resolvesInside(name string) bool {
	if tree.shut("stat", name) != nil {
		return false
	}
	physical, err := PhysicalDir(tree.full(name))
	return err == nil && tree.holds(physical)
}

// lookDir reports what is at name in the tree, for findInside: a
// directory reached, every link followed, under the physical staging
// directory, something else, or nothing; err is a failure of another kind.
func (tree *Tree) lookDir(name string) (dirFound, error) {
	_, err := tree.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dirMissing, nil
	case err != nil:
		return dirMissing, err
	case tree.resolvesInside(name):
		return dirInside, nil
	}
	return dirOutside, nil
}
