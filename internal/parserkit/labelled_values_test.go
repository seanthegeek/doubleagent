package parserkit

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestLabelledLeavesOutAnEmptyValue(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"text", `"a"`, "model=a"},
		{"a number as written", "1.50", "model=1.50"},
		{"null", "null", ""},
		{"the empty string", `""`, ""},
		{"false", "false", ""},
		{"zero", "0", ""},
		{"true", "true", "model=true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := Labelled("model=", value); got != test.want {
				t.Errorf("Labelled(%s) = %q, want %q", test.json, got, test.want)
			}
		})
	}
}

func TestObjectOrEmptyGivesAnObject(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"an object", `{"a":1}`, `{"a":1}`},
		{"null", "null", "{}"},
		{"a list", "[1]", "{}"},
		{"text", `"a"`, "{}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := ObjectOrEmpty(value).JSON(); got != test.want {
				t.Errorf("ObjectOrEmpty(%s) = %s, want %s", test.json, got, test.want)
			}
		})
	}
}

func TestColumnValueKeepsTheColumnsType(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"NULL", sqlitefile.Value{}, "null"},
		{"an integer", sqlitefile.IntValue(-3), "-3"},
		{"a float", sqlitefile.FloatValue(1.5), "1.5"},
		{"text", sqlitefile.TextValue("a\"b"), `"a\"b"`},
		{"a blob as its literal", sqlitefile.BlobValue([]byte("ab")), `"x'6162'"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnValue(test.value).JSON(); got != test.want {
				t.Errorf("ColumnValue = %s, want %s", got, test.want)
			}
		})
	}
}
