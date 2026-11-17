//go:build unix

package parser

import (
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

func TestReadersNeverFollowALinkOrOpenAPipe(t *testing.T) {
	directory := t.TempDir()
	target := writeFile(t, "target.json", []byte(`{"a":1}`))
	link := filepath.Join(directory, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(directory, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path, want string
	}{
		{"a link is not followed", link, "link.json could not be read (a symbolic link, not followed)"},
		{"a named pipe is not opened", pipe, "pipe could not be read (not a regular file)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			for range Lines(test.path, &problems) {
				t.Error("a line was yielded")
			}
			if want := (Problems{{Message: test.want}}); !slices.Equal(problems, want) {
				t.Errorf("got %+v, want %+v", problems, want)
			}
			problems = nil
			if _, ok := ReadText(test.path, &problems); ok || len(problems) > 0 {
				t.Errorf("ReadText of an optional side file gave %v, %+v", ok, problems)
			}
		})
	}
	if _, err := ReadDir(link); err == nil {
		t.Error("ReadDir followed a link")
	}
}

func TestAFileItsOwnerCannotReadIsOneProblem(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser reads a file of mode 000")
	}
	path := writeFile(t, "locked.jsonl", []byte("{}\n"))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	var problems Problems
	if _, ok := FileBytes(path, &problems); ok {
		t.Fatal("the file was read")
	}
	want := Problems{{Message: "locked.jsonl could not be read (permission denied)"}}
	if !slices.Equal(problems, want) {
		t.Errorf("got %+v, want %+v", problems, want)
	}
	problems = nil
	if _, ok := ReadText(path, &problems); ok || !slices.Equal(problems, want) {
		t.Errorf("ReadText gave %v, %+v", ok, problems)
	}
}
