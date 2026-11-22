package stage

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// fileAllAccess is FILE_ALL_ACCESS, the right the SDDL letters FA stand for.
const fileAllAccess = 0x1F01FF

// newTree makes a staging tree under a fresh directory and returns it with
// its parent; it is closed when the test ends.
func newTree(t *testing.T) (*Tree, string) {
	t.Helper()
	parent := syspath.FromNative(t.TempDir())
	tree, err := CreateTree(parent + "/stage")
	mustDo(t, err)
	t.Cleanup(func() { _ = tree.Close() })
	return tree, parent
}

func TestTheHeldStagingDirectoryCannotBeRenamedOrDeleted(t *testing.T) {
	tree, parent := newTree(t)
	native := syspath.ToNative(tree.Path())
	if err := os.Rename(native, syspath.ToNative(parent+"/moved")); err == nil {
		t.Error("another open renamed the held staging directory")
	}
	name, err := windows.UTF16PtrFromString(native)
	mustDo(t, err)
	if err := windows.RemoveDirectory(name); err == nil {
		t.Error("another open deleted the held staging directory")
	}
	mustDo(t, tree.Close())
	if err := os.Rename(native, syspath.ToNative(parent+"/moved")); err != nil {
		t.Errorf("rename once the hold is released: %v", err)
	}
}

func TestTheRunsOwnOpensOfTheHeldDirectoryStillWork(t *testing.T) {
	tree, _ := newTree(t)
	native := syspath.ToNative(tree.Path())
	file, err := tree.CreateNew("inside", 0o600)
	mustDo(t, err)
	mustDo(t, file.Close())
	mustDo(t, tree.MkdirAll("fs/a"))
	if entries, err := os.ReadDir(native); err != nil || len(entries) != 2 {
		t.Errorf("listing the held directory = %v, %v; want its two entries", entries, err)
	}
	if _, err := PhysicalDir(tree.Path()); err != nil {
		t.Errorf("PhysicalDir of the held directory: %v", err)
	}
	// A file system without per-directory case may refuse the mark; the
	// hold must never be the reason.
	if err := MarkCaseSensitive(tree.Path()); errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Errorf("MarkCaseSensitive of the held directory: %v", err)
	}
	mustDo(t, tree.Remove("inside"))
}

func TestTheStagingDirectoryHasItsOwnProtectedAccessList(t *testing.T) {
	tree, _ := newTree(t)
	descriptor, err := windows.GetNamedSecurityInfo(syspath.ToNative(tree.Path()),
		windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	mustDo(t, err)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	mustDo(t, err)
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	mustDo(t, err)
	control, _, err := descriptor.Control()
	mustDo(t, err)
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Error("the access list takes entries from the output directory")
	}
	if owner, _, err := descriptor.Owner(); err != nil || !owner.Equals(user.User.Sid) {
		t.Errorf("owner %v, %v; want %v", owner, err, user.User.Sid)
	}
	list, _, err := descriptor.DACL()
	mustDo(t, err)
	if list == nil || list.AceCount != 2 {
		t.Fatalf("access list %+v; want two entries", list)
	}
	want := map[string]bool{user.User.Sid.String(): true, administrators.String(): true}
	for i := range uint32(list.AceCount) {
		var entry *windows.ACCESS_ALLOWED_ACE
		mustDo(t, windows.GetAce(list, i, &entry))
		account := (*windows.SID)(unsafe.Pointer(&entry.SidStart)).String()
		inherit := uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE)
		if entry.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || entry.Header.AceFlags != inherit ||
			entry.Mask != fileAllAccess || !want[account] {
			t.Errorf("entry %d: type %d flags %#x mask %#x for %s", i, entry.Header.AceType,
				entry.Header.AceFlags, entry.Mask, account)
		}
		delete(want, account)
	}
}

