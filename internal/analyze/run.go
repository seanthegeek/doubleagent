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
	"github.com/seanthegeek/doubleagent/internal/parsers"
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
	return []string{"catalog"}
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
