// Command doubleagent collects the on-disk artifacts of AI agents from a host
// or a mounted disk image and analyses a collection on the analyst's
// workstation. The command only picks a subcommand and turns its result into
// an exit code; the analysis commands exit 0 (found), 1 (nothing found), 2
// (an error or a command-line mistake) or 130 (interrupted), the analyze.Exit
// constants.
package main
