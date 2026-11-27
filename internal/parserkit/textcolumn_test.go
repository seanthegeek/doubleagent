package parserkit

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestColumnsAsTextKeepWhatTheirNamesSay(t *testing.T) {
	tests := []struct {
		name     string
		value    sqlitefile.Value
		nonEmpty string
		text     string
	}{
		{"NULL", sqlitefile.Value{}, "", ""},
		{"empty text", sqlitefile.TextValue(""), "", ""},
		{"text", sqlitefile.TextValue("a b"), "a b", "a b"},
		{"zero", sqlitefile.IntValue(0), "", ""},
		{"an integer", sqlitefile.IntValue(7), "7", ""},
		{"a float", sqlitefile.FloatValue(1.5), "1.5", ""},
		{"a zero float", sqlitefile.FloatValue(0), "", ""},
		{"a blob", sqlitefile.BlobValue([]byte("ab")), "x'6162'", ""},
		{"an empty blob", sqlitefile.BlobValue(nil), "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NonEmptyColumn(test.value); got != test.nonEmpty {
				t.Errorf("NonEmptyColumn = %q, want %q", got, test.nonEmpty)
			}
			if got := TextColumn(test.value); got != test.text {
				t.Errorf("TextColumn = %q, want %q", got, test.text)
			}
		})
	}
}
