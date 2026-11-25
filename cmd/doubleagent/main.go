package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/analyze"
	"github.com/seanthegeek/doubleagent/internal/collect"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/version"
)

const usageText = `Usage: doubleagent <command> [options]

Commands:
  collect     archive the on-disk artifacts of AI agents, with a hashed manifest
  detect      list the homes, users and agents a collection holds
  timeline    parse transcripts into timeline.jsonl, sessions.jsonl and inventory.json
  fleet       merge saved collect --inventory outputs into one CSV
  catalog     print the bundled artifact catalog
  help        print this text

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
		return collect.ExitUsage
	}
	switch args[0] {
	case "-V", "--version":
		io.WriteString(stdout, "doubleagent "+version.String()+"\n")
		return collect.ExitOK
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usageText)
		return collect.ExitOK
	case "collect":
		return runCollect(args[1:], stdout, stderr)
	}
	if slices.Contains(analyze.Commands(), args[0]) {
		return runAnalyze(args, stdout, stderr)
	}
	io.WriteString(stderr, "Unknown command: "+runlog.Printable(args[0])+"\n\n"+usageText)
	return collect.ExitUsage
}

// runCollect parses ARGS, the arguments after `collect`, and runs the
// subcommand. A command-line mistake is printed on STDERR and exits with the
// usage code.
func runCollect(args []string, stdout, stderr io.Writer) int {
	options, action, err := collect.ParseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return collect.ExitUsage
	}
	return collect.Run(context.Background(), options, action, stdout, stderr)
}

// runAnalyze runs the analysis command ARGS[0] with the system's clock and
// parsers; SIGINT and SIGTERM cancel it, and a closed output's signal is
// dropped (pipeSignals), so the command's own exit code and its removal of
// an extracted archive follow a reader that ended (`detect ... | head`).
func runAnalyze(args []string, stdout, stderr io.Writer) int {
	ctx, stop := collect.CatchSignals(context.Background(), pipeSignals()...)
	defer stop()
	return analyze.Run(ctx, args, analyze.System(), stdout, stderr)
}
