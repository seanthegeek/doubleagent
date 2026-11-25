//go:build windows

package collect

import (
	"os"
	"os/user"
	"runtime"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/volumes"
	"golang.org/x/sys/windows"
)

// privilegeWarning is the start-log warning of a live run outside the
// Administrators group.
const privilegeWarning = "WARNING: not running as Administrator; " +
	"other users' profiles will probably be unreadable"

// restrictUmask does nothing: Windows has no umask.
func restrictUmask() {}

// liveRoot is always false on Windows: every -r is an image, even a drive
// root.
func liveRoot(string) bool { return false }

// runIdentity is the account the run runs as, DOMAIN\user, and whether its
// token holds the Administrators group. The account is empty when it cannot
// be looked up, and Admin false when the membership cannot be checked.
func runIdentity() summary.Identity {
	identity := summary.Identity{}
	if current, err := user.Current(); err == nil {
		identity.RunAs = current.Username
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return identity
	}
	// Token 0 makes CheckTokenMembership check the thread's impersonation
	// token or a copy of the process token; the process token itself is a
	// primary token, which the call refuses. A group UAC has made deny-only
	// does not count.
	identity.Admin, _ = windows.Token(0).IsMember(administrators)
	return identity
}

// identityText is the account part of the first log line.
func identityText(identity summary.Identity) string {
	return "run_as=" + identity.RunAs + " admin=" + switchText(identity.Admin)
}

// privileged reports whether identity can read every user's profile: a
// member of the Administrators group.
func privileged(identity summary.Identity) bool { return identity.Admin }

// systemBases are where the volume step looks for the system volume
// directories: under root in image mode, else the system drive and the
// ProgramData directory the environment names.
func systemBases(root string) volumes.Bases {
	if root != "" {
		return volumes.Bases{System: root, ProgramData: root + "/ProgramData"}
	}
	system := syspath.SystemBase("")
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return volumes.Bases{System: system, ProgramData: system + "/ProgramData"}
	}
	// The environment's spelling, respelled as the disk holds it.
	return volumes.Bases{System: system,
		ProgramData: fsmeta.OnDiskPath("", syspath.FromNative(programData))}
}

// registryReader is the registry the relocation step reads: this host's.
func registryReader() relocation.Registry { return relocation.SystemRegistry() }

// systemName is the system's name, release and machine.
type systemName struct {
	name    string
	release string
	machine string
}

// uname names the system for the platform field: Windows, the version
// RtlGetVersion reports (which, unlike GetVersionEx, no compatibility
// manifest lowers) and PROCESSOR_ARCHITECTURE, else the Go architecture.
func uname() systemName {
	version := windows.RtlGetVersion()
	machine := os.Getenv("PROCESSOR_ARCHITECTURE")
	if machine == "" {
		machine = runtime.GOARCH
	}
	release := strconv.FormatUint(uint64(version.MajorVersion), 10) + "." +
		strconv.FormatUint(uint64(version.MinorVersion), 10) + "." +
		strconv.FormatUint(uint64(version.BuildNumber), 10)
	return systemName{name: "Windows", release: release, machine: machine}
}

// continueSignals is empty on Windows: a write to a closed pipe or console
// fails without a signal, and Go delivers no other event than Ctrl+C and
// Ctrl+Break (os.Interrupt) and a console close, logoff or shutdown
// (SIGTERM), which are requests to stop.
func continueSignals() []os.Signal { return nil }
