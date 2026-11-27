package agentzero

import "testing"

func TestZeroAgentIsTheMainAgentOnly(t *testing.T) {
	tests := []struct {
		name, json string
		want       bool
	}{
		{"null", "null", true},
		{"zero", "0", true},
		{"false", "false", true},
		{"text zero", `"0"`, true},
		{"one", "1", false},
		{"true", "true", false},
		{"text one", `"1"`, false},
		{"a number beyond the float64 range is not the main agent", "1e999", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := zeroAgent(decoded(t, test.json)); got != test.want {
				t.Errorf("zeroAgent(%s) = %v, want %v", test.json, got, test.want)
			}
		})
	}
}
