// Package hostcmd is the one place the binary runs another program. It exists
// for the few answers no file gives: the user list from getent or dscl and a
// launchd value from plutil.
package hostcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Runner runs host programs and reports whether one is installed.
type Runner interface {
	// Output runs NAME with ARGS and returns its standard output. A program
	// is never started once ctx is cancelled, and one running is stopped.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
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

// pipeWait bounds how long Output waits, after a cancel, for the program's
// output pipe to close: a child the program started may hold it open.
const pipeWait = time.Second

// Output runs NAME with ARGS and returns its standard output. When ctx is
// cancelled the program is killed, the default of exec.CommandContext: the
// programs run here only read and print, so they have nothing to finish or
// clean up, and a kill is one behaviour on every platform where a terminate
// request is not. The error then wraps ctx's error.
func (system) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	command.WaitDelay = pipeWait
	output, err := command.Output()
	if err != nil && ctx.Err() != nil {
		return output, fmt.Errorf("run %s: %w: %w", name, ctx.Err(), err)
	}
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
