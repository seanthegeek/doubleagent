package analyze

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes of the analysis commands.
const (
	ExitFound       = 0
	ExitNothing     = 1
	ExitError       = 2
	ExitInterrupted = 130
)

// Environment is what a run takes from outside: the clock relative times
// are counted from, the parsers of the run, and the version inventory.json
// records. Tests pin each.
type Environment struct {
	Now     timeutil.Clock
	Parsers func() *parsers.Registry
	Version string
	// ReadDir lists a directory of a loose tree; nil means the file
	// system's listing. Tests deliver the entries in another order through
	// it.
	ReadDir func(path string) ([]fs.DirEntry, error)
}

// System returns the environment of a real run: the system clock, every
// registered parser and the binary's version.
func System() Environment {
	return Environment{Now: time.Now, Parsers: parsers.New, Version: version.String()}
}

// invocation is what a run of an analysis command takes besides its
// options.
type invocation struct {
	ctx            context.Context
	env            Environment
	stdout, stderr io.Writer
}

// commandEntry is one analysis command: its name, its help text, a function
// that returns a new flag set holding its options, and a function that
// parses its arguments into the help request, the usage error, or the run.
type commandEntry struct {
	name  string
	usage string
	flags func() *flag.FlagSet
	start func(args []string, call invocation) (run func() int, help bool, err error)
}

// commands returns the analysis commands, in the order the binary's usage
// lists them.
func commands() []commandEntry {
	return []commandEntry{
		{"detect", detectUsage, func() *flag.FlagSet { return detectFlags(&DetectOptions{}, new(bool)) },
			func(args []string, call invocation) (func() int, bool, error) {
				options, help, err := parseDetect(args)
				return func() int { return runDetect(call.ctx, options, call.env, call.stdout, call.stderr) }, help, err
			}},
		{"timeline", timelineUsage, func() *flag.FlagSet { return timelineFlags(&TimelineOptions{}, new(bool)) },
			func(args []string, call invocation) (func() int, bool, error) {
				options, help, err := parseTimeline(args)
				return func() int { return runTimeline(call.ctx, options, call.env, call.stdout, call.stderr) }, help, err
			}},
		{"fleet", fleetUsage, func() *flag.FlagSet { return fleetFlags(&FleetOptions{}, new(bool)) },
			func(args []string, call invocation) (func() int, bool, error) {
				options, help, err := parseFleet(args)
				return func() int { return runFleet(call.ctx, options, call.stdout, call.stderr) }, help, err
			}},
		{"catalog", catalogUsage, func() *flag.FlagSet { return catalogFlags(&CatalogOptions{}, new(bool)) },
			func(args []string, call invocation) (func() int, bool, error) {
				options, help, err := parseCatalog(args)
				return func() int { return runCatalog(options, call.env, call.stdout, call.stderr) }, help, err
			}},
	}
}

// commandNamed returns the analysis command called name, and false when
// none is.
func commandNamed(name string) (commandEntry, bool) {
	for _, each := range commands() {
		if each.name == name {
			return each, true
		}
	}
	return commandEntry{}, false
}

// Commands returns the names of the analysis commands, in the order the
// binary's usage lists them.
func Commands() []string {
	var names []string
	for _, each := range commands() {
		names = append(names, each.name)
	}
	return names
}

// Run runs the analysis command args[0] with the arguments after it and
// returns the exit code, writing its output to STDOUT and its errors to
// STDERR.
func Run(ctx context.Context, args []string, env Environment, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return ExitError
	}
	output := &recordingWriter{out: stdout}
	code := dispatch(ctx, args, env, output, stderr)
	if output.err != nil && code != ExitError && code != ExitInterrupted {
		fmt.Fprint(stderr, "error: the output could not be written: "+runlog.Printable(reason.For(output.err))+"\n")
		return ExitError
	}
	return code
}

// dispatch is Run with stdout recording its first write error.
func dispatch(ctx context.Context, args []string, env Environment, stdout, stderr io.Writer) int {
	found, ok := commandNamed(args[0])
	if !ok {
		fmt.Fprint(stderr, "error: unknown command "+runlog.Printable(args[0])+"\n")
		return ExitError
	}
	run, help, err := found.start(args[1:], invocation{ctx: ctx, env: env, stdout: stdout, stderr: stderr})
	return finish(args[0], help, err, run, stdout, stderr)
}

// recordingWriter passes writes to w and keeps the first error, so a
// command whose output could not be written does not exit as if it had
// been.
type recordingWriter struct {
	out io.Writer
	err error
}

// Write writes data to the underlying writer, once no write has failed.
func (r *recordingWriter) Write(data []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.out.Write(data)
	r.err = err
	return n, err
}

// finish prints the usage error or the help of a parsed command line, or
// runs the command.
func finish(command string, help bool, err error, run func() int, stdout, stderr io.Writer) int {
	switch {
	case err != nil:
		fmt.Fprintln(stderr, err)
		return ExitError
	case help:
		fmt.Fprint(stdout, usageOf(command))
		return ExitFound
	}
	return run()
}

// matcher returns the bundled catalog compiled for matching paths.
func matcher() (*catalog.Matcher, error) {
	tables, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	return catalog.NewMatcher(tables)
}

// printError writes an error message and returns the error exit code, or
// the interrupted code when the run's context was cancelled.
func printError(ctx context.Context, stderr io.Writer, message string) int {
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "interrupted")
		return ExitInterrupted
	}
	fmt.Fprint(stderr, "error: "+runlog.Printable(message)+"\n")
	return ExitError
}

// openInput opens the input of detect or timeline, listing a loose tree
// through env's lister, and gives every home without a user the --user
// name and every home the run's secrets recorder, in which the parsers
// record what they identify. It returns the work directories the open
// made, deepest first, for the caller to remove once the input is closed
// (removeCreated keeps one that holds anything). The message returned with
// a nil collection is the error line's text.
func openInput(ctx context.Context, options InputOptions, found *catalog.Matcher,
	env Environment, recorder *secrets.Recorder) (*input.Collection, []string, string) {
	var created []string
	if options.WorkDir != "" {
		// Cleaned once, so the directories made are the ones removed.
		options.WorkDir = filepath.Clean(options.WorkDir)
		created = missingDirectories(options.WorkDir)
	}
	collection, err := input.Open(ctx, options.Input, input.Options{Matcher: found, WorkDir: options.WorkDir,
		Host: options.Host, ReadDir: env.ReadDir})
	var inputError *input.Error
	switch {
	case errors.As(err, &inputError):
		return nil, created, inputError.Message
	case err != nil:
		return nil, created, reason.Describe(options.Input, err)
	}
	for _, home := range collection.Homes {
		if home.User == "" {
			home.User = options.User
		}
		home.Secrets = recorder
	}
	return collection, created, ""
}

// closeInput removes an extracted archive, or names where it was kept. An
// interrupted run removes it whatever --keep-extracted says, as a run
// leaves nothing half made behind.
func closeInput(ctx context.Context, collection *input.Collection, keep bool, out output, stderr io.Writer) {
	if !keep || ctx.Err() != nil {
		_ = collection.Cleanup()
		return
	}
	if extracted := collection.ExtractedDir(); extracted != "" {
		fmt.Fprint(stderr, "extracted to: "+out.shown(collection.ShownPath(extracted))+"\n")
	}
}
