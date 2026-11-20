package vscode

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestMarkdownTextJoinsTheLeavesOfNestedArraysWithLineBreaks(t *testing.T) {
	tests := []struct {
		name, value, want string
	}{
		{"a plain string", `"a"`, "a"},
		{"a markdown string", `{"value": "a"}`, "a"},
		{"an object whose value is not a string", `{"value": 1}`, ""},
		{"null", `null`, ""},
		{"a number", `3`, "3"},
		{"an empty array", `[]`, ""},
		{"an empty array beside a string", `[[], "b"]`, "\nb"},
		{"two empty arrays", `[[], []]`, "\n"},
		{"a nested array then a string", `[["a", "b"], "c"]`, "a\nb\nc"},
		{"items of every kind", `[{"value": "x"}, null, 3, [{"value": "y"}, []]]`, "x\n\n3\ny\n"},
		{"three levels", `[[["a", ""], ""], ""]`, "a\n\n\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if got := mdText(value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// nestedMarkdown returns depth arrays around text, each with an empty
// string after its one inner item: [[…[text, ""]…, ""], ""].
func nestedMarkdown(t *testing.T, text string, depth int) jsonvalue.Value {
	t.Helper()
	encoded := strings.Repeat("[", depth) + `"` + text + `"` + strings.Repeat(`, ""]`, depth)
	value, err := jsonvalue.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMarkdownTextCopiesTheTextOnceHoweverDeepTheArraysAroundIt(t *testing.T) {
	const length = 10000
	// perLevel is the most each level of arrays may add to what the call
	// allocates: room for one more leaf, never another copy of the text.
	const perLevel = 128
	text := strings.Repeat("w", length)
	allocated := map[int]uint64{}
	for _, depth := range []int{100, 400} {
		t.Run(fmt.Sprintf("%d levels", depth), func(t *testing.T) {
			value := nestedMarkdown(t, text, depth)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			got := mdText(value)
			runtime.ReadMemStats(&after)
			allocated[depth] = after.TotalAlloc - before.TotalAlloc
			if want := text + strings.Repeat("\n", depth); got != want {
				t.Errorf("got %d bytes, want the text and %d line breaks", len(got), depth)
			}
		})
	}
	if grown := allocated[400] - allocated[100]; grown > 300*perLevel {
		t.Errorf("300 more levels allocated %d more bytes (%d against %d), more than %d a level",
			grown, allocated[400], allocated[100], perLevel)
	}
}
