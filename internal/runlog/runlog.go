// Package runlog writes the run's log: every line goes to the log file with a
// UTC time, and to standard error unless the run is quiet. The standard log
// package stamps a line from the system clock in layouts of its own; this one
// writes its own time layout from a clock the caller injects, to
// the file and, unless quiet, to standard error.
package runlog

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// TimeFormat is the layout of the time in a log line and of started and
// finished in collection.json: UTC, ISO 8601 to the second.
const TimeFormat = "2006-01-02T15:04:05Z"

// Log writes log lines. The clock is injected so tests can pin the time.
type Log struct {
	file   *os.File
	stderr io.Writer
	quiet  bool
	now    func() time.Time
}

// Open creates (or empties) the log file at PATH and returns a Log that echoes
// each line to STDERR unless QUIET is set. NOW supplies the time of each line.
func Open(path string, quiet bool, stderr io.Writer, now func() time.Time) (*Log, error) {
	file, err := osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", syspath.ToNative(path), err)
	}
	return &Log{file: file, stderr: stderr, quiet: quiet, now: now}, nil
}

// Line appends "<time> <text>\n" to the log file and "<text>\n" to standard
// error unless the log is quiet, and returns the file's write error. The
// standard error copy is attempted even when the file's write fails, so a
// full disk does not hide a message from the operator, and its own failure
// is dropped: the file is the record, and a console that went away (a
// closed pipe, a detached remote shell) must not end the run.
func (log *Log) Line(text string) error {
	_, fileError := io.WriteString(log.file, log.now().UTC().Format(TimeFormat)+" "+text+"\n")
	if !log.quiet {
		_, _ = io.WriteString(log.stderr, text+"\n")
	}
	if fileError != nil {
		return fmt.Errorf("write log: %w", syspath.NativeErrors(fileError))
	}
	return nil
}

// FileLine appends "<time> <text>\n" to the log file only, for a line the
// operator is told about in other words.
func (log *Log) FileLine(text string) error {
	if _, err := io.WriteString(log.file, log.now().UTC().Format(TimeFormat)+" "+text+"\n"); err != nil {
		return fmt.Errorf("write log: %w", syspath.NativeErrors(err))
	}
	return nil
}

// Close closes the log file.
func (log *Log) Close() error {
	if err := log.file.Close(); err != nil {
		return fmt.Errorf("close log: %w", syspath.NativeErrors(err))
	}
	return nil
}
