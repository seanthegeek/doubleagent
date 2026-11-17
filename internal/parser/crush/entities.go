package crush

import (
	"strings"
	"unicode/utf8"
)

// The character references of HTML 5 that unescapeHTML replaces, after
// "Character references" in the HTML standard
// (https://html.spec.whatwg.org/multipage/syntax.html#character-references).
// Of the named references only the five XML ones are known, the escapes an
// attribute value written by an HTML or XML escaper holds; amp, lt, gt and
// quot are also known without their semicolon, as the standard's legacy
// names. Any other name is left as written.
var namedReferences = map[string]string{
	"amp;": "&", "lt;": "<", "gt;": ">", "quot;": "\"", "apos;": "'",
	"AMP;": "&", "LT;": "<", "GT;": ">", "QUOT;": "\"",
	"amp": "&", "lt": "<", "gt": ">", "quot": "\"",
	"AMP": "&", "LT": "<", "GT": ">", "QUOT": "\"",
}

// windowsReferences are the numeric references the standard maps to
// another character: NUL, CR, and the C1 controls read as Windows-1252.
var windowsReferences = map[int]rune{
	0x00: '�', 0x0d: '\r', 0x80: '€', 0x81: '\u0081', 0x82: '‚',
	0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
	0x88: 'ˆ', 0x89: '‰', 0x8a: 'Š', 0x8b: '‹', 0x8c: 'Œ',
	0x8d: '\u008d', 0x8e: 'Ž', 0x8f: '\u008f', 0x90: '\u0090', 0x91: '‘',
	0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–',
	0x97: '—', 0x98: '˜', 0x99: '™', 0x9a: 'š', 0x9b: '›',
	0x9c: 'œ', 0x9d: '\u009d', 0x9e: 'ž', 0x9f: 'Ÿ',
}

// maxNameLength is the most characters a named reference is read over.
const maxNameLength = 32

// unescapeHTML replaces the character references of text: decimal and hex
// numeric references, with or without their semicolon, and the named ones
// namedReferences knows, the longest legacy name that starts a longer
// unknown name included. A reference it cannot read stays as written.
func unescapeHTML(text string) string {
	if !strings.Contains(text, "&") {
		return text
	}
	var out strings.Builder
	for {
		found := strings.IndexByte(text, '&')
		if found < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:found])
		replacement, used := reference(text[found+1:])
		if used == 0 {
			out.WriteByte('&')
			text = text[found+1:]
			continue
		}
		out.WriteString(replacement)
		text = text[found+1+used:]
	}
}

// reference reads the reference that follows an ampersand and returns its
// replacement and how many bytes it took, 0 when nothing there is one.
func reference(text string) (string, int) {
	if strings.HasPrefix(text, "#") {
		return numericReference(text)
	}
	end, count := 0, 0
	for end < len(text) && count < maxNameLength {
		char, size := utf8.DecodeRuneInString(text[end:])
		if strings.ContainsRune("\t\n\f <&#;", char) {
			break
		}
		end += size
		count++
	}
	if end == 0 {
		return "", 0
	}
	name := text[:end]
	if strings.HasPrefix(text[end:], ";") {
		name = text[:end+1]
	}
	if replacement, ok := namedReferences[name]; ok {
		return replacement, len(name)
	}
	for cut := len(name) - 1; cut > 1; cut-- {
		if replacement, ok := namedReferences[name[:cut]]; ok {
			return replacement + name[cut:], len(name)
		}
	}
	return "&" + name, len(name)
}

// numericReference reads "#<digits>" or "#x<hex digits>", each with an
// optional semicolon, and returns the character it stands for.
func numericReference(text string) (string, int) {
	digits, base, start := "0123456789", 10, 1
	if len(text) > 1 && (text[1] == 'x' || text[1] == 'X') {
		digits, base, start = "0123456789abcdefABCDEF", 16, 2
	}
	end := start
	number, overflow := 0, false
	for end < len(text) && strings.IndexByte(digits, text[end]) >= 0 {
		if !overflow {
			number = number*base + hexValue(text[end])
			overflow = number > utf8.MaxRune
		}
		end++
	}
	if end == start {
		return "", 0
	}
	if end < len(text) && text[end] == ';' {
		end++
	}
	switch char, mapped := windowsReferences[number]; {
	case overflow || number >= 0xd800 && number <= 0xdfff:
		return "�", end
	case mapped:
		return string(char), end
	case invalidCodePoint(number):
		return "", end
	}
	return string(rune(number)), end
}

// hexValue returns the value of a decimal or hex digit.
func hexValue(digit byte) int {
	switch {
	case digit >= 'a':
		return int(digit-'a') + 10
	case digit >= 'A':
		return int(digit-'A') + 10
	}
	return int(digit - '0')
}

// invalidCodePoint reports whether a numeric reference to number gives
// nothing: the C0 and C1 controls other than the whitespace ones, DEL, and
// the noncharacters (U+FDD0 to U+FDEF and the last two of every plane).
func invalidCodePoint(number int) bool {
	switch {
	case number >= 0x01 && number <= 0x08, number == 0x0b, number >= 0x0e && number <= 0x1f,
		number >= 0x7f && number <= 0x9f, number >= 0xfdd0 && number <= 0xfdef:
		return true
	}
	return number&0xfffe == 0xfffe
}
