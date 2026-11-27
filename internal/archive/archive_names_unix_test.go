//go:build unix

package archive

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

// TestMemberHeaderLooksUpNoOwnerOrGroupName builds the header of a file the
// running account owns, whose name the host can give: a header built with a
// lookup would hold that name, so empty names show that none was made.
func TestMemberHeaderLooksUpNoOwnerOrGroupName(t *testing.T) {
	owner, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil || owner.Username == "" {
		t.Skipf("the host gives no name for user %d: %v", os.Getuid(), err)
	}
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	header, err := memberHeader(info, "", "f")
	if err != nil {
		t.Fatal(err)
	}
	if header.Uname != "" || header.Gname != "" || header.Uid != os.Getuid() || header.Gid != os.Getgid() {
		t.Errorf("header owner = %q:%q (%d:%d), want no names and %d:%d", header.Uname,
			header.Gname, header.Uid, header.Gid, os.Getuid(), os.Getgid())
	}
}
