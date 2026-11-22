//go:build darwin || linux

package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestUnreadableDatabaseMessagesNeverNameTheDirectory(t *testing.T) {
	data, err := osfile.ReadFile(filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-journal"} {
		t.Run("an unreadable state.vscdb"+suffix+" is reported without its directory", func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "state.vscdb")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if suffix != "" {
				if err := os.WriteFile(path+suffix, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(path+suffix, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path+suffix, 0o600) })
			if file, err := os.Open(path + suffix); err == nil {
				file.Close()
				t.Skip("a file with mode 0 is still readable here")
			}
			messages := databaseMessages(t, path)
			assertNoDirectory(t, directory, messages)
			want := "state.vscdb" + suffix + " could not be read (permission denied)"
			for _, message := range messages {
				if message != want && message != "" {
					t.Errorf("message %q, want %q", message, want)
				}
			}
		})
	}
}
