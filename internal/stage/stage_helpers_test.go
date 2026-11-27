package stage

import "testing"

// copyFile is Copy with the hash and size most tests read.
func copyFile(stager Stager, path string) (string, int64, error) {
	copied, err := stager.Copy(path)
	return copied.SHA256, copied.Size, err
}

// symlink is Link without the member most tests ignore.
func symlink(stager Stager, path, target string, directory bool) error {
	_, err := stager.Link(path, target, directory)
	return err
}

// makeParent makes the staged directory of path, as Copy and Link do
// before they stage it.
func makeParent(stager Stager, path string) error {
	_, dest, err := stager.Dest(path)
	if err != nil {
		return err
	}
	return stager.makeDir("mkdir", parentOf(dest))
}

// mustTree opens dir as a staging tree for the test, closed when it ends.
func mustTree(t *testing.T, dir string) *Tree {
	t.Helper()
	tree, err := OpenTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}
