package jsonvalue

import (
	"encoding/json"
	"math/big"
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
	return value, err == nil
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
// equal by value (1, 1.0 and 1e0 are equal), strings and booleans exactly,
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

// numbersEqual compares two numbers by value: two integers exactly, and
// otherwise as the nearest float64 of each.
func numbersEqual(left, right Value) bool {
	if left.text == right.text {
		return true
	}
	if !left.flag && !right.flag {
		var leftInt, rightInt big.Int
		_, leftOK := leftInt.SetString(left.text, 10)
		_, rightOK := rightInt.SetString(right.text, 10)
		return leftOK && rightOK && leftInt.Cmp(&rightInt) == 0
	}
	leftFloat, leftErr := strconv.ParseFloat(left.text, 64)
	rightFloat, rightErr := strconv.ParseFloat(right.text, 64)
	return leftErr == nil && rightErr == nil && leftFloat == rightFloat
}
