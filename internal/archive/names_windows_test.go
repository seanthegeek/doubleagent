package archive

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestWriteTarGzKeepsNamesEndingInADotOrASpace(t *testing.T) {
	stage := t.TempDir()
	// Win32 trims a trailing dot or space unless the path has the \\?\
	// prefix, so the staged tree is written through that form.
	contents := map[string]string{"fs/x./a": "dot", "fs/x/b": "plain", "fs/y ": "space", "fs/y": "y"}
	for relative, content := range contents {
		full := `\\?\` + filepath.Join(stage, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(context.Background(), stage, archivePath); err != nil {
		t.Fatal(err)
	}
	headers, members := readMembers(t, archivePath)
	var names []string
	for _, header := range headers {
		names = append(names, header.Name)
	}
	for relative, content := range contents {
		if got, ok := members["./"+relative]; !ok || got != content {
			t.Errorf("member ./%s = %q (present %v), want %q; members %q", relative, got, ok,
				content, strings.Join(names, " "))
		}
	}
}

func TestHashErrorNamesTheNativePath(t *testing.T) {
	canonical := syspath.FromNative(t.TempDir())
	_, _, err := SHA256File(context.Background(), canonical)
	if err == nil {
		t.Fatal("SHA256File(context.Background(), directory) succeeded, want a read error")
	}
	text := err.Error()
	if !strings.Contains(text, "hash "+syspath.ToNative(canonical)+":") || strings.Contains(text, `\\?\`) {
		t.Errorf("error %q, want it to name %q and carry no \\\\?\\ prefix", text, syspath.ToNative(canonical))
	}
}

func TestArchiveErrorNamesTheNativePath(t *testing.T) {
	stage := t.TempDir()
	locked := filepath.Join(stage, "locked")
	if err := os.WriteFile(locked, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
	holder, err := os.Open(locked)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	// A byte range locked by one handle cannot be read through another.
	if err := windows.LockFileEx(windows.Handle(holder.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0,
		new(windows.Overlapped)); err != nil {
		t.Fatal(err)
	}
	err = WriteTarGz(context.Background(), syspath.FromNative(stage), filepath.Join(t.TempDir(), "out.tar.gz"))
	if err == nil {
		t.Fatal("WriteTarGz of a locked file succeeded, want a read error")
	}
	text := err.Error()
	if !strings.Contains(text, "archive "+locked+":") || strings.Contains(text, `\\?\`) {
		t.Errorf("error %q, want it to name %q and carry no \\\\?\\ prefix", text, locked)
	}
}
