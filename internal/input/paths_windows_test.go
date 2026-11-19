package input

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// writeHome writes a Claude Code history file under home through osfile,
// so a last name ending in a dot or a space is kept.
func writeHome(t *testing.T, home string) {
	t.Helper()
	directory := filepath.Join(home, ".claude")
	if err := osfile.MkdirAll(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(directory, "history.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnInputIsReadWhereWindowsPlacesIt(t *testing.T) {
	// Windows places \case on the current drive's root and C:case in drive
	// C's current directory; an absolute path names its last element
	// exactly, a dot or a space ending it included.
	work := t.TempDir()
	home := filepath.Join(work, "case")
	writeHome(t, home)
	elsewhere := filepath.Join(work, "elsewhere")
	for _, name := range []string{"dotted.", "spaced "} {
		writeHome(t, filepath.Join(work, name))
		// The same name without the dot or space, holding nothing, which
		// Windows would open in its place through an ordinary path.
		if err := osfile.MkdirAll(filepath.Join(work, name[:len(name)-1]), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	if err := osfile.MkdirAll(elsewhere, 0o777); err != nil {
		t.Fatal(err)
	}
	volume := filepath.VolumeName(work)
	tests := []struct {
		name, working, input, root string
	}{
		{"a path rooted without a drive is on the working directory's drive", elsewhere,
			home[len(volume):], home},
		{"a path on a drive without a root is in that drive's current directory", work,
			volume + "case", home},
		{"a relative path is below the working directory", work, "case", home},
		{"an absolute path ending in a dot names that entry", elsewhere,
			filepath.Join(work, "dotted."), filepath.Join(work, "dotted.")},
		{"an absolute path ending in a space names that entry", elsewhere,
			filepath.Join(work, "spaced "), filepath.Join(work, "spaced ")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(volume) != 2 || volume[1] != ':' {
				t.Skip("the temporary directory is not on a drive letter:", work)
			}
			t.Chdir(test.working)
			collection, err := Open(t.Context(), test.input, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			defer collection.Cleanup()
			if !samePath(collection.Root, test.root) || len(collection.Artifacts) != 1 {
				t.Errorf("%s read at %s with %d artifacts, want %s with 1", test.input, collection.Root,
					len(collection.Artifacts), test.root)
			}
		})
	}
}
