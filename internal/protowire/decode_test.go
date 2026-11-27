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
			{7, "when", KindTimestamp},
			{8, "inner", "Inner"},
			{9, "+items", "Inner"},
			{10, "+numbers", KindInt},
			{11, "+flags", KindBool},
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
	for child, ok := message.Get("inner"); ok; child, ok = child.(*Message).Get("deeper") {
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
		{"a fixed-width integer cut short", append(slices.Clone(name), 0x1d, 0x01, 0x02),
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

func TestAFieldOfAnotherWireTypeThanItsKindIsSkippedAsUnknown(t *testing.T) {
	name := appendDelimited(1, []byte("ok"))
	cases := []struct {
		name string
		data []byte
	}{
		{"text written as a varint", []byte{0x10, 0x2a}},
		{"text written as a fixed width", []byte{0x15, 'a', 'b', 'c', 'd'}},
		{"an integer written as bytes", appendDelimited(3, []byte{0x01, 0x02, 0x03, 0x04})},
		{"a truth value written as bytes", appendDelimited(4, []byte{0x01})},
		{"a truth value written as a fixed width", []byte{0x25, 0x01, 0x00, 0x00, 0x00}},
		{"a timestamp written as a varint", []byte{0x38, 0x80, 0xe2, 0xcf, 0xaa, 0x06}},
		{"a nested message written as a fixed width", []byte{0x41, 0x10, 0x01, 0, 0, 0, 0, 0, 0}},
		{"a nested message written as a varint", []byte{0x40, 0x05}},
		{"an item of a repeated message written as a varint", []byte{0x48, 0x05}},
	}
	for _, test := range cases {
		t.Run(test.name+" is skipped", func(t *testing.T) {
			message, err := testSchema(t).Decode(append(slices.Clone(test.data), name...), "Outer")
			if got := render(message); err != nil || got != "{name="+render("ok")+"}" {
				t.Errorf("Decode() = %s, %v; want only the name", got, err)
			}
		})
	}
}

func TestCopiesOfOneNestedMessageFieldMerge(t *testing.T) {
	schema, err := NewSchema(map[string][]Field{
		"Step":  {{19, "user_input", "Input"}},
		"Input": {{1, "query", KindString}, {2, "user_response", KindString}, {3, "+marks", KindInt}, {4, "next", "Input"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := func(members ...[]byte) []byte { return appendDelimited(19, bytes.Join(members, nil)) }
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"the members of both copies are kept",
			append(input(appendDelimited(1, []byte("first"))), input(appendDelimited(2, []byte("second")))...),
			"{user_input={query=" + render("first") + ";user_response=" + render("second") + "}}"},
		{"a later plain member replaces the earlier one",
			append(input(appendDelimited(1, []byte("a"))), input(appendDelimited(1, []byte("b")))...),
			"{user_input={query=" + render("b") + "}}"},
		{"a repeated member's items join",
			append(input([]byte{0x18, 0x01}), input([]byte{0x18, 0x02})...),
			"{user_input={marks=[1,2]}}"},
		{"nested messages merge in turn",
			append(input(appendDelimited(4, appendDelimited(1, []byte("a")))),
				input(appendDelimited(4, appendDelimited(2, []byte("b"))))...),
			"{user_input={next={query=" + render("a") + ";user_response=" + render("b") + "}}}"},
		{"a malformed copy is left out alone",
			append(input(appendDelimited(1, []byte("a"))), input([]byte{0x0a, 0x05})...),
			"{user_input={query=" + render("a") + "}}"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			message, err := schema.Decode(test.data, "Step")
			if got := render(message); err != nil || got != test.want {
				t.Errorf("Decode() = %s, %v; want %s", got, err, test.want)
			}
		})
	}
}

func TestAZeroTimestampHoldsNoTime(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"a Timestamp message of zero seconds", appendDelimited(7, []byte{0x08, 0x00})},
		{"an empty Timestamp message", appendDelimited(7, nil)},
	}
	for _, test := range cases {
		t.Run(test.name+" gives no time", func(t *testing.T) {
			message, err := testSchema(t).Decode(test.data, "Outer")
			value, ok := message.Get("when")
			if text, isText := value.(string); err != nil || !ok || !isText || text != "" {
				t.Errorf("when = %q, %v (%v); want \"\"", text, ok, err)
			}
		})
	}
}

// repeatedFields returns count copies of one field's bytes.
func repeatedFields(field []byte, count int) []byte {
	return bytes.Repeat(field, count)
}

func TestValuesAreLimitedPerMessage(t *testing.T) {
	half := maxValues / 2
	cases := []struct {
		name string
		// fields returns a message of count values of the kind.
		fields func(count int) []byte
		list   string
	}{
		{"packed integers", func(count int) []byte {
			return append(appendDelimited(10, bytes.Repeat([]byte{0x01}, count-half)),
				appendDelimited(10, bytes.Repeat([]byte{0x01}, half))...)
		}, "numbers"},
		{"integers written one field each", func(count int) []byte {
			return repeatedFields([]byte{0x50, 0x01}, count)
		}, "numbers"},
		{"packed and separate integers together", func(count int) []byte {
			return append(appendDelimited(10, bytes.Repeat([]byte{0x01}, half)),
				repeatedFields([]byte{0x50, 0x01}, count-half)...)
		}, "numbers"},
		{"strings written one field each", func(count int) []byte {
			return repeatedFields([]byte{0x12, 0x00}, count)
		}, "tags"},
		{"nested messages written one field each", func(count int) []byte {
			return repeatedFields([]byte{0x4a, 0x00}, count)
		}, "items"},
	}
	for _, test := range cases {
		t.Run("a message may hold the most "+test.name, func(t *testing.T) {
			message, err := testSchema(t).Decode(test.fields(maxValues), "Outer")
			value, _ := message.Get(test.list)
			if values, _ := value.([]any); err != nil || len(values) != maxValues {
				t.Errorf("Decode() = %d values, %v; want %d", len(values), err, maxValues)
			}
		})
		t.Run("one more of the "+test.name+" is refused", func(t *testing.T) {
			_, err := testSchema(t).Decode(test.fields(maxValues+1), "Outer")
			if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "more than 1048576 values") {
				t.Errorf("Decode() error = %v, want ErrTooLarge naming the limit", err)
			}
		})
	}
	t.Run("a plain field written again and again counts each time", func(t *testing.T) {
		_, err := testSchema(t).Decode(repeatedFields([]byte{0x18, 0x01}, maxValues+1), "Outer")
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("Decode() error = %v, want ErrTooLarge", err)
		}
	})
	t.Run("the values of nested messages count toward the same limit", func(t *testing.T) {
		inner := appendDelimited(8, repeatedFields([]byte{0x10, 0x01}, half))
		_, err := testSchema(t).Decode(append(inner, repeatedFields([]byte{0x50, 0x01}, half)...), "Outer")
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("Decode() error = %v, want ErrTooLarge", err)
		}
	})
}
