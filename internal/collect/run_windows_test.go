package collect

import (
	"regexp"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/summary"
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
