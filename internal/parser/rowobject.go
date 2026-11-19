package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// ColumnName returns a column name with its ASCII letters lowered: the
// name SQL finds a column by, since it matches a column name in any ASCII
// case and every other character exactly. Two spellings of one column give
// the same name.
func ColumnName(name string) string {
	return strings.Map(func(char rune) rune {
		if char >= 'A' && char <= 'Z' {
			return char + 'a' - 'A'
		}
		return char
	}, name)
}

// RowObject returns a database row as a JSON object of its columns, in
// the order the query gave them, each keyed by its ColumnName and valued
// by ColumnValue. A parser that reads the row as a record looks a column
// up by the name the agent's schema gives it, in lower case, so it finds
// the column however the database at hand spells it.
func RowObject(row sqlitefile.Row) jsonvalue.Value {
	var members []jsonvalue.Member
	for i, column := range row.Columns() {
		members = append(members, jsonvalue.Member{Key: ColumnName(column), Value: ColumnValue(row.Values()[i])})
	}
	return jsonvalue.NewObject(members...)
}

// ColumnOf returns the column of table that SQL finds under name, in any
// ASCII case, and whether it has one.
func ColumnOf(table *sqlitefile.Table, name string) (sqlitefile.Column, bool) {
	for _, column := range table.Columns() {
		if ColumnName(column.Name) == ColumnName(name) {
			return column, true
		}
	}
	return sqlitefile.Column{}, false
}
