// Package hostcmd is the one place the binary runs another program. It exists
// for the few answers no file gives: the user list from getent or dscl and a
// launchd value from plutil.
//
// It builds on os/exec and adds what exec.Command does not give: a Runner
// interface, so tests and callers can substitute a fake; a search of fixed
// system directories before PATH, so a program of the same name earlier on the
// operator's PATH is not the one run; the C locale for the program's output;
// and a bounded wait for its output pipe after a cancel.
package hostcmd
