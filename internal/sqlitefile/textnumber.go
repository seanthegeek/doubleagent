package sqlitefile

import (
	"math"
	"strconv"
	"strings"
)

// realReading is what SQLite's text-to-real conversion (sqlite3AtoF)
// reads from a text: the number its longest numeric prefix holds, whether
// that prefix is the whole text (white space around it allowed), and
// whether it holds a decimal point or an exponent.
type realReading struct {
	value        float64
	whole        bool
	pointOrPower bool
}

// maxMantissa is where SQLite stops gathering digits into the number's
// 64-bit mantissa: the digits after the first that reaches it count only
// for the power of ten, so a number is read from about its first 19
// significant digits.
const maxMantissa = (math.MaxUint64 - 9) / 10

// readReal reads text as sqlite3AtoF does: ASCII white space, one sign,
// digits with an optional fraction and exponent, white space; the text
// ends at its first NUL. The value is the nearest float to the digits
// gathered times the power of ten, an infinity or zero past the range.
func readReal(text string) realReading {
	if end := strings.IndexByte(text, 0); end >= 0 {
		text = text[:end]
	}
	rest := strings.TrimLeftFunc(text, isASCIISpace)
	negative := strings.HasPrefix(rest, "-")
	if negative || strings.HasPrefix(rest, "+") {
		rest = rest[1:]
	}
	mantissa, power, digits, rest := gatherDigits(rest, 0, 0, false)
	var reading realReading
	if strings.HasPrefix(rest, ".") {
		var fraction int
		mantissa, power, fraction, rest = gatherDigits(rest[1:], mantissa, power, true)
		digits += fraction
		reading.pointOrPower = digits > 0
	}
	if digits == 0 {
		return realReading{}
	}
	if exponent, after, ok := readExponent(rest); ok {
		power, rest, reading.pointOrPower = power+exponent, after, true
	}
	reading.value = scaledFloat(mantissa, power)
	if negative {
		reading.value = -reading.value
	}
	reading.whole = strings.TrimLeftFunc(rest, isASCIISpace) == ""
	return reading
}

// gatherDigits reads the digits text starts with into mantissa while it is
// under maxMantissa, adding one to power for each integer digit past it
// and taking one off for each fraction digit kept. It returns the new
// mantissa and power, the digits read and the text after them.
func gatherDigits(text string, mantissa uint64, power int, fraction bool) (uint64, int, int, string) {
	digits := 0
	for digits < len(text) && isDigit(text[digits]) {
		full := mantissa >= maxMantissa
		switch {
		case !full:
			mantissa = mantissa*10 + uint64(text[digits]-'0')
			if fraction {
				power--
			}
		case !fraction:
			power++
		}
		digits++
	}
	return mantissa, power, digits, text[digits:]
}

// maxExponent is where SQLite stops reading an exponent's digits: one that
// reaches it reads as it, which is past the range of a float either way.
const maxExponent = 10000

// readExponent reads "e" or "E", an optional sign and at least one digit
// from the start of text, and returns the exponent and the text after it.
func readExponent(text string) (int, string, bool) {
	if text == "" || text[0]|0x20 != 'e' {
		return 0, text, false
	}
	rest, sign := text[1:], 1
	if strings.HasPrefix(rest, "-") {
		rest, sign = rest[1:], -1
	} else if strings.HasPrefix(rest, "+") {
		rest = rest[1:]
	}
	if rest == "" || !isDigit(rest[0]) {
		return 0, text, false
	}
	exponent := 0
	for rest != "" && isDigit(rest[0]) {
		if exponent < maxExponent {
			exponent = exponent*10 + int(rest[0]-'0')
		} else {
			exponent = maxExponent
		}
		rest = rest[1:]
	}
	return sign * exponent, rest, true
}

// scaledFloat returns the float nearest mantissa times ten to the power,
// an infinity or zero when that is past the range.
func scaledFloat(mantissa uint64, power int) float64 {
	if mantissa == 0 {
		return 0
	}
	number, _ := strconv.ParseFloat(strconv.FormatUint(mantissa, 10)+"e"+strconv.Itoa(power), 64)
	return number
}

// isASCIISpace reports whether char is one of the white space characters
// SQLite skips around a number: space, tab, newline, vertical tab, form
// feed and carriage return.
func isASCIISpace(char rune) bool {
	return char < 0x80 && isSpace(byte(char))
}

// readInteger reads text as SQLite's text-to-integer conversion
// (sqlite3Atoi64) does: ASCII white space, one sign and the digits that
// follow, the nearer end of the 64-bit range for a number past it, 0 when
// there are none. fits is false only when the digits are past the range
// (2^63 itself fits only negated); whole is true when they also fit and
// nothing but white space follows them.
func readInteger(text string) (value int64, fits, whole bool) {
	rest := strings.TrimLeftFunc(text, isASCIISpace)
	negative := strings.HasPrefix(rest, "-")
	if negative || strings.HasPrefix(rest, "+") {
		rest = rest[1:]
	}
	end := 0
	for end < len(rest) && isDigit(rest[end]) {
		end++
	}
	whole = end > 0 && strings.TrimLeftFunc(rest[end:], isASCIISpace) == ""
	digits := strings.TrimLeft(rest[:end], "0")
	if digits == "" {
		return 0, true, whole
	}
	magnitude, err := strconv.ParseUint(digits, 10, 64)
	switch {
	case negative && (err != nil || magnitude > 1<<63):
		return math.MinInt64, false, false
	case negative:
		return int64(-magnitude), true, whole
	case err != nil || magnitude >= 1<<63:
		return math.MaxInt64, false, false
	}
	return int64(magnitude), true, whole
}

// realToInteger converts a float as SQLite does where it must make an
// integer of one (sqlite3RealToI64): the fraction dropped, and a float past
// the 64-bit range, or within 1024 of its ends, the nearer end.
func realToInteger(number float64) int64 {
	switch {
	case number < -9223372036854774784:
		return math.MinInt64
	case number > 9223372036854774784:
		return math.MaxInt64
	case math.IsNaN(number):
		return 0
	}
	return int64(number)
}

// realSameAsInteger reports whether integer, converted from number, is the
// same value, for SQLite's purposes (sqlite3RealSameAsInt): number is zero,
// or the two are equal and integer is within 2^51 of zero.
func realSameAsInteger(number float64, integer int64) bool {
	return number == 0 || (number == float64(integer) &&
		integer >= -2251799813685248 && integer < 2251799813685248)
}
