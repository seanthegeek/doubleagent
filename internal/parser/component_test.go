package parser

import "testing"

func TestComponentRefusesNamesThatLeaveTheirDirectory(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"session-1.jsonl", true},
		{"...", true},
		{".hidden", true},
		{"a b", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../x", false},
		{"a/b", false},
		{`a\b`, false},
		{`..\x`, false},
		{"a\x00b", false},
		{"/", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := Component(test.name)
			if ok != test.want {
				t.Errorf("got %v, want %v", ok, test.want)
			}
			if want := map[bool]string{true: test.name}[ok]; got != want {
				t.Errorf("got the name %q, want %q", got, want)
			}
		})
	}
}
