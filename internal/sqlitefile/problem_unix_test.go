//go:build darwin || linux

package sqlitefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// denyRead makes path unreadable until the test ends, skipping the test
// where the user can read it anyway, as root can.
func denyRead(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	if file, err := os.Open(path); err == nil {
		file.Close()
		t.Skip("a file with mode 0 is still readable here")
	}
}

func TestProblemNamesTheFileThatCannotBeRead(t *testing.T) {
	for _, suffix := range []string{"", "-wal", "-journal"} {
		t.Run("an unreadable sessions.db"+suffix+" is named", func(t *testing.T) {
			directory := t.TempDir()
			path := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
			for _, sidecar := range []string{"-wal", "-shm", "-journal"} {
				writeFile(t, directory, "sessions.db"+sidecar, nil)
			}
			denyRead(t, path+suffix)
			err := Problem(path)
			want := "sessions.db" + suffix + " could not be read (permission denied)"
			if err == nil || err.Error() != want {
				t.Errorf("Problem() = %v, want %q", err, want)
			}
		})
	}
}

func TestProblemRefusesLinkedFiles(t *testing.T) {
	for _, suffix := range []string{"", "-wal", "-journal"} {
		t.Run("a link at sessions.db"+suffix+" is a problem", func(t *testing.T) {
			directory := t.TempDir()
			target := writeFile(t, directory, "target.db", testdataBytes(t, "page_1024.db"))
			path := directory + "/sessions.db"
			if suffix != "" {
				writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
			}
			if err := os.Symlink(target, path+suffix); err != nil {
				t.Fatal(err)
			}
			err := Problem(path)
			if err == nil || !strings.Contains(err.Error(), "symbolic link") {
				t.Errorf("Problem() = %v, want a refused link", err)
			}
			if suffix == "" && IsSQLite(path) {
				t.Error("IsSQLite() followed a link")
			}
		})
	}
}

func TestTheSharedMemoryFileIsNeverAProblem(t *testing.T) {
	// SQLite rebuilds the -shm file from the log, and OpenPath never reads
	// it, so no state of it keeps a database from being read.
	cases := []struct {
		name  string
		place func(t *testing.T, path string)
	}{
		{"an unreadable one", func(t *testing.T, path string) {
			writeFile(t, filepath.Dir(path), filepath.Base(path), nil)
			denyRead(t, path)
		}},
		{"a symbolic link", func(t *testing.T, path string) {
			if err := os.Symlink(filepath.Join(filepath.Dir(path), "elsewhere"), path); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name+" leaves the database readable", func(t *testing.T) {
			directory := t.TempDir()
			path := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
			test.place(t, path+"-shm")
			if err := Problem(path); err != nil {
				t.Errorf("Problem() = %v, want none", err)
			}
			database, err := OpenPath(path)
			if err != nil {
				t.Fatalf("OpenPath() = %v", err)
			}
			defer database.Close()
			if _, err := database.TableNames(); err != nil {
				t.Errorf("TableNames() = %v", err)
			}
		})
	}
}
