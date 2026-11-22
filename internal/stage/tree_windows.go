package stage

import (
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
// directory that lets other opens read and write it but not delete it
// (holdShare), so while it is open Windows refuses to rename or delete the
// directory, for this process and every other.
type heldDirectory struct {
	handle windows.Handle
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

// CreateTree makes the staging directory at path, new, with a protected
// access list of its own (ownAccess), and opens it. The directory opened
// must then be the one just made (madeByThisRun), so a directory swapped
// in between the two calls is refused. When the open or that check fails,
// the directory made is left at its name, which may no longer be the
// run's, and the error says so (madeButRefused).
func CreateTree(path string) (*Tree, error) {
	attributes, err := ownAccess()
	if err != nil {
		return nil, &fs.PathError{Op: "mkdir", Path: syspath.ToNative(path), Err: err}
	}
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err == nil {
		err = windows.CreateDirectory(name, attributes)
	}
	if err != nil {
		return nil, &fs.PathError{Op: "mkdir", Path: syspath.ToNative(path), Err: err}
	}
	tree, err := OpenTree(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path),
			Err: madeButRefused{reason: innerError(err)}}
	}
	if !tree.madeByThisRun() {
		_ = tree.Close()
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path),
			Err: madeButRefused{reason: swappedDirectory{}}}
	}
	return tree, nil
}

// ownAccess returns the security attributes the staging directory is made
// with: owned by the account the process runs as, and a protected access
// list (one that takes nothing from the output directory) giving that
// account and the built-in Administrators group full control, inherited by
// every file and directory made inside.
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

// madeByThisRun reports whether the held directory is what the tree's name
// holds, unfollowed, holds nothing, and, on a volume that keeps access
// lists (keepsAccessLists), is owned by the account the process runs as
// (ownerAccepted). FAT and exFAT keep no security descriptor, so there no
// owner can tell this run's directory from another's and none is compared.
func (tree *Tree) madeByThisRun() bool {
	if !tree.stillNamed() {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false
	}
	var owner *windows.SID
	keeps := tree.keepsAccessLists()
	if keeps {
		descriptor, err := windows.GetSecurityInfo(tree.held.handle, windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION)
		if err == nil {
			owner, _, _ = descriptor.Owner()
		}
	}
	if !ownerAccepted(owner, user.User.Sid, keeps) {
		return false
	}
	entries, err := tree.ReadDir(".")
	return err == nil && len(entries) == 0
}

// keepsAccessLists reports whether the held directory's volume preserves
// and enforces access lists (FILE_PERSISTENT_ACLS from
// GetVolumeInformationByHandle). A volume that does not say, such as an SMB
// share, which does not answer the call, counts as keeping none.
func (tree *Tree) keepsAccessLists() bool {
	var flags uint32
	err := windows.GetVolumeInformationByHandle(tree.held.handle, nil, 0, nil, nil, &flags, nil, 0)
	return err == nil && flags&windows.FILE_PERSISTENT_ACLS != 0
}

// ownerAccepted reports whether a directory whose owner is owner may be the
// one this process just made as user: always on a volume that keeps no
// access lists, else only when owner is user.
func ownerAccepted(owner, user *windows.SID, keepsAccessLists bool) bool {
	return !keepsAccessLists || owner != nil && owner.Equals(user)
}

// OpenTree opens the existing staging directory at path, never following
// a link at its last component, holds it (holdShare), and resolves its
// physical path.
func OpenTree(path string) (*Tree, error) {
	physical, err := PhysicalDir(path)
	if err != nil {
		return nil, err
	}
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
	return &Tree{path: path, physical: physical, held: heldDirectory{handle: handle}}, nil
}

// OpenDir returns the directory name in the tree as a tree of its own, so
// a walk down the tree can name each entry by its directory. The names stay
// paths under the held staging directory; nothing more is held.
func (tree *Tree) OpenDir(name string) (*Tree, error) {
	return &Tree{path: tree.full(name)}, nil
}

// Close releases the hold; the directory can be renamed and removed
// afterwards. A second Close does nothing.
func (tree *Tree) Close() error {
	handle := tree.held.handle
	if handle == 0 || handle == windows.InvalidHandle {
		return nil
	}
	tree.held.handle = windows.InvalidHandle
	return windows.CloseHandle(handle)
}

// releaseParent does nothing: a Windows tree holds no parent directory,
// since its names go by path under the held staging directory.
func (tree *Tree) releaseParent() {}

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

// MkdirAll creates the directory name in the tree and its missing parents,
// as mkdir -p does.
func (tree *Tree) MkdirAll(name string) error {
	return osfile.MkdirAll(tree.full(name), 0o777)
}

// openFile opens name in the tree with flag and perm; with O_CREATE and
// O_EXCL Windows creates it with CREATE_NEW and FILE_FLAG_OPEN_REPARSE_POINT
// (syscall/syscall_windows.go), so a final link is never followed.
func (tree *Tree) openFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return osfile.OpenFile(tree.full(name), flag, perm)
}

// Open opens the file name in the tree for reading.
func (tree *Tree) Open(name string) (*os.File, error) {
	return osfile.Open(tree.full(name))
}

// Lstat describes name in the tree itself, a link included.
func (tree *Tree) Lstat(name string) (fs.FileInfo, error) {
	return osfile.Lstat(tree.full(name))
}

// meta is Lstat as the identity and kind the stager compares: volume
// serial number, file index and whether name is a link.
func (tree *Tree) meta(name string) (fsmeta.Meta, error) {
	return fsmeta.Lstat(tree.full(name))
}

// ReadDir lists the directory name in the tree in directory order.
func (tree *Tree) ReadDir(name string) ([]fs.DirEntry, error) {
	return osfile.ReadDir(tree.full(name))
}

// Readlink returns the stored target of the link name in the tree.
func (tree *Tree) Readlink(name string) (string, error) {
	return osfile.Readlink(tree.full(name))
}

// Remove removes the file, link or empty directory name in the tree.
func (tree *Tree) Remove(name string) error {
	return osfile.Remove(tree.full(name))
}

// removeAll removes name in the tree and everything under it.
func (tree *Tree) removeAll(name string) error {
	return osfile.RemoveAll(tree.full(name))
}

// Symlink makes name in the tree a link to target, stored as given, a
// directory link when directory is set (createLink).
func (tree *Tree) Symlink(target, name string, directory bool) error {
	return createLink(target, tree.full(name), directory)
}

// resolvesInside reports whether name, every link and junction on its way
// followed, resolves to a directory under the physical staging directory.
func (tree *Tree) resolvesInside(name string) bool {
	physical, err := PhysicalDir(tree.full(name))
	return err == nil && tree.holds(physical)
}

// lookDir reports what is at name in the tree, for requireInside: a
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

// holds reports whether the physical directory lies at or under the
// staging directory. The tree's physical path is compared first; when it
// does not match, or is empty, the staging directory is resolved again, so
// a tree whose physical path was given unresolved still stages; physical
// and the resolved staging directory are PhysicalDir results, compared
// through Under.
func (tree *Tree) holds(physical string) bool {
	under := func(root string) bool {
		_, inside := Under(physical, root)
		return root != "" && inside
	}
	if under(tree.physical) {
		return true
	}
	root := tree.physical
	if root == "" {
		root = tree.path
	}
	resolved, err := PhysicalDir(root)
	return err == nil && under(resolved)
}
