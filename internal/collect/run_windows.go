//go:build windows

package collect

import (
	"os"
	"runtime"
)

// The access checks resolveOutDir and the inventory make; Windows has no
// permission bits to test, so every check passes.
const (
	accessEnter = 1
	accessWrite = 2
	accessRead  = 4
)

// restrictUmask does nothing: Windows has no umask.
func restrictUmask() {}

// canAccess is always true on Windows.
func canAccess(string, uint32) bool { return true }

// systemName is the system's name, release and machine.
type systemName struct {
	name    string
	release string
	machine string
}

// uname names the system by its Go platform, as Windows has no uname.
func uname() systemName {
	return systemName{name: "Windows", machine: runtime.GOARCH}
}

// hostname returns the host's name, or "unknown".
func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown"
	}
	return name
}
