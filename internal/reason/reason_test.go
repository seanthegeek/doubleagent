package reason

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The reasons below are this package's own wording, the same on every
// platform; the operating system's text differs between platforms.

func TestForGivesOneReasonPerConditionOnEveryPlatform(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := osfile.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		call func() error
		want string
	}{
		{"a missing file is not found", func() error {
			_, err := osfile.ReadFile(filepath.Join(root, "missing"))
			return err
		}, NotFound},
		{"a directory read as a file is a directory", func() error {
			_, err := osfile.ReadFile(root)
			return err
		}, IsDirectory},
		{"a file listed as a directory is not a directory", func() error {
			_, err := osfile.ReadDir(file)
			return err
		}, NotDirectory},
		{"a name longer than the file system allows is too long", func() error {
			_, err := osfile.ReadFile(filepath.Join(root, strings.Repeat("n", 300)))
			return err
		}, NameTooLong},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if got := For(err); got != test.want {
				t.Errorf("For(%v) = %q, want %q (%s)", err, got, test.want, errorChain(err))
			}
		})
	}
}

func TestForNamesALinkLoop(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	if err := os.Symlink(second, first); err != nil {
		t.Skipf("this account cannot create a symbolic link: %v", err)
	}
	if err := os.Symlink(first, second); err != nil {
		t.Fatal(err)
	}
	_, err := osfile.ReadFile(first)
	if got := For(err); got != LinkLoop {
		t.Errorf("For(%v) = %q, want %q (%s)", err, got, LinkLoop, errorChain(err))
	}
}

func TestForGivesTheInnermostTextOfAnUnclassifiedError(t *testing.T) {
	cause := errors.New("device went away")
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a path error gives its cause", &fs.PathError{Op: "read", Path: "/x", Err: cause},
			"device went away"},
		{"a wrapped path error gives its cause",
			fmt.Errorf("parse: %w", &fs.PathError{Op: "open", Path: "/x", Err: cause}),
			"device went away"},
		{"a link error gives its cause", &os.LinkError{Op: "rename", Old: "/a", New: "/b",
			Err: cause}, "device went away"},
		{"a system call error gives its cause", os.NewSyscallError("getdents", cause),
			"device went away"},
		{"a system call error inside a path error gives the innermost cause",
			&fs.PathError{Op: "readdir", Path: "/x", Err: os.NewSyscallError("getdents", cause)},
			"device went away"},
		{"a link error inside a system call error inside a path error gives the innermost cause",
			&fs.PathError{Op: "open", Path: "/c", Err: os.NewSyscallError("renameat",
				fmt.Errorf("step: %w", &os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: cause}))},
			"device went away"},
		{"an error of another kind gives its own text", errors.New("invalid JSON at line 1"),
			"invalid JSON at line 1"},
		{"no error gives nothing", nil, ""},
		{"a wrapped permission error is classified",
			fmt.Errorf("read: %w", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}),
			PermissionDenied},
		{"a wrapped missing-file error is classified",
			fmt.Errorf("read: %w", fs.ErrNotExist), NotFound},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := For(test.err); got != test.want {
				t.Errorf("For(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}

func TestDescribeNamesThePathTheErrorReports(t *testing.T) {
	cases := []struct {
		name, path string
		err        error
		want       string
	}{
		{"the error's own path is used", "/given",
			&fs.PathError{Op: "open", Path: "/inner/file", Err: fs.ErrNotExist},
			"/inner/file: " + NotFound},
		{"a link error names its first path", "/given",
			&os.LinkError{Op: "rename", Old: "/old", New: "/new", Err: fs.ErrPermission},
			"/old: " + PermissionDenied},
		{"an error without a path uses the given one", "/given", errors.New("bad archive"),
			"/given: bad archive"},
		{"a path error with an empty path uses the given one", "/given",
			&fs.PathError{Op: "open", Err: fs.ErrNotExist}, "/given: " + NotFound},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := Describe(test.path, test.err); got != test.want {
				t.Errorf("Describe(%q, %v) = %q, want %q", test.path, test.err, got, test.want)
			}
		})
	}
}

func TestUnreadableNamesTheFileAndTheReason(t *testing.T) {
	path := filepath.Join("home", "alice", ".claude", "history.jsonl")
	err := &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission}
	want := "history.jsonl could not be read (permission denied)"
	if got := Unreadable(path, err); got != want {
		t.Errorf("Unreadable = %q, want %q", got, want)
	}
}

// errorChain describes each error of err's chain with its Go type, so a
// failure on a runner shows which code the platform returned.
func errorChain(err error) string {
	var parts []string
	for ; err != nil; err = errors.Unwrap(err) {
		parts = append(parts, fmt.Sprintf("%T %#v", err, err))
	}
	return strings.Join(parts, " <- ")
}

func TestForGivesAWrapperItsOwnTextWhenItHoldsNoCause(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"a path error with no cause", &fs.PathError{Op: "open", Path: "/p"}, "open /p"},
		{"a link error with no cause", &os.LinkError{Op: "rename", Old: "a", New: "b"},
			"rename a b"},
		{"a system call error with no cause", &os.SyscallError{Syscall: "getdents"}, "getdents"},
		{"a wrapped path error with no cause",
			fmt.Errorf("reading: %w", &fs.PathError{Op: "open", Path: "/p"}), "open /p"},
		{"a path error over a system call error with no cause",
			&fs.PathError{Op: "open", Path: "/p", Err: &os.SyscallError{Syscall: "openat"}},
			"openat"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := For(test.err); got != test.want {
				t.Errorf("For gave %q, want %q", got, test.want)
			}
		})
	}
}
