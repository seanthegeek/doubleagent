//go:build darwin || linux

package parserkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnUnreadableOrLinkedSharedMemoryFileIsNoProblem(t *testing.T) {
	checkSharedMemoryCases(t, []sharedMemoryCase{
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
	})
}
