package protowire

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
)

// testSchema is the schema the oracle script testdata/gen_oracle.py decodes
// with; the two must stay equal.
func testSchema(t testing.TB) *Schema {
	t.Helper()
	schema, err := NewSchema(map[string][]Field{
		"Outer": {
			{1, "name", KindString},
			{2, "+tags", KindString},
			{3, "count", KindInt},
			{4, "flag", KindBool},
			{5, "ratio", KindFloat},
			{6, "wide", KindDouble},
			{7, "when", KindTimestamp},
			{8, "inner", "Inner"},
			{9, "+items", "Inner"},
			{10, "+numbers", KindInt},
			{11, "+flags", KindBool},
			{12, "raw", KindBytes},
			{13, "also", "Inner"},
		},
		"Inner": {
			{1, "label", KindString},
			{2, "value", KindInt},
			{3, "deeper", "Inner"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// render writes a decoded value in the form the oracle script prints.
func render(value any) string {
	switch typed := value.(type) {
	case *Message:
		parts := []string{}
		for _, entry := range typed.Entries() {
			parts = append(parts, entry.Name+"="+render(entry.Value))
		}
		return "{" + strings.Join(parts, ";") + "}"
	case []any:
		parts := []string{}
		for _, item := range typed {
			parts = append(parts, render(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case bool:
		return fmt.Sprint(typed)
	case *big.Int:
		return typed.String()
	case float64:
		return fmt.Sprintf("f%016x", math.Float64bits(typed))
	case []byte:
		return `x"` + hex.EncodeToString(typed) + `"`
	}
	return `s"` + hex.EncodeToString([]byte(value.(string))) + `"`
}

// The expected messages in testdata/oracle.txt are those gen_oracle.py
// printed, except each Timestamp's when value: this decoder reads seconds
// and nanoseconds through time.Unix and cuts to the millisecond, where the
// script's decoder took seconds above 1e11 as milliseconds and rounded to
// the microsecond, so those 33 values were rewritten to this decoder's.
func TestDecodeMatchesTheRecordedMessages(t *testing.T) {
	file, err := os.Open("testdata/oracle.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	schema := testSchema(t)
	scanner := bufio.NewScanner(file)
	cases := 0
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 3 {
			t.Fatalf("bad oracle line %q", scanner.Text())
		}
		cases++
		t.Run(parts[0], func(t *testing.T) {
			data, err := hex.DecodeString(parts[1])
			if err != nil {
				t.Fatal(err)
			}
			message, err := schema.Decode(data, "Outer")
			if parts[2] == "ERR" {
				if !errors.Is(err, ErrMalformed) {
					t.Fatalf("want a malformed error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if got := render(message); got != parts[2] {
				t.Fatalf("got  %s\nwant %s", got, parts[2])
			}
		})
	}
	if cases < 400 {
		t.Fatalf("only %d oracle cases", cases)
	}
}

func TestDecodeDifferencesFromPython(t *testing.T) {
	schema := testSchema(t)
	longVarint := []byte{0x18, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}
	overflow := []byte{0x18, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02}
	for name, data := range map[string][]byte{
		"a varint of eleven bytes is rejected":           longVarint,
		"a varint that overflows 64 bits is rejected":    overflow,
		"a varint of ten continuation bytes is rejected": {0x18, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := schema.Decode(data, "Outer"); !errors.Is(err, ErrMalformed) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestDecodeRejectsAnUnknownMessageName(t *testing.T) {
	_, err := testSchema(t).Decode(nil, "Missing")
	if err == nil || errors.Is(err, ErrMalformed) {
		t.Fatalf("got %v", err)
	}
}

func TestSelfReferencingSchemaStopsAtTheDepthLimit(t *testing.T) {
	deep := []byte{0x10, 0x01}
	for range 400 {
		deep = appendDelimited(3, deep)
	}
	message, err := testSchema(t).Decode(appendDelimited(8, deep), "Outer")
	if err != nil {
		t.Fatal(err)
	}
	levels := 0
	for child, ok := message.Child("inner"); ok; child, ok = child.Child("deeper") {
		levels++
	}
	if levels != maxDepth {
		t.Fatalf("kept %d levels, want %d", levels, maxDepth)
	}
}

func TestNewSchemaRejectsABadTable(t *testing.T) {
	for name, table := range map[string]map[string][]Field{
		"a nested kind naming no message": {"A": {{1, "x", "Missing"}}},
		"a field number used twice":       {"A": {{1, "x", KindInt}, {1, "y", KindInt}}},
		"a name both repeated and plain":  {"A": {{1, "x", KindInt}, {2, "+x", KindInt}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewSchema(table); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestFieldsComeBackInDeclarationOrder(t *testing.T) {
	fields := testSchema(t).Fields("Inner")
	if len(fields) != 3 || fields[0].Name != "label" || fields[2].Name != "deeper" {
		t.Fatalf("got %v", fields)
	}
	if testSchema(t).Fields("Missing") != nil {
		t.Fatal("want nil for an unknown message")
	}
}

func TestAccessorsReadTypedValues(t *testing.T) {
	data := appendDelimited(1, []byte("hi"))
	data = append(data, 0x18, 0x05)
	data = append(data, appendDelimited(8, appendDelimited(1, []byte("in")))...)
	data = append(data, appendDelimited(2, []byte("a"))...)
	data = append(data, 0x18, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01)
	message, err := testSchema(t).Decode(data, "Outer")
	if err != nil {
		t.Fatal(err)
	}
	if text, ok := message.Text("name"); !ok || text != "hi" {
		t.Fatalf("Text: %q %v", text, ok)
	}
	if _, ok := message.Int("count"); ok {
		t.Fatal("a count over int64 should not fit")
	}
	if _, ok := message.Text("count"); ok {
		t.Fatal("an int is not text")
	}
	if child, ok := message.Child("inner"); !ok || len(child.Entries()) != 1 {
		t.Fatal("Child")
	}
	if len(message.List("tags")) != 1 || message.List("absent") != nil {
		t.Fatal("List")
	}
}

// appendDelimited returns one length-delimited field for tests.
func appendDelimited(number int, payload []byte) []byte {
	out := appendVarint(nil, uint64(number)<<3|2)
	out = appendVarint(out, uint64(len(payload)))
	return append(out, payload...)
}

// appendVarint is the test-only varint encoder.
func appendVarint(out []byte, value uint64) []byte {
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func TestVarintRoundTrip(t *testing.T) {
	for _, value := range []uint64{0, 1, 127, 128, 300, 1 << 32, math.MaxUint64} {
		got, next, err := readVarint(appendVarint(nil, value), 0)
		if err != nil || got != value || next != len(appendVarint(nil, value)) {
			t.Fatalf("%d: got %d %d %v", value, got, next, err)
		}
	}
}

func FuzzDecode(fuzzer *testing.F) {
	file, err := os.Open("testdata/oracle.txt")
	if err != nil {
		fuzzer.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if data, err := hex.DecodeString(strings.Split(scanner.Text(), "\t")[1]); err == nil {
			fuzzer.Add(data)
		}
	}
	schema := testSchema(fuzzer)
	fuzzer.Fuzz(func(t *testing.T, data []byte) {
		if _, err := schema.Decode(data, "Outer"); err != nil && !errors.Is(err, ErrMalformed) {
			t.Fatal(err)
		}
	})
}
