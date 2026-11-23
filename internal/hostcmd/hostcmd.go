package hostcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Runner runs host programs and reports whether one is installed.
type Runner interface {
	// Output runs NAME with ARGS and returns its standard output. A program
	// is never started once ctx is cancelled, and one running is stopped.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	// Available reports whether NAME is found where Output would look.
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

// Output runs NAME with ARGS and returns its standard output. NAME is the
// program lookPath finds. When ctx is cancelled the program is killed, the
// default of exec.CommandContext: the programs run here only read and print,
// so they have nothing to finish or clean up, and a kill is one behaviour on
// every platform where a terminate request is not. The error then wraps
// ctx's error.
func (system) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	program, err := lookPath(name, os.Getenv("PATH"))
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", name, err)
	}
	command := exec.CommandContext(ctx, program, args...)
	command.Args[0] = name
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

// Available reports whether lookPath finds NAME.
func (system) Available(name string) bool {
	_, err := lookPath(name, os.Getenv("PATH"))
	return err == nil
}

// errNotFound is lookPath's error for a name no directory holds.
var errNotFound = errors.New("not found in the system directories or PATH")

// lookPath returns the program file NAME runs: NAME itself when it holds a
// separator, else the first executable file named NAME in the directories
// of systemDirectories, in order, then in those of searchPath (a PATH
// value), so a program of that name placed earlier in the operator's PATH
// is not the one run. The process environment is not changed. A relative
// PATH entry is skipped, as exec.LookPath refuses a result relative to the
// current directory. The executable test of each file is exec.LookPath's.
func lookPath(name, searchPath string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	for _, directory := range append(systemDirectories(), filepath.SplitList(searchPath)...) {
		if !filepath.IsAbs(directory) {
			continue
		}
		if program, err := exec.LookPath(filepath.Join(directory, name)); err == nil {
			return program, nil
		}
	}
	return "", &exec.Error{Name: name, Err: errNotFound}
}
