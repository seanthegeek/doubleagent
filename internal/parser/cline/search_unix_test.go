//go:build unix

package cline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// spelling returns sessionID with the letters whose bit is set in number in
// capitals, as testdata/search_spellings.py spells the ids it indexes.
func spelling(sessionID string, number int) string {
	var spelled strings.Builder
	for place, letter := range strings.Split(sessionID, "") {
		if number>>place&1 == 1 {
			letter = strings.ToUpper(letter)
		}
		spelled.WriteString(letter)
	}
	return spelled.String()
}

// spelledManifests writes the manifest of the session "abcdefgh" and makes
// each of the first n case spellings of its directory and name open it: on
// a disk that matches names in any case they already do, and elsewhere
// each spelling is a hard link to the one file.
func spelledManifests(t *testing.T, sessions string, n int, manifest string) {
	t.Helper()
	const sessionID = "abcdefgh"
	first := filepath.Join(sessions, sessionID, sessionID+".json")
	if err := os.MkdirAll(filepath.Dir(first), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	for number := 1; number < n; number++ {
		name := spelling(sessionID, number)
		path := filepath.Join(sessions, name, name+".json")
		if _, err := os.Lstat(path); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(first, path); err != nil {
			t.Fatal(err)
		}
	}
}

// The search index's sessions whose ids are case spellings of one id that
// open one manifest read that manifest once, and each takes its project
// from it: the reads are counted, and so are the bytes the parse
// allocates (runtime.MemStats.TotalAlloc), which do not grow by a copy of
// the manifest per session.
func TestEverySpellingOfOneSearchSessionReadsItsManifestOnce(t *testing.T) {
	const size = 100_000
	manifest := `{"cwd": "/p", "pad": "` + strings.Repeat("x", size) + `"}`
	allocated := map[int]int64{}
	for _, n := range []int{20, 40} {
		data, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("search_spellings_%d.db", n)))
		if err != nil {
			t.Fatal(err)
		}
		home := t.TempDir()
		path := filepath.Join(home, filepath.FromSlash(searchDatabaseRel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		spelledManifests(t, filepath.Join(home, ".cline", "data", "sessions"), n, manifest)
		artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + searchDatabaseRel, Rel: searchDatabaseRel,
			Agent: "cline", Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"},
			Manifest: jsonvalue.NewNull()}
		subject := New()
		subject.Work = parser.CountingWork()
		projects := 0
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
			if row.ProjectPath == "/p" {
				projects++
			}
		}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		allocated[n] = int64(after.TotalAlloc - before.TotalAlloc)
		if reads := subject.Work.Count("search manifests read"); reads != 1 || projects != n {
			t.Errorf("%d spellings: %d rows took the manifest's project after %d reads, want %d after one", n,
				projects, reads, n)
		}
	}
	if growth := allocated[40] - allocated[20]; growth > 5*size {
		t.Errorf("20 more spellings allocated %d more bytes, more than five copies of the %d-byte manifest",
			growth, size)
	}
}
