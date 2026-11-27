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
	file    *os.File
	console *Console
	quiet   bool
	now     func() time.Time
}

// Open creates the log file at path, new: a name already taken, by a file or
// a link, is an error and is neither followed nor changed. The Log echoes
// each line to standard error through console unless quiet is set; now
// supplies each line's time.
func Open(path string, quiet bool, console *Console, now func() time.Time) (*Log, error) {
	file, err := osfile.CreateNew(path)
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", syspath.ToNative(path), err)
	}
	return &Log{file: file, console: console, quiet: quiet, now: now}, nil
}

// Discard returns a Log that writes no file and echoes each line to standard
// error through console unless quiet is set, for a run that writes no output
// files.
func Discard(quiet bool, console *Console, now func() time.Time) *Log {
	return &Log{console: console, quiet: quiet, now: now}
}

// reordering reports whether char is one of Unicode's invisible
// bidirectional controls: the embeddings and overrides U+202A to U+202E, the
// isolates U+2066 to U+2069 and the marks U+200E, U+200F and U+061C. Each
// changes the order in which a terminal or viewer shows the text around it,
// so a name could read as another name.
func reordering(char rune) bool {
	return char >= 0x202a && char <= 0x202e || char >= 0x2066 && char <= 0x2069 ||
		char == 0x200e || char == 0x200f || char == 0x061c
}

// mayNeedEscaping reports whether text holds a byte Printable may change:
// below 0x20, DEL, or one at or above 0x80. Plain ASCII, most of what the
// analysis commands print, is then returned without being copied.
func mayNeedEscaping(text string) bool {
	for i := range len(text) {
		if text[i] < 0x20 || text[i] >= 0x7f {
			return true
		}
	}
	return false
}

// Printable returns text as a log line shows it and as the analysis commands
// print it for a person: each C0 control (newline and tab included), DEL and
// each byte that is not valid UTF-8 as \xNN, and as \uNNNN each C1 control
// (U+0080 to U+009F), U+2028 and U+2029, which some viewers take as line
// breaks, and the bidirectional controls (reordering). A file name or other
// evidence text then neither ends a line, forges one, shows in another order
// than it holds nor drives the operator's terminal. A name holding the text
// \x0a reads the same; the manifest and output files hold the text exactly.
func Printable(text string) string {
	if !mayNeedEscaping(text) {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); {
		char, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case char == utf8.RuneError && size == 1, char < 0x20, char == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, text[i])
		case char >= 0x80 && char <= 0x9f, char == 0x2028, char == 0x2029, reordering(char):
			fmt.Fprintf(&out, `\u%04x`, char)
		default:
			out.WriteString(text[i : i+size])
		}
		i += size
	}
	return out.String()
}

// write appends "<time> <text>\n" to the log file, when there is one.
func (log *Log) write(text string) error {
	if log.file == nil {
		return nil
	}
	_, err := io.WriteString(log.file, log.now().UTC().Format(TimeFormat)+" "+text+"\n")
	return err
}

// Line appends "<time> <text>\n" to the log file and queues "<text>\n" for
// standard error (Console) unless the log is quiet, text written through
// Printable, and returns the file's write error. The standard error copy is
// queued even when the file write fails, so a full disk hides no message; the
// file is the record, and a console that went away or stopped reading must
// neither end nor stall the run.
func (log *Log) Line(text string) error {
	text = Printable(text)
	fileError := log.write(text)
	if !log.quiet {
		log.console.Err(text + "\n")
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
