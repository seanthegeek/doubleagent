package collect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/hostcmd"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes of `collect`, as the collectors define them.
const (
	exitOK          = 0
	exitFatal       = 2
	exitInterrupted = 130
)

// Run performs ACTION and returns the exit code. It writes the catalog,
// version, help, stdout summary or inventory to STDOUT and diagnostics to
// STDERR. A run is cancelled by ctx or by SIGINT or SIGTERM.
func Run(ctx context.Context, options Options, action Action, stdout, stderr io.Writer) int {
	switch action {
	case ActList:
		stdout.Write(catalog.Text())
		return exitOK
	case ActVersion:
		io.WriteString(stdout, "doubleagent "+version.String()+"\n")
		return exitOK
	case ActHelp:
		fmt.Fprint(stdout, UsageText)
		return exitOK
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	host := environment{now: time.Now, runner: hostcmd.System(), getwd: canonicalGetwd,
		findUsers: users.Find, lastAccess: lastAccessUpdates, confine: follow.NewConfinement}
	return runCollection(ctx, options, host, stdout, stderr)
}

// canonicalGetwd returns the current directory in canonical form.
func canonicalGetwd() (string, error) {
	cwd, err := os.Getwd()
	return syspath.FromNative(cwd), err
}

// environment is what a run takes from the host, injected so tests can pin
// the clock: the time, the host programs, the current directory, in
// canonical form, the user enumeration (users.Find) and whether the system
// updates last-access times (lastAccessUpdates), and the confinement of an
// image run's opens (follow.NewConfinement).
type environment struct {
	now       func() time.Time
	runner    hostcmd.Runner
	getwd     func() (string, error)
	findUsers func(context.Context, bool, string, string, users.Runner, glob.Check,
		*fsmeta.Opener, func(string) string) users.Report
	lastAccess func() accessUpdates
	confine    func(root string, report func(path string)) (*follow.Confinement, error)
	// markCaseSensitive marks the empty staging fs directory case-sensitive
	// on Windows (stage.MarkCaseSensitive); nil marks it the same way.
	markCaseSensitive func(dir string) error
}

// fatalError is a fatal condition met before the log exists; its text is
// the sh collector's message on standard error.
type fatalError struct{ message string }

// Error returns the message.
func (e *fatalError) Error() string { return e.message }

// loggedError is a fatal condition whose FATAL line the log already holds.
type loggedError struct{ message string }

// Error returns the message.
func (e *loggedError) Error() string { return e.message }

// runCollection performs the sh collector's main program from the option
// resolution on: every file and directory it creates is its owner's only, as
// under the sh collector's umask 077, and the staging directory is removed at
// the end unless -k was given.
func runCollection(ctx context.Context, options Options, host environment,
	stdout, stderr io.Writer,
) int {
	restrictUmask()
	state, err := prepareRun(options, host, stdout, stderr)
	if err == nil {
		defer state.cleanup()
		err = state.openOutputs()
	}
	var fatal *fatalError
	if errors.As(err, &fatal) {
		io.WriteString(stderr, fatal.message+"\n")
		return exitFatal
	}
	if err == nil {
		err = state.execute(ctx)
	}
	return state.exitCode(ctx, err)
}

// exitCode maps the outcome of a run to its exit code: 130 after logging
// `interrupted` when ctx was cancelled, 2 after logging any other failure.
func (state *runState) exitCode(ctx context.Context, err error) int {
	switch {
	case ctx.Err() != nil:
		state.line("interrupted")
		return exitInterrupted
	case err == nil:
		return exitOK
	}
	var logged *loggedError
	if errors.As(err, &logged) {
		return exitFatal
	}
	state.line("FATAL: " + err.Error())
	return exitFatal
}

// execute runs the steps after the outputs are open, in the sh collector's
// order, stopping at the first failure or when ctx is cancelled. Every file
// is read by then, so the count of those read without the access-time hold
// is logged next, whatever the outcome, and before the archive takes its
// copy of the log.
func (state *runState) execute(ctx context.Context) error {
	err := state.runSteps(ctx)
	if countErr := state.logNotHeld(); err == nil {
		err = countErr
	}
	switch {
	case err != nil:
		return err
	case state.options.Inventory:
		return state.writeInventory(ctx)
	}
	return state.writeArchive(ctx)
}

// runSteps runs the steps that read the evidence, stopping at the first
// failure or when ctx is cancelled.
func (state *runState) runSteps(ctx context.Context) error {
	steps := []func(context.Context) error{
		state.startLog, state.confine, state.findHomes, state.collectHomes, state.collectVolumes,
		state.collectProjects, state.searchRelocation,
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}
