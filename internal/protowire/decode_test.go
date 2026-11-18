package protowire

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"strings"
	"testing"
)

// testSchema is the schema the messages of testdata/messages.txt are
// decoded with.
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

// render writes a decoded value as compact text: a message as
// {name=value;...} and a list as [value,...], strings and bytes in hex, a
// float as its bits.
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

// Each line of testdata/messages.txt is a label, an encoded Outer message in
// hex and its decoded form as render writes it, or ERR for a malformed
// buffer. A Timestamp's when value is its seconds and nanoseconds read
// through time.Unix and cut to the millisecond.
func TestDecodeMatchesTheRecordedMessages(t *testing.T) {
	file, err := os.Open("testdata/messages.txt")
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
			t.Fatalf("bad message line %q", scanner.Text())
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
		t.Fatalf("only %d message cases", cases)
	}
}

func TestDecodeRefusesVarintsBeyondSixtyFourBits(t *testing.T) {
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
	file, err := os.Open("testdata/messages.txt")
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
		if _, err := schema.Decode(data, "Outer"); err != nil && !errors.Is(err, ErrMalformed) &&
			!errors.Is(err, ErrTooLarge) {
			t.Fatal(err)
		}
	})
}

func TestOneDamagedFieldCostsOnlyItself(t *testing.T) {
	name := appendDelimited(1, []byte("ok"))
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a packed field holding a cut-off varint", append(slices.Clone(name), appendDelimited(10, []byte{0x80})...),
			"{name=" + render("ok") + "}"},
		{"a packed field of booleans holding a cut-off varint",
			append(slices.Clone(name), appendDelimited(11, []byte{0x01, 0x80})...), "{name=" + render("ok") + "}"},
		{"a fixed-width text field cut short", append(slices.Clone(name), 0x09, 'a', 'b'),
			"{name=" + render("ok") + "}"},
		{"a fixed-width float cut short", append(slices.Clone(name), 0x2d, 0x01, 0x02),
			"{name=" + render("ok") + "}"},
		{"a nested message ending in a fixed-width field cut short",
			appendDelimited(8, append(appendDelimited(1, []byte("in")), 0x11, 0x01)),
			"{inner={label=" + render("in") + "}}"},
	}
	for _, test := range cases {
		t.Run(test.name+" is left out", func(t *testing.T) {
			message, err := testSchema(t).Decode(test.data, "Outer")
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if got := render(message); got != test.want {
				t.Errorf("Decode() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestAZeroTimestampHoldsNoTime(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"a varint of zero", []byte{0x38, 0x00}},
		{"a Timestamp message of zero seconds", appendDelimited(7, []byte{0x08, 0x00})},
		{"an empty Timestamp message", appendDelimited(7, nil)},
	}
	for _, test := range cases {
		t.Run(test.name+" gives no time", func(t *testing.T) {
			message, err := testSchema(t).Decode(test.data, "Outer")
			if text, ok := message.Text("when"); err != nil || !ok || text != "" {
				t.Errorf("when = %q, %v (%v); want \"\"", text, ok, err)
			}
		})
	}
}

func TestPackedValuesAreLimitedPerMessage(t *testing.T) {
	half := bytes.Repeat([]byte{0x01}, maxPackedValues/2)
	atLimit := append(appendDelimited(10, half), appendDelimited(8, appendDelimited(3,
		appendDelimited(1, []byte("x"))))...)
	atLimit = append(atLimit, appendDelimited(10, half)...)
	t.Run("a message may hold the most packed values", func(t *testing.T) {
		message, err := testSchema(t).Decode(atLimit, "Outer")
		if err != nil || len(message.List("numbers")) != maxPackedValues {
			t.Errorf("Decode() = %d numbers, %v; want %d", len(message.List("numbers")), err, maxPackedValues)
		}
	})
	t.Run("one packed value more is refused", func(t *testing.T) {
		_, err := testSchema(t).Decode(append(atLimit, appendDelimited(11, []byte{0x01})...), "Outer")
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("Decode() error = %v, want ErrTooLarge", err)
		}
	})
}
