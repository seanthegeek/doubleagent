//go:build unix

package parser

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
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

func TestNoPathTheKitBuildsPassesThroughALinkedDirectory(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "tool-results"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "tool-results", "a.txt"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(root, "home", ".agent", "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sessions, "one")); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{Rel: ".agent/sessions/one.jsonl", DiskPath: filepath.Join(sessions, "one.jsonl"),
		Home: &input.Home{DiskPath: filepath.Join(root, "home")}}
	if path, ok := Beside(artifact, "one", "tool-results", "a.txt"); ok {
		t.Errorf("Beside passed through the link to %s", path)
	}
	if path, ok := artifact.Home.PathOf(".agent/sessions/one/tool-results/a.txt"); ok {
		t.Errorf("PathOf passed through the link to %s", path)
	}
	directory, ok := Beside(artifact, "one")
	if !ok {
		t.Fatal("the link itself, the last element, is the reader's to refuse")
	}
	if path, ok := Under(directory, "tool-results", "a.txt"); ok {
		t.Errorf("Under passed through the link to %s", path)
	}
	if _, err := ReadDir(directory); err == nil {
		t.Error("ReadDir listed the link")
	}
}

func TestADirectoryIsOnlyARealDirectory(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(filepath.Join(plain, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "linked")
	if err := os.Symlink(plain, linked); err != nil {
		t.Fatal(err)
	}
	file := writeFile(t, "file.json", []byte("{}"))
	cases := []struct {
		name, path string
		ok         bool
	}{
		{"a real directory is one", plain, true},
		{"a directory below a real one is one", filepath.Join(plain, "inner"), true},
		{"a link to a directory is not one", linked, false},
		{"a regular file is not one", file, false},
		{"an absent path is not one", filepath.Join(root, "absent"), false},
		{"an empty path is not one", "", false},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			directory, ok := DirectoryAt(each.path)
			if ok != each.ok {
				t.Fatalf("DirectoryAt gave %v, want %v", ok, each.ok)
			}
			child, childOK := directory.Child("a.json")
			if want := filepath.Join(each.path, "a.json"); childOK != each.ok || each.ok && child != want {
				t.Errorf("Child gave %q, %v", child, childOK)
			}
		})
	}
}

func TestAnEntryReplacedBetweenTheLookAndTheOpenIsRefused(t *testing.T) {
	looked := writeFile(t, "looked.json", []byte("{}"))
	info, err := os.Lstat(looked)
	if err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	other := writeFile(t, "other.json", []byte("{}"))
	for _, test := range []struct{ name, path string }{
		{"a named pipe in the file's place returns at once", pipe},
		{"another file in its place is refused", other},
	} {
		t.Run(test.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				file, err := openLooked(test.path, info)
				if err == nil {
					file.Close()
				}
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, errChanged) {
					t.Errorf("error %v, want %v", err, errChanged)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("the open is waiting on the named pipe")
			}
		})
	}
}

func TestUnderWithNoNamesRefusesALinkAsTheDirectory(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "real")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(plain, link); err != nil {
		t.Fatal(err)
	}
	if path, ok := Under(link); ok {
		t.Errorf("Under(link) = %s, true", path)
	}
	if path, ok := Under(plain); !ok || path != plain {
		t.Errorf("Under(plain) = %s, %v", path, ok)
	}
}
