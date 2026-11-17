//go:build darwin || linux

package sqlitefile

import (
	"os"
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
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
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
	for _, suffix := range []string{"", "-wal", "-shm"} {
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
