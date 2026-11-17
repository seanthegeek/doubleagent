package pyjson

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// mustDecode decodes text or fails the test.
func mustDecode(t *testing.T, text string) Value {
	t.Helper()
	value, err := DecodeString(text)
	if err != nil {
		t.Fatalf("DecodeString(%q): %v", text, err)
	}
	return value
}

func TestTruthyAndFloatFollowPython(t *testing.T) {
	// bool(v) and repr(float(v)) as Python 3.14 gives them; "" where
	// float() raises, which Float reports as not ok.
	cases := []struct {
		input  string
		truthy bool
		float  string
	}{
		{"null", false, ""}, {"true", true, ""}, {"false", false, ""}, {"0", false, "0.0"},
		{"-0", false, "0.0"}, {"0.0", false, "0.0"}, {"-0.0", false, "-0.0"},
		{"1e-400", false, "0.0"}, {"NaN", true, "nan"}, {"1", true, "1.0"},
		{"-1", true, "-1.0"}, {`""`, false, ""}, {`"0"`, true, ""}, {`" "`, true, ""},
		{"[]", false, ""}, {"[0]", true, ""}, {"{}", false, ""}, {`{"":0}`, true, ""},
		{"Infinity", true, "inf"}, {"1" + strings.Repeat("0", 400), true, ""},
	}
	for _, test := range cases {
		t.Run("value "+printable(test.input)+" is truthy and converts as in Python", func(t *testing.T) {
			value := mustDecode(t, test.input)
			if got := value.Truthy(); got != test.truthy {
				t.Errorf("Truthy() = %v, want %v", got, test.truthy)
			}
			number, ok := value.Float()
			got := ""
			if ok {
				got = NewFloat(number).Str()
			}
			if got != test.float {
				t.Errorf("Float() = %q, want %q", got, test.float)
			}
		})
	}
}

func TestObjectAccessorsKeepDocumentOrder(t *testing.T) {
	record := mustDecode(t, `{"b":1,"a":{"c":[10,"x",null]},"b":2.5,"t":true,"s":"text"}`)
	if got := strings.Join(record.Keys(), ","); got != "b,a,t,s" {
		t.Errorf("Keys() = %q, want b,a,t,s", got)
	}
	if got, ok := record.Get("b").Float(); !ok || got != 2.5 || !record.Get("b").IsFloat() {
		t.Errorf("duplicate key b = %v, %v; want the last value 2.5", got, ok)
	}
	list := record.Get("a").Get("c")
	if got, ok := list.Index(0).Int(); !ok || got != 10 {
		t.Errorf("Index(0).Int() = %d, %v", got, ok)
	}
	if list.Index(1).Text() != "x" || !list.Index(2).IsNull() || !list.Index(3).IsNull() {
		t.Errorf("items = %s", Compact(list))
	}
	if list.Len() != 3 || len(list.Items()) != 3 || record.Len() != 4 {
		t.Errorf("lengths %d, %d, %d", list.Len(), len(list.Items()), record.Len())
	}
	if value, ok := record.Get("t").Bool(); !ok || !value {
		t.Errorf("Bool() = %v, %v", value, ok)
	}
	if !record.Has("s") || record.Has("missing") || !record.Get("missing").IsNull() {
		t.Error("Has or Get of a missing key is wrong")
	}
	if _, ok := record.Get("missing").Get("deeper").Index(4).Lookup("x"); ok {
		t.Error("a chain through missing values found something")
	}
	if record.Get("s").Text() != "text" || record.Get("b").Text() != "" {
		t.Error("Text() is wrong")
	}
}

func TestAccessorsOnTheWrongKindGiveZeroValues(t *testing.T) {
	for _, input := range []string{"null", "1", `"s"`, "[1]", `{"a":1}`, "true"} {
		value := mustDecode(t, input)
		t.Run("value "+input+" answers every accessor without a panic", func(t *testing.T) {
			_ = value.Get("a")
			_ = value.Index(0)
			_ = value.Keys()
			_ = value.Members()
			_ = value.Items()
			_, _ = value.Int()
			_, _ = value.Bool()
			value.Set("k", NewInt(1))
			if !value.IsObject() && value.Has("k") {
				t.Error("Set changed a value that is not an object")
			}
		})
	}
}

