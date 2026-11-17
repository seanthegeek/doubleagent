package geminicli

import (
	"math/big"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// orDash returns a value as row text the way Python's "%s" % (value or
// "-") writes it: "-" for a value Python counts as false.
func orDash(value pyjson.Value) string {
	if text := parser.Str(value); text != "" {
		return text
	}
	return "-"
}

// dashUnlessSet returns a value as row text, "-" only when it is null or
// the empty string: a zero or false is written as it is.
func dashUnlessSet(value pyjson.Value) string {
	if value.IsNull() || value.IsString() && value.Text() == "" {
		return "-"
	}
	return value.Str()
}

// isText reports whether value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// equal reports whether two decoded values are equal as Python's == says:
// numbers by value whatever their form, true and false equal to 1 and 0,
// arrays item by item and objects key by key in any order.
func equal(left, right pyjson.Value) bool {
	if isNumeric(left) || isNumeric(right) {
		leftNumber, leftOK := number(left)
		rightNumber, rightOK := number(right)
		return leftOK && rightOK && leftNumber.Cmp(rightNumber) == 0
	}
	if left.Kind() != right.Kind() {
		return false
	}
	switch {
	case left.IsString():
		return left.Text() == right.Text()
	case left.IsArray():
		if left.Len() != right.Len() {
			return false
		}
		for i, item := range left.Items() {
			if !equal(item, right.Index(i)) {
				return false
			}
		}
		return true
	case left.IsObject():
		if left.Len() != right.Len() {
			return false
		}
		for _, member := range left.Members() {
			other, ok := right.Lookup(member.Key)
			if !ok || !equal(member.Value, other) {
				return false
			}
		}
		return true
	}
	return true
}

// isNumeric reports whether Python compares the value as a number: a
// number or a boolean.
func isNumeric(value pyjson.Value) bool {
	return value.IsNumber() || value.IsBool()
}

// number returns a number or a boolean as an exact rational, the way
// Python compares an int, a float and a bool; false for anything else and
// for NaN or an infinity, which equal no value here.
func number(value pyjson.Value) (*big.Rat, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return big.NewRat(1, 1), true
		}
		return new(big.Rat), true
	}
	if value.IsInt() {
		return new(big.Rat).SetString(value.NumberText())
	}
	if float, ok := value.Float(); ok {
		rational := new(big.Rat)
		if rational.SetFloat64(float) == nil {
			return nil, false
		}
		return rational, true
	}
	return nil, false
}
