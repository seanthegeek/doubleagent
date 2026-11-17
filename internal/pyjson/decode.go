package pyjson

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// MaxDepth is the deepest nesting of arrays and objects the decoder
// accepts. Python accepts at least this depth; a deeper document is an
// error here rather than a stack overflow.
const MaxDepth = 100_000

// decoder reads one document; depth counts the arrays and objects open.
type decoder struct {
	text  string
	depth int
}

// Decode reads data, raw bytes such as a file's content, as Python's
// json.loads(data.decode("utf-8", "replace")) reads it: each invalid UTF-8
// sequence becomes U+FFFD first. Whitespace may surround the value; other
// text after it is an error. Error offsets count in the decoded text.
func Decode(data []byte) (Value, error) {
	return DecodeString(pytext.DecodeUTF8(data))
}

// DecodeString reads text, a Python str as the package comment describes,
// as json.loads(text) reads it: a lone surrogate's three-byte form inside
// a string is kept, and any other invalid byte there becomes U+FFFD. A
// leading byte-order mark is an error, as it is in Python.
func DecodeString(text string) (Value, error) {
	if strings.HasPrefix(text, "\ufeff") {
		return Value{}, newSyntaxError(text, 0, "unexpected byte-order mark")
	}
	value, end, err := DecodePrefix(text, skipSpace(text, 0))
	if err != nil {
		return Value{}, err
	}
	if end = skipSpace(text, end); end != len(text) {
		return Value{}, newSyntaxError(text, end, "extra data after the value")
	}
	return value, nil
}

// DecodePrefix reads the one value that starts at byte start of text, as
// JSONDecoder.raw_decode(text, start) reads it, and returns it with the
// byte offset just past it; what follows is left unread. Whitespace at
// start is not skipped. Text is read as DecodeString reads it, and
// pytext.CodePointLen(text[:end]) gives the offset as Python counts it.
func DecodePrefix(text string, start int) (Value, int, error) {
	if start < 0 || start > len(text) {
		return Value{}, 0, newSyntaxError(text, start, "start offset out of range")
	}
	reader := decoder{text: text}
	return reader.value(start)
}

// skipSpace returns the first offset from i that is not JSON whitespace:
// space, tab, LF or CR.
func skipSpace(text string, i int) int {
	for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
		i++
	}
	return i
}

// fail returns a syntax error at offset i.
func (d *decoder) fail(i int, message string) (Value, int, error) {
	return Value{}, 0, newSyntaxError(d.text, i, message)
}

// value reads the value at offset i, choosing the reader by its first byte
// in the order Python's scanner does.
func (d *decoder) value(i int) (Value, int, error) {
	if i >= len(d.text) {
		return d.fail(i, "expected a value")
	}
	switch d.text[i] {
	case '"':
		text, end, err := d.stringBody(i)
		if err != nil {
			return Value{}, 0, err
		}
		return Value{kind: String, text: text}, end, nil
	case '{':
		return d.object(i)
	case '[':
		return d.array(i)
	case 'n':
		return d.word(i, "null", Value{})
	case 't':
		return d.word(i, "true", NewBool(true))
	case 'f':
		return d.word(i, "false", NewBool(false))
	case 'N':
		return d.word(i, "NaN", Value{kind: Number, flag: true, float: math.NaN(), text: "NaN"})
	case 'I':
		return d.word(i, "Infinity", NewFloat(math.Inf(1)))
	case '-':
		if strings.HasPrefix(d.text[i:], "-Infinity") {
			return NewFloat(math.Inf(-1)), i + len("-Infinity"), nil
		}
	}
	return d.number(i)
}

// word reads the literal word at offset i, giving value, or fails when the
// text there is not that word.
func (d *decoder) word(i int, word string, value Value) (Value, int, error) {
	if strings.HasPrefix(d.text[i:], word) {
		return value, i + len(word), nil
	}
	return d.fail(i, "expected a value")
}

