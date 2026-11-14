//go:build unix

package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These expect slash-separated absolute paths, which Windows spells with a drive letter.

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

func TestResolveRoot(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "img", "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	getwd := func() (string, error) { return base, nil }
	cases := []struct {
		name      string
		root      string
		want      string
		wantError string
	}{
		{"no root is live mode", "", "", ""},
		{"/ is live mode", "/", "", ""},
		{"a relative root becomes absolute", "img/etc/..", base + "/img", ""},
		{"an absolute root is kept", base + "/img", base + "/img", ""},
		{"a file is not a root", base + "/file", "", "Root is not a directory: " + base + "/file"},
		{"a missing root is not a root", "nope", "", "Root is not a directory: nope"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveRoot(Options{Root: testCase.root}, getwd)
			if testCase.wantError != "" {
				if err == nil || err.Error() != testCase.wantError {
					t.Fatalf("error %v, want %q", err, testCase.wantError)
				}
				return
			}
			if err != nil || got.Root != testCase.want {
				t.Errorf("root %q, error %v; want %q", got.Root, err, testCase.want)
			}
		})
	}
}
