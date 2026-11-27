package imagetest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

// Kind is the type of one image entry, the word after its path.
type Kind string

// The kinds of entry a manifest line can name.
const (
	// KindDir is a directory.
	KindDir Kind = "dir"
	// KindFile is a regular file holding the content parts after the kind:
	// quoted texts, or count*"text" for a text repeated count times.
	KindFile Kind = "file"
	// KindLink is a symbolic link to the quoted target after the kind.
	KindLink Kind = "link"
	// KindDirLink is a Windows directory symbolic link to the quoted target.
	KindDirLink Kind = "dirlink"
	// KindJunction is a Windows junction to the quoted target.
	KindJunction Kind = "junction"
	// KindFifo is a named pipe (Unix only).
	KindFifo Kind = "fifo"
	// KindSocket is a Unix domain socket file, bound and then closed.
	KindSocket Kind = "socket"
)

// runThreshold is the length from which a run of one repeated byte in a
// file's content is written as a count and the byte, so a large fixture is
// not stored in the manifest.
const runThreshold = 512

// DefaultModTime returns the modification time an entry gets when its line
// names none: later than every time a line names, as a fixture written today
// is later than the old times some lines set.
func DefaultModTime() time.Time {
	return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
}

// Entry is one line of an image manifest. Path is below the work directory,
// with "/" between its names; Text is a file's content or a link's target.
// The attributes: from=work (FromWork) makes a target a path below the work
// directory, "/" between names; mode=0700 sets the permission bits (Unix,
// not links; default 0644, 0755 for a directory or socket); mtime= an RFC
// 3339 modification time (default DefaultModTime); deny= the icacls rights
// RD, WD and AD, joined by commas, denied to the current user on the entry
// alone (Windows only). Parse fills in the defaults.
type Entry struct {
	Path     string
	Kind     Kind
	Text     string
	FromWork bool
	Mode     fs.FileMode
	ModTime  time.Time
	Deny     string
}

// defaultMode returns the permission bits an entry of kind gets when its line
// names none.
func defaultMode(kind Kind) fs.FileMode {
	switch kind {
	case KindDir, KindJunction, KindSocket:
		return 0o755
	case KindLink, KindDirLink:
		return 0
	}
	return 0o644
}

// hasMode reports whether an entry of kind takes a mode: a link's own mode is
// not set by the builder (Linux has none to set).
func hasMode(kind Kind) bool {
	return kind != KindLink && kind != KindDirLink
}

// isLink reports whether kind has a target.
func isLink(kind Kind) bool {
	return kind == KindLink || kind == KindDirLink || kind == KindJunction
}

// quotedField reads one Go double-quoted string from the start of text and
// returns its value and what follows it.
func quotedField(text string) (value, rest string, err error) {
	if !strings.HasPrefix(text, `"`) {
		return "", "", errNotQuoted
	}
	quoted, err := strconv.QuotedPrefix(text)
	if err != nil {
		return "", "", fmt.Errorf("%q: %w", text, err)
	}
	value, err = strconv.Unquote(quoted)
	return value, text[len(quoted):], err
}

// contentParts reads a file's content from the start of text: one or more
// parts separated by a space, each a quoted string or a count, "*" and a
// quoted string repeated that many times. It returns the content and the
// attributes after it.
func contentParts(text string) (content, rest string, err error) {
	var builder strings.Builder
	for parts := 0; ; parts++ {
		if text == "" || (text[0] != '"' && (text[0] < '0' || text[0] > '9')) {
			if parts == 0 {
				return "", "", errors.New("a file needs at least one content part")
			}
			return builder.String(), text, nil
		}
		count := 1
		if text[0] != '"' {
			number, after, found := strings.Cut(text, "*")
			if count, err = strconv.Atoi(number); !found || err != nil || count < 1 {
				return "", "", fmt.Errorf("a repeated part %q is not count*\"text\"", text)
			}
			text = after
		}
		part, after, err := quotedField(text)
		if err != nil {
			return "", "", err
		}
		builder.WriteString(strings.Repeat(part, count))
		text = strings.TrimPrefix(after, " ")
	}
}

// errNotQuoted is quotedField's error for a field that does not start with
// a quote.
var errNotQuoted = errors.New("a field must be a double-quoted string")

// checkPath fails on a path that is empty, absolute, or holds an empty, "."
// or ".." name, any of which could lead the builder outside the directory.
func checkPath(path string) error {
	for _, name := range strings.Split(path, "/") {
		if name == "" || name == "." || name == ".." {
			return fmt.Errorf("path %q must be relative, with no empty, \".\" or \"..\" name", path)
		}
	}
	return nil
}

