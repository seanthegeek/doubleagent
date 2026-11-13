package relocation

import (
	"io"
	"strings"
)

// environmentDReader holds the state of the environment.d reader across
// lines, named as in the sh collector's ENVIRONMENT_D_VALUES_AWK program: an
// assignment being joined (pending, from line start, joined on with
// separator), the quote and escape the scan of it is inside, and the flags
// the outcome depends on.
type environmentDReader struct {
	wanted                       map[string]bool
	home                         string
	nul, continued, joined, open bool
	escape                       bool
	quote                        byte
	pending, separator           string
	start                        int
	assignments                  []Assignment
}

// ResolveEnvironmentD reads an environment.d(5) file as systemd does and
// returns an Assignment for each assignment of one of names. White space
// around name and value is trimmed (an escaped trailing blank stays); `#` and
// `;` lines are comments; an odd run of backslashes ending an assignment
// joins the next line, and a quote left open joins it with its newline (the
// Assignment keeps the first line); one pair of enclosing quotes is removed;
// `$$` is a dollar and `$HOME` or `${HOME}` is home when home is set. See
// collectors/docs/relocation.md for every reason a value is refused.
func ResolveEnvironmentD(r io.Reader, names []string, home string) ([]Assignment, error) {
	reader := environmentDReader{wanted: nameSet(names), home: home}
	err := eachLine(r, func(number int, line []byte) {
		reader.line(number, markNulBytes(line))
	})
	if reader.continued {
		reader.joined = true
		reader.open = reader.quote != 0
		reader.assign(reader.pending, reader.start)
	}
	return reader.assignments, err
}

// line is the main body of the awk program for one record.
func (reader *environmentDReader) line(number int, record string) {
	text := strings.TrimSuffix(record, "\r")
	if strings.Contains(text, "\x01") {
		reader.nul = true
	}
	if reader.continued {
		reader.pending += reader.separator + text
		reader.track(text, 0)
	} else {
		reader.start, reader.quote, reader.escape, reader.joined = number, 0, false, false
		if !assigns(text) {
			reader.assign(text, reader.start)
			return
		}
		reader.pending = text
		reader.track(text, strings.IndexByte(text, '=')+1)
	}
	switch {
	case reader.escape:
		reader.pending = reader.pending[:len(reader.pending)-1]
		reader.escape, reader.continued, reader.separator, reader.joined = false, true, "", true
	case reader.quote != 0:
		reader.continued, reader.separator, reader.joined = true, "\n", true
	default:
		reader.continued = false
		reader.assign(reader.pending, reader.start)
	}
}

// track scans text from index from for the quote and escape state the next
// line starts in: a backslash escapes the next character outside single
// quotes, and a quote opens until its match.
func (reader *environmentDReader) track(text string, from int) {
	for i := from; i < len(text); i++ {
		char := text[i]
		switch {
		case reader.escape:
			reader.escape = false
		case reader.quote == '\'':
			if char == '\'' {
				reader.quote = 0
			}
		case char == '\\':
			reader.escape = true
		case reader.quote == 0:
			if char == '"' || char == '\'' {
				reader.quote = char
			}
		case char == '"':
			reader.quote = 0
		}
	}
}

// assigns reports whether text is an assignment line: not a comment, and
// holding an `=`.
func assigns(text string) bool {
	text = strings.TrimLeft(text, " \t\r")
	return (text == "" || (text[0] != '#' && text[0] != ';')) && strings.Contains(text, "=")
}

// assign records the outcome of the assignment in text, which starts on line
// number, when it assigns one of the wanted names.
func (reader *environmentDReader) assign(text string, number int) {
	text = strings.TrimLeft(text, " \t\r")
	if text == "" || text[0] == '#' || text[0] == ';' {
		return
	}
	equals := strings.IndexByte(text, '=')
	if equals < 1 {
		return
	}
	name := strings.TrimRight(text[:equals], " \t")
	if !reader.wanted[name] {
		return
	}
	outcome := Assignment{Line: number, Name: name}
	if reader.nul {
		outcome.Reason = "after a NUL byte"
	} else {
		outcome.Value, outcome.Reason = reader.valueOf(trimValue(text[equals+1:]))
	}
	reader.assignments = append(reader.assignments, outcome)
}

