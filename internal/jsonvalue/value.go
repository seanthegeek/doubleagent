// Package jsonvalue holds a decoded JSON document as an ordered value, read
// through encoding/json's token reader and written through its encoder.
//
// The value keeps what the evidence wrote: an object's keys stay in document
// order (a duplicate key keeps its first position and takes its last
// value), and a number keeps its source text, so "1.50" stays "1.50" and an
// integer of any size stays exact. Strings are valid UTF-8: the token reader
// replaces each byte of invalid UTF-8, and each lone UTF-16 surrogate escape,
// with U+FFFD. NaN and Infinity are not JSON and do not decode.
//
// Decode reads a whole document, DecodePrefix one value followed by other
// text. Arrays and objects nest at most 10,000 deep, the token reader's own
// limit; the value is built without recursion, so a deeper document is an
// error, never a stack overflow. Errors wrap the standard library's and give
// the byte offset, line and column where reading stopped.
//
// Text gives a value as row text: a string as it is, a number as written,
// true or false, nothing for null, and an array or object as compact JSON.
// IsEmpty is DoubleAgent's rule for a value that is left out of a row: null,
// false, zero, an empty string, array or object. Equal compares two values
// with numbers by value and objects without regard to key order.
//
// encoding/json can decode into any (map[string]any, float64): a map loses
// the evidence's key order, which rows and embedded JSON keep, and a float
// loses a number's text (1.50, a 20-digit id); UseNumber keeps the text but
// not the order. This package reads tokens to keep both.
package jsonvalue

import (
	"math"
	"strconv"
	"strings"
)

// Kind is the JSON type of a Value.
type Kind uint8

// The kinds of Value. The zero Value is Null.
const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// Value is one decoded JSON value. Arrays and objects share their contents
// when a Value is copied.
type Value struct {
	kind    Kind
	flag    bool // the boolean, or whether a number is in float form
	text    string
	items   []Value
	members *members
}

// Member is one key and value of an object.
type Member struct {
	Key   string
	Value Value
}

// members holds an object's members in order, with an index by key once
// the object is large enough for a linear search to cost more than a map.
type members struct {
	list  []Member
	index map[string]int
}

// indexThreshold is the member count from which an object keeps a map.
const indexThreshold = 8

// NewNull returns the null Value, the same as the zero Value.
func NewNull() Value { return Value{} }

// NewBool returns a boolean Value.
func NewBool(value bool) Value { return Value{kind: Bool, flag: value} }

// NewString returns a string Value. Invalid UTF-8 in text is kept in the
// value and written as U+FFFD.
func NewString(text string) Value { return Value{kind: String, text: text} }

// NewInt returns an integer Value.
func NewInt(value int64) Value {
	return Value{kind: Number, text: strconv.FormatInt(value, 10)}
}

// NewFloat returns a number Value in float form, its text the shortest that
// reads back as value (strconv's 'g' form). JSON has no NaN or infinity:
// such a value is a string of strconv's text for it ("NaN", "+Inf").
func NewFloat(value float64) Value {
	text := strconv.FormatFloat(value, 'g', -1, 64)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return NewString(text)
	}
	return Value{kind: Number, flag: true, text: text}
}

// NewNumber returns a number Value holding text, which must be a JSON
// number, as written; ok is false, and the value null, when it is not one.
func NewNumber(text string) (value Value, ok bool) {
	if !isNumberText(text) {
		return Value{}, false
	}
	return Value{kind: Number, flag: strings.ContainsAny(text, ".eE"), text: text}, true
}

// NewArray returns an array Value holding items, which it keeps.
func NewArray(items ...Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{kind: Array, items: items}
}

// NewObject returns an object Value holding members in order; a repeated
// key keeps its first position and takes its last value.
func NewObject(list ...Member) Value {
	object := Value{kind: Object, members: &members{list: make([]Member, 0, len(list))}}
	for _, member := range list {
		object.Set(member.Key, member.Value)
	}
	return object
}

// Kind returns the JSON type of v.
func (v Value) Kind() Kind { return v.kind }

// IsNull reports whether v is null.
func (v Value) IsNull() bool { return v.kind == Null }

// IsBool reports whether v is true or false.
func (v Value) IsBool() bool { return v.kind == Bool }

// IsNumber reports whether v is a number, in integer or float form.
func (v Value) IsNumber() bool { return v.kind == Number }

// IsInt reports whether v is a number written as an integer: digits with
// no fraction or exponent.
func (v Value) IsInt() bool { return v.kind == Number && !v.flag }

// IsFloat reports whether v is a number written with a fraction or an
// exponent, or made by NewFloat.
func (v Value) IsFloat() bool { return v.kind == Number && v.flag }

// IsString reports whether v is a string.
func (v Value) IsString() bool { return v.kind == String }

// IsArray reports whether v is an array.
func (v Value) IsArray() bool { return v.kind == Array }

// IsObject reports whether v is an object.
func (v Value) IsObject() bool { return v.kind == Object }

// isNumberText reports whether text is a JSON number (RFC 8259, section 6):
// an optional minus, 0 or digits not starting with 0, then an optional
// fraction and an optional exponent, each with at least one digit.
func isNumberText(text string) bool {
	i := 0
	if i < len(text) && text[i] == '-' {
		i++
	}
	switch {
	case i < len(text) && text[i] == '0':
		i++
	case i < len(text) && text[i] >= '1' && text[i] <= '9':
		i = skipDigits(text, i)
	default:
		return false
	}
	if i < len(text) && text[i] == '.' {
		if i = skipDigits(text, i+1); text[i-1] == '.' {
			return false
		}
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			i++
		}
		start := i
		if i = skipDigits(text, i); i == start {
			return false
		}
	}
	return i == len(text)
}

// skipDigits returns the first offset from i that is not an ASCII digit.
func skipDigits(text string, i int) int {
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	return i
}
