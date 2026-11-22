package tomldoc

import (
	"fmt"
	"strings"
)

// checkDefinitions refuses a document the reader has accepted that defines
// a key path in a way TOML 1.1 makes invalid (the table of the kinds of
// definition says which) or writes a date-time offset out of range. It
// reads the statements once, in order, keeping each path's definition in a
// tree of maps, so its cost grows with the document. The reader has
// checked the syntax: where the scanner cannot follow a statement it stops
// and accepts the document, so it never refuses one for its own sake.
// seams, nil but in tests, is what a test watches of the check.
func checkDefinitions(text string, seams *checkSeams) error {
	scanner := statementScanner{text: trimByteOrderMark(text), definitions: newDefinitions()}
	scanner.seams = seams
	for scanner.skipSpace(); scanner.position < len(scanner.text); scanner.skipSpace() {
		var followed bool
		if scanner.text[scanner.position] == '[' {
			followed = scanner.header()
		} else {
			followed = scanner.keyValue()
		}
		if scanner.refusal != nil || !followed {
			return scanner.refusal
		}
	}
	return nil
}

// trimByteOrderMark drops the byte order marks the reader drops, so
// positions count from where the reader's do.
func trimByteOrderMark(text string) string {
	for _, mark := range []string{"\xff\xfe", "\xfe\xff", "\xef\xbb\xbf"} {
		if strings.HasPrefix(text, mark) {
			return text[len(mark):]
		}
	}
	return text
}

// statementScanner reads a document's headers and key/value lines, and
// the key/value pairs of its inline tables, as the TOML 1.1 grammar gives
// them. Each method returns false where the text is not what the grammar
// allows there, which ends the scan.
type statementScanner struct {
	text     string
	position int
	definitions
	refusal error
}

// refuse records why the statement at position is invalid.
func (s *statementScanner) refuse(position int, reason string) {
	line := 1 + strings.Count(s.text[:position], "\n")
	column := position - strings.LastIndexByte(s.text[:position], '\n')
	s.refusal = &Error{Kind: InvalidSyntax, Line: line, Column: column,
		text: fmt.Sprintf("invalid TOML at line %d, column %d: %s", line, column, reason)}
}

// header reads a [table] or [[array]] header and applies it.
func (s *statementScanner) header() bool {
	start := s.position
	array := strings.HasPrefix(s.text[s.position:], "[[")
	s.position += 1 + boolInt(array)
	parts, ok := s.key()
	closing := strings.Repeat("]", 1+boolInt(array))
	if !ok || !strings.HasPrefix(s.text[s.position:], closing) {
		return false
	}
	s.position += len(closing)
	if reason := s.defineHeader(parts, array); reason != "" {
		s.refuse(start, reason)
	}
	return true
}

// keyValue reads a key, its equals sign and its value, and applies the key
// before reading the value, as the document defines them in that order.
func (s *statementScanner) keyValue() bool {
	start := s.position
	parts, ok := s.key()
	if !ok || s.position >= len(s.text) || s.text[s.position] != '=' {
		return false
	}
	s.position++
	if reason := s.defineKey(parts); reason != "" {
		s.refuse(start, reason)
		return true
	}
	s.skipBlanks()
	return s.value(parts)
}

// key reads a key of one or more parts, dotted, with the blanks around
// each part, and returns the parts as the reader names them.
func (s *statementScanner) key() ([]string, bool) {
	var parts []string
	for {
		s.skipBlanks()
		part, ok := s.simpleKey()
		if !ok {
			return nil, false
		}
		parts = append(parts, part)
		s.skipBlanks()
		if s.position >= len(s.text) || s.text[s.position] != '.' {
			return parts, true
		}
		s.position++
	}
}

// simpleKey reads one part of a key: a bare key, a basic string or a
// literal string, the quoted forms on one line.
func (s *statementScanner) simpleKey() (string, bool) {
	start := s.position
	if start >= len(s.text) {
		return "", false
	}
	switch quote := s.text[start]; quote {
	case '"', '\'':
		s.position = stringEnd(s.text, start)
		if s.position-start < 2 || s.text[s.position-1] != quote {
			return "", false
		}
		body := s.text[start+1 : s.position-1]
		if quote == '"' {
			return decodeBasicKey(body), true
		}
		return body, true
	}
	for s.position < len(s.text) && isBareKeyCharacter(s.text[s.position]) {
		s.position++
	}
	return s.text[start:s.position], s.position > start
}

