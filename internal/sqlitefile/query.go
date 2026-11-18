package sqlitefile

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// ErrInvalidText reports a selected TEXT value that is not valid UTF-8, the
// encoding the database declares, so its text would not be what was
// stored.
var ErrInvalidText = errors.New("text is not valid UTF-8")

// errStop ends a scan once a query's limit is reached.
var errStop = errors.New("stop")

// Query is a read of one table, built like the SELECT it replaces:
//
//	rows, err := table.Select("id", "data").
//		Where(func(row sqlitefile.Row) bool { return row.Equals("kind", sqlitefile.TextValue("x")) }).
//		OrderBy("created_at", "rowid").All()
//
// Rows come in storage order (rowid order, or primary key order for a
// WITHOUT ROWID table) unless OrderBy sorts them; the sort is stable, so
// ties keep storage order. A Query is a value: each method returns a copy.
type Query struct {
	table    *Table
	columns  []string
	computed []computedColumn
	keep     []func(Row) bool
	order    []string
	limit    int
}

// computedColumn is a column a query adds, computed from each whole row.
type computedColumn struct {
	name    string
	compute func(Row) Value
}

// orderKey is one resolved sort key: a column position, or -1 for the rowid.
type orderKey struct {
	position   int
	descending bool
}

// Select starts a query returning the named columns, in that order, or
// every column in declared order when none is named. "rowid" (or _rowid_
// or oid) names the rowid where no column has that name, as in SQL. A name
// the table does not have is an error when the query runs.
func (t *Table) Select(columns ...string) Query {
	return Query{table: t, columns: slices.Clone(columns), limit: -1}
}

// Compute adds a column called name after the selected ones, whose value
// is compute applied to the whole row: the form of "length(content) AS
// size", which reads a column without selecting it.
func (q Query) Compute(name string, compute func(Row) Value) Query {
	q.computed = append(slices.Clone(q.computed), computedColumn{name: name, compute: compute})
	return q
}

// Where keeps only the rows for which keep returns true. The Row it is
// given holds every column of the table, selected or not; several Where
// calls must all hold.
func (q Query) Where(keep func(Row) bool) Query {
	q.keep = append(slices.Clone(q.keep), keep)
	return q
}

// OrderBy sorts the rows by the keys, each a column name or the rowid,
// followed by " DESC" for a descending key ("ASC" may be written too); a
// key may also be a comma-separated list, as in SQL.
// Values are ordered as Compare orders them, text under the column's
// collation; a descending key reverses that order, NULLs last.
func (q Query) OrderBy(keys ...string) Query {
	q.order = append(slices.Clone(q.order), keys...)
	return q
}

// Limit returns at most n rows: the first n in the query's order.
func (q Query) Limit(n int) Query {
	q.limit = max(n, 0)
	return q
}

// All runs the query and returns its rows. When it fails, the rows read
// before the failure come with the error, as a loop over a cursor would
// have seen them: for an unordered query, the rows before damage or before
// a row with invalid text; for a sorted one, which must read every row
// first, none on damage and the rows before one with invalid text.
func (q Query) All() ([]Row, error) {
	var rows []Row
	err := q.run(func(_, selected Row) error {
		rows = append(rows, selected)
		return nil
	})
	return rows, err
}

// Each runs the query and calls visit with each row as it is read; it
// returns the first error, from the database or from visit.
func (q Query) Each(visit func(Row) error) error {
	return q.run(func(_, selected Row) error { return visit(selected) })
}

// First returns the query's first row and whether there is one.
func (q Query) First() (Row, bool, error) {
	rows, err := q.Limit(1).All()
	if err != nil || len(rows) == 0 {
		return Row{}, false, err
	}
	return rows[0], true, nil
}

// run executes the query, passing visit each row whole (for the
// caller's keys) and as selected.
func (q Query) run(visit func(whole, selected Row) error) error {
	if q.table == nil {
		return errors.New("query of no table")
	}
	columns := make([]*Column, len(q.table.shape.columns))
	for i := range columns {
		columns[i] = &q.table.shape.columns[i]
	}
	whole := newRowShape(columns)
	projection, err := q.projection(columns)
	if err != nil {
		return err
	}
	keys, err := q.orderKeys()
	if err != nil {
		return err
	}
	emit := func(row Row) error {
		selected, err := q.project(row, projection)
		if err != nil {
			return err
		}
		return visit(row, selected)
	}
	if len(keys) == 0 {
		return q.stream(whole, emit)
	}
	return q.sorted(whole, keys, emit)
}

