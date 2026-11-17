package pytext

import "strings"

// asciiWhitespace is the set bytes.strip() removes: Python's ASCII
// whitespace.
const asciiWhitespace = " \t\n\r\f\v"

// Strip returns text without the leading and trailing characters Python's
// argument-less str.strip() removes: ASCII whitespace, the separators
// U+001C to U+001F, U+0085, U+00A0, U+1680, U+2000 to U+200A, U+2028,
// U+2029, U+202F, U+205F and U+3000. U+200B and U+FEFF stay.
func Strip(text string) string {
	return strings.TrimFunc(text, isPythonSpace)
}

// StripLeft returns text without the leading characters Strip removes, as
// str.lstrip() does.
func StripLeft(text string) string {
	return strings.TrimLeftFunc(text, isPythonSpace)
}

// StripRight returns text without the trailing characters Strip removes, as
// str.rstrip() does.
func StripRight(text string) string {
	return strings.TrimRightFunc(text, isPythonSpace)
}

// StripASCII returns text without leading and trailing ASCII whitespace
// (space, tab, LF, CR, form feed, vertical tab), as bytes.strip() does.
func StripASCII(text string) string {
	return strings.Trim(text, asciiWhitespace)
}

// isPythonSpace reports whether str.isspace() holds for char: the
// characters whose Unicode category is Zs or whose bidirectional class is
// WS, B or S.
func isPythonSpace(char rune) bool {
	switch {
	case char <= 0x20:
		return char == ' ' || (char >= '\t' && char <= '\r') || (char >= 0x1C && char <= 0x1F)
	case char < 0x2000:
		return char == 0x85 || char == 0xA0 || char == 0x1680
	case char <= 0x200A:
		return true
	}
	switch char {
	case 0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return true
	}
	return false
}

// Fields returns the pieces of text between runs of the whitespace Strip
// removes, none of them empty, as Python's argument-less str.split() does.
func Fields(text string) []string {
	return strings.FieldsFunc(text, isPythonSpace)
}

// IsSpace reports whether char is whitespace as Python's str.isspace() has
// it, the set Strip and Fields use.
func IsSpace(char rune) bool {
	return isPythonSpace(char)
}
