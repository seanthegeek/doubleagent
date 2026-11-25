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
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes of `collect` and of the top-level command line; the analysis
// commands use analyze's own.
const (
	ExitOK          = 0
	ExitUsage       = 1
	ExitFatal       = 2
	ExitInterrupted = 130
)

// Run performs ACTION and returns the exit code. It writes the catalog,
// version, help, stdout summary or inventory to STDOUT and diagnostics to
// STDERR. A collection or inventory run is cancelled by ctx or by a request
// to stop (catchSignals); --list, --version and --help stage nothing and
// keep the default actions of every signal.
func Run(ctx context.Context, options Options, action Action, stdout, stderr io.Writer) int {
	switch action {
	case ActList:
		stdout.Write(catalog.Text())
		return ExitOK
	case ActVersion:
		io.WriteString(stdout, "doubleagent "+version.String()+"\n")
		return ExitOK
	case ActHelp:
		fmt.Fprint(stdout, UsageText)
		return ExitOK
	}
	ctx, release := catchSignals(ctx)
	defer release()
	host := environment{now: time.Now, runner: hostcmd.System(), getwd: canonicalGetwd,
		findUsers: users.Find, lastAccess: lastAccessUpdates, confine: follow.NewConfinement}
	return runCollection(ctx, options, host, stdout, stderr)
}

// catchSignals sets a run's signal handlers: a run ends before its archive
// is written only when told to stop or on a fatal error, and both remove the
// staging directory unless -k is given; losing its terminal or the reader of
// its output is neither, so the run continues, its log file the record. The returned context is cancelled by SIGINT or SIGTERM;
// continueSignals are caught and dropped (caught, not ignored, so the
// programs the run starts keep their defaults); every other signal keeps
// Go's default (Signals in docs/internals.md); release removes the handlers.
func catchSignals(ctx context.Context) (stopped context.Context, release func()) {
	stopped, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	dropped := continueSignals()
	if len(dropped) == 0 {
		// Notify with no signals would relay every signal.
		return stopped, stop
	}
	// Notify never blocks on a full channel: this one, never read, keeps one
	// signal and the rest are dropped, with no goroutine waiting on it.
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, dropped...)
	return stopped, func() { signal.Stop(caught); stop() }
}

// canonicalGetwd returns the current directory in canonical form.
func canonicalGetwd() (string, error) {
	cwd, err := os.Getwd()
	return syspath.FromNative(cwd), err
}

// hostname returns the system's host name (os.Hostname: on Windows the DNS
// host name, never cut to the 15 characters of the NetBIOS name), or
// "unknown" when it cannot be read.
func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown"
	}
	return name
}

// environment is what a run takes from the host, injected so tests can pin
// it: the time, the host programs, the canonical current directory, the user
// enumeration, whether the system updates last-access times, and the
// confinement of an image run's opens.
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
	// rename and sleep move the archive to its name and pause between tries
	// (osfile.Rename and time.Sleep when nil), and reached, when set, is told
	// each point of the archive step a test stops a run at.
	rename  func(oldPath, newPath string) error
	sleep   func(time.Duration)
	reached func(point string)
}

// fatalError is a fatal condition met before the log exists; its text is
// the message printed on standard error.
type fatalError struct{ message string }

// Error returns the message.
func (e *fatalError) Error() string { return e.message }

// loggedError is a fatal condition whose FATAL line the log already holds.
type loggedError struct{ message string }

// Error returns the message.
func (e *loggedError) Error() string { return e.message }

// runCollection performs a collection from the option resolution on: every
// file and directory it creates is its owner's only (umask 077 on Unix), and
// the staging directory is removed at the end unless -k was given.
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
		return ExitFatal
	}
	if err == nil {
		err = state.execute(ctx)
	}
	return state.exitCode(ctx, err)
}

// exitCode maps a run's outcome to its exit code: 0 when it finished,
// whatever ctx says by then; 130 after logging `interrupted` when it failed
// with ctx cancelled before the archive's move began, removing the
// collection.json beside the archive so only the log and manifest are left;
// 2 after logging any other failure.
func (state *runState) exitCode(ctx context.Context, err error) int {
	switch {
	case err == nil:
		return ExitOK
	case ctx.Err() != nil && !state.moving:
		state.line("interrupted")
		if state.summaryMade {
			_ = osfile.Remove(state.summaryPath)
		}
		return ExitInterrupted
	}
	var logged *loggedError
	if errors.As(err, &logged) {
		return ExitFatal
	}
	state.line("FATAL: " + err.Error())
	return ExitFatal
}

// execute runs the steps after the outputs are open, in order, stopping at
// the first failure or when ctx is cancelled. Then, whatever the outcome and
// before the archive copies the log, it logs the count of files read without
// the access-time hold.
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
