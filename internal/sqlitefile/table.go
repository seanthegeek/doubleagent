package sqlitefile

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
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
	// layout says where each declared column's value is in a stored row.
	layout *recordLayout
	// content is the fts5 content table, whose b-tree holds the rows.
	content *Table
}

// TableNames returns the names of the schema's tables, as SQL's
// "select name from sqlite_master where type='table'" lists them: virtual
// tables, their shadow tables and SQLite's own tables included. Names keep
// their case; the map holds each one as true. To ask whether a table
// exists, use HasTable, which matches a name as SQL does.
func (d *Database) TableNames() (map[string]bool, error) {
	index, err := d.schemaIndex()
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, row := range index.rows {
		if row.Type == "table" {
			names[row.Name] = true
		}
	}
	return names, nil
}

// HasTable reports whether the schema holds a table called name, matched
// as Table matches it: ASCII letters in any case.
func (d *Database) HasTable(name string) (bool, error) {
	index, err := d.schemaIndex()
	if err != nil {
		return false, err
	}
	_, ok := index.tables[foldASCII(name)]
	return ok, nil
}

// Table returns the table called name, matched as SQL matches a table
// name (ASCII letters in any case). It fails with ErrNoTable when there is
// none, and with an error naming the table when its CREATE TABLE text
// cannot be read, declares a column twice or more than the 2000 columns
// SQLite allows, or is a virtual table of a module other than fts5.
func (d *Database) Table(name string) (*Table, error) {
	index, err := d.schemaIndex()
	if err != nil {
		return nil, err
	}
	return d.tableFrom(index, name, false)
}

// tableFrom finds and parses the table called name in the schema, as an
// fts5 table's content table when content is true. fts5 keeps its rows
// in an ordinary stored table, so a virtual content table is damage, and
// refusing it ends the lookup after one step where a content table names
// itself or a ring of them.
func (d *Database) tableFrom(index *schemaIndex, name string, content bool) (*Table, error) {
	position, ok := index.tables[foldASCII(name)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoTable, name)
	}
	row := index.rows[position]
	shape, err := parseCreateTable(row.SQL)
	if err != nil {
		return nil, fmt.Errorf("table %s: %w", row.Name, err)
	}
	table := &Table{database: d, shape: shape, root: row.RootPage}
	if shape.fts5 && content {
		return nil, malformed("its content table %s is a virtual table", row.Name)
	}
	if shape.fts5 {
		return table, d.attachContent(index, table)
	}
	if row.RootPage == 0 {
		return nil, malformed("table %s has no root page", row.Name)
	}
	table.layout = newLayout(shape)
	return table, nil
}

// attachContent finds an fts5 table's content table and maps each declared
// column to the content table's column c<i> ("The Table Contents (%_content
// table)" in https://www.sqlite.org/fts5.html).
func (d *Database) attachContent(index *schemaIndex, table *Table) error {
	content, err := d.tableFrom(index, table.shape.name+"_content", true)
	if err != nil {
		return fmt.Errorf("fts5 table %s: %w", table.shape.name, err)
	}
	if !content.Has("id") || content.shape.withoutRowid {
		return errors.New("fts5 table " + table.shape.name + ": its content table has no id column")
	}
	sources := make([]columnSource, len(table.shape.columns))
	for i := range table.shape.columns {
		index := content.shape.columnIndex("c" + strconv.Itoa(i))
		if index < 0 {
			return fmt.Errorf("fts5 table %s: its content table has no column c%d", table.shape.name, i)
		}
		sources[i] = content.layout.sources[index]
	}
	table.content, table.layout = content, layoutOf(sources)
	return nil
}

// recordPositions returns where each declared column sits in a stored
// record: the columns in declared order, a virtual generated one left out
// ("Representation Of SQL Tables"), and in a WITHOUT ROWID table the
// primary key columns first, in key order ("Representation of WITHOUT
// ROWID Tables").
func recordPositions(shape *tableShape) []int {
	positions := make([]int, len(shape.columns))
	inKey := make([]bool, len(shape.columns))
	next := 0
	if shape.withoutRowid {
		for _, key := range shape.primaryKey {
			positions[key], inKey[key] = next, true
			next++
		}
	}
	for i, column := range shape.columns {
		switch {
		case inKey[i]:
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
// "select *", virtual generated columns included, which Select leaves out.
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
// for a WITHOUT ROWID table) and passes visit each one whole, its columns
// in declared order. It stops at visit's first error or the first damage.
func (t *Table) scan(visit func(row Row) error) error {
	stored := t
	if t.content != nil {
		stored = t.content
	}
	rows := t.database.TableRows(stored.root)
	if stored.shape.withoutRowid {
		rows = t.database.IndexRows(stored.root)
	}
	shape := t.wholeShape()
	for rows.Next() {
		record, err := rows.recordUpTo(len(t.layout.read))
		if err != nil {
			return fmt.Errorf("table %s: %w", stored.shape.name, err)
		}
		if err := t.layout.checkDefaults(rows.Rowid(), record); err != nil {
			return err
		}
		if err := visit(Row{Rowid: rows.Rowid(), shape: shape, values: record}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("table %s: %w", stored.shape.name, err)
	}
	return nil
}

// wholeShape returns the shape of rows holding every column of the table
// in declared order, read from the stored records through the layout. A
// virtual generated column keeps its place but cannot be looked up by
// name, so such a row does not hold it.
func (t *Table) wholeShape() *rowShape {
	shape := newRowShape(t.columnPointers())
	shape.layout = t.layout
	for i, column := range shape.columns {
		key := foldASCII(column.Name)
		if column.Generated == VirtualGenerated && shape.position[key] == i {
			delete(shape.position, key)
		}
	}
	return shape
}

// columnPointers returns a pointer to each of the table's columns.
func (t *Table) columnPointers() []*Column {
	columns := make([]*Column, len(t.shape.columns))
	for i := range columns {
		columns[i] = &t.shape.columns[i]
	}
	return columns
}
