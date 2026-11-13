// Command doubleagent collects the on-disk artifacts of AI agents from a host
// or a mounted disk image and analyses a collection on the analyst's
// workstation. This file only dispatches a subcommand and maps its result to
// an exit code; the work is in the internal packages.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes shared by every subcommand, as the collectors define them.
const (
	exitOK          = 0
	exitUsage       = 1
	exitFatal       = 2
	exitInterrupted = 130
)

const usageText = `Usage: doubleagent <command> [options]

Commands:
  collect     archive the on-disk artifacts of AI agents, with a hashed manifest

Options:
  -V, --version   print "doubleagent <version>" and exit
  -h, --help      print this text and exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches ARGS to a subcommand and returns the process exit code,
// writing to STDOUT and STDERR so tests can call it in place of main.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return exitUsage
	}
	switch args[0] {
	case "-V", "--version":
		fmt.Fprintf(stdout, "doubleagent %s\n", version.String())
		return exitOK
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usageText)
		return exitOK
	case "collect":
		fmt.Fprintln(stderr, "doubleagent collect: not implemented yet")
		return exitFatal
	}
	fmt.Fprintf(stderr, "Unknown command: %s\n\n%s", args[0], usageText)
	return exitUsage
}
