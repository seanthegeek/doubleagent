package stage

import (
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