func TestDiscardRemovesTheHeldDirectory(t *testing.T) {
	tree, _ := newTree(t)
	mustDo(t, tree.MkdirAll("fs/a"))
	file, err := tree.CreateNew("fs/a/x.", 0o600)
	mustDo(t, err)
	mustDo(t, file.Close())
	mustDo(t, tree.Discard())
	if _, err := os.Lstat(syspath.ToNative(tree.Path())); !os.IsNotExist(err) {
		t.Errorf("the staging directory is left: %v", err)
	}
}

func TestAnOwnerIsComparedOnlyOnAVolumeThatKeepsAccessLists(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	mustDo(t, err)
	other, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	mustDo(t, err)
	self := user.User.Sid
	cases := []struct {
		name               string
		directory, created *windows.SID
		keeps              bool
		want               bool
	}{
		{"the owner a new entry gets is taken", self, self, true, true},
		{"another account's directory is refused", other, self, true, false},
		{"on a share giving every entry its own owner that owner is taken", other, other, true, true},
		{"a directory with no owner is refused", nil, self, true, false},
		{"a probe with no owner refuses the directory", self, nil, true, false},
		{"on a volume keeping none another owner is taken", other, self, false, true},
		{"on a volume keeping none no owner is taken", nil, nil, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := ownerAccepted(test.directory, test.created, test.keeps); got != test.want {
				t.Errorf("ownerAccepted = %v, want %v", got, test.want)
			}
		})
	}
}

// shareDeletion replaces the tree's hold with a handle on the same
// directory that lets it be renamed, standing in for a hold an earlier
// version took and released, so a test can move the directory the tree
// has open.
func shareDeletion(t *testing.T, tree *Tree) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(tree.Path()))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, holdAccess, shareAll, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	mustDo(t, windows.CloseHandle(tree.held.handle))
	tree.held.handle = handle
}

func TestTheTreesPhysicalPathIsTheHeldHandlesPath(t *testing.T) {
	tree, _ := newTree(t)
	final, err := finalPath(tree.held.handle)
	mustDo(t, err)
	if tree.held.physical != syspath.FromNative(final) {
		t.Errorf("physical %q, want the held handle's %q", tree.held.physical, syspath.FromNative(final))
	}
}

func TestNothingResolvesTheNameOnceTheTreeIsOpen(t *testing.T) {
	tree, parent := newTree(t)
	shareDeletion(t, tree)
	elsewhere := syspath.FromNative(t.TempDir())
	mustDo(t, os.Rename(syspath.ToNative(tree.Path()), syspath.ToNative(parent+"/moved")))
	makeJunction(t, elsewhere, tree.Path())
	moved, err := PhysicalDir(parent + "/moved")
	mustDo(t, err)
	target, err := PhysicalDir(elsewhere)
	mustDo(t, err)
	if got := tree.openedPath(); got != moved {
		t.Errorf("openedPath = %q, want %q, where the open directory now is", got, moved)
	}
	// The tree's physical path no longer matches either; only the handle's
	// current path may decide, never the junction at the name.
	tree.held.physical = parent + "/gone"
	if !tree.holds(moved + "/fs") {
		t.Errorf("holds(%s/fs) = false; it is in the directory the tree has open", moved)
	}
	if tree.holds(target + "/fs") {
		t.Errorf("holds(%s/fs) = true; only the junction at the name leads there", target)
	}
	mustDo(t, tree.Close())
	mustDo(t, os.Remove(syspath.ToNative(tree.Path())))
}

