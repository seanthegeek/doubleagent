package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestCopyObjectLeavesTheOriginalAsItWas(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"an object keeps its order", `{"b":1,"a":2}`, `{"b":1,"a":2,"c":3}`},
		{"an empty object", `{}`, `{"c":3}`},
		{"a list is no object", `[1]`, `{"c":3}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			copied := CopyObject(original)
			copied.Set("c", jsonvalue.NewInt(3))
			if got := copied.JSON(); got != test.want {
				t.Errorf("the copy is %s, want %s", got, test.want)
			}
			if got := original.JSON(); got != test.json {
				t.Errorf("the original became %s, want %s", got, test.json)
			}
		})
	}
}
