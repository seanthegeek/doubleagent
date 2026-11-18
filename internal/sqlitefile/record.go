package sqlitefile

import (
	"encoding/binary"
	"math"
)

// Type is the storage class of one value in a record.
type Type uint8

// The storage classes a record holds, after the serial types of "Record
// Format" in the file format document.
const (
	Null Type = iota
	Integer
	Float
	Text
	Blob
)

// String names the storage class as SQL's typeof() does.
func (t Type) String() string {
	switch t {
	case Integer:
		return "integer"
	case Float:
		return "real"
	case Text:
		return "text"
	case Blob:
		return "blob"
	}
	return "null"
}

// Value is one value of a record. Integer holds Int, Float holds Float, and
// Text and Blob hold Bytes: the text in the database's encoding (always UTF-8
// here, unchecked) or the blob's content. Bytes may share memory with other
// values read from the same page and must not be changed.
type Value struct {
	Type  Type
	Int   int64
	Float float64
	Bytes []byte
}

// Text returns the bytes of a Text or Blob value as a string, and "" for
// the other storage classes.
func (v Value) Text() string {
	return string(v.Bytes)
}

// maxRecordHeader is the largest record header SQLite accepts: a longer one
// is reported as corruption by its record decoder, so no valid file has one.
const maxRecordHeader = 98307

// readVarint decodes the varint at the start of data, as "B-tree Pages"
// defines it: up to eight bytes of seven bits, most significant first, with
// the high bit set to go on, then a ninth byte of eight bits. That is not
// the varint encoding/binary reads, which puts the least significant bits
// first and has no eight-bit byte. It returns the value and its length, or
// a length of 0 when data ends first.
func readVarint(data []byte) (uint64, int) {
	var value uint64
	for i := range 8 {
		if i >= len(data) {
			return 0, 0
		}
		value = value<<7 | uint64(data[i]&0x7f)
		if data[i] < 0x80 {
			return value, i + 1
		}
	}
	if len(data) < 9 {
		return 0, 0
	}
	return value<<8 | uint64(data[8]), 9
}

// decodeRecord splits a record payload into its values: a header of a
// size varint and one serial type per value, then the values' bytes. Like
// SQLite, it requires the values to fill the payload exactly, and reads the
// reserved serial types 10 and 11 as NULL and a stored NaN as NULL.
func decodeRecord(payload []byte) ([]Value, error) {
	headerSize, n := readVarint(payload)
	if n == 0 || headerSize < uint64(n) || headerSize > uint64(len(payload)) ||
		headerSize > maxRecordHeader {
		return nil, malformed("record header size %d does not fit its %d-byte payload",
			headerSize, len(payload))
	}
	header := payload[n:headerSize]
	body := payload[headerSize:]
	var values []Value
	for len(header) > 0 {
		serialType, used := readVarint(header)
		if used == 0 {
			return nil, malformed("record header ends inside a serial type")
		}
		header = header[used:]
		size := serialSize(serialType)
		if size > uint64(len(body)) {
			return nil, malformed("record value of %d bytes runs past its payload", size)
		}
		values = append(values, decodeValue(serialType, body[:size]))
		body = body[size:]
	}
	if len(body) != 0 {
		return nil, malformed("record leaves %d bytes of its payload unused", len(body))
	}
	return values, nil
}

// serialSize is the number of body bytes a serial type takes.
func serialSize(serialType uint64) uint64 {
	switch serialType {
	case 0, 8, 9, 10, 11:
		return 0
	case 1, 2, 3, 4:
		return serialType
	case 5:
		return 6
	case 6, 7:
		return 8
	}
	return (serialType - 12) / 2
}

// decodeValue turns the bytes of one serial type into a Value.
func decodeValue(serialType uint64, data []byte) Value {
	switch {
	case serialType >= 12 && serialType%2 == 0:
		return Value{Type: Blob, Bytes: data}
	case serialType >= 13:
		return Value{Type: Text, Bytes: data}
	case serialType == 8:
		return Value{Type: Integer, Int: 0}
	case serialType == 9:
		return Value{Type: Integer, Int: 1}
	case serialType == 7:
		number := math.Float64frombits(binary.BigEndian.Uint64(data))
		if math.IsNaN(number) {
			return Value{Type: Null}
		}
		return Value{Type: Float, Float: number}
	case serialType >= 1 && serialType <= 6:
		return Value{Type: Integer, Int: bigEndianSigned(data)}
	}
	return Value{Type: Null}
}

// bigEndianSigned reads a big-endian two's-complement integer of 1 to 8 bytes.
func bigEndianSigned(data []byte) int64 {
	value := int64(int8(data[0]))
	for _, octet := range data[1:] {
		value = value<<8 | int64(octet)
	}
	return value
}
