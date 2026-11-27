package sqlitefile

import (
	"fmt"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// schemaRecord is one row of the schema table on page 1 ("Storage Of The SQL
// Database Schema"): Type is table, index, view or trigger; TableName the
// table an index or trigger belongs to; RootPage the b-tree's root, 0 for a
// view, a trigger or a virtual table; SQL the CREATE statement, "" for the
// indexes SQLite makes itself.
type schemaRecord struct {
	Type      string
	Name      string
	TableName string
	RootPage  uint32
	SQL       string
}

// schemaIndex is the schema table as read once per database: its rows, and
// the position of the first table row of each folded name.
type schemaIndex struct {
	rows   []schemaRecord
	tables map[string]int
}

// schemaRows returns the schema table's rows in b-tree order, the order SQLite
// lists them. An empty database has none. The table is read once per
// Database; each call returns its own copy of the rows.
func (d *Database) schemaRows() ([]schemaRecord, error) {
	index, err := d.schemaIndex()
	if err != nil {
		return nil, err
	}
	return slices.Clone(index.rows), nil
}

// schemaIndex returns the schema table, reading it on first use.
func (d *Database) schemaIndex() (*schemaIndex, error) {
	d.schemaOnce.Do(func() {
		rows, err := d.readSchema()
		if err != nil {
			d.schemaErr = err
			return
		}
		index := &schemaIndex{rows: rows, tables: map[string]int{}}
		for i, row := range rows {
			key := textlines.LowerASCII(row.Name)
			if _, taken := index.tables[key]; row.Type == "table" && !taken {
				index.tables[key] = i
			}
		}
		d.schema = index
	})
	return d.schema, d.schemaErr
}

// readSchema walks the schema table's b-tree and converts its rows.
func (d *Database) readSchema() ([]schemaRecord, error) {
	if d.pageCount == 0 {
		return nil, nil
	}
	var schema []schemaRecord
	rows := d.tableRows(1)
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
func schemaRow(values []Value) (schemaRecord, error) {
	values = append(values, make([]Value, max(0, 5-len(values)))...)
	texts := make([]string, 0, 4)
	for _, column := range []int{0, 1, 2, 4} {
		switch values[column].Type {
		case Text:
			texts = append(texts, values[column].Text())
		case Null:
			texts = append(texts, "")
		default:
			return schemaRecord{}, malformed("schema column %d holds a %s", column+1, values[column].Type)
		}
	}
	root := values[3]
	if root.Type != Null && (root.Type != Integer || root.Int < 0 || root.Int > 0xffffffff) {
		return schemaRecord{}, malformed("schema root page is not a page number")
	}
	return schemaRecord{Type: texts[0], Name: texts[1], TableName: texts[2],
		RootPage: uint32(root.Int), SQL: texts[3]}, nil
}
