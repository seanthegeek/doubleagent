package jsonvalue

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestDecodeKeepsTheEvidenceAsWritten(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"keys stay in document order", `{"b":1,"a":2}`, `{"b":1,"a":2}`},
		{"a duplicate key keeps its first place and its last value", `{"a":1,"b":2,"a":3}`,
			`{"a":3,"b":2}`},
		{"a number keeps its source text", `[1.50,1E5,-0,0.0,1e-400]`, `[1.50,1E5,-0,0.0,1e-400]`},
		{"an integer of any size stays exact", `123456789012345678901234567890`,
			`123456789012345678901234567890`},
		{"whitespace around and inside is dropped", " \t\n[ 1 , {\"a\" : true} ]\r\n",
			`[1,{"a":true}]`},
		{"HTML characters are not escaped", `"<a>&amp;"`, `"<a>&amp;"`},
		{"escapes are read and written minimally", `"é\/\n\u0001"`, `"é/\n\u0001"`},
		{"a surrogate pair is one character", `"😀"`, `"😀"`},
		{"a lone surrogate becomes U+FFFD", `"a\ud83db"`, `"a�b"`},
		{"each byte of invalid UTF-8 becomes U+FFFD", "\"\xe2\x82x\xff\"", `"��x�"`},
		{"empty containers stay empty", `{"a":[],"b":{}}`, `{"a":[],"b":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := DecodeString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.JSON(); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestDecodeRefusesWhatIsNotJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset int
		cause  error
	}{
		{"an empty text", ``, 0, io.ErrUnexpectedEOF},
		{"a value cut short", `{"a":[1,`, 8, io.ErrUnexpectedEOF},
		{"a string cut short", `"abc`, 4, io.ErrUnexpectedEOF},
		{"a trailing comma", `[1,]`, 2, nil},
		{"a leading zero", `01`, 1, errExtraData},
		{"a second value", `{} {}`, 3, errExtraData},
		{"NaN", `NaN`, 0, nil},
		{"Infinity", `[Infinity]`, 1, nil},
		{"a byte-order mark", "\ufeff{}", 0, nil},
		{"a control character in a string", "\"a\tb\"", 2, nil},
		{"a single-quoted string", `'a'`, 0, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeString(test.input)
			var decodeErr *Error
			if !errors.As(err, &decodeErr) {
				t.Fatalf("got %v, want an *Error", err)
			}
			if decodeErr.Offset != test.offset {
				t.Errorf("offset %d, want %d (%v)", decodeErr.Offset, test.offset, err)
			}
			var syntax *json.SyntaxError
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Errorf("%v does not wrap %v", err, test.cause)
			}
			if test.cause == nil && !errors.As(err, &syntax) {
				t.Errorf("%v does not wrap the standard library's syntax error", err)
			}
		})
	}
}

func TestAnErrorNamesItsLineAndColumnInCharacters(t *testing.T) {
	_, err := DecodeString("{\n  \"é\": ]")
	want := "invalid JSON at line 2, column 8: invalid character ']' after object key:value pair"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

func TestDecodeReadsAnyDepthUpToItsLimitWithoutRecursion(t *testing.T) {
	deep := func(levels int) string {
		return strings.Repeat(`{"a":[`, levels) + strings.Repeat(`]}`, levels)
	}
	if _, err := DecodeString(deep(5000)); err != nil {
		t.Fatalf("10,000 levels: %v", err)
	}
	if _, err := DecodeString(deep(5001)); err == nil {
		t.Fatal("10,002 levels decoded")
	}
	if _, err := DecodeString(strings.Repeat("[", 1_000_000)); err == nil {
		t.Fatal("a million open arrays decoded")
	}
}

func TestDecodePrefixReadsOneValueAndLeavesTheRest(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		start int
		want  string
		end   int
	}{
		{"concatenated documents", `{"a":1}{"b":2}`, 0, `{"a":1}`, 7},
		{"the second of them", `{"a":1}{"b":2}`, 7, `{"b":2}`, 14},
		{"whitespace before the value", `x  [1] tail`, 1, `[1]`, 6},
		{"a number before a letter", `12abc`, 0, `12`, 2},
		{"a word before more text", `truex`, 0, `true`, 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, end, err := NewDocument(test.text).DecodePrefix(test.start)
			if err != nil {
				t.Fatal(err)
			}
			if value.JSON() != test.want || end != test.end {
				t.Errorf("got %s ending at %d, want %s at %d", value.JSON(), end, test.want, test.end)
			}
		})
	}
	if _, _, err := NewDocument(`[1, 2`).DecodePrefix(0); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a value cut short gave %v", err)
	}
}