// number reads a number at offset i as Python's scanner does: an optional
// minus, 0 or digits without a leading zero, then a fraction only when a
// digit follows the point and an exponent only when a digit follows the e
// and its sign. Reading stops before anything else, so "01" reads as 0 and
// leaves "1". A fraction or an exponent makes it a float.
func (d *decoder) number(i int) (Value, int, error) {
	text, end := d.text, i
	if end < len(text) && text[end] == '-' {
		end++
	}
	if end >= len(text) || !isDigit(text[end]) {
		return d.fail(i, "expected a value")
	}
	if text[end] == '0' {
		end++
	} else {
		end = skipDigits(text, end)
	}
	isFloat := false
	if end+1 < len(text) && text[end] == '.' && isDigit(text[end+1]) {
		isFloat, end = true, skipDigits(text, end+1)
	}
	if end < len(text) && (text[end] == 'e' || text[end] == 'E') {
		digits := end + 1
		if digits < len(text) && (text[digits] == '+' || text[digits] == '-') {
			digits++
		}
		if after := skipDigits(text, digits); after > digits {
			isFloat, end = true, after
		}
	}
	source := text[i:end]
	if !isFloat {
		return Value{kind: Number, text: source}, end, nil
	}
	value, err := strconv.ParseFloat(source, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return d.fail(i, "invalid number")
	}
	return Value{kind: Number, flag: true, float: value, text: source}, end, nil
}

// isDigit reports whether char is an ASCII digit.
func isDigit(char byte) bool { return char >= '0' && char <= '9' }

// skipDigits returns the first offset from i that is not an ASCII digit.
func skipDigits(text string, i int) int {
	for i < len(text) && isDigit(text[i]) {
		i++
	}
	return i
}

// enter counts one more open array or object at offset i, failing past
// MaxDepth.
func (d *decoder) enter(i int) error {
	if d.depth >= MaxDepth {
		return newSyntaxError(d.text, i, "arrays and objects nested too deeply")
	}
	d.depth++
	return nil
}

// array reads the array whose '[' is at offset i.
func (d *decoder) array(i int) (Value, int, error) {
	if err := d.enter(i); err != nil {
		return Value{}, 0, err
	}
	defer func() { d.depth-- }()
	items := []Value{}
	offset := skipSpace(d.text, i+1)
	if offset < len(d.text) && d.text[offset] == ']' {
		return Value{kind: Array, items: items}, offset + 1, nil
	}
	for {
		item, end, err := d.value(offset)
		if err != nil {
			return Value{}, 0, err
		}
		items = append(items, item)
		offset = skipSpace(d.text, end)
		switch {
		case offset < len(d.text) && d.text[offset] == ']':
			return Value{kind: Array, items: items}, offset + 1, nil
		case offset >= len(d.text) || d.text[offset] != ',':
			return d.fail(offset, "expected ',' or ']' after an array item")
		}
		offset = skipSpace(d.text, offset+1)
		if offset < len(d.text) && d.text[offset] == ']' {
			return d.fail(offset, "trailing comma before ']'")
		}
	}
}

// object reads the object whose '{' is at offset i.
func (d *decoder) object(i int) (Value, int, error) {
	if err := d.enter(i); err != nil {
		return Value{}, 0, err
	}
	defer func() { d.depth-- }()
	object := Value{kind: Object, members: &members{}}
	offset := skipSpace(d.text, i+1)
	if offset < len(d.text) && d.text[offset] == '}' {
		return object, offset + 1, nil
	}
	for {
		end, err := d.member(object, offset)
		if err != nil {
			return Value{}, 0, err
		}
		offset = skipSpace(d.text, end)
		switch {
		case offset < len(d.text) && d.text[offset] == '}':
			return object, offset + 1, nil
		case offset >= len(d.text) || d.text[offset] != ',':
			return d.fail(offset, "expected ',' or '}' after an object member")
		}
		offset = skipSpace(d.text, offset+1)
		if offset < len(d.text) && d.text[offset] == '}' {
			return d.fail(offset, "trailing comma before '}'")
		}
	}
}

// member reads one `"key": value` pair at offset i into object and returns
// the offset after the value.
func (d *decoder) member(object Value, i int) (int, error) {
	if i >= len(d.text) || d.text[i] != '"' {
		return 0, newSyntaxError(d.text, i, "expected a property name in double quotes")
	}
	key, end, err := d.stringBody(i)
	if err != nil {
		return 0, err
	}
	offset := skipSpace(d.text, end)
	if offset >= len(d.text) || d.text[offset] != ':' {
		return 0, newSyntaxError(d.text, offset, "expected ':' after a property name")
	}
	value, end, err := d.value(skipSpace(d.text, offset+1))
	if err != nil {
		return 0, err
	}
	object.Set(key, value)
	return end, nil
}
