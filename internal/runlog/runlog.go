// Package runlog writes the run's log: every line goes to the log file with a
// UTC time, and to standard error unless the run is quiet. The standard log
// package stamps a line from the system clock in layouts of its own; this one
// writes its own time layout from a clock the caller injects, to
// the file and, unless quiet, to standard error. A line is always one line:
// the control characters a file name can bring into it are escaped
// (Printable).
package runlog

import (
	"fmt"
	"io"
	"os"
	"strings"
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

// Open creates the log file at path, new: a name already taken, by a file or
// by a link, which is neither followed nor changed, is an error. The Log
// echoes each line to stderr unless quiet is set; now supplies the time of
// each line.
func Open(path string, quiet bool, stderr io.Writer, now func() time.Time) (*Log, error) {
	file, err := osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", syspath.ToNative(path), err)
	}
	return &Log{file: file, stderr: stderr, quiet: quiet, now: now}, nil
}

// Discard returns a Log that writes no file and echoes each line to stderr
// unless quiet is set, for a run that writes no output files.
func Discard(quiet bool, stderr io.Writer, now func() time.Time) *Log {
	return &Log{stderr: stderr, quiet: quiet, now: now}
}

// Printable returns text with each control character written as `\xNN`, one
// escape per byte: the bytes below 0x20 and 0x7F, and the UTF-8 forms of the
// C1 controls (U+0080 to U+009F) and of U+2028 and U+2029, which some
// viewers take as a line break. A file name holding a newline, a carriage
// return or a terminal escape can then neither end a log line, start a
// forged one nor drive the operator's terminal. A name that itself holds the
// text `\x0a` reads the same; the manifest holds every name exactly.
func Printable(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if width := controlWidth(text[i:]); width > 0 {
			for _, char := range []byte(text[i : i+width]) {
				fmt.Fprintf(&out, `\x%02x`, char)
			}
			i += width - 1
			continue
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

// controlWidth returns the length in bytes of the control character text
// starts with, as Printable defines one, or 0 when it starts with none.
func controlWidth(text string) int {
	switch first := text[0]; {
	case first < 0x20 || first == 0x7f:
		return 1
	case first == 0xc2 && len(text) > 1 && text[1] >= 0x80 && text[1] <= 0x9f:
		return 2
	case strings.HasPrefix(text, "\u2028") || strings.HasPrefix(text, "\u2029"):
		return 3
	}
	return 0
}

// Line appends "<time> <text>\n" to the log file and "<text>\n" to standard
// error unless the log is quiet, text written through Printable, and returns
// the file's write error. The standard error copy is attempted even when the
// file's write fails, so a full disk does not hide a message from the
// operator, and its own failure is dropped: the file is the record, and a
// console that went away (a closed pipe, a detached remote shell) must not
// end the run.
func (log *Log) Line(text string) error {
	text = Printable(text)
	fileError := log.write(text)
	if !log.quiet {
		_, _ = io.WriteString(log.stderr, text+"\n")
	}
	if fileError != nil {
		return fmt.Errorf("write log: %w", syspath.NativeErrors(fileError))
	}
	return nil
}

// FileLine appends "<time> <text>\n" to the log file only, for a line the
// operator is told about in other words; text is written through Printable.
func (log *Log) FileLine(text string) error {
	if err := log.write(Printable(text)); err != nil {
		return fmt.Errorf("write log: %w", syspath.NativeErrors(err))
	}
	return nil
}

// write appends "<time> <text>\n" to the log file, when there is one.
func (log *Log) write(text string) error {
	if log.file == nil {
		return nil
	}
	_, err := io.WriteString(log.file, log.now().UTC().Format(TimeFormat)+" "+text+"\n")
	return err
}

// Close closes the log file, when there is one.
func (log *Log) Close() error {
	if log.file == nil {
		return nil
	}
	if err := log.file.Close(); err != nil {
		return fmt.Errorf("close log: %w", syspath.NativeErrors(err))
	}
	return nil
}
