//go:build darwin || linux

package sqlitefile

import (
	"os"
	"strings"
	"testing"
)

func TestOpenPathRefusesSymbolicLinks(t *testing.T) {
	for _, suffix := range []string{"", "-wal", "-journal"} {
		t.Run("a link at the path"+suffix+" is not followed", func(t *testing.T) {
			directory := t.TempDir()
			target := copyTestdata(t, directory, "log_rows.db", "-wal")
			path := directory + "/linked.db"
			if suffix != "" {
				copyTestdata(t, directory, "log_rows.db")
				if err := os.Rename(target, path); err != nil {
					t.Fatal(err)
				}
				target += "-wal"
			}
			if err := os.Symlink(target, path+suffix); err != nil {
				t.Fatal(err)
			}
			_, err := OpenPath(path)
			if err == nil || !strings.Contains(err.Error(), "symbolic link") {
				t.Errorf("OpenPath() error = %v, want a refused link", err)
			}
		})
	}
}