// damagedValues returns n damaged values, each on its own line after one
// character that is not ASCII, and the offset where each starts.
func damagedValues(n int) (string, []int) {
	var text strings.Builder
	var starts []int
	for range n {
		text.WriteString("é ")
		starts = append(starts, text.Len())
		text.WriteString(`{"a": ]` + "\n")
	}
	return text.String(), starts
}

func TestADocumentCountsEachErrorsPlaceOnFromTheLast(t *testing.T) {
	for _, n := range []int{100, 200} {
		text, starts := damagedValues(n)
		document := NewDocument(text)
		for _, start := range starts {
			_, _, fresh := NewDocument(text).DecodePrefix(start)
			if _, _, err := document.DecodePrefix(start); errorText(err) != errorText(fresh) {
				t.Fatalf("value at %d: %v, want %v", start, err, fresh)
			}
		}
		if document.Scanned() > len(text) {
			t.Errorf("%d errors read %d bytes of a %d-byte text to place them", n, document.Scanned(), len(text))
		}
	}
}

func TestADocumentPlacesAnErrorAsAFreshCountDoesFromAnyStart(t *testing.T) {
	tests := []struct{ name, text string }{
		{"a character of two bytes", "\u00e9 [x \u00e9 [x"},
		{"a character of three bytes", "\u20ac [x\n\u20ac [x \u20ac"},
		{"a character of four bytes", "\U0001F600 [x \U0001F600 [x"},
		{"a character cut short", "\xe2\x82 [x \u20ac [x"},
	}
	for _, test := range tests {
		t.Run(test.name+" before each start, inside it or not", func(t *testing.T) {
			document := NewDocument(test.text)
			for start := range len(test.text) {
				_, _, fresh := NewDocument(test.text).DecodePrefix(start)
				if _, _, err := document.DecodePrefix(start); errorText(err) != errorText(fresh) {
					t.Errorf("value at %d: %v, want %v", start, err, fresh)
				}
			}
		})
	}
}

func TestADocumentPlacesAnEarlierErrorFromTheStart(t *testing.T) {
	text, starts := damagedValues(3)
	document := NewDocument(text)
	for _, start := range []int{starts[2], starts[0], -1, len(text) + 4} {
		_, _, fresh := NewDocument(text).DecodePrefix(start)
		_, _, err := document.DecodePrefix(start)
		var got, want *Error
		if !errors.As(err, &got) || !errors.As(fresh, &want) || got.Offset != want.Offset || got.Error() != want.Error() {
			t.Errorf("value at %d: %v at %d, want %v at %d", start, got, got.Offset, want, want.Offset)
		}
	}
}

