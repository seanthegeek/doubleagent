package parser

import (
	"math"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestColumnKeyIsEqualForEqualValues(t *testing.T) {
	tests := []struct {
		name        string
		left, right sqlitefile.Value
		equal       bool
	}{
		{"an integer and a whole float", sqlitefile.IntValue(3), sqlitefile.FloatValue(3), true},
		{"two integers", sqlitefile.IntValue(3), sqlitefile.IntValue(4), false},
		{"a fraction", sqlitefile.FloatValue(3.5), sqlitefile.FloatValue(3.5), true},
		{"a fraction and an integer", sqlitefile.FloatValue(3.5), sqlitefile.IntValue(3), false},
		{"a float too large for an integer", sqlitefile.FloatValue(1e19), sqlitefile.FloatValue(1e19), true},
		{"infinity", sqlitefile.FloatValue(math.Inf(1)), sqlitefile.FloatValue(math.Inf(1)), true},
		{"text and a number", sqlitefile.TextValue("3"), sqlitefile.IntValue(3), false},
		{"text and a blob of its bytes", sqlitefile.TextValue("a"), sqlitefile.BlobValue([]byte("a")), false},
		{"two texts", sqlitefile.TextValue("a"), sqlitefile.TextValue("a"), true},
		{"NULL and NULL", sqlitefile.Value{}, sqlitefile.Value{}, true},
		{"NULL and the text null", sqlitefile.Value{}, sqlitefile.TextValue("null"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnKey(test.left) == ColumnKey(test.right); got != test.equal {
				t.Errorf("keys %q and %q: equal %v, want %v", ColumnKey(test.left),
					ColumnKey(test.right), got, test.equal)
			}
		})
	}
}

func TestColumnLineTakesOnlyAnInteger(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  int
	}{
		{"an integer", sqlitefile.IntValue(12), 12},
		{"a float", sqlitefile.FloatValue(12), 0},
		{"text of digits", sqlitefile.TextValue("12"), 0},
		{"NULL", sqlitefile.Value{}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnLine(test.value); got != test.want {
				t.Errorf("ColumnLine = %d, want %d", got, test.want)
			}
		})
	}
}
