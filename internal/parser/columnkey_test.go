package parser

import (
	"math"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestColumnKeyIsSharedExactlyByValuesSQLFindsEqual(t *testing.T) {
	// From SQLite 3.51: SELECT a = b with a and b columns of no declared type.
	lowest, highest := math.Ldexp(-1, 63), math.Ldexp(1, 63)
	tests := []struct {
		name        string
		left, right sqlitefile.Value
		equal       bool
	}{
		{"an integer and a whole float", sqlitefile.IntValue(3), sqlitefile.FloatValue(3), true},
		{"zero and negative zero", sqlitefile.IntValue(0), sqlitefile.FloatValue(math.Copysign(0, -1)), true},
		{"two integers", sqlitefile.IntValue(3), sqlitefile.IntValue(4), false},
		{"a fraction", sqlitefile.FloatValue(3.5), sqlitefile.FloatValue(3.5), true},
		{"a fraction and an integer", sqlitefile.FloatValue(3.5), sqlitefile.IntValue(3), false},
		{"the lowest integer and the same float", sqlitefile.IntValue(math.MinInt64),
			sqlitefile.FloatValue(lowest), true},
		{"the float just above the lowest integer and that integer",
			sqlitefile.FloatValue(math.Nextafter(lowest, 0)), sqlitefile.IntValue(int64(math.Nextafter(lowest, 0))), true},
		{"the float just below the lowest integer and the lowest integer",
			sqlitefile.FloatValue(math.Nextafter(lowest, math.Inf(-1))), sqlitefile.IntValue(math.MinInt64), false},
		{"the float just below the lowest integer and itself",
			sqlitefile.FloatValue(math.Nextafter(lowest, math.Inf(-1))),
			sqlitefile.FloatValue(math.Nextafter(lowest, math.Inf(-1))), true},
		{"the highest integer and the float 2^63", sqlitefile.IntValue(math.MaxInt64),
			sqlitefile.FloatValue(highest), false},
		{"the float just below 2^63 and that integer", sqlitefile.FloatValue(math.Nextafter(highest, 0)),
			sqlitefile.IntValue(int64(math.Nextafter(highest, 0))), true},
		{"the float 2^63 and itself", sqlitefile.FloatValue(highest), sqlitefile.FloatValue(highest), true},
		{"a float too large for an integer", sqlitefile.FloatValue(1e19), sqlitefile.FloatValue(1e19), true},
		{"infinity", sqlitefile.FloatValue(math.Inf(1)), sqlitefile.FloatValue(math.Inf(1)), true},
		{"text and a number", sqlitefile.TextValue("3"), sqlitefile.IntValue(3), false},
		{"text and a blob of its bytes", sqlitefile.TextValue("a"), sqlitefile.BlobValue([]byte("a")), false},
		{"two texts", sqlitefile.TextValue("a"), sqlitefile.TextValue("a"), true},
		{"empty text and an empty blob", sqlitefile.TextValue(""), sqlitefile.BlobValue(nil), false},
		{"NULL and NULL", sqlitefile.Value{}, sqlitefile.Value{}, false},
		{"NULL and empty text", sqlitefile.Value{}, sqlitefile.TextValue(""), false},
		{"NULL and the text null", sqlitefile.Value{}, sqlitefile.TextValue("null"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, right := ColumnKey(test.left), ColumnKey(test.right)
			if got := left != "" && left == right; got != test.equal {
				t.Errorf("keys %q and %q: joined %v, want %v", left, right, got, test.equal)
			}
		})
	}
}

func TestOnlyNULLHasNoColumnKey(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		empty bool
	}{
		{"NULL", sqlitefile.Value{}, true},
		{"empty text", sqlitefile.TextValue(""), false},
		{"an empty blob", sqlitefile.BlobValue(nil), false},
		{"zero", sqlitefile.IntValue(0), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnKey(test.value) == ""; got != test.empty {
				t.Errorf("ColumnKey() = %q, want empty %v", ColumnKey(test.value), test.empty)
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
