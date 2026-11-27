package gitmeta

import (
	"strings"
	"unicode/utf8"
)

// header is a section header that was read: its name lowered, its quoted
// subsection with escapes resolved, and the offset just past its ].
type header struct {
	section, subsection string
	quoted              bool
	end                 int
}

// remote returns the remote a header opens and true, or false for any
// other section: [remote "<name>"] keeps the name's case, and the dotted
// [remote.<name>] has it lowered, as git lowers it. git names a remote by
// everything between "remote." and the key, so [remote.<a> "<b>"] opens
// the remote <a>.<b>. [remote.] and [remote ""] both open the remote whose
// name is empty, as git reads them.
func (h header) remote() (string, bool) {
	if h.quoted && h.section == "remote" {
		return h.subsection, true
	}
	name, ok := strings.CutPrefix(h.section, "remote.")
	if h.quoted {
		name += "." + h.subsection
	}
	return name, ok
}

// readSubsection reads a quoted subsection from offset start, just past
// its opening quote, to its closing quote, a backslash keeping the
// character after it; it returns the text and the offset past the quote.
func readSubsection(line string, start int) (string, int, bool) {
	var text strings.Builder
	for position := start; position < len(line); {
		switch line[position] {
		case '"':
			return text.String(), position + 1, true
		case '\\':
			if position+1 >= len(line) {
				return "", 0, false
			}
			_, width := utf8.DecodeRuneInString(line[position+1:])
			text.WriteString(line[position+1 : position+1+width])
			position += 1 + width
		default:
			text.WriteByte(line[position])
			position++
		}
	}
	return "", 0, false
}

// isDigit reports whether char is an ASCII digit.
func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

// isNameByte reports whether char may appear in a key name (ASCII
// letters, digits and hyphens) or, with section set, in a section name,
// which may also hold points.
func isNameByte(char byte, section bool) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || isDigit(char) ||
		char == '-' || (section && char == '.')
}

// readHeader reads a section header at the start of line: [, then a name
// of letters, digits, points and hyphens, then ] or a quoted subsection
// after white space and then ]. git allows white space only between the
// name and the subsection, so any other character, or white space anywhere
// else, makes the header malformed and false is returned.
func readHeader(line string) (header, bool) {
	nameEnd := 1
	for nameEnd < len(line) && isNameByte(line[nameEnd], true) {
		nameEnd++
	}
	if nameEnd == 1 {
		return header{}, false
	}
	found := header{section: strings.ToLower(line[1:nameEnd])}
	position := nameEnd
	if quoteStart := len(line) - len(strings.TrimLeft(line[nameEnd:], gitSpace)); quoteStart > nameEnd {
		if quoteStart >= len(line) || line[quoteStart] != '"' {
			return header{}, false
		}
		subsection, after, ok := readSubsection(line, quoteStart+1)
		if !ok {
			return header{}, false
		}
		found.subsection, found.quoted, position = subsection, true, after
	}
	if position >= len(line) || line[position] != ']' {
		return header{}, false
	}
	found.end = position + 1
	return found, true
}

// skipBlanks returns the offset of the first byte of line at or after
// position that is not a space or tab.
func skipBlanks(line string, position int) int {
	for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
		position++
	}
	return position
}

// readKey reads a pair at the start of line: a key of a letter then
// letters, digits and hyphens, optional spaces or tabs, then = and the
// value's raw text. It returns the key lowered and the raw value, and
// false for a key without = (a boolean, never a URL) and for any other
// line.
func readKey(line string) (string, string, bool) {
	if line == "" || !isNameByte(line[0], false) || isDigit(line[0]) || line[0] == '-' {
		return "", "", false
	}
	end := 1
	for end < len(line) && isNameByte(line[end], false) {
		end++
	}
	position := skipBlanks(line, end)
	if position >= len(line) || line[position] != '=' {
		return "", "", false
	}
	return strings.ToLower(line[:end]), line[position+1:], true
}

// valueText is a value being read: its bytes, whether quotes or an escape
// protect each from the trim, and whether a quote is open.
type valueText struct {
	bytes     []byte
	protected []bool
	quoted    bool
}

// escapeOf returns the character an escape stands for, and false for a
// character git defines no escape for.
func escapeOf(char byte) (byte, bool) {
	switch char {
	case '\\', '"':
		return char, true
	case 'n':
		return '\n', true
	case 't':
		return '\t', true
	case 'b':
		return '\b', true
	}
	return 0, false
}

// scan adds one line's part of a value, stopping at a comment outside
// quotes. It reports whether a backslash ends the line, and the reason to
// reject an escape git does not define.
func (v *valueText) scan(text string) (bool, string) {
	for position := 0; position < len(text); position++ {
		switch char := text[position]; {
		case char == '\\' && position+1 == len(text):
			return true, ""
		case char == '\\':
			escaped, ok := escapeOf(text[position+1])
			if !ok {
				_, width := utf8.DecodeRuneInString(text[position+1:])
				return false, "unsupported escape \\" + text[position+1:position+1+width]
			}
			v.bytes, v.protected = append(v.bytes, escaped), append(v.protected, true)
			position++
		case char == '"':
			v.quoted = !v.quoted
		case (char == '#' || char == ';') && !v.quoted:
			return false, ""
		default:
			v.bytes, v.protected = append(v.bytes, char), append(v.protected, v.quoted)
		}
	}
	return false, ""
}

// isSpace reports whether char is git's white space.
func isSpace(char byte) bool {
	return strings.IndexByte(gitSpace, char) >= 0
}

// trimmed returns the value without the unprotected whitespace around it,
// in one pass from each end, since a value padded with kilobytes of
// spaces must cost no more than its length.
func (v *valueText) trimmed() string {
	first, last := 0, len(v.bytes)
	for first < last && !v.protected[first] && isSpace(v.bytes[first]) {
		first++
	}
	for last > first && !v.protected[last-1] && isSpace(v.bytes[last-1]) {
		last--
	}
	return string(v.bytes[first:last])
}

// gitSpace is git's white space: space, tab, CR and LF, not form feed or
// vertical tab.
const gitSpace = " \t\r\n"

// readValue resolves a value that starts with text on line index: quotes
// and escapes resolved, a trailing comment removed and only unprotected
// whitespace around it trimmed. A backslash ending the line outside a
// comment continues the value on the next line; one ending the file is
// dropped. It returns the value, the index of its last line, and git's
// reason to reject it, "" when accepted; a rejected value is "".
func readValue(lines []string, index int, text string) (string, int, string) {
	var value valueText
	for {
		continued, reason := value.scan(text)
		if reason != "" {
			return "", index, reason
		}
		if !continued || index+1 >= len(lines) {
			break
		}
		index++
		text = lines[index]
	}
	if value.quoted {
		// A quote still open at the end is a value the file ends inside:
		// git rejects the line, and the text so far is not the destination.
		return "", index, "unterminated quoted value"
	}
	return value.trimmed(), index, ""
}