func TestIntReportsRangeAndForm(t *testing.T) {
	cases := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"9223372036854775807", math.MaxInt64, true},
		{"-9223372036854775808", math.MinInt64, true},
		{"9223372036854775808", 0, false},
		{"1.0", 0, false},
		{"-0", 0, true},
		{"true", 0, false},
	}
	for _, test := range cases {
		t.Run("integer "+test.input+" converts only when it fits", func(t *testing.T) {
			got, ok := mustDecode(t, test.input).Int()
			if got != test.want || ok != test.ok {
				t.Errorf("Int() = %d, %v, want %d, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestNumberTextKeepsTheSource(t *testing.T) {
	inputs := []string{"1.50", "1E5", "-0", "NaN", "-Infinity", "123456789012345678901234567890"}
	for _, input := range inputs {
		t.Run("number "+input+" keeps its text", func(t *testing.T) {
			if got := mustDecode(t, input).NumberText(); got != input {
				t.Errorf("NumberText() = %q", got)
			}
		})
	}
	if mustDecode(t, `"1"`).NumberText() != "" {
		t.Error("a string has number text")
	}
}

func TestConstructedValuesEncodeLikeDecodedOnes(t *testing.T) {
	object := NewObject(
		Member{"b", NewInt(-3)},
		Member{"a", NewArray(NewFloat(1), NewFloat(math.Inf(-1)), NewNull(), NewBool(false))},
		Member{"b", NewString("café")},
	)
	object.Set("c", NewArray())
	want := `{"b":"café","a":[1.0,-Infinity,null,false],"c":[]}`
	if got := Compact(object); got != want {
		t.Errorf("Compact = %s, want %s", got, want)
	}
	want = `{'b': 'café', 'a': [1.0, -inf, None, False], 'c': []}`
	if got := object.Repr(); got != want {
		t.Errorf("Repr = %s, want %s", got, want)
	}
	copied := object
	copied.Set("a", NewNull())
	if !object.Get("a").IsNull() {
		t.Error("a copy of an object does not share its members")
	}
}

func TestLargeObjectsFindKeysAndKeepTheDuplicateRule(t *testing.T) {
	var text strings.Builder
	text.WriteString("{")
	for i := range 40 {
		text.WriteString(`"k` + strconv.Itoa(i) + `":` + strconv.Itoa(i%10) + ",")
	}
	text.WriteString(`"k0":"last"}`)
	value := mustDecode(t, text.String())
	if value.Len() != 40 || value.Get("k0").Text() != "last" || value.Members()[0].Key != "k0" {
		t.Errorf("Len %d, k0 %s, first %s", value.Len(), Compact(value.Get("k0")), value.Members()[0].Key)
	}
	if got, ok := value.Get("k25").Int(); !ok || got != 5 {
		t.Errorf("k25 = %d, %v", got, ok)
	}
}

func TestSyntaxErrorsNameTheirPosition(t *testing.T) {
	cases := []struct {
		input, want string
		offset      int
	}{
		{"", "invalid JSON at line 1, column 1: expected a value", 0},
		{"[1,\n  x]", "invalid JSON at line 2, column 3: expected a value", 6},
		{`{"café":1,}`, "invalid JSON at line 1, column 11: trailing comma before '}'", 11},
		{`"abc`, "invalid JSON at line 1, column 1: unterminated string", 0},
		{"[1] 2", "invalid JSON at line 1, column 5: extra data after the value", 4},
		{"\"a\tb\"", "invalid JSON at line 1, column 3: invalid control character in string", 2},
		{`{"a" 1}`, "invalid JSON at line 1, column 6: expected ':' after a property name", 5},
		{`"\q"`, "invalid JSON at line 1, column 2: invalid escape", 1},
	}
	for _, test := range cases {
		t.Run("document "+printable(test.input)+" reports where it breaks", func(t *testing.T) {
			_, err := DecodeString(test.input)
			var syntax *SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("error %v is not a *SyntaxError", err)
			}
			if err.Error() != test.want || syntax.Offset != test.offset {
				t.Errorf("got %q at %d, want %q at %d", err, syntax.Offset, test.want, test.offset)
			}
		})
	}
}

func TestNestingIsLimited(t *testing.T) {
	deep := strings.Repeat("[", MaxDepth) + strings.Repeat("]", MaxDepth)
	if _, err := DecodeString(deep); err != nil {
		t.Fatalf("nesting of %d: %v", MaxDepth, err)
	}
	tooDeep := strings.Repeat(`{"a":`, MaxDepth+1)
	_, err := DecodeString(tooDeep)
	if err == nil || !strings.Contains(err.Error(), "nested too deeply") {
		t.Errorf("nesting past the limit gave %v", err)
	}
}

func TestDecodePrefixRejectsAnOffsetOutsideTheText(t *testing.T) {
	for _, start := range []int{-1, 4} {
		if _, _, err := DecodePrefix("[1]", start); err == nil {
			t.Errorf("DecodePrefix at %d gave no error", start)
		}
	}
}

func TestAppendStringEscapesAsPythonDoes(t *testing.T) {
	// json.dumps of "a", U+007F, U+2028, U+1F600, a lone U+D83D and U+0000,
	// with and without ensure_ascii, as Python 3.14 writes it.
	text := "a\x7f\xe2\x80\xa8\xf0\x9f\x98\x80\xed\xa0\xbd\x00"
	cases := []struct {
		format Format
		want   string
	}{
		{Format{ASCII: true}, `"a\u007f\u2028\ud83d\ude00\ud83d\u0000"`},
		{Format{}, "\"a\x7f\xe2\x80\xa8\xf0\x9f\x98\x80\xed\xa0\xbd\\u0000\""},
	}
	for _, test := range cases {
		if got := string(test.format.AppendString(nil, text)); got != test.want {
			t.Errorf("AppendString(%+v) = %q, want %q", test.format, got, test.want)
		}
	}
	if got := Compact(NewString("a\xffb")); got != "\"a\xef\xbf\xbdb\"" {
		t.Errorf("an invalid byte is written as %q", got)
	}
}

func TestGoUnicodeTablesAreTheVersionIsPrintableCorrects(t *testing.T) {
	// isPrintable removes what Unicode 17.0 added to reach Python 3.14's
	// Unicode 16.0; a Go release with other tables needs its list redone.
	if unicode.Version != "17.0.0" {
		t.Errorf("unicode.Version = %s; regenerate unicode17Additions for it", unicode.Version)
	}
	cases := []struct {
		char rune
		want bool
	}{{'a', true}, {0x1FA8A, false}, {0x1F600, true}, {0x00A0, false}, {0x0378, false}}
	for _, test := range cases {
		if got := isPrintable(test.char); got != test.want {
			t.Errorf("isPrintable(%U) = %v, want %v", test.char, got, test.want)
		}
	}
}
