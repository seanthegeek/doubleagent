package collect_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// linkSizeIsTargetLength is false: a link's size is what the reparse point
// reports, not checked against its target here.
const linkSizeIsTargetLength = false

// ownerField matches a manifest row's non-empty owner, the account that
// owns the file, which is the test's own.
var ownerField = regexp.MustCompile(`"owner":"(\\.|[^"\\])+"`)

// replaceOwner returns a manifest row with a non-empty owner replaced.
func replaceOwner(line string) string {
	return ownerField.ReplaceAllString(line, `"owner":"{OWNER}"`)
}

// replaceAccount replaces collection.json's run_as and run_as_admin, after
// checking the account is named and the membership is a boolean.
func (n normaliser) replaceAccount(summary jsonvalue.Value) {
	n.requireText("collection.json run_as", summary.Get("run_as").StringValue() != "")
	n.requireText("collection.json run_as_admin", summary.Get("run_as_admin").IsBool())
	summary.Set("run_as", jsonvalue.NewString("{ACCOUNT}"))
	summary.Set("run_as_admin", jsonvalue.NewString("{ADMIN}"))
}

// systemName returns the operating system's name, which collection.json's
// platform starts with.
func systemName() string { return "Windows" }

// pinUmask does nothing: Windows has no umask, and the image's modes are
// not compared there.
func pinUmask(testing.TB) {}

// pathSpellings returns the spellings a run writes a path in: as given,
// with \ between names, and with /, as a resolved relocation value is
// written.
func pathSpellings(path string) []string {
	return []string{path, filepath.ToSlash(path)}
}

// lastAccessWordings are the start lines that say how this system's
// NtfsDisableLastAccessUpdate setting has files opened: on, off or
// unknown. The setting is the machine's, not the run's.
var lastAccessWordings = []string{
	"last-access updates are on; files are opened with the right to hold their access time",
	"last-access updates are off for NTFS on this system; files on NTFS are opened read-only, " +
		"others with the right to hold their access time",
	"last-access updates could not be determined; files are opened with the right to hold their access time",
}

// machineSetting returns a log line saying how the system's last-access
// setting has files opened as {LAST_ACCESS_SETTING}, and any other line as
// it is.
func machineSetting(line string) string {
	if slices.Contains(lastAccessWordings, line) {
		return "{LAST_ACCESS_SETTING}"
	}
	return line
}
