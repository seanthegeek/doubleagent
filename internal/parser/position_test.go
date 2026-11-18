package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestJSONPositionSaysWhereButNotWhat(t *testing.T) {
	tests := []struct {
		name, document, want string
	}{
		{"an unquoted value", `{"api_key": sk-SECRET123}`, "invalid JSON at line 1, column 13"},
		{"a value on a later line", "{\n  \"key\": 0123456789\n}", "invalid JSON at line 2, column 11"},
		{"a document cut off", `{"key": "sk-SEC`, "invalid JSON at line 1, column 16"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := jsonvalue.DecodeString(test.document)
			if err == nil {
				t.Fatal("decoded")
			}
			got := JSONPosition(err)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
			for _, secret := range []string{"sk", "SEC", "0123"} {
				if strings.Contains(got, secret) {
					t.Errorf("%q holds %q from the document", got, secret)
				}
			}
		})
	}
	t.Run("an error without a position", func(t *testing.T) {
		if got := JSONPosition(errors.New("sk-SECRET")); got != "invalid JSON" {
			t.Errorf("got %q", got)
		}
	})
}
