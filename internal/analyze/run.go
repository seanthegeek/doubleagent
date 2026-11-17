// Package analyze implements the analysis commands of the binary: detect
// (the homes, users and agents a collection holds), timeline (every
// transcript parsed into timeline.jsonl, sessions.jsonl and inventory.json),
// fleet (saved outputs of collect --inventory merged into one CSV) and
// catalog (the bundled artifact catalog).
//
// Each command has its own option parser, and Run maps what a run found to
// its exit code: 0 when it found something, 1 when it found nothing, 2 for a
// command-line mistake or an error, 130 when it was interrupted. Every
// output file and every table is written by hand in a fixed field order, so
// analysts can build on them.
package analyze

import (
	"context"
	"errors"
	"fmt"
	"io"
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
}

// System returns the environment of a real run: the system clock, every
// registered parser and the binary's version.
func System() Environment {
	return Environment{Now: time.Now, Parsers: parsers.New, Version: version.String()}
}

// Commands returns the names of the analysis commands, in the order the
// binary's usage lists them.
func Commands() []string {
	return []string{"detect", "fleet", "catalog"}
}

// Run runs the analysis command args[0] with the arguments after it and
// returns the exit code, writing its output to STDOUT and its errors to
// STDERR.
func Run(ctx context.Context, args []string, env Environment, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return ExitError
	}
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
	case "fleet":
		var options FleetOptions
		options, help, err = ParseFleet(args[1:])
		run = func() int { return runFleet(options, stdout, stderr) }
	case "catalog":
		var options CatalogOptions
		options, help, err = ParseCatalog(args[1:])
		run = func() int { return runCatalog(options, env, stdout, stderr) }
	default:
		fmt.Fprintf(stderr, "error: unknown command %s\n", args[0])
		return ExitError
	}
	return finish(args[0], help, err, run, stdout, stderr)
}

// finish prints the help or the usage error of a parsed command line, or
// runs the command.
func finish(command string, help bool, err error, run func() int, stdout, stderr io.Writer) int {
	var usageError *UsageError
	switch {
	case help:
		fmt.Fprint(stdout, usageOf(command))
		return ExitFound
	case errors.As(err, &usageError):
		usageError.Print(stderr)
		return ExitError
	case err != nil:
		fmt.Fprintf(stderr, "error: %v\n", err)
		return ExitError
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
	fmt.Fprintf(stderr, "error: %s\n", message)
	return ExitError
}

// openInput opens the input of detect or timeline and gives every home
// without a user the --user name. The message returned with a nil
// collection is the error line's text.
func openInput(options InputOptions, found *catalog.Matcher) (*input.Collection, string) {
	workDir := options.WorkDir
	if workDir == "" && options.workDirSet {
		workDir = "."
	}
	collection, err := input.Open(options.Input, input.Options{Matcher: found, WorkDir: workDir,
		Host: options.Host})
	var inputError *input.Error
	switch {
	case errors.As(err, &inputError):
		return nil, inputError.Message
	case err != nil:
		return nil, reason.Describe(pathText(options.Input), err)
	}
	if options.User != "" {
		for _, home := range collection.Homes {
			if home.User == "" {
				home.User = options.User
			}
		}
	}
	return collection, ""
}

// closeInput removes an extracted archive, or names where it was kept.
func closeInput(collection *input.Collection, keep bool, stderr io.Writer) {
	if !keep {
		_ = collection.Cleanup()
		return
	}
	if extracted := collection.ExtractedDir(); extracted != "" {
		fmt.Fprintf(stderr, "extracted to: %s\n", extracted)
	}
}
