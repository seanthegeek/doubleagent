package sqlitefile

import (
	"encoding/hex"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// TextValue returns a TEXT value holding text.
func TextValue(text string) Value {
	return Value{Type: Text, Bytes: []byte(text)}
}

// IntValue returns an INTEGER value.
func IntValue(number int64) Value {
	return Value{Type: Integer, Int: number}
}

// FloatValue returns a REAL value.
func FloatValue(number float64) Value {
	return Value{Type: Float, Float: number}
}

// BlobValue returns a BLOB value holding data.
func BlobValue(data []byte) Value {
	return Value{Type: Blob, Bytes: data}
}

// IsNull reports whether the value is NULL.
func (v Value) IsNull() bool {
	return v.Type == Null
}

// IsEmpty reports whether the value is empty, which leaves it out of a row
// or gives a fallback its place: NULL, a number equal to zero, or text or a
// blob of no bytes.
func (v Value) IsEmpty() bool {
	switch v.Type {
	case Integer:
		return v.Int == 0
	case Float:
		return v.Float == 0
	case Text, Blob:
		return len(v.Bytes) == 0
	}
	return true
}

// String returns the value as row text: NULL as "", an integer in decimal,
// a float in the shortest form that reads back as it (strconv's 'g'), text
// as it is, and a blob as SQLite writes a blob literal: x, then its bytes
// in lower-case hexadecimal between single quotes.
func (v Value) String() string {
	switch v.Type {
	case Integer:
		return strconv.FormatInt(v.Int, 10)
	case Float:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case Text:
		return string(v.Bytes)
	case Blob:
		return "x'" + hex.EncodeToString(v.Bytes) + "'"
	}
	return ""
}

// UTF8Text returns the value as the text its JSON is read from: a blob
// decoded as UTF-8 with U+FFFD for each invalid byte, any other value as
// String gives it.
func (v Value) UTF8Text() string {
	if v.Type == Blob {
		return textlines.ValidUTF8(v.Bytes)
	}
	return v.String()
}
