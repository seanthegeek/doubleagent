package sqlitefile

import "fmt"

// SchemaRow is one row of the schema table on page 1 ("Storage Of The SQL
// Database Schema"): Type is table, index, view or trigger; TableName the
// table an index or trigger belongs to; RootPage the b-tree's root, 0 for a
// view, a trigger or a virtual table; SQL the CREATE statement, "" for the
// indexes SQLite makes itself.
type SchemaRow struct {
	Type      string
	Name      string
	TableName string
	RootPage  uint32
	SQL       string
}

// Schema returns the schema table's rows in b-tree order, the order SQLite
// lists them. An empty database has none.
func (d *Database) Schema() ([]SchemaRow, error) {
	if d.pageCount == 0 {
		return nil, nil
	}
	var schema []SchemaRow
	rows := d.TableRows(1)
	for rows.Next() {
		values, err := rows.Record()
		if err != nil {
			return nil, fmt.Errorf("read schema: %w", err)
		}
		row, err := schemaRow(values)
		if err != nil {
			return nil, fmt.Errorf("read schema row %d: %w", rows.Rowid(), err)
		}
		schema = append(schema, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	return schema, nil
}

// schemaRow converts a schema record's five values, missing ones read as
// NULL: four texts and a root page number.
func schemaRow(values []Value) (SchemaRow, error) {
	values = append(values, make([]Value, max(0, 5-len(values)))...)
	texts := make([]string, 0, 4)
	for _, column := range []int{0, 1, 2, 4} {
		switch values[column].Type {
		case Text:
			texts = append(texts, values[column].Text())
		case Null:
			texts = append(texts, "")
		default:
			return SchemaRow{}, malformed("schema column %d holds a %s", column+1, values[column].Type)
		}
	}
	root := values[3]
	if root.Type != Null && (root.Type != Integer || root.Int < 0 || root.Int > 0xffffffff) {
		return SchemaRow{}, malformed("schema root page is not a page number")
	}
	return SchemaRow{Type: texts[0], Name: texts[1], TableName: texts[2],
		RootPage: uint32(root.Int), SQL: texts[3]}, nil
}
