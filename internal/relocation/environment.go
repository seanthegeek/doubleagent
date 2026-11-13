package relocation

import (
	"io"
	"strings"
)

// Assignment is what a value reader makes of one assignment of a relocation
// variable: the line it starts on (0 for a value plutil read from a whole
// plist), the variable, and either the raw text assigned in Value or, when
// the reader refused it, the reason. Value is never logged before
// AcceptValue passes it.
type Assignment struct {
	Line   int
	Name   string
	Value  string
	Reason string
}

// environmentReader holds the state of the /etc/environment reader across
// lines: whether a NUL byte has been seen, and whether the line before ended
// in a backslash and blanks, which pam_env joins with the next.
type environmentReader struct {
	wanted      map[string]bool
	nul, joined bool
	assignments []Assignment
}

// ResolveEnvironment reads /etc/environment as pam_env does and returns an
// Assignment for each `NAME=VALUE` line of one of names. The value is the
// text after `=` less one enclosing pair of double quotes, expanded nowhere.
// Refused: a line after a NUL byte (`after a NUL byte`), a line joined to the
// one before (`continued`), a value holding `#` (`holds a #`), ending in a
// backslash and blanks (`continued`) or holding another quote (`quoted`). A
// `#` line is a comment; a carriage return ending the line stays in the value.
func ResolveEnvironment(r io.Reader, names []string) ([]Assignment, error) {
	reader := environmentReader{wanted: nameSet(names)}
	err := eachLine(r, func(number int, line []byte) {
		reader.line(number, markNulBytes(line))
	})
	return reader.assignments, err
}

// line is the body of the sh collector's ENVIRONMENT_VALUES_AWK program for
// one record, in its order: the join test runs before the `=` test, so a
// blank or comment line keeps the join of the line before it.
func (reader *environmentReader) line(number int, record string) {
	text := strings.TrimLeft(record, " \t")
	if strings.Contains(text, "\x01") {
		reader.nul = true
	}
	if text == "" || text[0] == '#' {
		return
	}
	joinedHere := reader.joined
	reader.joined = !strings.Contains(text, "#") && endsInBackslashBlanks(text)
	equals := strings.IndexByte(text, '=')
	if equals < 1 || !reader.wanted[text[:equals]] {
		return
	}
	reader.assignments = append(reader.assignments,
		environmentOutcome(number, text[:equals], text[equals+1:], reader.nul, joinedHere))
}

// environmentOutcome refuses or keeps the value of one wanted assignment.
func environmentOutcome(number int, name, value string, nul, joinedHere bool) Assignment {
	refuse := func(reason string) Assignment {
		return Assignment{Line: number, Name: name, Reason: reason}
	}
	switch {
	case nul:
		return refuse("after a NUL byte")
	case joinedHere:
		return refuse("continued")
	case strings.Contains(value, "#"):
		return refuse("holds a #")
	case endsInBackslashBlanks(value):
		return refuse("continued")
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	if strings.ContainsAny(value, `"'`) {
		return refuse("quoted")
	}
	return Assignment{Line: number, Name: name, Value: value}
}

// endsInBackslashBlanks reports whether text ends in a backslash followed by
// nothing but spaces and tabs.
func endsInBackslashBlanks(text string) bool {
	return strings.HasSuffix(strings.TrimRight(text, " \t"), `\`)
}
