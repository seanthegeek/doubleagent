package musecode

import (
	"math"
	"testing"
)

func TestExitSucceededOnlyForNoFailure(t *testing.T) {
	tests := []struct {
		name, json string
		want       bool
	}{
		{"null", "null", true},
		{"false", "false", true},
		{"zero", "0", true},
		{"a float zero", "0.0", true},
		{"true", "true", false},
		{"one", "1", false},
		{"a number beyond the float64 range is a failure", "1e999", false},
		{"text zero", `"0"`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := exitSucceeded(decoded(t, test.json)); got != test.want {
				t.Errorf("exitSucceeded(%s) = %v, want %v", test.json, got, test.want)
			}
		})
	}
}

func TestIndexNumberOrdersAChildIndex(t *testing.T) {
	tests := []struct {
		name, json string
		want       float64
	}{
		{"an integer", "3", 3},
		{"true is one", "true", 1},
		{"false is zero", "false", 0},
		{"null is zero", "null", 0},
		{"a number beyond the float64 range sorts last", "1e999", math.Inf(1)},
		{"a negative one sorts first", "-1e999", math.Inf(-1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := indexNumber(decoded(t, test.json)); got != test.want {
				t.Errorf("indexNumber(%s) = %v, want %v", test.json, got, test.want)
			}
		})
	}
}
