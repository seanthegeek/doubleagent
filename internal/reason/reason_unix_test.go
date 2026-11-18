//go:build !windows

package reason

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestForOnUnixConditions(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	locked := filepath.Join(root, "locked.txt")
	for _, path := range []string{file, locked} {
		if err := osfile.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Run("a path through a file is not a directory", func(t *testing.T) {
		_, err := osfile.ReadFile(filepath.Join(file, "child"))
		if got := For(err); got != NotDirectory {
			t.Errorf("For(%v) = %q, want %q (%s)", err, got, NotDirectory, errorChain(err))
		}
	})
	t.Run("a file without read permission is permission denied", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a file whatever its mode")
		}
		_, err := osfile.ReadFile(locked)
		if got := For(err); got != PermissionDenied {
			t.Errorf("For(%v) = %q, want %q (%s)", err, got, PermissionDenied, errorChain(err))
		}
	})
}

func TestForGivesTheInnermostCauseOfARealErrorWrappedTwice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := osfile.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := osfile.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, writeErr := file.Write([]byte("y"))
	if !errors.Is(writeErr, syscall.EBADF) {
		t.Fatalf("writing a file opened to read gave %v (%s)", writeErr, errorChain(writeErr))
	}
	nested := &fs.PathError{Op: "close", Path: path, Err: os.NewSyscallError("fsync", writeErr)}
	if got, want := For(nested), syscall.EBADF.Error(); got != want {
		t.Errorf("For(%v) = %q, want %q (%s)", nested, got, want, errorChain(nested))
	}
}
