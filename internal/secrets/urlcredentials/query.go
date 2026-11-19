package urlcredentials

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// query reads the parameters of the query of one token (rules 3, 6 and 7)
// in one pass from the token's first URL to its end.
type query struct {
	text   string
	starts []urlStart
	found  func(Credential)
	// open counts the (, [ and { opened in the parameter being read and
	// not closed yet.
	open [3]int
	// nextURL is the first URL whose scheme starts after the byte read.
	nextURL int
	// fragment is set once the fragment has begun.
	fragment bool
}

// parameters adds the value of each credential parameter of the query of
// a token that ends where text ends, which starts at the first ? or #
// after the first URL's authority, which ends at byte authorityEnd (rule 6).
func parameters(text string, starts []urlStart, authorityEnd int, found func(Credential), work textset.Counter) {
	count(work, len(text)-starts[0].scheme)
	start := strings.IndexAny(text[authorityEnd:], "?#")
	if start < 0 {
		return
	}
	start += authorityEnd
	scan := &query{text: text, starts: starts, found: found, fragment: text[start] == '#'}
	for i := start + 1; i <= len(text); {
		i = scan.parameter(i)
	}
}

// parameter reads the parameter that starts at byte start, adds the value
// of each credential named in it, and returns where the next one starts. A
// name is read from the parameter's start and again after each ? and each
// # that ends no value (rule 6) up to the first = after it. A credential's
// value runs to the end of the parameter, or to the first byte of rule 3
// after its start, and a name read inside it is part of it, so each byte is
// in one value at most.
func (q *query) parameter(start int) int {
	nameStart, named, valueStart := start, false, -1
	q.open = [3]int{}
	for i := start; i < len(q.text); i++ {
		if length := q.separatorAt(i); length > 0 {
			q.add(valueStart, i, false)
			return i + length
		}
		if q.endsValue(i) {
			q.add(valueStart, i, true)
			valueStart = -1
		}
		switch q.text[i] {
		case '?', '#':
			nameStart, named = i+1, false
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
// or the value is empty (rule 7). A value that a byte of rule 3 or the
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

// separatorAt returns the length of what ends a value and the parameter at
// byte i (rule 6), or 0: & or ;, \u0026 or \U0026, a backslash not before a
// slash, or the # that begins the fragment.
func (q *query) separatorAt(i int) int {
	switch char := q.text[i]; {
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
// single quote, a closing bracket that closes none opened earlier in its
// parameter, or a comma directly before a scheme. It counts the brackets
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
