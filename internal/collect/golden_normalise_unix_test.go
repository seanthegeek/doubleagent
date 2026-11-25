//go:build unix

package collect_test

import (
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// linkSizeIsTargetLength says a symlink row's size is its target's length,
// as lstat gives it.
const linkSizeIsTargetLength = true

// digits matches a decimal account number.
var digits = regexp.MustCompile(`^[0-9]+$`)

// replaceOwner returns a manifest row as it is: Unix rows have no owner
// name, only the numeric ids normalise replaces.
func replaceOwner(line string) string { return line }

// replaceAccount replaces collection.json's run_as_uid, after checking it is
// a decimal number.
func (n normaliser) replaceAccount(summary jsonvalue.Value) {
	n.requireText("collection.json run_as_uid", digits.MatchString(summary.Get("run_as_uid").StringValue()))
	summary.Set("run_as_uid", jsonvalue.NewString("{UID}"))
}

// systemName returns the operating system's name as uname prints it, which
// collection.json's platform starts with.
func systemName() string {
	if runtime.GOOS == "darwin" {
		return "Darwin"
	}
	return strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
}

// pinUmask sets the process umask to 022 until the test ends. The package's
// in-process tests of Run leave it at the run's 077, and the image's
// links and the files its builders write take their modes from it.
func pinUmask(test testing.TB) {
	previous := syscall.Umask(0o022)
	test.Cleanup(func() { syscall.Umask(previous) })
}

// pathSpellings returns the spellings a run writes a path in: on Unix the
// path alone.
func pathSpellings(path string) []string { return []string{path} }

// machineSetting returns a log line as it is: on Unix the log states no
// setting of the machine's own.
func machineSetting(line string) string { return line }
