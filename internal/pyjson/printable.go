package pyjson

import "unicode"

// isPrintable reports whether Python 3.14's str.isprintable() holds for
// char: letters, marks, numbers, punctuation, symbols and the ASCII space,
// by Unicode 16.0. Go's tables are Unicode 17.0 (unicode.Version, checked
// by a test), so the characters 17.0 assigned are taken out again.
func isPrintable(char rune) bool {
	if !unicode.IsPrint(char) {
		return false
	}
	for _, added := range unicode17Additions() {
		if char >= added[0] && char <= added[1] {
			return false
		}
		if char < added[0] {
			break
		}
	}
	return true
}

// unicode17Additions returns, in order, the ranges of printable characters
// Unicode 17.0 assigned, which Unicode 16.0 left unassigned. Python 3.14
// printed them by comparing its isprintable() with Go's unicode.IsPrint
// over every code point.
func unicode17Additions() [47][2]rune {
	return [47][2]rune{
		{0x088F, 0x088F},
		{0x0C5C, 0x0C5C},
		{0x0CDC, 0x0CDC},
		{0x1ACF, 0x1ADD},
		{0x1AE0, 0x1AEB},
		{0x20C1, 0x20C1},
		{0x2B96, 0x2B96},
		{0xA7CE, 0xA7CF},
		{0xA7D2, 0xA7D2},
		{0xA7D4, 0xA7D4},
		{0xA7F1, 0xA7F1},
		{0xFBC3, 0xFBD2},
		{0xFD90, 0xFD91},
		{0xFDC8, 0xFDCE},
		{0x10940, 0x10959},
		{0x10EC5, 0x10EC7},
		{0x10ED0, 0x10ED8},
		{0x10EFA, 0x10EFB},
		{0x11B60, 0x11B67},
		{0x11DB0, 0x11DDB},
		{0x11DE0, 0x11DE9},
		{0x16EA0, 0x16EB8},
		{0x16EBB, 0x16ED3},
		{0x16FF2, 0x16FF6},
		{0x187F8, 0x187FF},
		{0x18D09, 0x18D1E},
		{0x18D80, 0x18DF2},
		{0x1CCFA, 0x1CCFC},
		{0x1CEBA, 0x1CED0},
		{0x1CEE0, 0x1CEF0},
		{0x1E6C0, 0x1E6DE},
		{0x1E6E0, 0x1E6F5},
		{0x1E6FE, 0x1E6FF},
		{0x1F6D8, 0x1F6D8},
		{0x1F777, 0x1F77A},
		{0x1F8D0, 0x1F8D8},
		{0x1FA54, 0x1FA57},
		{0x1FA8A, 0x1FA8A},
		{0x1FA8E, 0x1FA8E},
		{0x1FAC8, 0x1FAC8},
		{0x1FACD, 0x1FACD},
		{0x1FAEA, 0x1FAEA},
		{0x1FAEF, 0x1FAEF},
		{0x1FBFA, 0x1FBFA},
		{0x2B73A, 0x2B73F},
		{0x2CEA2, 0x2CEAD},
		{0x323B0, 0x33479},
	}
}