// isBareKeyCharacter says whether a byte may be part of a bare key: an
// ASCII letter or digit, an underscore or a dash.
func isBareKeyCharacter(character byte) bool {
	return 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z' ||
		'0' <= character && character <= '9' || character == '_' || character == '-'
}

// value reads the value of the key of these parts in the innermost scope:
// a string, an array, an inline table, or a number, boolean or date and
// time, whose offset it checks.
func (s *statementScanner) value(key []string) bool {
	if s.position >= len(s.text) {
		return false
	}
	switch s.text[s.position] {
	case '"', '\'':
		s.position = stringEnd(s.text, s.position)
		return true
	case '[':
		return s.array(key)
	case '{':
		return s.inlineTable(key)
	}
	start := s.position
	s.skipScalar()
	if reason := checkOffset(s.text[start:s.position]); reason != "" {
		s.refuse(start, reason)
	}
	return s.position > start
}

// array reads an array's values, its commas, and the line breaks and
// comments between them; an inline table in it has the array's path.
func (s *statementScanner) array(key []string) bool {
	s.position++
	for s.skipSpace(); s.position < len(s.text); s.skipSpace() {
		switch s.text[s.position] {
		case ']':
			s.position++
			return true
		case ',':
			s.position++
		default:
			if !s.value(key) || s.refusal != nil {
				return s.refusal != nil
			}
		}
	}
	return false
}

// inlineTable reads an inline table's key/value pairs as a table of their
// own, which nothing outside the braces adds to (the parent key was defined
// as a value before), with the line breaks and comments TOML 1.1 allows.
func (s *statementScanner) inlineTable(key []string) bool {
	s.position++
	s.scopes = append(s.scopes, scope{node: &definition{kind: dottedTable}, path: s.fullPath(key)})
	defer func() { s.scopes = s.scopes[:len(s.scopes)-1] }()
	for s.skipSpace(); s.position < len(s.text); s.skipSpace() {
		switch s.text[s.position] {
		case '}':
			s.position++
			return true
		case ',':
			s.position++
		default:
			if !s.keyValue() || s.refusal != nil {
				return s.refusal != nil
			}
		}
	}
	return false
}

// skipScalar moves past a number, boolean, or date and time: up to the
// blank, comma, bracket, brace, comment or line break that ends it, taking
// the one space TOML allows between a date and its time as part of it.
func (s *statementScanner) skipScalar() {
	start := s.position
	for s.position < len(s.text) {
		if !strings.ContainsRune(" \t\r\n,]}#", rune(s.text[s.position])) {
			s.position++
			continue
		}
		dateBefore := s.position-start == 10 && s.text[start+4] == '-' && s.text[start+7] == '-'
		if s.text[s.position] == ' ' && dateBefore && s.position+3 < len(s.text) &&
			isDigit(s.text[s.position+1]) && isDigit(s.text[s.position+2]) && s.text[s.position+3] == ':' {
			s.position++
			continue
		}
		return
	}
}

// isDigit says whether a byte is an ASCII digit.
func isDigit(character byte) bool { return '0' <= character && character <= '9' }

// skipBlanks moves past spaces and tabs.
func (s *statementScanner) skipBlanks() {
	for s.position < len(s.text) && (s.text[s.position] == ' ' || s.text[s.position] == '\t') {
		s.position++
	}
}

// skipSpace moves past spaces, tabs, line breaks and comments. The lexer
// ends a comment at a carriage return as well as at a line feed.
func (s *statementScanner) skipSpace() {
	for s.position < len(s.text) {
		switch s.text[s.position] {
		case ' ', '\t', '\n', '\r':
			s.position++
		case '#':
			end := strings.IndexAny(s.text[s.position:], "\r\n")
			if end < 0 {
				s.position = len(s.text)
				return
			}
			s.position += end
		default:
			return
		}
	}
}