func TestMadeByThisRunTakesOnlyTheEmptyDirectoryItMade(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  bool
	}{
		{"the empty directory just made is taken", func(*testing.T, string) {}, true},
		{"a directory holding an entry is refused", func(t *testing.T, path string) {
			mustDo(t, os.WriteFile(syspath.ToNative(path+"/planted"), nil, 0o600))
		}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := syspath.FromNative(t.TempDir()) + "/stage"
			mustDo(t, os.Mkdir(syspath.ToNative(path), 0o700))
			test.setup(t, path)
			if got := mustTree(t, path).madeByThisRun(); got != test.want {
				t.Errorf("madeByThisRun = %v, want %v", got, test.want)
			}
		})
	}
	t.Run("a junction at the name is refused", func(t *testing.T) {
		path := syspath.FromNative(t.TempDir()) + "/stage"
		makeJunction(t, syspath.FromNative(t.TempDir()), path)
		if tree, err := OpenTree(path); err == nil {
			_ = tree.Close()
			t.Error("OpenTree held a junction at the name")
		}
		if tree, err := CreateTree(path); err == nil {
			_ = tree.Close()
			t.Error("CreateTree took a name a junction held")
		}
	})
}

func TestStillNamedIsFalseOnceTheNameHoldsAnotherDirectory(t *testing.T) {
	tree, parent := newTree(t)
	shareDeletion(t, tree)
	mustDo(t, os.Rename(syspath.ToNative(tree.Path()), syspath.ToNative(parent+"/moved")))
	mustDo(t, os.Mkdir(syspath.ToNative(tree.Path()), 0o700))
	if tree.stillNamed() {
		t.Error("stillNamed = true; the name holds another directory")
	}
}

func TestADirectoryWhoseOwnerProbeCannotBeMadeIsRefused(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	mustDo(t, err)
	path := syspath.FromNative(t.TempDir()) + "/stage"
	// The account may list, read and delete the directory but add nothing.
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FRSD;;;" +
		user.User.Sid.String() + ")")
	mustDo(t, err)
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	mustDo(t, err)
	mustDo(t, windows.CreateDirectory(name, attributes))
	if mustTree(t, path).madeByThisRun() {
		t.Error("madeByThisRun took a directory in which the owner probe could not be made")
	}
}

func TestTheTemporaryDirectorysVolumeKeepsAccessLists(t *testing.T) {
	tree, _ := newTree(t)
	if !keepsAccessLists(tree.held.handle) {
		t.Error("the NTFS volume of the test's temporary directory keeps no access lists, says the tree")
	}
}

func TestADirectoryOwnedByAnotherAccountIsRefused(t *testing.T) {
	path := syspath.FromNative(t.TempDir()) + "/stage"
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	mustDo(t, err)
	// Owned by the Administrators group, an owner only an elevated account
	// may set, while the probe is made owned by the account itself.
	descriptor, err := windows.SecurityDescriptorFromString("O:BAD:P(A;OICI;FA;;;" +
		user.User.Sid.String() + ")")
	mustDo(t, err)
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	if err := makeDirectory(path, attributes); errors.Is(err, windows.ERROR_INVALID_OWNER) {
		t.Skip("the account may not make the Administrators group an owner: not elevated")
	} else if err != nil {
		t.Fatal(err)
	}
	tree := mustTree(t, path)
	tree.held.attributes, err = ownAccess()
	mustDo(t, err)
	if tree.madeByThisRun() {
		t.Error("madeByThisRun took a directory whose owner is not the one a new entry gets")
	}
}

func TestATreeOpenDirGaveIsClosedWithTheTreeItCameFrom(t *testing.T) {
	tree, _ := newTree(t)
	mustDo(t, tree.MkdirAll("fs"))
	sub, err := tree.OpenDir("fs")
	mustDo(t, err)
	mustDo(t, sub.Close())
	if _, err := sub.Lstat("."); err != nil {
		t.Errorf("Lstat after the subtree's own Close = %v; the tree it came from is open", err)
	}
	mustDo(t, tree.Close())
	if err := sub.MkdirAll("made"); !errors.Is(err, fs.ErrClosed) {
		t.Errorf("MkdirAll in a subtree of a closed tree = %v; want fs.ErrClosed", err)
	}
	if entries, _ := os.ReadDir(syspath.ToNative(tree.Path() + "/fs")); len(entries) != 0 {
		t.Errorf("the subtree of a closed tree made %v", entries)
	}
}