func TestTextGivesEachKindAsRowText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"a string as it is", `"a \"b\""`, `a "b"`},
		{"a number as written", `1.50`, `1.50`},
		{"true", `true`, `true`},
		{"false", `false`, `false`},
		{"null as nothing", `null`, ``},
		{"an object as compact JSON", `{ "b" : [1, "<x>"] , "a":null }`, `{"b":[1,"<x>"],"a":null}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := DecodeString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.Text(); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestIsEmptyHoldsNullFalseZeroAndEmptyContainers(t *testing.T) {
	tests := []struct {
		input string
		empty bool
	}{
		{`null`, true}, {`false`, true}, {`0`, true}, {`-0`, true}, {`0.0`, true}, {`0e10`, true},
		{`0.000e-999999999999999999999`, true}, {`""`, true}, {`[]`, true}, {`{}`, true},
		{`true`, false}, {`1`, false}, {`0.5`, false}, {`1e400`, false}, {`" "`, false},
		{`1e-400`, false}, {`2e-324`, false}, {`-1e-324`, false},
		{`0.` + strings.Repeat("0", 330) + `1`, false}, {`9e-100000000000000000`, false},
		{`[0]`, false}, {`{"a":null}`, false},
	}
	for _, test := range tests {
		t.Run("the value "+test.input, func(t *testing.T) {
			value, err := DecodeString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.IsEmpty(); got != test.empty {
				t.Errorf("got %v, want %v", got, test.empty)
			}
		})
	}
}

func TestNumberAccessorsReadTheSourceText(t *testing.T) {
	value, _ := DecodeString(`[7, 1.5, 12345678901234567890, 1e400, "7"]`)
	items := value.Items()
	if got, ok := items[0].Int(); got != 7 || !ok || !items[0].IsInt() {
		t.Errorf("7 read as %d, %v", got, ok)
	}
	if _, ok := items[1].Int(); ok || !items[1].IsFloat() {
		t.Error("1.5 read as an integer")
	}
	if got, ok := items[1].Float(); got != 1.5 || !ok {
		t.Errorf("1.5 read as %v, %v", got, ok)
	}
	if _, ok := items[2].Int(); ok {
		t.Error("an integer beyond int64 read as one")
	}
	if _, ok := items[3].Float(); ok {
		t.Error("a number beyond float64 read as one")
	}
	if items[2].Number() != "12345678901234567890" || items[4].Number() != "" {
		t.Errorf("Number gave %q and %q", items[2].Number(), items[4].Number())
	}
}

func TestEqualComparesNumbersByValueAndObjectsWithoutOrder(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		equal       bool
	}{
		{"an integer and its float", `1`, `1.0`, true},
		{"two spellings of a float", `1.50`, `15e-1`, true},
		{"large integers exactly", `12345678901234567890`, `12345678901234567891`, false},
		{"objects in another key order", `{"a":1,"b":[2]}`, `{"b":[2.0],"a":1}`, true},
		{"objects with another key", `{"a":1}`, `{"b":1}`, false},
		{"arrays in another order", `[1,2]`, `[2,1]`, false},
		{"a number and its text", `1`, `"1"`, false},
		{"false and zero", `false`, `0`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, _ := DecodeString(test.left)
			right, _ := DecodeString(test.right)
			if got := Equal(left, right); got != test.equal {
				t.Errorf("got %v, want %v", got, test.equal)
			}
		})
	}
}

func TestBuiltValuesEncodeAsJSON(t *testing.T) {
	object := NewObject(Member{"text", NewString("a\xffb<")}, Member{"n", NewInt(-3)},
		Member{"f", NewFloat(0.1)}, Member{"whole", NewFloat(2)}, Member{"nan", NewFloat(math.NaN())},
		Member{"list", NewArray(NewBool(true), NewNull())}, Member{"n", NewInt(4)})
	want := `{"text":"a�b<","n":4,"f":0.1,"whole":2,"nan":"NaN","list":[true,null]}`
	if got := object.JSON(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if !object.Get("whole").IsFloat() {
		t.Error("NewFloat(2) is not in float form")
	}
}

func TestSetKeepsAKeysPlaceAndCopiesShareTheObject(t *testing.T) {
	object := NewObject()
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		object.Set(key, NewString(key))
	}
	copied := object
	copied.Set("a", NewInt(1))
	if !slices.Equal(object.Keys(), strings.Split("a b c d e f g h i", " ")) ||
		object.Get("a").Text() != "1" || !object.Has("i") || object.Has("z") {
		t.Errorf("object is %s", object.JSON())
	}
}

func TestJSONEscapesEveryControlCharacterInStringsAndKeys(t *testing.T) {
	var control strings.Builder
	for char := rune(0); char < 0x20; char++ {
		control.WriteRune(char)
	}
	control.WriteString("\x7f\u2028")
	value := NewObject(Member{Key: control.String(), Value: NewArray(NewString(control.String()))})
	text := value.JSON()
	if strings.ContainsFunc(text, func(char rune) bool { return char < 0x20 }) {
		t.Errorf("a control character is written raw: %q", text)
	}
	for _, want := range []string{`\u0000`, `\u001b`, `\t`, "\x7f", `\u2028`} {
		if strings.Count(text, want) != 2 {
			t.Errorf("%q is not in both the key and the string: %q", want, text)
		}
	}
}
