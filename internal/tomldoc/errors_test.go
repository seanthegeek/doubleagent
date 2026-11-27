package tomldoc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseErrorsSayWhichRuleRefusedTheDocument(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		limit   bool
		message string
	}{
		{"a syntax error names its line and column", "a = 1\nb = \n", false,
			"invalid TOML at line 2, column 5: expected value but found '\\n' instead"},
		{"a document over the size limit names no position",
			strings.Repeat("#", maxDocumentBytes+1), true,
			"TOML document of 1048577 bytes is larger than the 1048576 bytes this reader accepts"},
		{"a document nested too deep names its line", "a = 1\n" + nestedArray(maxDepth), true,
			"TOML document nests keys, tables and arrays more than 32 levels deep at line 2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.text)
			var refusal *Error
			if !errors.As(fmt.Errorf("reading config.toml: %w", err), &refusal) {
				t.Fatalf("got %v, want an *Error through the wrapping", err)
			}
			if refusal.IsLimit() != test.limit {
				t.Errorf("IsLimit is %v, want %v", refusal.IsLimit(), test.limit)
			}
			if err.Error() != test.message {
				t.Errorf("message %q, want %q", err.Error(), test.message)
			}
		})
	}
}

func TestALimitRefusalQuotesNothingFromTheDocument(t *testing.T) {
	secret := "SECRET"
	tests := []struct {
		name, text string
	}{
		{"a document too large", "key = \"" + strings.Repeat(secret, maxDocumentBytes/len(secret)) + "\"\n"},
		{"a document too deep", "key = \"" + secret + "\"\n" + secret + " = " +
			strings.Repeat("["+secret+", ", maxDepth+1) + "]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.text)
			var refusal *Error
			if !errors.As(err, &refusal) || !refusal.IsLimit() {
				t.Fatalf("got %v, want a limit refusal", err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the message %q quotes the document", err.Error())
			}
		})
	}
}
