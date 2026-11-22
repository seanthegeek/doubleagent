// Command doubleagent collects the on-disk artifacts of AI agents from a host
// or a mounted disk image and analyses a collection on the analyst's
// workstation. This file only dispatches a subcommand and maps its result to
// an exit code; the work is in the internal packages. The analysis commands
// use the exit codes 0 (found), 1 (nothing found), 2 (an error or a
// command-line mistake) and 130 (interrupted), analyze.ExitFound and its
// neighbours.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/analyze"
	"github.com/seanthegeek/doubleagent/internal/collect"
	"github.com/seanthegeek/doubleagent/internal/version"
)

// Exit codes of collect and of the top-level command line; the analysis
// commands use analyze's own.
const (
	exitOK          = 0
	exitUsage       = 1
	exitFatal       = 2
	exitInterrupted = 130
)

const usageText = `Usage: doubleagent <command> [options]

Commands:
  collect     archive the on-disk artifacts of AI agents, with a hashed manifest
  detect      list the homes, users and agents a collection holds
  timeline    parse transcripts into timeline.jsonl, sessions.jsonl and inventory.json
  fleet       merge saved collect --inventory outputs into one CSV
  catalog     print the bundled artifact catalog

Run "doubleagent <command> --help" for a command's options.

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
		io.WriteString(stdout, "doubleagent "+version.String()+"\n")
		return exitOK
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usageText)
		return exitOK
	case "collect":
		return runCollect(args[1:], stdout, stderr)
	}
	if slices.Contains(analyze.Commands(), args[0]) {
		return runAnalyze(args, stdout, stderr)
	}
	io.WriteString(stderr, "Unknown command: "+analyze.Shown(args[0])+"\n\n"+usageText)
	return exitUsage
}

// runCollect parses ARGS, the arguments after `collect`, and runs the
// subcommand. A command-line mistake is printed on STDERR and exits with the
// usage code.
func runCollect(args []string, stdout, stderr io.Writer) int {
	options, action, err := collect.ParseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	return collect.Run(context.Background(), options, action, stdout, stderr)
}

// runAnalyze runs the analysis command ARGS[0] with the system's clock and
// parsers; SIGINT and SIGTERM cancel it.
func runAnalyze(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return analyze.Run(ctx, args, analyze.System(), stdout, stderr)
}
