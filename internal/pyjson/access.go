package pyjson

import (
	"errors"
	"math"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// Bool returns the boolean v holds and true, or false and false when v is
// not a boolean.
func (v Value) Bool() (value, ok bool) {
	return v.flag && v.kind == Bool, v.kind == Bool
}

// Text returns the string v holds, or "" when v is not a string; IsString
// tells an empty string from a value of another kind.
func (v Value) Text() string {
	if v.kind != String {
		return ""
	}
	return v.text
}

// Int returns the integer v holds and true, or 0 and false when v is not a
// number in integer form or does not fit in an int64.
func (v Value) Int() (int64, bool) {
	if !v.IsInt() {
		return 0, false
	}
	value, err := strconv.ParseInt(v.text, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// Float returns v as Python's float(v) gives it for a number: the float
// itself, or an integer rounded to the nearest float. It returns false for
// a value that is not a number and for an integer too large for a float,
// where Python raises OverflowError.
func (v Value) Float() (float64, bool) {
	switch {
	case v.IsFloat():
		return v.float, true
	case v.IsInt():
		value, err := strconv.ParseFloat(intText(v.text), 64)
		return value, !errors.Is(err, strconv.ErrRange)
	}
	return 0, false
}

// NumberText returns a number's text as the document wrote it ("1.50",
// "1E5", "-0", "NaN"), or "" when v is not a number. Encoders and Str write
// Python's form instead.
func (v Value) NumberText() string {
	if v.kind != Number {
		return ""
	}
	return v.text
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
// object or has no such key, as Python's dict.get(key) does; calls chain:
// record.Get("message").Get("content").
func (v Value) Get(key string) Value {
	value, _ := v.Lookup(key)
	return value
}

// Has reports whether v is an object holding key, as Python's `key in v`.
func (v Value) Has(key string) bool {
	_, ok := v.Lookup(key)
	return ok
}

// Set gives key the value in an object, keeping the key's position when it
// is already present and adding it last otherwise, as assigning to a Python
// dict does. Every copy of the object sees the change. Set does nothing
// when v is not an object.
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

// Truthy reports whether Python's bool(v) is true: false for null, false,
// a zero number, an empty string, array or object, and true otherwise, NaN
// included.
func (v Value) Truthy() bool {
	switch v.kind {
	case Bool:
		return v.flag
	case Number:
		if v.flag {
			return v.float != 0
		}
		return intText(v.text) != "0"
	case String:
		return v.text != ""
	case Array, Object:
		return v.Len() > 0
	}
	return false
}

// intText returns an integer's digits as Python's int writes them: the
// source text, except that "-0" is "0".
func intText(text string) string {
	if text == "-0" {
		return "0"
	}
	return text
}

// floatText returns a float as json.dumps writes it: NaN, Infinity and
// -Infinity by name, any other float as repr writes it.
func floatText(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "Infinity"
	case math.IsInf(value, -1):
		return "-Infinity"
	}
	return pytext.Float(value)
}
