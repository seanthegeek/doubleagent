//go:build parity && unix

package replay

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestMaterialiseGivesModesLinksAndADeniedFileItsBytes(t *testing.T) {
	built := newTestCorpus(t)
	secret := built.blob("kept from the earlier snapshot\n")
	locked := built.tree(map[string]any{"name": "secret", "kind": "file", "mode": 0, "blob": secret})
	top := built.tree(
		map[string]any{"name": "link", "kind": "link", "mode": 0o777,
			"target": "{ROOT}/top/locked/secret"},
		map[string]any{"name": "locked", "kind": "dir", "mode": 0, "tree": locked},
		map[string]any{"name": "pipe", "kind": "fifo", "mode": 0o600},
		map[string]any{"name": map[string]string{"hex": "c3a96e"}, "kind": "file", "mode": 0o604,
			"blob": built.blob("")})
	root := built.tree(map[string]any{"name": "top", "kind": "dir", "mode": 0o750, "tree": top})
	materialised := built.open().Materialise(t, root, "")
	tests := []struct {
		name string
		path string
		mode fs.FileMode
	}{
		{"the denied directory has mode 000", "top/locked", fs.ModeDir},
		{"the top directory keeps its mode", "top", fs.ModeDir | 0o750},
		{"a file named in the hex form keeps its mode", "top/én", 0o604},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := osfile.Lstat(filepath.Join(materialised, test.path))
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode() != test.mode {
				t.Errorf("mode %v, want %v", info.Mode(), test.mode)
			}
		})
	}
	target, err := osfile.Readlink(filepath.Join(materialised, "top", "link"))
	if err != nil || target != materialised+"/top/locked/secret" {
		t.Errorf("link target %q (%v)", target, err)
	}
	if _, err := osfile.Lstat(filepath.Join(materialised, "top", "pipe")); err == nil {
		t.Error("a fifo was created")
	}
	if err := os.Chmod(filepath.Join(materialised, "top", "locked"), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := osfile.Lstat(filepath.Join(materialised, "top", "locked", "secret"))
	if err != nil || info.Mode() != 0 || info.Size() != int64(len("kept from the earlier snapshot\n")) {
		t.Errorf("the denied file is %v (%v)", info, err)
	}
}
