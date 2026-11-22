package stage

import (
	"errors"
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
	cases := []struct {
		name  string
		owner *windows.SID
		keeps bool
		want  bool
	}{
		{"on a volume keeping access lists the running account's directory is taken",
			user.User.Sid, true, true},
		{"on a volume keeping access lists another account's directory is refused", other, true, false},
		{"on a volume keeping access lists a directory with no owner is refused", nil, true, false},
		{"on a volume keeping none another owner is taken", other, false, true},
		{"on a volume keeping none no owner is taken", nil, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := ownerAccepted(test.owner, user.User.Sid, test.keeps); got != test.want {
				t.Errorf("ownerAccepted = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTheTemporaryDirectorysVolumeKeepsAccessLists(t *testing.T) {
	tree, _ := newTree(t)
	if !tree.keepsAccessLists() {
		t.Error("the NTFS volume of the test's temporary directory keeps no access lists, says the tree")
	}
}
