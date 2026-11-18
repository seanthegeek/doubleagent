package jsonvalue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Bool returns the boolean v holds and true, or false and false when v is
// not a boolean.
func (v Value) Bool() (value, ok bool) {
	return v.flag && v.kind == Bool, v.kind == Bool
}

// StringValue returns the string v holds, or "" when v is not a string;
// IsString tells an empty string from a value of another kind.
func (v Value) StringValue() string {
	if v.kind != String {
		return ""
	}
	return v.text
}

// Number returns a number's text as the document wrote it ("1.50", "1E5",
// "-0"), or "" when v is not a number.
func (v Value) Number() json.Number {
	if v.kind != Number {
		return ""
	}
	return json.Number(v.text)
}

// Int returns the integer v holds and true, or 0 and false when v is not a
// number written as an integer or does not fit in an int64.
func (v Value) Int() (int64, bool) {
	if !v.IsInt() {
		return 0, false
	}
	value, err := strconv.ParseInt(v.text, 10, 64)
	return value, err == nil
}

// Float returns the float64 nearest to a number and true, or 0 and false
// when v is not a number or is beyond the range of a float64.
func (v Value) Float() (float64, bool) {
	if v.kind != Number {
		return 0, false
	}
	value, err := strconv.ParseFloat(v.text, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// Text returns v as row text: a string as it is, a number as the document
// wrote it, "true" or "false", "" for null, and an array or object as
// compact JSON (JSON).
func (v Value) Text() string {
	switch v.kind {
	case String, Number:
		return v.text
	case Bool:
		return strconv.FormatBool(v.flag)
	case Array, Object:
		return v.JSON()
	}
	return ""
}

// IsEmpty reports whether v is empty, which leaves it out of a row or
// gives a fallback its place: null, false, a number equal to zero, the
// empty string, and an array or object with nothing in it.
func (v Value) IsEmpty() bool {
	switch v.kind {
	case Bool:
		return !v.flag
	case Number:
		return isZero(v.text, v.flag)
	case String:
		return v.text == ""
	case Array, Object:
		return v.Len() == 0
	}
	return true
}

// isZero reports whether a number's text is zero by value; a fraction too
// small for a float64 is zero.
func isZero(text string, floatForm bool) bool {
	if !floatForm {
		return strings.Trim(text, "-0") == ""
	}
	value, err := strconv.ParseFloat(text, 64)
	return err == nil && value == 0
}

// Len returns the number of items of an array or members of an object, and
// 0 for any other value.
func (v Value) Len() int {
	switch v.kind {
	case Array:
		return len(v.items)
	case Object:
		return len(v.members.list)
	}
	return 0
}

// Index returns item i of an array, or null when v is not an array or i is
// out of range.
func (v Value) Index(i int) Value {
	if v.kind != Array || i < 0 || i >= len(v.items) {
		return Value{}
	}
	return v.items[i]
}

// Items returns an array's items in order, nil for any other value. The
// slice is the array's own: a caller does not modify it.
func (v Value) Items() []Value {
	if v.kind != Array {
		return nil
	}
	return v.items
}

// Members returns an object's members in order, nil for any other value.
// The slice is the object's own: a caller does not modify it.
func (v Value) Members() []Member {
	if v.kind != Object {
		return nil
	}
	return v.members.list
}

// Keys returns an object's keys in order, nil for any other value.
func (v Value) Keys() []string {
	if v.kind != Object {
		return nil
	}
	keys := make([]string, len(v.members.list))
	for i, member := range v.members.list {
		keys[i] = member.Key
	}
	return keys
}

// Lookup returns the value of key in an object and true, or null and false
// when v is not an object or has no such key.
func (v Value) Lookup(key string) (Value, bool) {
	if v.kind != Object {
		return Value{}, false
	}
	position := v.members.find(key)
	if position < 0 {
		return Value{}, false
	}
	return v.members.list[position].Value, true
}

// Get returns the value of key in an object, or null when v is not an
// object or has no such key; calls chain:
// record.Get("message").Get("content").
func (v Value) Get(key string) Value {
	value, _ := v.Lookup(key)
	return value
}

// Has reports whether v is an object holding key.
func (v Value) Has(key string) bool {
	_, ok := v.Lookup(key)
	return ok
}

// Set gives key the value in an object, keeping the key's position when it
// is already present and adding it last otherwise. Every copy of the object
// sees the change. Set does nothing when v is not an object.
func (v Value) Set(key string, value Value) {
	if v.kind != Object {
		return
	}
	object := v.members
	if position := object.find(key); position >= 0 {
		object.list[position].Value = value
		return
	}
	object.list = append(object.list, Member{Key: key, Value: value})
	if object.index != nil {
		object.index[key] = len(object.list) - 1
	} else if len(object.list) >= indexThreshold {
		object.index = make(map[string]int, len(object.list)*2)
		for i, member := range object.list {
			object.index[member.Key] = i
		}
	}
}

// find returns the position of key among the members, or -1.
func (object *members) find(key string) int {
	if object.index != nil {
		if position, ok := object.index[key]; ok {
			return position
		}
		return -1
	}
	for i, member := range object.list {
		if member.Key == key {
			return i
		}
	}
	return -1
}

// Equal reports whether two values hold the same information: numbers
// equal by value, exactly, whatever their size (1, 1.0 and 1e0 are equal,
// 0.1 and 0.10000000000000001 are not), strings and booleans exactly,
// arrays item by item in order, and objects with the same keys holding equal
// values whatever their order.
func Equal(left, right Value) bool {
	if left.kind != right.kind {
		return false
	}
	switch left.kind {
	case Bool:
		return left.flag == right.flag
	case Number:
		return numbersEqual(left, right)
	case String:
		return left.text == right.text
	case Array:
		if len(left.items) != len(right.items) {
			return false
		}
		for i := range left.items {
			if !Equal(left.items[i], right.items[i]) {
				return false
			}
		}
		return true
	case Object:
		return objectsEqual(left, right)
	}
	return true
}

// objectsEqual reports whether two objects have the same keys holding
// equal values.
func objectsEqual(left, right Value) bool {
	if left.Len() != right.Len() {
		return false
	}
	for _, member := range left.members.list {
		other, ok := right.Lookup(member.Key)
		if !ok || !Equal(member.Value, other) {
			return false
		}
	}
	return true
}

// numbersEqual compares two numbers by value.
func numbersEqual(left, right Value) bool {
	return left.text == right.text || canonicalNumber(left.text) == canonicalNumber(right.text)
}

// canonicalNumber returns a JSON number's value in one spelling, so two
// numbers are equal by value exactly when these are equal: "0" for a zero
// of either sign, otherwise an optional minus, the significant digits with
// no leading or trailing zero, "e" and the power of ten of the last digit
// (100 is "1e2", 1.50 is "15e-1"). It works on the text alone, so its cost
// grows in step with the number's length however large the number is.
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
	return sign + significant + "e" + addToExponent(exponent, shift)
}

// decimalDigits is the most digits an exponent may have to be read as an
// int64 with room to add a shift, which is at most a number's length.
const decimalDigits = 18

// addToExponent returns a JSON exponent ("", "7", "+07", "-3") plus shift,
// as decimal text with no leading zero. An exponent too long for an int64
// is at least 10^18 in size, more than any shift, so the shift changes only
// its last digits and never its sign.
func addToExponent(exponent string, shift int64) string {
	negative := strings.HasPrefix(exponent, "-")
	digits := strings.TrimLeft(strings.TrimLeft(exponent, "+-"), "0")
	if len(digits) <= decimalDigits {
		value, _ := strconv.ParseInt("0"+digits, 10, 64)
		if negative {
			value = -value
		}
		return strconv.FormatInt(value+shift, 10)
	}
	if negative {
		shift = -shift
	}
	head, tail := digits[:len(digits)-decimalDigits], digits[len(digits)-decimalDigits:]
	low, _ := strconv.ParseInt(tail, 10, 64)
	switch low += shift; {
	case low >= 1e18:
		head, low = stepDecimal(head, 1), low-1e18
	case low < 0:
		head, low = stepDecimal(head, -1), low+1e18
	}
	result := strings.TrimLeft(head+fmt.Sprintf("%018d", low), "0")
	if negative {
		return "-" + result
	}
	return result
}

// stepDecimal returns a positive decimal plus one, or minus one, with a
// carry or borrow through its digits; a borrow may leave a leading zero.
func stepDecimal(digits string, step int) string {
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
