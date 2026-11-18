package input

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// openArchiveOf writes data as an archive called name under a new test
// directory and opens it.
func openArchiveOf(t *testing.T, name string, data []byte) (*Collection, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	writeFile(t, path, string(data))
	collection, err := Open(t.Context(), path, Options{Matcher: matcherOf(t, "")})
	if err == nil {
		t.Cleanup(func() { _ = collection.Cleanup() })
	}
	return collection, err
}

// rels returns the Rel of each artifact, sorted.
func rels(collection *Collection) []string {
	var found []string
	for _, artifact := range collection.Artifacts {
		found = append(found, artifact.Rel)
	}
	slices.Sort(found)
	return found
}

func TestAnArchiveIsToldByItsFirstBytes(t *testing.T) {
	inner := zipBytes(t, []string{"inner-only.txt"}, map[string]string{"inner-only.txt": "x"})
	tarEndingInZip := tarBytes(t, false, tarMember{name: "home/a/.claude/history.jsonl", typeflag: '0', body: "{}\n"},
		tarMember{name: "home/a/.claude/backup.zip", typeflag: '0', body: string(inner)})
	collection, err := openArchiveOf(t, "c.tar", tarEndingInZip)
	opened(t, err)
	wantSlice(t, "a tar ending in a zip is read as the tar", rels(collection),
		[]string{".claude/backup.zip", ".claude/history.jsonl"})

	collection, err = openArchiveOf(t, "empty.zip", zipBytes(t, nil, nil))
	opened(t, err)
	wantEqual(t, "an empty zip is an empty loose tree", len(collection.Artifacts), 0)

	for _, test := range []struct{ name, data string }{
		{"a zip with bytes before it", "#!/bin/sh\nexit 0\n" + string(inner)},
		{"a file that starts like a zip but is none", "PK\x03\x04 and nothing of a zip"},
	} {
		_, err := openArchiveOf(t, "x.zip", []byte(test.data))
		var inputError *Error
		if !errors.As(err, &inputError) || !strings.HasSuffix(err.Error(), " is neither a tar nor a zip archive") {
			t.Errorf("%s: error %v", test.name, err)
		}
	}
}

func TestAMemberNamingNoFileIsAProblemNamingTheArchive(t *testing.T) {
	cases := []struct {
		name, archive string
		data          func(t *testing.T) []byte
		want          []string
	}{
		{"zip members named empty and dot", "c.zip", func(t *testing.T) []byte {
			return zipBytes(t, []string{"", ".", "alice/.claude/x"}, map[string]string{"alice/.claude/x": "x"})
		}, []string{"c.zip: member  not extracted: its name names no file",
			"c.zip: member . not extracted: its name names no file"}},
		{"a tar file member named dot", "c.tar", func(t *testing.T) []byte {
			return tarBytes(t, false, tarMember{name: ".", typeflag: '0', body: "x"},
				tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"})
		}, []string{"c.tar: member . not extracted: its name names no file"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			collection, err := openArchiveOf(t, test.archive, test.data(t))
			opened(t, err)
			var problems []string
			for _, problem := range collection.Problems {
				problems = append(problems, filepath.Base(problem))
			}
			wantSlice(t, "problems", problems, test.want)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/x"})
		})
	}
}
