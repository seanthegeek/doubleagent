package pytext

import (
	"unicode/utf8"
)

// DecodeUTF8 returns data decoded as Python's bytes.decode("utf-8",
// "replace") decodes it: each maximal invalid sequence (the longest start
// of a well-formed sequence, or one byte that starts none) becomes one
// U+FFFD, so E2 82 followed by "A" gives U+FFFD "A", and the three bytes
// ED A0 80 of an encoded surrogate give three.
func DecodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	out := make([]byte, 0, len(data)+8)
	for i := 0; i < len(data); {
		size, valid := sequenceLength(data[i:])
		if valid {
			out = append(out, data[i:i+size]...)
		} else {
			out = utf8.AppendRune(out, utf8.RuneError)
		}
		i += size
	}
	return string(out)
}

// sequenceLength returns how many bytes of data, which is not empty, form
// its first sequence and whether that sequence is well formed. An invalid
// sequence is the lead byte and the continuation bytes that fit it, at
// least one byte. The second-byte ranges are those of the Unicode
// Standard's table of well-formed UTF-8 (chapter 3, table 3-7).
func sequenceLength(data []byte) (int, bool) {
	lead := data[0]
	var size int
	low, high := byte(0x80), byte(0xBF)
	switch {
	case lead < 0x80:
		return 1, true
	case lead >= 0xC2 && lead <= 0xDF:
		size = 2
	case lead == 0xE0:
		size, low = 3, 0xA0
	case lead == 0xED:
		size, high = 3, 0x9F
	case lead >= 0xE1 && lead <= 0xEF:
		size = 3
	case lead == 0xF0:
		size, low = 4, 0x90
	case lead == 0xF4:
		size, high = 4, 0x8F
	case lead >= 0xF1 && lead <= 0xF3:
		size = 4
	default:
		return 1, false
	}
	for i := 1; i < size; i++ {
		if i >= len(data) || data[i] < low || data[i] > high {
			return i, false
		}
		low, high = 0x80, 0xBF
	}
	return size, true
}

// NextCodePoint returns the first code point of text, a Python str as the
// package comment describes, and its width in bytes: a lone surrogate's
// three-byte form is one code point, and any other invalid byte is one
// code point read as U+FFFD with width 1. Empty text gives (U+FFFD, 0).
func NextCodePoint(text string) (rune, int) {
	if text != "" && text[0] < utf8.RuneSelf {
		return rune(text[0]), 1
	}
	char, width := utf8.DecodeRuneInString(text)
	if char != utf8.RuneError || width != 1 {
		return char, width
	}
	if isSurrogateForm(text) {
		return 0xD000 | rune(text[1]&0x3F)<<6 | rune(text[2]&0x3F), 3
	}
	return utf8.RuneError, width
}

// isSurrogateForm reports whether text starts with the three-byte form of a
// surrogate: ED, then A0 to BF, then a continuation byte.
func isSurrogateForm(text string) bool {
	return len(text) >= 3 && text[0] == 0xED && text[1] >= 0xA0 && text[1] <= 0xBF &&
		text[2] >= 0x80 && text[2] <= 0xBF
}

// AppendCodePoint appends the code point char to out, writing a surrogate
// in its three-byte form, which utf8.AppendRune would replace.
func AppendCodePoint(out []byte, char rune) []byte {
	if char >= 0xD800 && char <= 0xDFFF {
		return append(out, 0xED, byte(0x80|(char>>6)&0x3F), byte(0x80|char&0x3F))
	}
	return utf8.AppendRune(out, char)
}

// HasLoneSurrogate reports whether text holds a lone surrogate, which
// Python cannot write to a UTF-8 file.
func HasLoneSurrogate(text string) bool {
	for i := 0; i+2 < len(text); i++ {
		if isSurrogateForm(text[i:]) {
			return true
		}
	}
	return false
}
