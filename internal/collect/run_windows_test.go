package collect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/shortname"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

func TestWindowsHostDescription(t *testing.T) {
	identity := summary.Identity{RunAs: `HOST\alice`, Admin: true}
	if got, want := identityText(identity), `run_as=HOST\alice admin=1`; got != want {
		t.Errorf("identityText = %q, want %q", got, want)
	}
	if got := identityText(summary.Identity{RunAs: `HOST\bob`}); got != `run_as=HOST\bob admin=0` {
		t.Errorf("identityText = %q for an account outside Administrators", got)
	}
	if !privileged(identity) || privileged(summary.Identity{UID: 0}) {
		t.Error("privileged must follow the Administrators flag alone")
	}
	system := uname()
	if system.name != "Windows" || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(system.release) ||
		system.machine == "" {
		t.Errorf("uname = %+v, want Windows, major.minor.build and a machine", system)
	}
	if runIdentity().RunAs == "" {
		t.Error("runIdentity found no account name")
	}
}

func TestWindowsImageBasesAndRoots(t *testing.T) {
	want := volumes.Bases{System: "D:/img", ProgramData: "D:/img/ProgramData"}
	if got := systemBases("D:/img"); got != want {
		t.Errorf("systemBases = %+v, want %+v", got, want)
	}
	if liveRoot("C:") || liveRoot("/") {
		t.Error("a root named live mode on Windows")
	}
}

func TestCopyFileErrorNamesTheNativePath(t *testing.T) {
	// Reading a directory opened as a file fails with the directory's name,
	// which osfile opened in its \\?\ form.
	source := t.TempDir()
	err := copyFile(source, filepath.Join(t.TempDir(), "copy"))
	if err == nil || strings.Contains(err.Error(), `\\?\`) || !strings.Contains(err.Error(), source) {
		t.Errorf("copyFile error = %v, want one naming %s without the \\\\?\\ prefix", err, source)
	}
}

func TestOutDirUnderAShortNameTakesTheLongName(t *testing.T) {
	short, long := shortname.Dir(t)
	// The sweep compares OutDir with homes as text, and homes carry long
	// names, so a short name kept here would let it read the run's own
	// staging directory.
	got, err := resolveOutDir(short+`\out`, os.Getwd)
	if err != nil {
		t.Fatal(err)
	}
	if want := syspath.FromNative(long) + "/out"; strings.Contains(got, "~") || !syspath.Equal(got, want) {
		t.Errorf("resolveOutDir = %q, want the long form %q", got, want)
	}
}

// longTempDir returns a new temporary directory with every short 8.3 name
// in it replaced by the long name, as the resolution of -r spells it.
func longTempDir(t *testing.T) string {
	t.Helper()
	long, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return long
}
