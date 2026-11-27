package jsonvalue

import (
	"math"
	"strconv"
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
