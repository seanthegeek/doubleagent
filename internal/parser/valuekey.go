package parser

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ValueKey returns a map key for a JSON value: two values have the same key
// exactly when jsonvalue.Equal holds for them, so numbers that are equal by
// value (1, 1.0, 1e0, 10e-1) share a key, as do objects with the same
// members in another order. Its cost grows in step with the value's size,
// however large a number's exponent is written.
func ValueKey(value jsonvalue.Value) string {
	var out strings.Builder
	writeValueKey(&out, value)
	return out.String()
}

// writeValueKey writes a value's key. Every piece is self-delimiting: a
// kind letter, then a length before text and a count before items.
func writeValueKey(out *strings.Builder, value jsonvalue.Value) {
	switch value.Kind() {
	case jsonvalue.Null:
		out.WriteByte('n')
	case jsonvalue.Bool:
		flag, _ := value.Bool()
		out.WriteString(strconv.FormatBool(flag)[:1])
	case jsonvalue.Number:
		out.WriteString("#" + canonicalNumber(string(value.Number())) + ";")
	case jsonvalue.String:
		writeTextKey(out, value.StringValue())
	case jsonvalue.Array:
		out.WriteString("[" + strconv.Itoa(value.Len()) + ":")
		for _, item := range value.Items() {
			writeValueKey(out, item)
		}
	case jsonvalue.Object:
		members := slices.SortedFunc(slices.Values(value.Members()), func(left, right jsonvalue.Member) int {
			return strings.Compare(left.Key, right.Key)
		})
		out.WriteString("{" + strconv.Itoa(len(members)) + ":")
		for _, member := range members {
			writeTextKey(out, member.Key)
			writeValueKey(out, member.Value)
		}
	}
}

// writeTextKey writes text with its length before it.
func writeTextKey(out *strings.Builder, text string) {
	out.WriteString("s" + strconv.Itoa(len(text)) + ":" + text)
}

// canonicalNumber returns a JSON number's value in one spelling, the one
// jsonvalue.Equal compares: "0" for a zero of either sign, otherwise an
// optional minus, the significant digits with no leading or trailing zero,
// "e" and the power of ten of the last digit (100 is "1e2", 1.50 is
// "15e-1"). It works on the text alone, so a number of any size costs work
// in step with its length.
func canonicalNumber(text string) string {
	sign := ""
	if after, negative := strings.CutPrefix(text, "-"); negative {
		sign, text = "-", after
	}
	mantissa, exponent := text, ""
	if position := strings.IndexAny(text, "eE"); position >= 0 {
		mantissa, exponent = text[:position], text[position+1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0"
	}
	significant := strings.TrimRight(digits, "0")
	shift := int64(len(digits)-len(significant)) - int64(len(fraction))
	return sign + significant + "e" + exponentPlus(exponent, shift)
}

// exponentDigits is the most digits an exponent may have to be read as an
// int64 with room to add a shift, which is at most a number's length.
const exponentDigits = 18

// exponentPlus returns a JSON exponent ("", "7", "+07", "-3") plus shift, as
// decimal text with no leading zero. An exponent too long for an int64 is
// at least 10^18 in size, more than any shift, so the shift changes only
// its last 18 digits, with a carry or borrow into the rest, and never its
// sign.
func exponentPlus(exponent string, shift int64) string {
	negative := strings.HasPrefix(exponent, "-")
	digits := strings.TrimLeft(strings.TrimLeft(exponent, "+-"), "0")
	if len(digits) <= exponentDigits {
		value, _ := strconv.ParseInt("0"+digits, 10, 64)
		if negative {
			value = -value
		}
		return strconv.FormatInt(value+shift, 10)
	}
	if negative {
		shift = -shift
	}
	head, tail := digits[:len(digits)-exponentDigits], digits[len(digits)-exponentDigits:]
	low, _ := strconv.ParseInt(tail, 10, 64)
	switch low += shift; {
	case low >= 1e18:
		head, low = decimalStep(head, 1), low-1e18
	case low < 0:
		head, low = decimalStep(head, -1), low+1e18
	}
	result := strings.TrimLeft(head+fmt.Sprintf("%018d", low), "0")
	if negative {
		return "-" + result
	}
	return result
}

// decimalStep returns a positive decimal plus one, or minus one, with a
// carry or borrow through its digits; a borrow may leave a leading zero.
func decimalStep(digits string, step int) string {
	out := []byte(digits)
	for i := len(out) - 1; i >= 0; i-- {
		switch {
		case step > 0 && out[i] == '9':
			out[i] = '0'
		case step < 0 && out[i] == '0':
			out[i] = '9'
		default:
			out[i] = byte(int(out[i]) + step)
			return string(out)
		}
	}
	return "1" + string(out)
}
