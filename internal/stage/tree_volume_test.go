package stage

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// stageVolume names the environment variable holding a directory on the
// volume TestCreateTreeOnTheVolumeTheEnvironmentNames stages on, such as a
// FAT or exFAT mount that keeps no owners or access lists.
const stageVolume = "DOUBLEAGENT_TEST_STAGE_VOLUME"

func TestCreateTreeOnTheVolumeTheEnvironmentNames(t *testing.T) {
	volume := os.Getenv(stageVolume)
	if volume == "" {
		t.Skip(stageVolume + " names no directory on a volume to stage on")
	}
	parent, err := os.MkdirTemp(volume, "doubleagent-stage-")
	mustDo(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(parent) })
	tree, err := CreateTree(syspath.FromNative(parent) + "/stage")
	if err != nil {
		t.Fatalf("CreateTree on %s: %v", volume, err)
	}
	mustDo(t, tree.MkdirAll("fs/a"))
	file, err := tree.CreateNew("fs/a/copy", 0o600)
	mustDo(t, err)
	_, err = file.Write([]byte("staged"))
	mustDo(t, err)
	mustDo(t, file.Close())
	mustDo(t, tree.Discard())
	if _, err := os.Lstat(parent + "/stage"); !os.IsNotExist(err) {
		t.Errorf("the staging directory is left on %s: %v", volume, err)
	}
}

func TestAClosedTreeDoesNothing(t *testing.T) {
	parent := syspath.FromNative(t.TempDir())
	tree, err := CreateTree(parent + "/stage")
	mustDo(t, err)
	mustDo(t, tree.MkdirAll("fs"))
	mustDo(t, tree.Close())
	calls := map[string]func() error{
		"MkdirAll": func() error { return tree.MkdirAll("made") },
		"CreateNew": func() error {
			file, err := tree.CreateNew("created", 0o600)
			if err == nil {
				_ = file.Close()
			}
			return err
		},
		"Open":           func() error { _, err := tree.Open("fs"); return err },
		"Lstat":          func() error { _, err := tree.Lstat("fs"); return err },
		"ReadDir":        func() error { _, err := tree.ReadDir("."); return err },
		"Readlink":       func() error { _, err := tree.Readlink("fs"); return err },
		"Remove":         func() error { return tree.Remove("fs") },
		"Symlink":        func() error { return tree.Symlink("fs", "link", true) },
		"OpenDir":        func() error { _, err := tree.OpenDir("fs"); return err },
		"RemoveContents": tree.RemoveContents,
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, fs.ErrClosed) {
			t.Errorf("%s on a closed tree = %v; want an error wrapping fs.ErrClosed", name, err)
		}
	}
	entries, err := os.ReadDir(syspath.ToNative(parent + "/stage"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "fs" {
		t.Errorf("the closed tree's directory holds %v (%v); want only fs", entries, err)
	}
	if entries, _ := os.ReadDir(syspath.ToNative(parent + "/stage/fs")); len(entries) != 0 {
		t.Errorf("the closed tree made %v in fs", entries)
	}
}
