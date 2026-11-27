package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// columnName returns a column name with its ASCII letters lowered, as
// textlines.LowerASCII folds it: the name SQL finds a column by, since it
// matches a column name in any ASCII case and every other byte exactly.
// Two spellings of one column give the same name.
func columnName(name string) string {
	return textlines.LowerASCII(name)
}

// RowObject returns a database row as a JSON object of its columns, in
// the order the query gave them, each keyed by its columnName and valued
// by ColumnValue. A parser that reads the row as a record looks a column
// up by the name the agent's schema gives it, in lower case, so it finds
// the column however the database at hand spells it.
func RowObject(row columnsAndValues) jsonvalue.Value {
	columns, values := row.Columns(), row.Values()
	members := make([]jsonvalue.Member, len(columns))
	for i, column := range columns {
		members[i] = jsonvalue.Member{Key: columnName(column), Value: ColumnValue(values[i])}
	}
	return jsonvalue.NewObject(members...)
}

// columnsAndValues is what RowObject reads of a row: a sqlitefile.Row, or
// a test's stand-in that counts the values built. Values builds every
// column afresh on each call, so RowObject calls it once per row.
type columnsAndValues interface {
	Columns() []string
	Values() []sqlitefile.Value
}

// ColumnOf returns the column of table that SQL finds under name, in any
// ASCII case, and whether it has one.
func ColumnOf(table *sqlitefile.Table, name string) (sqlitefile.Column, bool) {
	for _, column := range table.Columns() {
		if columnName(column.Name) == columnName(name) {
			return column, true
		}
	}
	return sqlitefile.Column{}, false
}
