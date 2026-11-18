package sqlitefile

import (
	"errors"
	"math"
	"slices"
	"testing"
)

func TestReadVarint(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		value  uint64
		length int
	}{
		{"a one-byte varint is its own value", []byte{0x7f}, 127, 1},
		{"a two-byte varint joins seven bits from each", []byte{0x81, 0x00}, 128, 2},
		{"bytes after the varint are not read", []byte{0x05, 0xff}, 5, 1},
		{"the ninth byte gives all eight of its bits", []byte{0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff}, math.MaxUint64, 9},
		{"a varint cut short has no length", []byte{0x81}, 0, 0},
		{"a nine-byte varint cut short has no length", []byte{0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff}, 0, 0},
		{"no bytes has no length", nil, 0, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, length := readVarint(test.data)
			if value != test.value || length != test.length {
				t.Errorf("readVarint(%x) = %d, %d, want %d, %d", test.data, value, length,
					test.value, test.length)
			}
		})
	}
}

func TestDecodeRecord(t *testing.T) {
	nan := []byte{0x7f, 0xf8, 0, 0, 0, 0, 0, 0}
	cases := []struct {
		name    string
		payload []byte
		want    []Value
	}{
		{"an empty record has no values", []byte{0x01}, nil},
		{"integers of every width are signed and big-endian",
			[]byte{0x07, 1, 2, 3, 4, 5, 6, 0xff, 0x80, 0x00, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xfe, 0, 0, 0, 0, 0, 1, 0x80, 0, 0, 0, 0, 0, 0, 0},
			[]Value{{Type: Integer, Int: -1}, {Type: Integer, Int: -32768},
				{Type: Integer, Int: 8388607}, {Type: Integer, Int: -2},
				{Type: Integer, Int: 1}, {Type: Integer, Int: math.MinInt64}}},
		{"serial types 8 and 9 are the integers 0 and 1", []byte{0x03, 8, 9},
			[]Value{{Type: Integer, Int: 0}, {Type: Integer, Int: 1}}},
		{"a stored NaN reads as NULL, as SQLite reads it", append([]byte{0x02, 7}, nan...),
			[]Value{{Type: Null}}},
		{"the reserved serial types 10 and 11 read as NULL", []byte{0x03, 10, 11},
			[]Value{{Type: Null}, {Type: Null}}},
		{"text and blobs take their length from the serial type",
			[]byte{0x03, 19, 16, 'a', 'b', 'c', 0, 1},
			[]Value{{Type: Text, Bytes: []byte("abc")}, {Type: Blob, Bytes: []byte{0, 1}}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			values, err := decodeRecord(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(values, test.want, sameValue) {
				t.Errorf("decodeRecord(%x) = %v, want %v", test.payload, values, test.want)
			}
		})
	}
}

// sameValue compares two values field by field.
func sameValue(first, second Value) bool {
	return first.Type == second.Type && first.Int == second.Int && first.Float == second.Float &&
		string(first.Bytes) == string(second.Bytes)
}

func TestDecodeRecordRefusesDamage(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"an empty payload has no header", nil},
		{"a header longer than the payload", []byte{0x05, 1}},
		{"a header size smaller than its own varint", []byte{0x00}},
		{"a serial type cut by the header's end", []byte{0x02, 0x81}},
		{"a value running past the payload", []byte{0x02, 6, 1, 2}},
		{"bytes left over after the last value", []byte{0x02, 1, 5, 6}},
		{"a header over SQLite's limit", append([]byte{0x86, 0x80, 0x04}, make([]byte, 98308)...)},
	}
	for _, test := range cases {
		t.Run(test.name+" is malformed", func(t *testing.T) {
			if _, err := decodeRecord(test.payload); !errors.Is(err, ErrMalformed) {
				t.Errorf("decodeRecord() error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestTypeNames(t *testing.T) {
	names := map[Type]string{Null: "null", Integer: "integer", Float: "real", Text: "text", Blob: "blob"}
	for kind, want := range names {
		if kind.String() != want {
			t.Errorf("Type(%d).String() = %q, want %q", kind, kind.String(), want)
		}
	}
}

func TestDecodeRecordUpToChecksTheValuesItLeavesOut(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    []Value
		damaged bool
	}{
		{"the values past the limit are left out", []byte{0x04, 1, 19, 0, 7, 'a', 'b', 'c'},
			[]Value{{Type: Integer, Int: 7}}, false},
		{"a limit past the last value keeps every value", []byte{0x02, 9},
			[]Value{{Type: Integer, Int: 1}}, false},
		{"a value past the limit that runs past the payload is damage",
			[]byte{0x03, 1, 6, 7, 1, 2}, nil, true},
		{"bytes left over after values past the limit are damage",
			[]byte{0x03, 1, 0, 7, 9}, nil, true},
		{"a serial type past the limit cut by the header's end is damage",
			[]byte{0x03, 1, 0x81, 7}, nil, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			values, err := decodeRecordUpTo(test.payload, 1)
			if test.damaged {
				if !errors.Is(err, ErrMalformed) {
					t.Errorf("decodeRecordUpTo(%x) error = %v, want ErrMalformed", test.payload, err)
				}
				return
			}
			if err != nil || !slices.EqualFunc(values, test.want, sameValue) {
				t.Errorf("decodeRecordUpTo(%x) = %v, %v; want %v", test.payload, values, err, test.want)
			}
		})
	}
}
