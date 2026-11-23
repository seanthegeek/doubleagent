package parserkit

import "github.com/seanthegeek/doubleagent/internal/sqlitefile"

// ColumnLine returns an integer column, such as a row id, as a row's
// source line, and 0 for a value of any other type.
func ColumnLine(value sqlitefile.Value) int {
	if value.Type == sqlitefile.Integer {
		return int(value.Int)
	}
	return 0
}
