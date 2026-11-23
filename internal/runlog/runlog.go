package runlog

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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

// Printable returns text as a log line shows it, in the form the analysis
// commands print text for a person: each C0 control (a newline and a tab
// included), DEL and each byte that is not valid UTF-8 as \xNN, and each C1
// control (U+0080 to U+009F) as \u00NN; U+2028 and U+2029 too, as \u2028
// and \u2029, since some viewers take them as a line break. A file name holding a newline, a carriage
// return or a terminal escape then neither ends a log line, starts a forged
// one nor drives the operator's terminal. Decoding decides only how a byte
// is shown; a name that itself holds the text \x0a reads the same, and the
// manifest holds every name exactly.
func Printable(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		char, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case char == utf8.RuneError && size == 1, char < 0x20, char == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, text[i])
		case char >= 0x80 && char <= 0x9f, char == 0x2028, char == 0x2029:
			fmt.Fprintf(&out, `\u%04x`, char)
		default:
			out.WriteString(text[i : i+size])
		}
		i += size
	}
	return out.String()
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
