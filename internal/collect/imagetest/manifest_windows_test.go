package imagetest

import (
	"os"
	"testing"
)

// platformKinds is a manifest of every kind Windows replays, with times, and
// a directory a deny rule shuts.
const platformKinds = `"d" dir mtime=2026-09-14T00:00:00Z
"d/f" file "x\n" mtime=2020-01-02T03:04:05.5Z
"d/l" link "f" mtime=2021-01-01T00:00:00Z
"d/w" dirlink "d" from=work
"j" junction "d" from=work
"s" socket
"shut" dir deny=RD
`

// requireShut skips the test where the deny rule on the directory at path
// does not stop its listing (an enabled SeBackupPrivilege lets it through).
func requireShut(t *testing.T, path string) {
	t.Helper()
	if _, err := os.ReadDir(path); err == nil {
		t.Skip("the deny rule does not stop this account listing the directory")
	}
}

// openedAgain returns entries as Read finds them after the undo, which takes
// the deny rules away; Read does not read rules.
func openedAgain(entries []Entry) []Entry {
	opened := append([]Entry{}, entries...)
	for index := range opened {
		opened[index].Deny = ""
	}
	return opened
}
