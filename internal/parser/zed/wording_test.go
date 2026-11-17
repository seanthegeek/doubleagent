package zed

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestAValueThatDoesNotDecodeSaysWhyInPlainWords(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"text that is not JSON gives the reader's message", sqlitefile.TextValue(`[{"a"`),
			"invalid JSON at line 1, column 6: expected ':' after a property name"},
		{"a number is not text", sqlitefile.IntValue(7), "the value is not text"},
		{"JSON text decodes", sqlitefile.TextValue(`[1]`), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, got := decodeColumn(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAWorktreeRecordWithoutACreationTimeSaysWhy(t *testing.T) {
	tests := []struct {
		name, record, want string
	}{
		{"a record that is not an object", `[1]`, "the record is not a JSON object"},
		{"a record without the seconds", `{"created_at_nanos": 1}`, "no created_at_seconds field"},
		{"seconds that are not a number", `{"created_at_seconds": "1"}`, "the creation time is not a number"},
		{"nanoseconds that are not a number", `{"created_at_seconds": 1, "created_at_subsec_nanos": []}`,
			"the creation time is not a number"},
		{"a whole record", `{"created_at_seconds": 1, "created_at_subsec_nanos": 500000000}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, got := createdSeconds(decoded(t, test.record)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
