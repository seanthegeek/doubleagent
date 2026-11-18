package input

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPathElementsDropTheVolumeAndTheRoot(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{filepath.Join("/", "home", "alice"), []string{"home", "alice"}},
		{filepath.Join("a", "b"), []string{"a", "b"}},
		{string(filepath.Separator), nil},
	}
	for _, test := range tests {
		if got := pathElements(test.path); !slices.Equal(got, test.want) {
			t.Errorf("pathElements(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestMemberElementsDropEmptyAndDotElements(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"a/b", []string{"a", "b"}}, {"a//./b/", []string{"a", "b"}}, {"./", nil}, {"", nil},
	}
	for _, test := range tests {
		if got := memberElements(test.name); !slices.Equal(got, test.want) {
			t.Errorf("memberElements(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}
