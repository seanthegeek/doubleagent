package parser

import (
	"math"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestNumberOrderSortsANumberBeyondTheRangeByItsSign(t *testing.T) {
	tests := []struct {
		name, json string
		want       float64
	}{
		{"an integer", "12", 12},
		{"a float", "-2.5", -2.5},
		{"a number too large for a float sorts last", "1e999", math.Inf(1)},
		{"a negative number too large sorts first", "-1e999", math.Inf(-1)},
		{"a number too small for a float is zero", "1e-999", 0},
		{"null", "null", 0},
		{"text of digits", `"7"`, 0},
		{"true", "true", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := NumberOrder(value); got != test.want {
				t.Errorf("NumberOrder(%s) = %v, want %v", test.json, got, test.want)
			}
		})
	}
}
