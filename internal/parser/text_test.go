package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// decoded returns the JSON text decoded, failing the test on an error.
func decoded(t *testing.T, text string) pyjson.Value {
	t.Helper()
	value, err := pyjson.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// The expected values of these tables are what the Python analyzer's
// helpers return for the same JSON.

func TestTextOfReadsEachShapeOfMessageContent(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"null is empty", `null`, ""},
		{"a string is itself", `"s"`, "s"},
		{"a block list joins strings, texts and images", `["a",{"text":"b"},{"type":"image"},` +
			`{"type":"image","text":"t"},5,{"text":5,"type":"text"},["x"]]`, "a\nb\n[image]\nt"},
		{"an object gives its text", `{"text":"x"}`, "x"},
		{"an object without text is spaced JSON in document order",
			`{"a":"é","b":[1,2.0],"c":{"d":null}}`, `{"a": "é", "b": [1, 2.0], "c": {"d": null}}`},
		{"an image object whose text is null is JSON", `{"text":null,"type":"image"}`,
			`{"text": null, "type": "image"}`},
		{"an integer is its digits", `5`, "5"},
		{"a big integer keeps every digit", `12345678901234567890123`, "12345678901234567890123"},
		{"a float is Python's repr", `1.5`, "1.5"},
		{"an exponent float is Python's repr", `1e16`, "1e+16"},
		{"true is Python's True", `true`, "True"},
		{"false is Python's False", `false`, "False"},
		{"an empty list is empty", `[]`, ""},
		{"an empty object is {}", `{}`, "{}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TextOf(decoded(t, test.content)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestCompactJSONSortsKeysAndKeepsNumberForms(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"keys are sorted and non-ASCII kept", `{"b":1,"a":"é","c":[1.0,1e-5,true,null]}`,
			`{"a":"é","b":1,"c":[1.0,1e-05,true,null]}`},
		{"a line separator is kept as it is", "\"x\u2028\"", "\"x\u2028\""},
		{"an empty list", `[]`, `[]`},
		{"negative zero", `-0.0`, `-0.0`},
		{"an overflowing float is Infinity", `1E400`, `Infinity`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CompactJSON(decoded(t, test.value)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSettingValueWritesBooleansLowerAndTheRestAsJSON(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"true", `true`, "true"},
		{"false", `false`, "false"},
		{"a string as it is", `"s"`, "s"},
		{"an empty string as it is", `""`, ""},
		{"an integer", `5`, "5"},
		{"null", `null`, "null"},
		{"an object with sorted keys", `{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{"a float keeps its fraction", `1.0`, "1.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SettingValue(decoded(t, test.value)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSettingOrDashDashesOnlyNullAndTheEmptyString(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"null", `null`, "-"},
		{"the empty string", `""`, "-"},
		{"a space is kept", `" "`, " "},
		{"a string as it is", `"s"`, "s"},
		{"false is written", `false`, "false"},
		{"true is written", `true`, "true"},
		{"zero is written", `0`, "0"},
		{"a float zero is written", `0.0`, "0.0"},
		{"a float as Python writes it", `1.50`, "1.5"},
		{"an empty list is JSON", `[]`, "[]"},
		{"an empty object is JSON", `{}`, "{}"},
		{"an object is sorted JSON", `{"b":1,"a":"é"}`, `{"a":"é","b":1}`},
		{"NaN", `NaN`, "NaN"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SettingOrDash(decoded(t, test.value)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTrackerReportsTheFirstValueAndEachChangePerKey(t *testing.T) {
	tracker := Tracker{}
	steps := []struct{ key, value, want string }{
		{"s", "a", "model: a"},
		{"s", "a", ""},
		{"s", "", ""},
		{"s", "b", "model: a -> b"},
		{"t", "b", "model: b"},
		{"s", "a", "model: b -> a"},
	}
	for i, step := range steps {
		if got := tracker.Change("model", step.key, step.value); got != step.want {
			t.Errorf("step %d: got %q, want %q", i, got, step.want)
		}
	}
}

func TestStrGivesNothingForAFalseValue(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"a string", `"s1"`, "s1"},
		{"an empty string", `""`, ""},
		{"null", `null`, ""},
		{"zero", `0`, ""},
		{"a float zero", `0.0`, ""},
		{"false", `false`, ""},
		{"true", `true`, "True"},
		{"a number", `42`, "42"},
		{"an empty list", `[]`, ""},
		{"a list", `["a"]`, "['a']"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Str(decoded(t, test.value)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