// setAttribute sets one attribute of the entry.
func (entry *Entry) setAttribute(name, value string) error {
	switch {
	case name == "mode" && hasMode(entry.Kind):
		mode, err := strconv.ParseUint(value, 8, 32)
		if err != nil || len(value) != 4 || mode > 0o777 {
			return fmt.Errorf("mode %q is not four octal digits up to 0777", value)
		}
		entry.Mode = fs.FileMode(mode)
	case name == "mtime":
		when, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return fmt.Errorf("mtime %q is not an RFC 3339 time: %w", value, err)
		}
		entry.ModTime = when.UTC()
	case name == "from" && value == "work" && isLink(entry.Kind):
		entry.FromWork = true
	case name == "deny" && value != "":
		entry.Deny = value
	default:
		return fmt.Errorf("attribute %q does not apply to a %s", name+"="+value, entry.Kind)
	}
	return nil
}

// readAttributes reads the name=value words after the kind's argument.
func (entry *Entry) readAttributes(text string) error {
	if text == "" {
		return nil
	}
	for _, word := range strings.Split(text, " ") {
		name, value, found := strings.Cut(word, "=")
		if !found {
			return fmt.Errorf("attribute %q is not name=value", word)
		}
		if err := entry.setAttribute(name, value); err != nil {
			return err
		}
	}
	return nil
}

// ParseLine reads one manifest line: a quoted path, a kind, the kind's
// argument (a file's content parts or a link's quoted target), then
// attributes as name=value words.
func ParseLine(line string) (Entry, error) {
	path, rest, err := quotedField(line)
	if err != nil {
		return Entry{}, fmt.Errorf("path: %w", err)
	}
	if err := checkPath(path); err != nil {
		return Entry{}, err
	}
	word, rest, _ := strings.Cut(strings.TrimPrefix(rest, " "), " ")
	entry := Entry{Path: path, Kind: Kind(word), Mode: defaultMode(Kind(word)), ModTime: DefaultModTime()}
	switch entry.Kind {
	case KindFile:
		if entry.Text, rest, err = contentParts(rest); err != nil {
			return Entry{}, fmt.Errorf("file: %w", err)
		}
	case KindLink, KindDirLink, KindJunction:
		if entry.Text, rest, err = quotedField(rest); err != nil {
			return Entry{}, fmt.Errorf("%s: %w", word, err)
		}
		rest = strings.TrimPrefix(rest, " ")
	case KindDir, KindFifo, KindSocket:
	default:
		return Entry{}, fmt.Errorf("unknown kind %q", word)
	}
	if err := entry.readAttributes(rest); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Parse reads a manifest: one entry per line, blank lines and lines starting
// with "#" ignored. A bad line is an error naming its number. Two entries for
// one path are an error too.
func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	seen := map[string]int{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 1<<24)
	for number := 1; scanner.Scan(); number++ {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, err := ParseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", number, err)
		}
		if first, found := seen[entry.Path]; found {
			return nil, fmt.Errorf("line %d: %q was already given on line %d", number, entry.Path, first)
		}
		seen[entry.Path] = number
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// formatContent returns the content parts of a file's content: each run of
// one byte at least runThreshold long as count*"byte", the text between runs
// quoted, and "" for an empty file.
func formatContent(content string) string {
	var parts []string
	start := 0
	for index := 0; index < len(content); {
		end := index + 1
		for end < len(content) && content[end] == content[index] {
			end++
		}
		if end-index >= runThreshold {
			if start < index {
				parts = append(parts, strconv.Quote(content[start:index]))
			}
			parts = append(parts, strconv.Itoa(end-index)+"*"+strconv.Quote(content[index:index+1]))
			start = end
		}
		index = end
	}
	if start < len(content) || len(parts) == 0 {
		parts = append(parts, strconv.Quote(content[start:]))
	}
	return strings.Join(parts, " ")
}

// Format returns the manifest line of the entry, naming only the attributes
// that differ from the defaults, so ParseLine(Format(entry)) gives the entry
// back.
func Format(entry Entry) string {
	line := strconv.Quote(entry.Path) + " " + string(entry.Kind)
	switch {
	case entry.Kind == KindFile:
		line += " " + formatContent(entry.Text)
	case isLink(entry.Kind):
		line += " " + strconv.Quote(entry.Text)
	}
	if entry.FromWork {
		line += " from=work"
	}
	if hasMode(entry.Kind) && entry.Mode != defaultMode(entry.Kind) {
		line += fmt.Sprintf(" mode=%04o", uint32(entry.Mode))
	}
	if !entry.ModTime.Equal(DefaultModTime()) {
		line += " mtime=" + entry.ModTime.UTC().Format(time.RFC3339Nano)
	}
	if entry.Deny != "" {
		line += " deny=" + entry.Deny
	}
	return line
}
