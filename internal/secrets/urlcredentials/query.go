package urlcredentials

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// query reads the parameters of the query of one token (rules 3 and 6 to
// 8) in one pass from the token's first URL to its end.
type query struct {
	text          string
	starts        []urlStart
	authorityEnds []int
	found         func(Credential)
	// open counts the (, [ and { opened since the first URL began and not
	// closed yet.
	open [3]int
	// nextURL is the first URL whose scheme starts after the byte read;
	// nextAuthority the first later URL whose authority has not ended.
	nextURL, nextAuthority int
	// redirect is set once a later URL's authority has ended and no ? or #
	// has followed it yet; fragment once the fragment has begun.
	redirect, fragment bool
}

// parameters adds the value of each credential parameter of the query of
// a token that ends where text ends, which starts at the first ? or #
// after the first URL's authority (rule 6); authorityEnds holds where each
// URL's authority ends.
func parameters(text string, starts []urlStart, authorityEnds []int, found func(Credential), work textset.Counter) {
	count(work, len(text)-starts[0].scheme)
	start := strings.IndexAny(text[authorityEnds[0]:], "?#")
	if start < 0 {
		return
	}
	start += authorityEnds[0]
	scan := &query{text: text, starts: starts, authorityEnds: authorityEnds, found: found,
		fragment: text[start] == '#', nextAuthority: 1}
	for i := starts[0].scheme; i < start; i++ {
		scan.endsValue(i)
	}
	// A later URL whose authority ends before the query starts has the
	// query's own ? or # as its first.
	for scan.nextAuthority < len(starts) && authorityEnds[scan.nextAuthority] <= start {
		scan.nextAuthority++
	}
	for i := start + 1; i <= len(text); {
		i = scan.parameter(i)
	}
}

// parameter reads the parameter that starts at byte start, adds the value
// of each credential named in it, and returns where the next one starts. A
// name is read from the parameter's start and again after each ? (rule 7)
// up to the first = after it. A credential's value runs to the end of the
// parameter, or to the first byte of rule 3 after its start, and a name
// read inside it is part of it, so each byte is in one value at most.
func (q *query) parameter(start int) int {
	nameStart, named, valueStart := start, false, -1
	for i := start; i < len(q.text); i++ {
		q.passAuthorities(i)
		if length := q.separatorAt(i); length > 0 {
			q.add(valueStart, i, false)
			return i + length
		}
		if q.endsValue(i) {
			q.add(valueStart, i, true)
			valueStart = -1
		}
		switch q.text[i] {
		case '?':
			nameStart, named = i+1, false
			q.redirect = false
		case '=':
			if !named && valueStart < 0 && isCredentialParameter(decodedName(q.text[nameStart:i])) {
				valueStart = i + 1
			}
			named = true
		}
	}
	q.add(valueStart, len(q.text), true)
	return len(q.text) + 1
}

// add adds as a credential the value text[start:end], unless start is -1
// or the value is empty (rule 8). A value that a byte of rule 3 or the
// token's end ended (trim) loses the punctuation at its end.
func (q *query) add(start, end int, trim bool) {
	if start < 0 {
		return
	}
	value := q.text[start:end]
	if trim {
		value = strings.TrimRight(value, ".,:;!?")
	}
	if value != "" {
		q.found(Credential{Start: start, End: start + len(value)})
	}
}

// passAuthorities notes each later URL whose authority ends by byte i, so
// the first # after it starts parameters (rule 6).
func (q *query) passAuthorities(i int) {
	for q.nextAuthority < len(q.authorityEnds) && q.authorityEnds[q.nextAuthority] <= i {
		q.redirect = true
		q.nextAuthority++
	}
}

// separatorAt returns the length of what ends a parameter at byte i
// (rules 6 and 7), or 0: the first # after a later URL's authority, & or ;,
// # before the fragment, \u0026 or \U0026, or a backslash not before a
// slash.
func (q *query) separatorAt(i int) int {
	switch char := q.text[i]; {
	case q.redirect && char == '#':
		q.redirect = false
		q.fragment = true
		return 1
	case char == '#' && !q.fragment:
		q.fragment = true
		return 1
	case char == '&' || char == ';':
		return 1
	case char != '\\' || strings.HasPrefix(q.text[i:], `\/`):
		return 0
	case strings.HasPrefix(q.text[i:], `\u0026`) || strings.HasPrefix(q.text[i:], `\U0026`):
		return 6
	}
	return 1
}

// endsValue reports whether byte i ends the value being read (rule 3): a
// single quote, a closing bracket that closes none opened since the first
// URL began, or a comma directly before a scheme. It counts the brackets
// opened and closed on the way, so it reads each byte once and in order.
func (q *query) endsValue(i int) bool {
	for q.nextURL < len(q.starts) && q.starts[q.nextURL].scheme <= i {
		q.nextURL++
	}
	switch q.text[i] {
	case '\'':
		return true
	case ',':
		return q.nextURL < len(q.starts) && q.starts[q.nextURL].scheme == i+1
	}
	bracket := strings.IndexByte("([{)]}", q.text[i])
	switch {
	case bracket < 0:
		return false
	case bracket < 3:
		q.open[bracket]++
		return false
	case q.open[bracket-3] == 0:
		return true
	}
	q.open[bracket-3]--
	return false
}
