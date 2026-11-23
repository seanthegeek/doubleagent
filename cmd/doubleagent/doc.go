// Command doubleagent collects the on-disk artifacts of AI agents from a host
// or a mounted disk image and analyses a collection on the analyst's
// workstation. This file only dispatches a subcommand and maps its result to
// an exit code; the work is in the internal packages. The analysis commands
// use the exit codes 0 (found), 1 (nothing found), 2 (an error or a
// command-line mistake) and 130 (interrupted), analyze.ExitFound and its
// neighbours.
package main
