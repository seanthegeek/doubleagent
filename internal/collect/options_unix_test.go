//go:build unix

package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveProjects(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "real", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	getwd := func() (string, error) { return base, nil }
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{"an absolute path is kept", []string{"/abs/p"}, []string{"/abs/p"}},
		{"an existing relative directory is cleaned", []string{"real/sub/../sub"},
			[]string{base + "/real/sub"}},
		{"a missing relative path is joined as typed", []string{"./gone/../x"},
			[]string{base + "/gone/../x"}},
		{"duplicates are listed once", []string{"real", "./real", base + "/real"},
			[]string{base + "/real"}},
		{"an empty path is dropped", []string{"", "/a"}, []string{"/a"}},
		{"no path gives none", nil, nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveProjects(Options{Projects: testCase.input}, getwd)
			if err != nil || !reflect.DeepEqual(got.Projects, testCase.want) {
				t.Errorf("projects %q, error %v; want %q", got.Projects, err, testCase.want)
			}
		})
	}
}
