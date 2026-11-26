package sqlitefile

import (
	"math"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// Affinity is a column's preferred storage class ("Type Affinity" in
// https://www.sqlite.org/datatype3.html), which decides how a value is
// converted when it is compared with the column, and that a REAL column's
// integers read as floats.
type Affinity uint8

// The five affinities. BlobAffinity converts nothing.
const (
	BlobAffinity Affinity = iota
	TextAffinity
	NumericAffinity
	IntegerAffinity
	RealAffinity
)

// String names the affinity as the documentation does.
func (a Affinity) String() string {
	return [...]string{"BLOB", "TEXT", "NUMERIC", "INTEGER", "REAL"}[a]
}

// numeric reports whether the affinity is NUMERIC, INTEGER or REAL.
func (a Affinity) numeric() bool {
	return a >= NumericAffinity
}

// affinityOf returns the affinity of a declared type, after the five rules
// of "Determination Of Column Affinity", applied in order. A STRICT table's
// ANY column keeps every value as given, which is BLOB affinity.
func affinityOf(declared string, strict bool) Affinity {
	upper := textlines.UpperASCII(declared)
	switch {
	case strings.Contains(upper, "INT"):
		return IntegerAffinity
	case strings.Contains(upper, "CHAR") || strings.Contains(upper, "CLOB") ||
		strings.Contains(upper, "TEXT"):
		return TextAffinity
	case strings.Contains(upper, "BLOB") || upper == "":
		return BlobAffinity
	case strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") ||
		strings.Contains(upper, "DOUB"):
		return RealAffinity
	case strict && upper == "ANY":
		return BlobAffinity
	}
	return NumericAffinity
}

// applyAffinity converts value as SQLite does before storing it in, or
// comparing it with, a column of affinity ("Type Conversions Prior To
// Comparison"): TEXT turns numbers into text; NUMERIC and INTEGER turn
// well-formed numeric text into a number, an integer when it is exactly
// one; REAL does the same and keeps the number a float. Blobs and NULL
// never change.
func applyAffinity(value Value, affinity Affinity) Value {
	switch {
	case affinity == TextAffinity && (value.Type == Integer || value.Type == Float):
		return TextValue(numberText(value))
	case affinity.numeric() && value.Type == Text:
		if number, ok := parseNumeric(string(value.Bytes)); ok {
			value = number
		}
	case affinity.numeric() && value.Type == Float:
		value = integerByAffinity(value)
	}
	if affinity == RealAffinity && value.Type == Integer {
		return FloatValue(float64(value.Int))
	}
	return value
}

// parseNumeric reads text as NUMERIC affinity does (applyNumericAffinity):
// only when readReal reads all of it as one number, an integer when the
// text has no decimal point or exponent and is one that fits 64 bits, else
// a float, an Integer when integerByAffinity makes one of it.
func parseNumeric(text string) (Value, bool) {
	reading := readReal(text)
	if !reading.whole {
		return Value{}, false
	}
	if !reading.pointOrPower {
		if integer := realToInteger(reading.value); realSameAsInteger(reading.value, integer) {
			return IntValue(integer), true
		}
		if integer, _, whole := readInteger(text); whole {
			return IntValue(integer), true
		}
	}
	return integerByAffinity(FloatValue(reading.value)), true
}

// integerByAffinity returns a Float as the Integer that NUMERIC and
// INTEGER affinity make of it: a whole number strictly between the least
// and the greatest 64-bit integers, as SQLite's integer affinity tests the
// range (sqlite3VdbeIntegerAffinity). The least, -2^63, stays a Float; no
// Float equals the greatest. Any other value is returned unchanged.
func integerByAffinity(value Value) Value {
	if value.Type == Float && value.Float == math.MinInt64 {
		return value
	}
	return integerIfExact(value)
}

// integerIfExact returns a Float that is exactly a 64-bit integer as that
// Integer, -2^63 included, and any other value unchanged.
func integerIfExact(value Value) Value {
	if value.Type != Float {
		return value
	}
	number := value.Float
	if number >= -9223372036854775808 && number < 9223372036854775808 &&
		number == math.Trunc(number) {
		return IntValue(int64(number))
	}
	return value
}

// numberText returns an Integer or Float as text, after SQLite 3.53: an
// integer in decimal; a float with 15 significant digits when they read
// back as the same number, else 17, in exponent form below 1e-4 or from
// 1e17, and always with a fraction or an exponent. SQLite makes the digits
// and tests the read-back with its own arithmetic, so for some floats its
// text has 17 digits where this has 15, or another last digit; SQLite 3.51
// always writes 15. The text functions and TEXT affinity read numbers so.
func numberText(value Value) string {
	if value.Type == Integer {
		return strconv.FormatInt(value.Int, 10)
	}
	number := value.Float
	switch {
	case math.IsInf(number, 1):
		return "Inf"
	case math.IsInf(number, -1):
		return "-Inf"
	case number == 0:
		return "0.0"
	}
	digits := strconv.FormatFloat(number, 'e', 14, 64)
	if again, _ := strconv.ParseFloat(digits, 64); again != number ||
		math.Abs(number) < 0x1p-1022 {
		digits = strconv.FormatFloat(number, 'e', 16, 64)
	}
	return layoutFloat(digits)
}

// layoutFloat rewrites strconv's exponent form d.ddde±XX as SQLite's "%!g":
// trailing zeros dropped, fixed notation for exponents from -4 to 16.
func layoutFloat(scientific string) string {
	mantissa, exponentText, _ := strings.Cut(scientific, "e")
	exponent, _ := strconv.Atoi(exponentText)
	sign := ""
	if strings.HasPrefix(mantissa, "-") {
		sign, mantissa = "-", mantissa[1:]
	}
	digits := strings.TrimRight(strings.Replace(mantissa, ".", "", 1), "0")
	if digits == "" {
		digits = "0"
	}
	if exponent < -4 || exponent > 16 {
		fraction := digits[1:]
		if fraction == "" {
			fraction = "0"
		}
		marker := "+"
		if exponent < 0 {
			marker, exponent = "-", -exponent
		}
		return sign + digits[:1] + "." + fraction + "e" + marker + padExponent(exponent)
	}
	if exponent < 0 {
		return sign + "0." + strings.Repeat("0", -exponent-1) + digits
	}
	if len(digits) <= exponent+1 {
		return sign + digits + strings.Repeat("0", exponent+1-len(digits)) + ".0"
	}
	return sign + digits[:exponent+1] + "." + digits[exponent+1:]
}

// padExponent writes an exponent with at least two digits.
func padExponent(exponent int) string {
	text := strconv.Itoa(exponent)
	if len(text) < 2 {
		text = "0" + text
	}
	return text
}
