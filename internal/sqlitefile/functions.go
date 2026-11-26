package sqlitefile

import (
	"bytes"
)

// The SQL functions parsers use in WHERE clauses and select lists, after
// "Built-In Scalar SQL Functions" (https://www.sqlite.org/lang_corefunc.html)
// and LIKE ("The LIKE, GLOB, REGEXP, MATCH, and extract operators" in
// https://www.sqlite.org/lang_expr.html). Each reads a number as its text
// and a blob's bytes as text where SQLite does. A character is a byte, and
// for a byte from 0xC0 up also the continuation bytes (0x80 to 0xBF) after
// it: SQLite's count, which reads invalid UTF-8 without error.

// textOf returns the text SQLite gives a value in a text function, and
// false for NULL.
func textOf(value Value) ([]byte, bool) {
	switch value.Type {
	case Null:
		return nil, false
	case Integer, Float:
		return []byte(numberText(value)), true
	}
	return value.Bytes, true
}

// untilNul cuts text at its first NUL byte: the functions that read text
// as a C string stop there.
func untilNul(text []byte) []byte {
	if end := bytes.IndexByte(text, 0); end >= 0 {
		return text[:end]
	}
	return text
}

// characterLength returns the length in bytes of the character at the
// start of text.
func characterLength(text []byte) int {
	length := 1
	if text[0] >= 0xc0 {
		for length < len(text) && text[length]&0xc0 == 0x80 {
			length++
		}
	}
	return length
}

// Length returns SQL's length(value): NULL for NULL, a blob's bytes, and
// for text (or a number's text) the characters before the first NUL.
func Length(value Value) Value {
	text, ok := textOf(value)
	switch {
	case !ok:
		return Value{}
	case value.Type == Blob:
		return IntValue(int64(len(text)))
	}
	text = untilNul(text)
	count := int64(0)
	for len(text) > 0 {
		text = text[characterLength(text):]
		count++
	}
	return IntValue(count)
}

// Substr returns SQL's substr(value, start, count): count characters (bytes
// for a blob) from the start-th, the first being 1; a negative start counts
// from the end and a negative count takes the characters before start. The
// result is a blob for a blob and text otherwise, NULL for NULL and for an
// empty blob; text ends at its first NUL.
func Substr(value Value, start, count int64) Value {
	text, ok := textOf(value)
	if !ok || value.Type == Blob && len(text) == 0 {
		return Value{}
	}
	isBlob := value.Type == Blob
	if !isBlob {
		text = untilNul(text)
	}
	length := int64(len(text))
	if !isBlob && start < 0 {
		length = Length(TextValue(string(text))).Int
	}
	first, taken := substrRange(start, count, length)
	if isBlob {
		first = min(first, length)
		taken = min(taken, length-first)
		return BlobValue(bytes.Clone(text[first : first+taken]))
	}
	for ; first > 0 && len(text) > 0; first-- {
		text = text[characterLength(text):]
	}
	end := 0
	for ; taken > 0 && end < len(text); taken-- {
		end += characterLength(text[end:])
	}
	return TextValue(string(text[:end]))
}

// substrRange turns substr's start and count into the number of characters
// to skip and to take, after SQLite's own arithmetic.
func substrRange(start, count, length int64) (int64, int64) {
	negative := count < 0
	if negative {
		count = -count
	}
	switch {
	case start < 0:
		start += length
		if start < 0 {
			count = max(count+start, 0)
			start = 0
		}
	case start > 0:
		start--
	case count > 0:
		count--
	}
	if negative {
		start -= count
		if start < 0 {
			count += start
			start = 0
		}
	}
	return start, count
}

