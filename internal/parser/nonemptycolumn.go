package parser

import "github.com/seanthegeek/doubleagent/internal/sqlitefile"

// NonEmptyColumn returns a column value as row text (sqlitefile.Value's
// String), "" for an empty value: NULL, zero, or empty text or blob.
func NonEmptyColumn(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.String()
}
