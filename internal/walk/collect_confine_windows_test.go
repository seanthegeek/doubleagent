package walk

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// walkFiles calls each with every regular file below dir.
func walkFiles(dir string, each func(path string)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := dir + "/" + entry.Name()
		if entry.IsDir() {
			if err := walkFiles(path, each); err != nil {
				return err
			}
		} else if entry.Type().IsRegular() {
			each(path)
		}
	}
	return nil
}

// fileTimes returns the creation, access, write and change times of path
// (FILE_BASIC_INFO), read through a handle opened for attributes alone.
func fileTimes(t *testing.T, path string) [4]uint64 {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|
		windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	var basic struct {
		times      [4]uint64
		attributes uint32
		_          uint32
	}
	mustDo(t, windows.GetFileInformationByHandleEx(handle, windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))))
	return basic.times
}

// TestTheOpenerRefusesWhatWindowsOpensOutsideTheRoot runs the walk with a
// root check that lets everything through, as a mistake of the check would,
// over a junction out of the image root, so only the opener's confinement
// stands between the analyst's files and the archive.
func TestTheOpenerRefusesWhatWindowsOpensOutsideTheRoot(t *testing.T) {
	root, outside := physicalTemp(t), physicalTemp(t)
	writeFiles(t, root, "Users/alice/notes.txt")
	writeFiles(t, outside, "secret.txt", "dir/inner.txt", "other/x.txt")
	makeJunction(t, outside, root+"/Users/alice/j")
	collector, recorder := windowsCollector(t, root, nil)
	collector.RootCheck = RootCheck{}
	confinement, err := follow.NewConfinement(root, func(path string) {
		_ = recorder.Line("refused " + path)
	})
	mustDo(t, err)
	var heldReports []string
	collector.Stager.Opener = &fsmeta.Opener{Admit: confinement.Admit,
		Report: func(path, reason string) { heldReports = append(heldReports, path) }}
	before := fileTimes(t, outside+"/secret.txt")
	home, ctx := root+"/Users/alice", context.Background()
	for _, path := range []string{home + "/notes.txt", home + "/j/secret.txt",
		home + "/j/secret.txt"} {
		_, err := collector.CollectPath(ctx, "alice", home, "agent", path)
		mustDo(t, err)
	}
	_, err = collector.CollectTree(ctx, "alice", home, "agent", home+"/j/dir")
	mustDo(t, err)
	// A directory reached by CollectPath is tested by DirReadError, whose
	// listing is admitted too.
	_, err = collector.CollectPath(ctx, "alice", home, "agent", home+"/j/other")
	mustDo(t, err)
	if row := recorder.rowOf(t, home+"/notes.txt"); row["status"] != "collected" {
		t.Errorf("the file inside the root was not collected: %v", row)
	}
	for _, row := range recorder.rows(t) {
		path, _ := row["path"].(string)
		if strings.Contains(path, `\j\`) {
			t.Errorf("a row for a path outside the root: %v", row)
		}
	}
	var refused []string
	for _, event := range recorder.written {
		if strings.HasPrefix(event, "log refused ") {
			refused = append(refused, strings.TrimPrefix(event, "log refused "))
		}
	}
	want := []string{home + "/j/secret.txt", home + "/j/dir", home + "/j/other"}
	if !slices.Equal(refused, want) {
		t.Errorf("refused %q, want %q", refused, want)
	}
	staged := false
	mustDo(t, walkFiles(collector.Stager.Dir, func(path string) {
		staged = staged || strings.HasSuffix(path, "secret.txt") || strings.HasSuffix(path, "inner.txt")
	}))
	if staged {
		t.Error("a file outside the root was staged")
	}
	for _, path := range heldReports {
		if strings.Contains(path, "/j/") {
			t.Errorf("the access-time hold was reported for %s, outside the root", path)
		}
	}
	if after := fileTimes(t, outside+"/secret.txt"); after != before {
		t.Errorf("the outside file's times moved: %v, then %v", before, after)
	}
}
