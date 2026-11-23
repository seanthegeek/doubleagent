package parserkit

import (
	"path/filepath"
	"testing"
)

func TestADirectorysChildRefusesAColonOnWindows(t *testing.T) {
	root := t.TempDir()
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("a real directory was refused")
	}
	cases := []struct {
		name, child string
		ok          bool
	}{
		{"a plain name is joined", "a.json", true},
		{"a name naming a stream of a file is refused", "a.json:stream", false},
		{"a name naming a drive is refused", "C:", false},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			got, ok := directory.Child(each.child)
			if ok != each.ok || ok && got != filepath.Join(root, each.child) {
				t.Errorf("Child gave %q, %v; want ok %v", got, ok, each.ok)
			}
			if _, under := Under(root, each.child); under != each.ok {
				t.Errorf("Under gave %v; Child and Under must agree", under)
			}
		})
	}
}
