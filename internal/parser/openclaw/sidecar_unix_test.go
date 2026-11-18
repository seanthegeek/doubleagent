//go:build unix

package openclaw

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseWithSharedMemory copies the named testdata database into a new home
// at rel, lets place make its -shm file when place is not nil, and returns
// the text of every row the parser gives for it.
func parseWithSharedMemory(t *testing.T, rel, name string, place func(t *testing.T, path string)) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if place != nil {
		place(t, path+"-shm")
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "openclaw",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// SQLite rebuilds the -shm file from the log, so a database is read the
// same whatever stands at that name.
func TestADatabaseIsReadWhateverStandsAsItsSharedMemoryFile(t *testing.T) {
	const rel, name = ".openclaw/state/openclaw.sqlite", "bad_exec_config.db"
	tests := []struct {
		name  string
		place func(t *testing.T, path string)
	}{
		{"an unreadable -shm file", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("not shared memory"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		}},
		{"a -shm symbolic link to a file", func(t *testing.T, path string) {
			target := filepath.Join(filepath.Dir(path), "elsewhere")
			if err := os.WriteFile(target, []byte("not shared memory"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"a -shm symbolic link to nothing", func(t *testing.T, path string) {
			if err := os.Symlink(filepath.Join(filepath.Dir(path), "absent"), path); err != nil {
				t.Fatal(err)
			}
		}},
		{"a directory named as its -shm file", func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	}
	want := parseWithSharedMemory(t, rel, name, nil)
	if len(want) == 0 {
		t.Fatal("the database alone gave no rows")
	}
	for _, test := range tests {
		t.Run("a database with "+test.name+" beside it gives its rows", func(t *testing.T) {
			if got := parseWithSharedMemory(t, rel, name, test.place); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
