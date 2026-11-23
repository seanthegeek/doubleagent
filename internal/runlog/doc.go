// Package runlog writes the run's log: every line goes to the log file with a
// UTC time, and to standard error unless the run is quiet. The standard log
// package stamps a line from the system clock in layouts of its own; this one
// writes its own time layout from a clock the caller injects, to
// the file and, unless quiet, to standard error. A line is always one line:
// the control characters a file name can bring into it are escaped
// (Printable).
package runlog
