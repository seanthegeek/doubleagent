// Package runlog writes the run's log: each line goes to the log file with a
// UTC time and, unless the run is quiet, to standard error. Unlike the
// standard log package, it takes its time from a clock the caller injects
// and escapes the control characters a file name can bring into a line, so a
// line is always one line and shows in the order it holds (Printable). The
// analysis commands print text for a person through Printable too.
package runlog
