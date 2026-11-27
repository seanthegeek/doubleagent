package stage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheVolumesSpellingIsTheMemberWithoutTheWalk(t *testing.T) {
	stager, root := newStager(t)
	mustDo(t, os.MkdirAll(stager.Dir+"/fs/home/User", 0o700))
	if _, err := os.Lstat(stager.Dir + "/fs/home/user"); err != nil {
		t.Skip("the volume of the test's temporary directory tells letter case apart")
	}
	// FoldsCase stays false, so the walk through the tree is not asked:
	// only the path macOS gives for the new copy can spell it User.
	source := root + "/home/user/f"
	mustDo(t, os.MkdirAll(filepath.Dir(source), 0o755))
	mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
	copied, err := stager.Copy(source)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ArchivePath != "fs/home/User/f" {
		t.Errorf("ArchivePath = %q, want fs/home/User/f, as the volume spells it",
			copied.ArchivePath)
	}
}
