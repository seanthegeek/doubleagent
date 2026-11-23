package walk

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// projectScope decides RootCheck.OutOfRoot's question for root with probe,
// resolving the root on each call, for tests.
func projectScope(root, project string, probe probes) rootScope {
	return NewRootCheck(root).scope(project, probe)
}

// ProjectOutOfRoot is RootCheck.OutOfRoot for root, resolved on each call.
func ProjectOutOfRoot(root, project string) (string, bool) {
	return NewRootCheck(root).OutOfRoot(project)
}

// ProjectLeavesRoot reports whether RootCheck.Exit refuses project under
// root, resolved on each call.
func ProjectLeavesRoot(root, project string) bool {
	_, leaves := NewRootCheck(root).Exit(project)
	return leaves
}

// stagingTree opens dir as the staging tree of a test, closed when it ends.
func stagingTree(t *testing.T, dir string) *stage.Tree {
	t.Helper()
	tree, err := stage.OpenTree(syspath.FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}

// stagedNames returns the name of every entry under the staging directory,
// read without following a link.
func stagedNames(t *testing.T, stageDir string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	err := filepath.WalkDir(syspath.ToNative(stageDir), func(_ string, entry fs.DirEntry, err error) error {
		if err == nil {
			names[entry.Name()] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}
