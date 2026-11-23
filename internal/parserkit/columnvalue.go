package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// ColumnValue returns a column value as the JSON value of the same type,
// so a row reads as a record does: NULL as null, an integer or float as a
// number, text as a string, and a blob as its row text
// (sqlitefile.Value's String), x'<hex>'.
func ColumnValue(value sqlitefile.Value) jsonvalue.Value {
	switch value.Type {
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int)
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float)
	case sqlitefile.Text, sqlitefile.Blob:
		return jsonvalue.NewString(value.String())
	}
	return jsonvalue.NewNull()
}
