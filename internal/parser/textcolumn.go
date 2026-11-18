package parser

import "github.com/seanthegeek/doubleagent/internal/sqlitefile"

// TextColumn returns a text column value as it is, and "" for a value of
// any other type, for a column whose number or blob means nothing as text.
func TextColumn(value sqlitefile.Value) string {
	if value.Type == sqlitefile.Text {
		return value.Text()
	}
	return ""
}
