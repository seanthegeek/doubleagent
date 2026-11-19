// Package analyze implements the analysis commands of the binary: detect
// (the homes, users and agents a collection holds), timeline (every
// transcript parsed into timeline.jsonl, sessions.jsonl and inventory.json),
// fleet (saved outputs of collect --inventory merged into one CSV) and
// catalog (the bundled artifact catalog).
//
// Each command reads its options with its own flag set, and Run maps what a
// run found to its exit code: 0 when it found something, 1 when it found
// nothing, 2 for a command-line mistake or an error, 130 when it was
// interrupted. Every output file and every table has a fixed field order
// (a struct's for the JSON files, the column list for the CSV), so analysts
// can build on them.
package analyze

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/reason"
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

// Commands returns the names of the analysis commands, in the order the
// binary's usage lists them.
func Commands() []string {
	return []string{"detect", "timeline", "fleet", "catalog"}
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
		fmt.Fprint(stderr, "error: the output could not be written: "+shown(reason.For(output.err))+"\n")
		return ExitError
	}
	return code
}

// dispatch is Run with stdout recording its first write error.
func dispatch(ctx context.Context, args []string, env Environment, stdout, stderr io.Writer) int {
	var (
		help bool
		err  error
		run  func() int
	)
	switch args[0] {
	case "detect":
		var options DetectOptions
		options, help, err = ParseDetect(args[1:])
		run = func() int { return runDetect(ctx, options, env, stdout, stderr) }
	case "timeline":
		var options TimelineOptions
		options, help, err = ParseTimeline(args[1:])
		run = func() int { return runTimeline(ctx, options, env, stdout, stderr) }
	case "fleet":
		var options FleetOptions
		options, help, err = ParseFleet(args[1:])
		run = func() int { return runFleet(ctx, options, stdout, stderr) }
	case "catalog":
		var options CatalogOptions
		options, help, err = ParseCatalog(args[1:])
		run = func() int { return runCatalog(options, env, stdout, stderr) }
	default:
		fmt.Fprint(stderr, "error: unknown command "+shown(args[0])+"\n")
		return ExitError
	}
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
	fmt.Fprint(stderr, "error: "+shown(message)+"\n")
	return ExitError
}

// openInput opens the input of detect or timeline, listing a loose tree
// through env's lister, and gives every home without a user the --user
// name and every home the run's secrets setting, which the parsers and the
// repository reader take from it. It returns the work directories the open
// made, deepest first, for the caller to remove once the input is closed
// (removeCreated keeps one that holds anything). The message returned with
// a nil collection is the error line's text.
func openInput(ctx context.Context, options InputOptions, found *catalog.Matcher,
	env Environment) (*input.Collection, []string, string) {
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
		home.Secrets = options.Secrets
	}
	return collection, created, ""
}

// closeInput removes an extracted archive, or names where it was kept. An
// interrupted run removes it whatever --keep-extracted says, as a run
// leaves nothing half made behind.
func closeInput(ctx context.Context, collection *input.Collection, keep bool, stderr io.Writer) {
	if !keep || ctx.Err() != nil {
		_ = collection.Cleanup()
		return
	}
	if extracted := collection.ExtractedDir(); extracted != "" {
		fmt.Fprint(stderr, "extracted to: "+shown(collection.ShownPath(extracted))+"\n")
	}
}
