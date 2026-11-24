package collect_test

import (
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// identity is what a manifest row records of its path that the comparison
// replaces with placeholders: the change and birth times, the owner and
// group numbers, and on Windows the owner's name.
type identity struct {
	ctime, btime, uid, gid int64
	owner                  string
}

// checkIdentity fails the test unless a manifest row's change time, birth
// time, owner and group are what the file system reports for its path now,
// lstat'ed without following a link: the image outlives the run and nothing
// in it changes those fields. A row the format leaves without metadata, or
// whose path no lstat describes, must hold zeros. A path the manifest
// cannot spell (a name that is not UTF-8, written with U+FFFD) is skipped.
func (n normaliser) checkIdentity(value jsonvalue.Value, numbers map[string]int64, line string) {
	path := value.Get("path").StringValue()
	if strings.ContainsRune(path, utf8.RuneError) {
		return
	}
	got := identity{ctime: numbers["ctime"], btime: numbers["btime"], uid: numbers["uid"],
		gid: numbers["gid"], owner: value.Get("owner").StringValue()}
	want, exists := pathIdentity(path)
	if !exists || rowHasNoMetadata(value) {
		want = identity{}
	}
	if got.owner == "" {
		// Rows the walk gives no owner (an excluded path, a refused one)
		// leave it empty; a named owner must be the path's.
		want.owner = ""
	}
	if got != want {
		n.test.Errorf("manifest row's ctime, btime, uid, gid and owner %+v are not its path's %+v: %s",
			got, want, line)
	}
}

// rowHasNoMetadata reports whether the row is one the manifest format
// writes without times or owner: a Docker or Podman volume no table line
// matches and a -p project that is not a directory (its own home, with no
// modification time, where a home the walk cannot list keeps its times).
func rowHasNoMetadata(value jsonvalue.Value) bool {
	status := value.Get("status").StringValue()
	mtime, _ := value.Get("mtime").Int()
	switch {
	case status == "skipped_unmatched_volume":
		return true
	case status == "error_read" && mtime == 0 &&
		value.Get("home").StringValue() == value.Get("path").StringValue():
		return true
	}
	return false
}
