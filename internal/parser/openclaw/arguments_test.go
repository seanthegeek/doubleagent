package openclaw

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestToolArgsTextGivesTheRowText(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{"arguments as a JSON string give their command", `"{\"command\": \"ls -la\"}"`, "ls -la"},
		{"an empty command gives way to the path", `{"path": "/a", "command": ""}`, "/a"},
		{"a path that is not a string gives way to the url", `{"url": "https://x", "path": 5}`,
			"https://x"},
		{"other arguments are sorted compact JSON", `{"b": 1, "a": "é"}`, `{"b":1,"a":"é"}`},
		{"a string that is not JSON is kept", `"not json {"`, "not json {"},
		{"blanks that are not JSON are kept", `"  "`, "  "},
		{"an empty string is empty", `""`, ""},
		{"null is empty", `null`, ""},
		{"an empty object is empty", `{}`, ""},
		{"an empty object in a string is empty", `"{}"`, ""},
		{"an empty array is JSON", `[]`, "[]"},
		{"a JSON string in a string is JSON", `"\"quoted\""`, `"quoted"`},
		{"zero is JSON", `0`, "0"},
		{"false is JSON", `false`, "false"},
		{"an array in a string is compact JSON", `"[1, 2]"`, "[1,2]"},
		{"a command that is not a string is JSON", `{"command": ["ls"]}`, `{"command":["ls"]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arguments, err := jsonvalue.DecodeString(test.arguments)
			if err != nil {
				t.Fatal(err)
			}
			if got := ToolArgsText(nil, arguments); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
