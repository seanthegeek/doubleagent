// Package hostcmd is the one place the binary runs another program. It exists
// for the few answers no file gives: the user list from getent or dscl and a
// launchd value from plutil.
package hostcmd

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner runs host programs and reports whether one is installed.
type Runner interface {
	// Output runs NAME with ARGS and returns its standard output.
	Output(name string, args ...string) ([]byte, error)
	// Available reports whether NAME is found on the search path.
	Available(name string) bool
}

// system is the Runner over os/exec.
type system struct{}

// System returns the Runner that runs real programs, in the C locale so their
// output does not depend on the operator's language.
func System() Runner {
	return system{}
}

// Output runs NAME with ARGS and returns its standard output.
func (system) Output(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		return output, fmt.Errorf("run %s: %w", name, err)
	}
	return output, nil
}

// Available reports whether NAME is found on the search path.
func (system) Available(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
