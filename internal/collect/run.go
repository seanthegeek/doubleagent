package collect

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes of `collect`, as the collectors define them.
const (
	exitOK    = 0
	exitFatal = 2
)

// Run performs ACTION and returns the exit code. It writes the catalog,
// version or help to STDOUT and diagnostics to STDERR.
func Run(ctx context.Context, options Options, action Action, stdout, stderr io.Writer) int {
	switch action {
	case ActList:
		stdout.Write(catalog.Text())
		return exitOK
	case ActVersion:
		fmt.Fprintf(stdout, "doubleagent %s\n", version.String())
		return exitOK
	case ActHelp:
		fmt.Fprint(stdout, UsageText)
		return exitOK
	}
	return run(ctx, options, stderr)
}

// run resolves the paths of an ordinary run, in the sh collector's order, then
// stops: the collection itself is not written yet.
func run(_ context.Context, options Options, stderr io.Writer) int {
	options, err := ResolveProjects(options, os.Getwd)
	if err == nil {
		_, err = ResolveRoot(options, os.Getwd)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitFatal
	}
	fmt.Fprintln(stderr, "doubleagent collect: not implemented yet")
	return exitFatal
}
