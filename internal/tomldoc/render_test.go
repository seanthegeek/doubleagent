package tomldoc

import (
	"math"
	"testing"
)

func TestSettingTextWritesEachKindAsRowText(t *testing.T) {
	nested := &Table{}
	for i, key := range []string{"z", "a", "é"} {
		nested.set(key, Value{kind: KindInteger, integer: int64(i)})
	}
	tests := []struct {
		name  string
		value Value
		want  string
	}{
		{"a boolean is true or false", Value{kind: KindBoolean, boolean: true}, "true"},
		{"a string is as it is", Value{kind: KindString, text: "a <b>"}, "a <b>"},
		{"a date is its text", Value{kind: KindLocalDate, text: "1979-05-27"}, "1979-05-27"},
		{"a float is its shortest text", Value{kind: KindFloat, float: 0.1}, "0.1"},
		{"a whole float has no fraction", Value{kind: KindFloat, float: 100}, "100"},
		{"a large float has an exponent", Value{kind: KindFloat, float: 1e21}, "1e+21"},
		{"an infinite float is a JSON string", Value{kind: KindFloat, float: math.Inf(1)}, `"+Inf"`},
		{"an integer is its digits", Value{kind: KindInteger, integer: math.MinInt64},
			"-9223372036854775808"},
		{"a table keeps its key order", Value{kind: KindTable, table: nested}, `{"z":0,"a":1,"é":2}`},
		{"an array holds dates as strings", Value{kind: KindArray, items: []Value{
			{kind: KindLocalDate, text: "1979-05-27"}, {kind: KindString, text: "\u2028"}}},
			`["1979-05-27","\u2028"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SettingText(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
