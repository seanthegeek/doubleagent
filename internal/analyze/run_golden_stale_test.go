package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestStaleGoldenPathsNamesEveryExpectedFileNothingReads(t *testing.T) {
	cases := []goldenCase{{name: "detect", outputs: []string{"timeline.jsonl"}}}
	table := []struct {
		name  string
		files []string
		want  []string
	}{
		{"a case's compared files in one form each are not stale",
			[]string{"detect/stdout.txt", "detect/stderr.txt", "detect/timeline.jsonl.gz"}, nil},
		{"a compressed file beside its plain form is stale",
			[]string{"detect/stdout.txt", "detect/stdout.txt.gz"}, []string{"detect/stdout.txt.gz"}},
		{"a file its case does not compare is stale",
			[]string{"detect/stdout.txt", "detect/extra.txt"}, []string{"detect/extra.txt"}},
		{"a directory that names no case is stale",
			[]string{"detect/stdout.txt", "renamed/stdout.txt"}, []string{"renamed"}},
	}
	for _, testCase := range table {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range testCase.files {
				writeFile(t, filepath.Join(directory, file), "text\n")
			}
			stale, err := staleGoldenPaths(directory, cases)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, path := range stale {
				relative, _ := filepath.Rel(directory, path.path)
				got = append(got, filepath.ToSlash(relative))
			}
			if !slices.Equal(got, testCase.want) {
				t.Errorf("stale paths %v, want %v", got, testCase.want)
			}
		})
	}
}
