package input

import (
	"path/filepath"
	"testing"
)

func TestAPathBelowTheInputIsNamedAsTheInputWasGiven(t *testing.T) {
	absolute := filepath.Join(string(filepath.Separator)+"work", "case")
	root := string(filepath.Separator)
	current := filepath.Join(string(filepath.Separator)+"here", "now")
	names := &inputNames{}
	names.add(absolute, "case")
	names.add(current, ".")
	names.add(root, "top")
	tests := []struct {
		name, path, want string
	}{
		{"the input itself", absolute, "case"},
		{"a path below the input", filepath.Join(absolute, "a", "b"), filepath.Join("case", "a", "b")},
		{"a sibling whose name starts with the input's", absolute + "-other",
			filepath.Join("top", "work", "case-other")},
		{"a path below a root directory", filepath.Join(root, "x"), filepath.Join("top", "x")},
		{"a path below an input given as the working directory", filepath.Join(current, "home", "bob"),
			filepath.Join("home", "bob")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := names.path(test.path); got != test.want {
				t.Errorf("path(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
	if got := (*inputNames)(nil).path(absolute); got != absolute {
		t.Errorf("no names gave %q", got)
	}
}

func TestTheParentOfAPrintedPathIsAPrintedPath(t *testing.T) {
	tests := []struct {
		name, shown, want string
	}{
		{"a directory below the working directory", filepath.Join("copy", ".claude"), "copy"},
		{"a name in the working directory", ".claude", "."},
		{"the working directory", ".", ".."},
		{"the directory above it", "..", filepath.Join("..", "..")},
		{"an absolute path", filepath.Join(string(filepath.Separator)+"a", "b"),
			string(filepath.Separator) + "a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shownParent(test.shown); got != test.want {
				t.Errorf("shownParent(%q) = %q, want %q", test.shown, got, test.want)
			}
		})
	}
}
