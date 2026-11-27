package tomldoc

import (
	"fmt"
	"strings"
)

// maxDocumentBytes is the largest document Parse reads, one mebibyte, which
// bounds one file's cost to about a second, far above a real file's size;
// the measurement is in docs/internals.md.
const maxDocumentBytes = 1 << 20

// maxDepth is the deepest path Parse reads: the parts of the table header
// and of each key on the way to a value, plus one for each array the value
// lies in, far above the handful of levels agents write. The reader
// recurses once per array and inline table without a bound of its own, and
// copies a key's whole path for every key.
const maxDepth = 32

// boolInt returns 1 for true and 0 for false.
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// shapeScanner measures how deep a document's paths are, reading the
// document as the TOML lexer does: brackets, braces, dots, commas and
// equals signs count only outside the four string forms and comments. It
// need not tell a valid document from an invalid one, since the reader does
// that after; where the two could differ it counts more, never less.
type shapeScanner struct {
	text     string
	position int
	line     int
	// held is the depth of the place being read: the header's parts, and
	// what each open array or inline table added, recorded in opened.
	held   int
	opened []opening
	// inHeader is true inside a table header's brackets, and inValue while
	// a value is being read, where a dot belongs to a number.
	inHeader, inValue bool
	// dots counts the dots of the key being read; pending is the parts of
	// the key whose value is being read.
	dots, pending int
}

// opening is an open array or inline table.
type opening struct {
	added int
	array bool
}

// open starts a table header, or an array or inline table value, and
// returns the depth inside it.
func (s *shapeScanner) open(character byte) int {
	switch {
	case s.inHeader:
		// The second bracket of an array of tables header.
	case character == '[' && len(s.opened) == 0 && !s.inValue:
		s.inHeader, s.held, s.dots = true, 0, 0
	default:
		array := character == '['
		added := s.pending + boolInt(array)
		s.opened = append(s.opened, opening{added, array})
		s.held += added
		s.pending, s.dots, s.inValue = 0, 0, array
	}
	return s.held
}

// close ends a table header, or the array or inline table opened last.
func (s *shapeScanner) close() {
	switch {
	case s.inHeader:
		s.inHeader, s.held, s.dots = false, s.dots+1, 0
	case len(s.opened) > 0:
		last := len(s.opened) - 1
		s.held -= s.opened[last].added
		s.opened = s.opened[:last]
		s.dots, s.pending, s.inValue = 0, 0, true
	}
}

// skipComment moves to the line break that ends a comment. The lexer ends a
// comment at a carriage return as well as at a line feed.
func (s *shapeScanner) skipComment() {
	end := strings.IndexAny(s.text[s.position:], "\r\n")
	if end < 0 {
		s.position = len(s.text)
		return
	}
	s.position += end
}

// skipMultiLine moves past the end of a multi-line string: the first run of
// three or more quotes not escaped, whose extra quotes (TOML allows two)
// belong to the string.
func (s *shapeScanner) skipMultiLine(quote byte, escapes bool) {
	delimiter := strings.Repeat(string(quote), 3)
	for s.position < len(s.text) {
		character := s.text[s.position]
		switch {
		case character == '\\' && escapes:
			s.position++
		case strings.HasPrefix(s.text[s.position:], delimiter):
			for s.position < len(s.text) && s.text[s.position] == quote {
				s.position++
			}
			return
		}
		if s.position < len(s.text) && s.text[s.position] == '\n' {
			s.line++
		}
		s.position++
	}
}

// skipString moves past the string that starts at the scanner's position,
// counting the lines a multi-line string holds. A one-line string ends at a
// line break as the lexer's does, leaving the break to be read.
func (s *shapeScanner) skipString() {
	quote := s.text[s.position]
	escapes := quote == '"'
	delimiter := strings.Repeat(string(quote), 3)
	if strings.HasPrefix(s.text[s.position:], delimiter) {
		s.position += len(delimiter)
		s.skipMultiLine(quote, escapes)
		return
	}
	for s.position++; s.position < len(s.text); s.position++ {
		switch character := s.text[s.position]; {
		case character == quote:
			s.position++
			return
		case character == '\n' || character == '\r':
			return
		case character == '\\' && escapes && s.position+1 < len(s.text) &&
			s.text[s.position+1] != '\n' && s.text[s.position+1] != '\r':
			s.position++
		}
	}
}

// scan walks the document and returns an error at the first point where the
// path passes maxDepth.
func (s *shapeScanner) scan() error {
	for s.position < len(s.text) {
		depth := 0
		switch character := s.text[s.position]; character {
		case '"', '\'':
			s.skipString()
			continue
		case '#':
			s.skipComment()
			continue
		case '\n', '\r':
			s.line += boolInt(character == '\n')
			if len(s.opened) == 0 {
				s.dots, s.pending, s.inValue = 0, 0, false
			}
		case ',':
			s.dots, s.pending = 0, 0
			s.inValue = len(s.opened) > 0 && s.opened[len(s.opened)-1].array
		case '.':
			if !s.inValue {
				s.dots++
				depth = s.held + s.dots + 1
			}
		case '=':
			s.pending, s.dots, s.inValue = s.dots+1, 0, true
			depth = s.held + s.pending
		case '[', '{':
			depth = s.open(character)
		case ']', '}':
			s.close()
		}
		s.position++
		if depth > maxDepth {
			return &Error{limit: true, text: fmt.Sprintf(
				"TOML document nests keys, tables and arrays more than %d levels deep at line %d",
				maxDepth, s.line)}
		}
	}
	return nil
}

// checkShape refuses a document over maxDocumentBytes, or one whose paths
// are deeper than maxDepth, before the scanner or the reader sees it.
func checkShape(text string) error {
	if len(text) > maxDocumentBytes {
		return &Error{limit: true, text: fmt.Sprintf(
			"TOML document of %d bytes is larger than the %d bytes this reader accepts",
			len(text), maxDocumentBytes)}
	}
	scanner := shapeScanner{text: text, line: 1}
	return scanner.scan()
}
