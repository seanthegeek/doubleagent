package sqlitefile

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrNoTable reports a table the schema does not hold.
var ErrNoTable = errors.New("no such table")

// Table is one table of a database, read through its CREATE TABLE text:
// an ordinary table, a WITHOUT ROWID table, or an fts5 virtual table,
// whose rows are read from its content table (name_content), column c0
// for the first declared column, c1 for the next, and id as the rowid.
type Table struct {
	database *Database
	shape    *tableShape
	root     uint32
	// positions maps each declared column to its place in the stored
	// record, -1 for a virtual generated column, which is not stored.
	positions []int
	// content is the fts5 content table, and contentColumns the index of
	// each declared column's value in its rows.
	content        *Table
	contentColumns []int
}

// TableNames returns the names of the schema's tables, as SQL's
// "select name from sqlite_master where type='table'" lists them: virtual
// tables, their shadow tables and SQLite's own tables included. Names keep
// their case; the map holds each one as true.
func (d *Database) TableNames() (map[string]bool, error) {
	schema, err := d.Schema()
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, row := range schema {
		if row.Type == "table" {
			names[row.Name] = true
		}
	}
	return names, nil
}

// Table returns the table called name, matched as SQL matches a table
// name (ASCII letters in any case). It fails with ErrNoTable when there is
// none, and with an error naming the table when its CREATE TABLE text
// cannot be read or it is a virtual table of a module other than fts5.
func (d *Database) Table(name string) (*Table, error) {
	schema, err := d.Schema()
	if err != nil {
		return nil, err
	}
	return d.tableFrom(schema, name)
}

// tableFrom finds and parses the table called name among schema's rows.
func (d *Database) tableFrom(schema []SchemaRow, name string) (*Table, error) {
	index := slices.IndexFunc(schema, func(row SchemaRow) bool {
		return row.Type == "table" && strings.EqualFold(row.Name, name)
	})
	if index < 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoTable, name)
	}
	row := schema[index]
	shape, err := parseCreateTable(row.SQL)
	if err != nil {
		return nil, fmt.Errorf("table %s: %w", row.Name, err)
	}
	table := &Table{database: d, shape: shape, root: row.RootPage}
	if shape.fts5 {
		return table, d.attachContent(schema, table)
	}
	if row.RootPage == 0 {
		return nil, malformed("table %s has no root page", row.Name)
	}
	table.positions = recordPositions(shape)
	return table, nil
}

// attachContent finds an fts5 table's content table and maps each declared
// column to the content table's column c<i> ("The Table Contents (%_content
// table)" in https://www.sqlite.org/fts5.html).
func (d *Database) attachContent(schema []SchemaRow, table *Table) error {
	content, err := d.tableFrom(schema, table.shape.name+"_content")
	if err != nil {
		return fmt.Errorf("fts5 table %s: %w", table.shape.name, err)
	}
	if !content.Has("id") || content.shape.withoutRowid {
		return fmt.Errorf("fts5 table %s: its content table has no id column", table.shape.name)
	}
	for i := range table.shape.columns {
		index := content.shape.columnIndex("c" + strconv.Itoa(i))
		if index < 0 {
			return fmt.Errorf("fts5 table %s: its content table has no column c%d", table.shape.name, i)
		}
		table.contentColumns = append(table.contentColumns, index)
	}
	table.content = content
	return nil
}

// recordPositions returns where each declared column sits in a stored
// record: the columns in declared order, a virtual generated one left out
// ("Representation Of SQL Tables"), and in a WITHOUT ROWID table the
// primary key columns first, in key order ("Representation of WITHOUT
// ROWID Tables").
func recordPositions(shape *tableShape) []int {
	positions := make([]int, len(shape.columns))
	next := 0
	if shape.withoutRowid {
		for _, key := range shape.primaryKey {
			positions[key] = next
			next++
		}
	}
	for i, column := range shape.columns {
		switch {
		case shape.withoutRowid && containsIndex(shape.primaryKey, i):
		case column.Generated == VirtualGenerated:
			positions[i] = -1
		default:
			positions[i] = next
			next++
		}
	}
	return positions
}

// Name returns the table's name as its CREATE TABLE text spells it.
func (t *Table) Name() string {
	return t.shape.name
}

// Columns returns the table's columns in declared order, the order of
// "select *".
func (t *Table) Columns() []Column {
	return slices.Clone(t.shape.columns)
}

// Has reports whether the table has a column called column, in any ASCII
// case, among the columns "PRAGMA table_info" lists: generated columns are
// left out there, as SQLite leaves them out.
func (t *Table) Has(column string) bool {
	index := t.shape.columnIndex(column)
	return index >= 0 && t.shape.columns[index].Generated == NotGenerated
}

// WithoutRowid reports whether the table is a WITHOUT ROWID table, whose
// rows have no rowid and are read in primary key order.
func (t *Table) WithoutRowid() bool {
	return t.shape.withoutRowid
}

// scan reads every row in storage order (rowid order, or primary key order
// for a WITHOUT ROWID table) and passes visit the rowid and the values in
// declared order. It stops at visit's first error or the first damage.
func (t *Table) scan(visit func(rowid int64, values []Value) error) error {
	if t.content != nil {
		return t.content.scan(func(rowid int64, stored []Value) error {
			values := make([]Value, len(t.contentColumns))
			for i, index := range t.contentColumns {
				values[i] = stored[index]
			}
			return visit(rowid, values)
		})
	}
	rows := t.database.TableRows(t.root)
	if t.shape.withoutRowid {
		rows = t.database.IndexRows(t.root)
	}
	for rows.Next() {
		record, err := rows.Record()
		if err != nil {
			return fmt.Errorf("table %s: %w", t.shape.name, err)
		}
		values, err := t.decode(rows.Rowid(), record)
		if err != nil {
			return err
		}
		if err := visit(rows.Rowid(), values); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("table %s: %w", t.shape.name, err)
	}
	return nil
}

// decode turns a stored record into the row's values in declared order:
// the rowid for the rowid alias, an integer read as a float in a REAL
// column, and a column's default where the record is too short.
func (t *Table) decode(rowid int64, record []Value) ([]Value, error) {
	values := make([]Value, len(t.shape.columns))
	for i := range t.shape.columns {
		column := &t.shape.columns[i]
		position := t.positions[i]
		switch {
		case column.RowidAlias:
			values[i] = IntValue(rowid)
		case position < 0:
		case position < len(record):
			values[i] = record[position]
			if column.Affinity == RealAffinity && values[i].Type == Integer {
				values[i] = FloatValue(float64(values[i].Int))
			}
		case column.defaultErr != nil:
			return nil, fmt.Errorf("table %s row %d: column %s: %w", t.shape.name, rowid,
				column.Name, column.defaultErr)
		default:
			values[i] = column.Default
		}
	}
	return values, nil
}
