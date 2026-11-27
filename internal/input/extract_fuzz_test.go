package input

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func FuzzParseManifest(fuzz *testing.F) {
	fuzz.Add([]byte(collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x") + "\n"))
	fuzz.Add([]byte("[1]\n\xff\xfe{\"size\": \"1_0\", \"status\": \"collected\", \"type\": \"file\"}\r\n \n{"))
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		rows, bad, _, err := parseManifest(t.Context(), bytes.NewReader(data), &jsonvalue.Repeats{})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if !row.IsObject() {
				t.Fatalf("kept a row that is not an object: %v", row.JSON())
			}
		}
		if bad < 0 {
			t.Fatalf("bad count %d", bad)
		}
	})
}

// fuzzExtraction returns a fuzz function that extracts data as an archive
// into a new directory under base and fails when anything lands outside it
// or a link is made inside it. The directory is made and removed by hand:
// t.TempDir per input slows fuzzing by orders of magnitude.
func fuzzExtraction(base string) func(*testing.T, []byte) {
	return func(t *testing.T, data []byte) {
		dir, err := os.MkdirTemp(base, "case")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)
		checkExtraction(t, dir, data)
	}
}

// checkExtraction extracts data into dir/out and checks the result.
func checkExtraction(t *testing.T, dir string, data []byte) {
	archive := filepath.Join(dir, "archive")
	destination := filepath.Join(dir, "out")
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	_, _ = extractArchive(t.Context(), archive, archive, destination)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("the test directory holds %d entries (%v)", len(entries), err)
	}
	err = filepath.WalkDir(destination, func(path string, entry os.DirEntry, err error) error {
		if err == nil && entry.Type()&os.ModeSymlink != 0 {
			t.Fatalf("extraction made a link at %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func FuzzExtractTar(fuzz *testing.F) {
	fuzz.Add(tarBytes(fuzz, true, tarMember{name: "fs/home/a/x", typeflag: '0', body: "x"}))
	fuzz.Add(tarBytes(fuzz, false, tarMember{name: "a", typeflag: '2', linkname: "/"},
		tarMember{name: "a/b", typeflag: '0', body: "x"}, tarMember{name: "../c", typeflag: '0'}))
	fuzz.Add(make([]byte, 1024))
	base := fuzz.TempDir()
	fuzz.Fuzz(fuzzExtraction(base))
}

func FuzzExtractZip(fuzz *testing.F) {
	fuzz.Add(zipBytes(fuzz, []string{"fs/", "fs/x", "../y", "/z", `a\b`}, map[string]string{"fs/x": "x"}))
	fuzz.Add(zipBytes(fuzz, []string{"a", "a"}, map[string]string{"a": "second"}))
	base := fuzz.TempDir()
	fuzz.Fuzz(fuzzExtraction(base))
}