// valueOf unquotes and expands a trimmed value, refusing one left open at
// the end of the file and one holding a newline.
func (reader *environmentDReader) valueOf(value string) (text, reason string) {
	if reader.open {
		return "", "quoted"
	}
	text, reason = unquote(value, reader.joined)
	if reason != "" {
		return "", reason
	}
	if strings.Contains(text, "\n") {
		return "", "holds a newline"
	}
	return expand(text, reader.home)
}

// trimValue removes the blanks around a value; a trailing blank after an odd
// run of backslashes is escaped, so its first character is kept.
func trimValue(value string) string {
	value = strings.TrimLeft(value, " \t\r")
	trimmed := strings.TrimRight(value, " \t\r")
	if trimmed == value {
		return value
	}
	blank := value[len(trimmed)]
	backslashes := len(trimmed) - len(strings.TrimRight(trimmed, `\`))
	if backslashes%2 == 1 {
		return trimmed + string(blank)
	}
	return trimmed
}

// unquote removes one enclosing pair of quotes and the backslash escapes
// systemd removes (inside double quotes only before `"`, `\`, a backquote or
// `$`); a stray quote, a single-quoted value that was joined and an unclosed
// pair are `quoted`.
func unquote(value string, joined bool) (text, reason string) {
	var first byte
	if value != "" {
		first = value[0]
	}
	switch first {
	case '\'':
		if len(value) < 2 || value[len(value)-1] != '\'' || joined ||
			strings.Contains(value[1:len(value)-1], "'") {
			return "", "quoted"
		}
		return value[1 : len(value)-1], ""
	case '"':
		if len(value) < 2 || value[len(value)-1] != '"' {
			return "", "quoted"
		}
		value = value[1 : len(value)-1]
	}
	return unescape(value, first == '"')
}

// unescape removes backslash escapes from an unquoted or double-quoted
// value, refusing a quote that is not escaped.
func unescape(value string, doubleQuoted bool) (text, reason string) {
	var result strings.Builder
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char == '\\' && i < len(value)-1 &&
			(!doubleQuoted || strings.IndexByte("\"\\`$", value[i+1]) >= 0) {
			i++
			char = value[i]
		} else if char == '"' || (char == '\'' && !doubleQuoted) {
			return "", "quoted"
		}
		result.WriteByte(char)
	}
	return result.String(), ""
}

// expand replaces `$$` by a dollar and `$HOME` or `${HOME}` by home, left to
// right. Any other reference, `${HOME:-x}` included, is refused as
// `references $NAME`; a `$` starting none, `holds a $`, once the whole
// value is read.
func expand(value, home string) (text, reason string) {
	var result strings.Builder
	literal := false
	for {
		offset := strings.IndexByte(value, '$')
		if offset < 0 {
			break
		}
		result.WriteString(value[:offset])
		rest := value[offset+1:]
		if strings.HasPrefix(rest, "$") {
			result.WriteByte('$')
			literal, value = true, rest[1:]
			continue
		}
		reference, plain, length := parseReference(rest)
		if length == 0 {
			result.WriteByte('$')
			literal, value = true, rest
			continue
		}
		if !plain || reference != "HOME" || home == "" {
			return "", "references $" + reference
		}
		result.WriteString(home)
		value = rest[length:]
	}
	if literal {
		return "", "holds a $"
	}
	return result.String() + value, ""
}

// parseReference reads the reference rest starts with after a `$`: a name,
// or `{NAME}`, `{NAME:-x}` or `{NAME:+x}`. length is 0 when there is none;
// plain is false for the forms with a default or alternative.
func parseReference(rest string) (name string, plain bool, length int) {
	if rest != "" && isNameStart(rest[0]) {
		length = wordLength(rest)
		return rest[:length], true, length
	}
	if len(rest) < 2 || rest[0] != '{' || !isNameStart(rest[1]) {
		return "", false, 0
	}
	nameEnd := 1 + wordLength(rest[1:])
	name = rest[1:nameEnd]
	if nameEnd < len(rest) && rest[nameEnd] == '}' {
		return name, true, nameEnd + 1
	}
	if nameEnd+1 < len(rest) && rest[nameEnd] == ':' && strings.IndexByte("-+", rest[nameEnd+1]) >= 0 {
		if closing := strings.IndexByte(rest[nameEnd:], '}'); closing >= 0 {
			return name, false, nameEnd + closing + 1
		}
	}
	return "", false, 0
}
