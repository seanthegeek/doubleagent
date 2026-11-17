package reason

import (
	"io/fs"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestForOnWindowsConditions(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := osfile.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"an access-denied code is permission denied",
			&fs.PathError{Op: "open", Path: file, Err: syscall.ERROR_ACCESS_DENIED}, PermissionDenied},
		{"the directory-name code is not a directory",
			&fs.PathError{Op: "open", Path: file, Err: errorDirectory}, NotDirectory},
		{"the cannot-resolve code is a link loop",
			&fs.PathError{Op: "open", Path: file, Err: errorCantResolveFilename}, LinkLoop},
		{"the name-range code is too long",
			&fs.PathError{Op: "open", Path: file, Err: errorFilenameExcedRange}, NameTooLong},
		{"a path not found outside a listing is not found",
			&fs.PathError{Op: "open", Path: file, Err: errorPathNotFound}, NotFound},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := For(test.err); got != test.want {
				t.Errorf("For(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
	t.Run("a path through a file is not found, as Windows reports it", func(t *testing.T) {
		_, err := osfile.ReadFile(filepath.Join(file, "child"))
		if got := For(err); got != NotFound {
			t.Errorf("For(%v) = %q, want %q (%s)", err, got, NotFound, errorChain(err))
		}
	})
}
