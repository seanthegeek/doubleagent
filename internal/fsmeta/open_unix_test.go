//go:build unix

package fsmeta

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenRegular(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		wantError  error
	}{
		{"a regular file opens", "file", nil},
		{"a symlink to a regular file is refused", "link", ErrNotRegular},
		{"a pipe is refused", "pipe", ErrNotRegular},
		{"a directory is refused", ".", ErrNotRegular},
		{"a missing file is refused", "missing", os.ErrNotExist},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.path)
			handle, err := openWithin(t, path)
			if test.wantError == nil {
				if err != nil {
					t.Fatal(err)
				}
				defer handle.Close()
				if content, _ := io.ReadAll(handle); string(content) != "text" {
					t.Errorf("read %q, want %q", content, "text")
				}
				return
			}
			var pathError *os.PathError
			if !errors.Is(err, test.wantError) || !errors.As(err, &pathError) ||
				pathError.Path != path {
				t.Errorf("err = %v, want %v naming %s", err, test.wantError, path)
			}
		})
	}
}

// openWithin is OpenRegular, failing the test when the open takes more than a
// few seconds, as an open that waits on a pipe would.
func openWithin(t *testing.T, path string) (*os.File, error) {
	type result struct {
		handle *os.File
		err    error
	}
	done := make(chan result, 1)
	go func() {
		handle, err := OpenRegular(path)
		done <- result{handle, err}
	}()
	select {
	case got := <-done:
		return got.handle, got.err
	case <-time.After(5 * time.Second):
		t.Fatalf("OpenRegular(%s) did not return", path)
		return nil, nil
	}
}
