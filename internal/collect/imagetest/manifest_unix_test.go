//go:build unix

package imagetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// platformKinds is a manifest of every kind Unix replays, with modes and
// times, and a directory its mode shuts.
const platformKinds = `"d" dir mode=0700 mtime=2026-09-14T00:00:00Z
"d/f" file "x\n" mode=0600 mtime=2020-01-02T03:04:05.5Z
"d/l" link "f" mtime=2021-01-01T00:00:00Z
"d/w" link "d/f" from=work
"p" fifo mode=0600
"s" socket mode=0700
"shut" dir mode=0000
`

// requireShut fails unless the directory at path has mode 000.
func requireShut(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0 {
		t.Errorf("the shut directory is %v, %v; want mode 0000", info, err)
	}
}

// openedAgain returns entries as Read finds them after the undo: a shut
// directory open to its owner.
func openedAgain(entries []Entry) []Entry {
	opened := append([]Entry{}, entries...)
	for index := range opened {
		if opened[index].Kind == KindDir && opened[index].Mode&0o700 != 0o700 {
			opened[index].Mode = 0o700
		}
	}
	return opened
}

func TestASocketTooLongForBindIsMadeWhateverTheTemporaryDirectory(t *testing.T) {
	cases := []struct {
		name          string
		longTemporary bool
	}{
		{"a long path is bound through a link in the temporary directory", false},
		{"a TMPDIR too long for the link sends it to /tmp", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			long := filepath.Join(t.TempDir(), strings.Repeat("d", socketPathLimit))
			if err := os.Mkdir(long, 0o700); err != nil {
				t.Fatal(err)
			}
			if testCase.longTemporary {
				t.Setenv("TMPDIR", long)
			}
			path := filepath.Join(long, "s")
			if err := makeSocket(path); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(path)
			if err != nil || info.Mode().Type() != os.ModeSocket {
				t.Errorf("lstat %s: %v, %v; want a socket", path, info, err)
			}
		})
	}
}
