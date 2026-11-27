//go:build darwin || linux

package sqlitefile

import (
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
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
			_, err := OpenPath(path, osfile.ListSpellings)
			if err == nil || !strings.Contains(err.Error(), "symbolic link") {
				t.Errorf("OpenPath(, osfile.ListSpellings) error = %v, want a refused link", err)
			}
		})
	}
}

func TestOpenPathRefusesALinkFoundInAnotherCase(t *testing.T) {
	directory := t.TempDir()
	skipWhereCaseIsIgnored(t, directory)
	target := copyTestdata(t, t.TempDir(), "log_rows.db", "-wal")
	path := copyTestdata(t, directory, "log_rows.db")
	if err := os.Symlink(target+"-wal", path+"-WAL"); err != nil {
		t.Fatal(err)
	}
	_, err := OpenPath(path, osfile.ListSpellings)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("OpenPath(, osfile.ListSpellings) error = %v, want a refused link", err)
	}
}
