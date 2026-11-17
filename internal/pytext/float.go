package pytext

import (
	"math"
	"strconv"
	"strings"
)

// None is Python's text for None, as str(None) and %s write it.
const None = "None"

// Bool returns Python's text for a bool: True or False.
func Bool(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

// Float returns repr(value) as Python writes it: the shortest digits that
// read back as the same float, in positional form with at least one digit
// after the point (1.0, 0.0001) while the decimal exponent lies in -4 to 15,
// else in exponent form with a signed exponent of at least two digits
// (1e+16, 1e-05, 1.5e+300); nan, inf and -inf for the special values.
func Float(value float64) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(value) {
		sign = "-"
		value = -value
	}
	if value == 0 {
		return sign + "0.0"
	}
	digits, point := shortestDigits(value)
	if point <= -4 || point > 16 {
		return sign + exponentForm(digits, point-1)
	}
	return sign + positionalForm(digits, point)
}

// shortestDigits returns the shortest decimal digits that read back as
// value (positive, finite, not zero) and the position of the decimal point
// counted from the left of those digits: 1.5 is ("15", 1), 0.001 ("1", -2).
func shortestDigits(value float64) (string, int) {
	text := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponentText, _ := strings.Cut(text, "e")
	exponent, _ := strconv.Atoi(exponentText)
	return strings.Replace(mantissa, ".", "", 1), exponent + 1
}

// exponentForm writes digits as d.ddd followed by e, the exponent's sign
// and at least two exponent digits.
func exponentForm(digits string, exponent int) string {
	var out strings.Builder
	out.WriteString(digits[:1])
	if len(digits) > 1 {
		out.WriteByte('.')
		out.WriteString(digits[1:])
	}
	out.WriteByte('e')
	if exponent < 0 {
		out.WriteByte('-')
		exponent = -exponent
	} else {
		out.WriteByte('+')
	}
	if exponent < 10 {
		out.WriteByte('0')
	}
	out.WriteString(strconv.Itoa(exponent))
	return out.String()
}

// positionalForm writes digits with the decimal point at point, padding
// with zeros on either side and adding ".0" when no fraction remains.
func positionalForm(digits string, point int) string {
	switch {
	case point <= 0:
		return "0." + strings.Repeat("0", -point) + digits
	case point >= len(digits):
		return digits + strings.Repeat("0", point-len(digits)) + ".0"
	}
	return digits[:point] + "." + digits[point:]
}
