package relocation

import (
	"io"
	"strings"
)

// assignment is what a value reader makes of one assignment of a relocation
// variable: the line it starts on (0 for a value plutil read from a whole
// plist), the variable, and either the raw text assigned in Value or, when
// the reader refused it, the reason. Value is never logged before
// acceptValue passes it.
type assignment struct {
	Line   int
	Name   string
	Value  string
	Reason string
}

// environmentReader holds the state of the /etc/environment reader across
// lines: whether a NUL byte has been seen, whether the line before ended in
// a backslash and blanks, which pam_env joins with the next, and the join
// being built under each pam_env rule for a blank or comment line inside a
// join: before version 1.6 it is passed over and the join goes on
// (joins[0]), from 1.6 on it ends the join (joins[1]).
type environmentReader struct {
	names       []string
	wanted      map[string]bool
	nul, joined bool
	assignments []assignment
	joins       [2]pendingJoin
	joinedHits  []lineName
	joinedSeen  map[lineName]bool
}

// pendingJoin is one join of lines as pam_env builds it: its first line, its
// line count, its text, and the names a line of it holds whole.
type pendingJoin struct {
	first, lines int
	text         []byte
	held         map[string]bool
}

// resolveEnvironment reads /etc/environment as pam_env does and returns an
// assignment for each `NAME=VALUE` line of one of names. The value is the
// text after `=` less one enclosing pair of double quotes, expanded nowhere.
// A value longer than 4096 bytes is refused first (`longer than 4096 bytes`,
// see tooLong). Then refused: a line after a NUL byte (`after a NUL byte`),
// a line joined to the one before (`continued`), a value holding `#` (`holds
// a #`), ending in a backslash and blanks (`continued`) or holding another
// quote (`quoted`). A `#` line is a comment; a carriage return ending the
// line stays in the value. Each joinedNames hit is refused (`continued`).
func resolveEnvironment(r io.Reader, names []string) ([]assignment, error) {
	reader, err := readEnvironment(r, names)
	return reader.assignments, err
}

// joinedNames returns the names that only the joining of /etc/environment's
// lines makes whole (`CODEX_\` then `HOME=/x`), each on the join's first
// line, under either pam_env rule for a blank or comment line in a join. A
// name a line of the join holds whole is left out: that line is its hit.
func joinedNames(r io.Reader, names []string) ([]lineName, error) {
	reader, err := readEnvironment(r, names)
	return reader.joinedHits, err
}

// readEnvironment runs an environmentReader over every line of r.
func readEnvironment(r io.Reader, names []string) (*environmentReader, error) {
	reader := &environmentReader{names: names, wanted: nameSet(names),
		joinedSeen: map[lineName]bool{}}
	err := eachLine(r, func(number int, line []byte) {
		reader.line(number, string(line))
	})
	reader.finish(0)
	reader.finish(1)
	return reader, err
}

// line reads one record: the join test runs before the `=` test, so a
// blank or comment line keeps the join of the line before it.
func (reader *environmentReader) line(number int, record string) {
	text := strings.TrimLeft(record, " \t")
	if strings.Contains(text, "\x00") {
		reader.nul = true
	}
	if text == "" || text[0] == '#' {
		reader.finish(1)
		return
	}
	joinedHere := reader.joined
	reader.joined = !strings.Contains(text, "#") && endsInBackslashBlanks(text)
	reader.join(number, record, text)
	equals := strings.IndexByte(text, '=')
	if equals < 1 || !reader.wanted[text[:equals]] {
		return
	}
	reader.assignments = append(reader.assignments,
		environmentOutcome(number, text[:equals], text[equals+1:], reader.nul, joinedHere))
}

// join adds a line that is neither blank nor a comment to the join of each
// rule, as pam_env joins it: the text before any `#`, its leading blanks
// gone (pam_env 1.5 keeps them, which makes no name whole that dropping
// them does not), and on a line that continues the backslash and the blanks
// after it gone; a line that does not continue ends the join. A NUL reads as
// a space, as in boundedNames.
func (reader *environmentReader) join(number int, record, text string) {
	part, _, _ := strings.Cut(text, "#")
	if reader.joined {
		part = strings.TrimSuffix(strings.TrimRight(part, " \t"), `\`)
	}
	part = strings.ReplaceAll(part, "\x00", " ")
	held := map[string]bool{}
	whole := strings.ReplaceAll(record, "\x00", " ")
	for _, name := range reader.names {
		if bounded(whole, name, false) {
			held[name] = true
		}
	}
	for rule := range reader.joins {
		pending := &reader.joins[rule]
		if pending.lines == 0 {
			*pending = pendingJoin{first: number, held: map[string]bool{}}
		}
		pending.lines++
		pending.text = append(pending.text, part...)
		for name := range held {
			pending.held[name] = true
		}
		if !reader.joined {
			reader.finish(rule)
		}
	}
}

// finish ends the join of one rule: a join of two or more lines gives each
// name its text holds whole and none of its lines does a hit on its first
// line, once, and an assignment refused as `continued` (`after a NUL byte`
// once a NUL has been seen), so the hint has its reason.
func (reader *environmentReader) finish(rule int) {
	pending := reader.joins[rule]
	reader.joins[rule] = pendingJoin{}
	if pending.lines < 2 {
		return
	}
	reason := "continued"
	if reader.nul {
		reason = "after a NUL byte"
	}
	text := string(pending.text)
	for _, name := range reader.names {
		hit := lineName{Line: pending.first, Name: name}
		if pending.held[name] || reader.joinedSeen[hit] || !bounded(text, name, false) {
			continue
		}
		reader.joinedSeen[hit] = true
		reader.joinedHits = append(reader.joinedHits, hit)
		reader.assignments = append(reader.assignments,
			assignment{Line: pending.first, Name: name, Reason: reason})
	}
}

// environmentOutcome refuses or keeps the value of one wanted assignment.
func environmentOutcome(number int, name, value string, nul, joinedHere bool) assignment {
	refuse := func(reason string) assignment {
		return assignment{Line: number, Name: name, Reason: reason}
	}
	switch {
	case tooLong(value):
		return refuse(reasonTooLong)
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
	return assignment{Line: number, Name: name, Value: value}
}

// endsInBackslashBlanks reports whether text ends in a backslash followed by
// nothing but spaces and tabs.
func endsInBackslashBlanks(text string) bool {
	return strings.HasSuffix(strings.TrimRight(text, " \t"), `\`)
}