// projection returns the positions of the selected columns and their
// shape, refusing a name the table lacks and a virtual generated column,
// whose value is computed when read and never stored.
func (q Query) projection(columns []*Column) (*projected, error) {
	result := &projected{}
	names := q.columns
	if len(names) == 0 {
		for _, column := range columns {
			names = append(names, column.Name)
		}
	}
	var chosen []*Column
	for _, name := range names {
		position := q.table.shape.columnIndex(name)
		if position < 0 && q.table.isRowidName(name) {
			chosen = append(chosen, &Column{Name: name, Affinity: IntegerAffinity})
			result.positions = append(result.positions, -1)
			continue
		}
		if position < 0 {
			return nil, errorNoColumn(q.table, name)
		}
		if columns[position].Generated == VirtualGenerated {
			return nil, errors.New("table " + q.table.shape.name + ": column " + name +
				" is computed when read and is not stored")
		}
		renamed := *columns[position]
		renamed.Name = name
		if len(q.columns) == 0 {
			renamed.Name = columns[position].Name
		}
		chosen = append(chosen, &renamed)
		result.positions = append(result.positions, position)
	}
	for _, column := range q.computed {
		chosen = append(chosen, &Column{Name: column.name})
	}
	result.shape = newRowShape(chosen)
	return result, nil
}

// projected is a query's selection: the positions of the selected
// columns in a whole row and the shape of the rows it returns.
type projected struct {
	positions []int
	shape     *rowShape
}

// project returns the selected columns of a whole row, refusing text that
// is not valid UTF-8 (ErrInvalidText).
func (q Query) project(row Row, selection *projected) (Row, error) {
	values := make([]Value, 0, len(selection.shape.names))
	for _, position := range selection.positions {
		if position < 0 {
			values = append(values, IntValue(row.Rowid))
		} else {
			values = append(values, row.values[position])
		}
	}
	for _, column := range q.computed {
		values = append(values, column.compute(row))
	}
	for i, value := range values {
		if value.Type == Text && !utf8.Valid(value.Bytes) {
			return Row{}, fmt.Errorf("table %s row %d: column %s: %w", q.table.shape.name,
				row.Rowid, selection.shape.names[i], ErrInvalidText)
		}
	}
	return Row{Rowid: row.Rowid, shape: selection.shape, values: values}, nil
}

// orderKeys resolves the OrderBy keys.
func (q Query) orderKeys() ([]orderKey, error) {
	var keys []orderKey
	var texts []string
	for _, text := range q.order {
		texts = append(texts, strings.Split(text, ",")...)
	}
	for _, text := range texts {
		fields := strings.Fields(text)
		if len(fields) == 0 || len(fields) > 2 {
			return nil, fmt.Errorf("table %s: cannot sort by %q", q.table.shape.name, text)
		}
		key := orderKey{position: q.table.shape.columnIndex(fields[0])}
		if len(fields) == 2 {
			switch upperASCII(fields[1]) {
			case "DESC":
				key.descending = true
			case "ASC":
			default:
				return nil, fmt.Errorf("table %s: cannot sort by %q", q.table.shape.name, text)
			}
		}
		if key.position < 0 && !q.table.isRowidName(fields[0]) {
			return nil, errorNoColumn(q.table, fields[0])
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// kept reports whether every Where function keeps the row.
func (q Query) kept(row Row) bool {
	for _, keep := range q.keep {
		if !keep(row) {
			return false
		}
	}
	return true
}

// stream emits the kept rows as the scan reads them, up to the limit.
func (q Query) stream(whole *rowShape, emit func(Row) error) error {
	if q.limit == 0 {
		return nil
	}
	count := 0
	err := q.table.scan(func(rowid int64, values []Value) error {
		row := Row{Rowid: rowid, shape: whole, values: values}
		if !q.kept(row) {
			return nil
		}
		if err := emit(row); err != nil {
			return err
		}
		if count++; count == q.limit {
			return errStop
		}
		return nil
	})
	if errors.Is(err, errStop) {
		return nil
	}
	return err
}

// sorted reads every kept row, sorts them stably by the keys and emits
// them up to the limit.
func (q Query) sorted(whole *rowShape, keys []orderKey, emit func(Row) error) error {
	var rows []Row
	err := q.table.scan(func(rowid int64, values []Value) error {
		row := Row{Rowid: rowid, shape: whole, values: values}
		if q.kept(row) {
			rows = append(rows, row)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortStableFunc(rows, func(left, right Row) int { return compareKeys(left, right, keys) })
	if q.limit >= 0 && q.limit < len(rows) {
		rows = rows[:q.limit]
	}
	for _, row := range rows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

// compareKeys orders two whole rows by the sort keys.
func compareKeys(left, right Row, keys []orderKey) int {
	for _, key := range keys {
		var order int
		if key.position < 0 {
			order = cmpInt(left.Rowid, right.Rowid)
		} else {
			collation := left.shape.columns[key.position].Collation
			order = compareCollated(left.values[key.position], right.values[key.position], collation)
		}
		if key.descending {
			order = -order
		}
		if order != 0 {
			return order
		}
	}
	return 0
}

// isRowidName reports whether name stands for the rowid in this table: one
// of SQL's three names for it, in a table with rowids, where no column
// takes the name.
func (t *Table) isRowidName(name string) bool {
	if t.shape.withoutRowid || t.shape.columnIndex(name) >= 0 {
		return false
	}
	switch foldASCII(name) {
	case "rowid", "_rowid_", "oid":
		return true
	}
	return false
}
