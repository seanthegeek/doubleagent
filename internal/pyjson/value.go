// Package pyjson reads and writes JSON with the results of Python's json
// module, which the analyzer's output was defined by: objects keep their
// keys in document order, a duplicate key keeps its first position and
// takes its last value, numbers keep their source text so integers of any
// size stay exact, and NaN, Infinity and -Infinity are numbers.
//
// Strings are Go strings in the representation pytext describes: UTF-8,
// with a lone surrogate from an escape such as \ud83d held in its
// three-byte form, so that it survives a round trip (written back as
// \ud83d, or raw, as Python writes it).
//
// Decode reads raw file bytes as json.loads reads them after
// bytes.decode("utf-8", "replace"), which is what the parsers do;
// DecodeString reads text that is already a Python str. DecodePrefix reads
// one value and reports where it ended, as JSONDecoder.raw_decode does.
//
// The encoders match json.dumps byte for byte for every Value: Compact,
// Sorted, Spaced, SpacedUnicode and Indented name the forms the analyzer
// uses, and Format covers the other combinations of json.dumps's
// arguments. Errors are this package's own wording, with a position.
package pyjson

import (
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
// when a Value is copied, as Python lists and dicts do.
type Value struct {
	kind    Kind
	flag    bool // the boolean, or whether a number is in float form
	text    string
	float   float64
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

// NewString returns a string Value.
func NewString(text string) Value { return Value{kind: String, text: text} }

// NewInt returns an integer Value.
func NewInt(value int64) Value {
	return Value{kind: Number, text: strconv.FormatInt(value, 10)}
}

// NewFloat returns a Value in float form, as a Python float.
func NewFloat(value float64) Value {
	return Value{kind: Number, flag: true, float: value, text: floatText(value)}
}

// NewArray returns an array Value holding items, which it keeps.
func NewArray(items ...Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{kind: Array, items: items}
}

// NewObject returns an object Value holding members in order; a repeated
// key keeps its first position and takes its last value, as a Python dict
// built from the pairs does.
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

// IsInt reports whether v is a number in integer form (Python's int; a
// boolean is not one here, though Python's isinstance would say it is).
func (v Value) IsInt() bool { return v.kind == Number && !v.flag }

// IsFloat reports whether v is a number in float form: one written with a
// fraction or an exponent, or NaN, Infinity or -Infinity.
func (v Value) IsFloat() bool { return v.kind == Number && v.flag }

// IsString reports whether v is a string.
func (v Value) IsString() bool { return v.kind == String }

// IsArray reports whether v is an array.
func (v Value) IsArray() bool { return v.kind == Array }

// IsObject reports whether v is an object.
func (v Value) IsObject() bool { return v.kind == Object }
