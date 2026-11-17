package sqlitefile

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pytext"
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

// IsNull reports whether the value is NULL, Python's None.
func (v Value) IsNull() bool {
	return v.Type == Null
}

// Truthy reports whether Python treats the value as true: a nonzero
// number, or a non-empty text or blob.
func (v Value) Truthy() bool {
	switch v.Type {
	case Integer:
		return v.Int != 0
	case Float:
		return v.Float != 0
	case Text, Blob:
		return len(v.Bytes) > 0
	}
	return false
}

// Str returns the value as Python's str() writes the value the sqlite3
// module returns for it: an integer in decimal, a float as repr(float),
// text as itself, a blob as a bytes literal (b'...') and NULL as None.
func (v Value) Str() string {
	switch v.Type {
	case Integer:
		return strconv.FormatInt(v.Int, 10)
	case Float:
		return pytext.Float(v.Float)
	case Text:
		return string(v.Bytes)
	case Blob:
		return bytesRepr(v.Bytes)
	}
	return pytext.None
}

// bytesRepr writes data as Python's repr(bytes): single quotes unless the
// data holds a single quote and no double quote, \t \n \r and \\ escaped,
// printable ASCII as itself and every other byte as \xNN.
func bytesRepr(data []byte) string {
	quote := byte('\'')
	if strings.IndexByte(string(data), '\'') >= 0 && strings.IndexByte(string(data), '"') < 0 {
		quote = '"'
	}
	var out strings.Builder
	out.WriteString("b")
	out.WriteByte(quote)
	for _, octet := range data {
		switch {
		case octet == quote || octet == '\\':
			out.WriteByte('\\')
			out.WriteByte(octet)
		case octet == '\t':
			out.WriteString(`\t`)
		case octet == '\n':
			out.WriteString(`\n`)
		case octet == '\r':
			out.WriteString(`\r`)
		case octet < 0x20 || octet >= 0x7f:
			out.WriteString(`\x`)
			out.WriteString(strconv.FormatUint(uint64(octet)>>4, 16))
			out.WriteString(strconv.FormatUint(uint64(octet)&0xf, 16))
		default:
			out.WriteByte(octet)
		}
	}
	out.WriteByte(quote)
	return out.String()
}