// Instr returns SQL's instr(haystack, needle): the character position,
// from 1, of needle's first occurrence, counted in bytes when both are
// blobs, or 0 when it does not occur. Where SQL returns NULL, for a NULL
// argument, Instr returns 0, which tests the same in "instr(...) > 0".
func Instr(haystack, needle Value) int64 {
	text, ok := textOf(haystack)
	pattern, patternOk := textOf(needle)
	if !ok || !patternOk {
		return 0
	}
	if len(pattern) == 0 {
		return 1
	}
	isText := haystack.Type != Blob || needle.Type != Blob
	position := int64(1)
	for len(pattern) <= len(text) {
		if bytes.HasPrefix(text, pattern) {
			return position
		}
		position++
		text = text[1:]
		for isText && len(text) > 0 && text[0]&0xc0 == 0x80 {
			text = text[1:]
		}
	}
	return 0
}

// Like reports whether SQL's "value LIKE pattern" holds: % matches any run
// of characters and _ one character, ASCII letters match in either case and
// every other character only itself. Both sides end at a NUL, and NULL
// matches nothing. No escape character is supported.
func Like(value Value, pattern string) bool {
	text, ok := textOf(value)
	if !ok {
		return false
	}
	return likeMatch(untilNul([]byte(pattern)), untilNul(text))
}

// likeMatch matches text against a LIKE pattern, going back to the last %
// when a character does not match.
func likeMatch(pattern, text []byte) bool {
	step, offset := 0, 0
	resumePattern, resumeText := -1, 0
	for offset < len(text) {
		if step < len(pattern) {
			wanted, wantedLength := readCharacter(pattern[step:])
			if wanted == '%' {
				step += wantedLength
				resumePattern, resumeText = step, offset
				continue
			}
			got, gotLength := readCharacter(text[offset:])
			if wanted == '_' || sameCharacter(wanted, got) {
				step, offset = step+wantedLength, offset+gotLength
				continue
			}
		}
		if resumePattern < 0 {
			return false
		}
		_, skipped := readCharacter(text[resumeText:])
		resumeText += skipped
		step, offset = resumePattern, resumeText
	}
	for step < len(pattern) && pattern[step] == '%' {
		step++
	}
	return step == len(pattern)
}

// sameCharacter compares two characters, ASCII letters in either case.
func sameCharacter(first, second uint32) bool {
	if first < 0x80 && second < 0x80 {
		return lowerASCII(first) == lowerASCII(second)
	}
	return first == second
}

// lowerASCII lowers an ASCII capital: the fold of textlines.LowerByte for a
// decoded character (a uint32) rather than a byte.
func lowerASCII(char uint32) uint32 {
	if char >= 'A' && char <= 'Z' {
		return char + 'a' - 'A'
	}
	return char
}

// readCharacter decodes the character at the start of text as SQLite's
// UTF-8 reader does: a lead byte from 0xC0 takes every continuation byte
// after it, and a result that is not a valid code point (overlong, a
// surrogate, U+FFFE or U+FFFF) reads as U+FFFD. It returns the code point
// and the character's length.
func readCharacter(text []byte) (uint32, int) {
	char := uint32(text[0])
	length := characterLength(text)
	if char < 0xc0 {
		return char, 1
	}
	char = leadBits(byte(char))
	for _, octet := range text[1:length] {
		char = char<<6 + uint32(octet&0x3f)
	}
	if char < 0x80 || char&0xfffff800 == 0xd800 || char&0xfffffffe == 0xfffe {
		char = 0xfffd
	}
	return char, length
}

// leadBits returns the payload bits of a UTF-8 lead byte from 0xC0, as
// SQLite's table of them gives it (0 for 0xFE and 0xFF).
func leadBits(lead byte) uint32 {
	switch {
	case lead < 0xe0:
		return uint32(lead - 0xc0)
	case lead < 0xf0:
		return uint32(lead - 0xe0)
	case lead < 0xf8:
		return uint32(lead - 0xf0)
	case lead < 0xfc:
		return uint32(lead - 0xf8)
	case lead < 0xfe:
		return uint32(lead - 0xfc)
	}
	return 0
}
