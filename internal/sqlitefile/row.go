package sqlitefile

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// Row is one row a query returns: its rowid (0 in a WITHOUT ROWID table)
// and its values by column name. A Row passed to a Where function holds
// every column; a Row a query returns holds the selected columns.
type Row struct {
	Rowid int64
	shape *rowShape
	// values holds the row's values in column order, or, when the shape
	// has a layout, the stored record's values the layout reads them from.
	values []Value
}

// rowShape is what the rows of one query share: the column names in
// order, each one's column, a lookup of positions by folded name and, for
// rows that keep their stored record, the table's record layout.
type rowShape struct {
	names    []string
	columns  []*Column
	position map[string]int
	layout   *recordLayout
}

// newRowShape returns the shape of rows holding columns in that order.
func newRowShape(columns []*Column) *rowShape {
	shape := &rowShape{columns: columns, position: map[string]int{}}
	for i, column := range columns {
		shape.names = append(shape.names, column.Name)
		key := textlines.LowerASCII(column.Name)
		if _, taken := shape.position[key]; !taken {
			shape.position[key] = i
		}
	}
	return shape
}

// upperASCII raises the ASCII letters of name and keeps every other byte.
func upperASCII(name string) string {
	raised := []byte(name)
	for i, octet := range raised {
		if octet >= 'a' && octet <= 'z' {
			raised[i] = octet - 'a' + 'A'
		}
	}
	return string(raised)
}

// value returns the value of the column at position.
func (r Row) value(position int) Value {
	if r.shape.layout == nil {
		return r.values[position]
	}
	return r.shape.layout.sources[position].read(r.Rowid, r.values)
}

// lookup returns the position of the column called name, or -1.
func (r Row) lookup(name string) int {
	if r.shape == nil {
		return -1
	}
	if position, ok := r.shape.position[textlines.LowerASCII(name)]; ok {
		return position
	}
	return -1
}

// Get returns the value of the column called name, in any ASCII case, and
// NULL when the row holds no such column.
func (r Row) Get(name string) Value {
	if position := r.lookup(name); position >= 0 {
		return r.value(position)
	}
	return Value{}
}

// Has reports whether the row holds a column called name.
func (r Row) Has(name string) bool {
	return r.lookup(name) >= 0
}

// Columns returns the names of the row's columns in the order the query
// selected them.
func (r Row) Columns() []string {
	if r.shape == nil {
		return nil
	}
	return slices.Clone(r.shape.names)
}

// Values returns the row's values in column order.
func (r Row) Values() []Value {
	if r.shape == nil || r.shape.layout == nil {
		return slices.Clone(r.values)
	}
	values := make([]Value, len(r.shape.columns))
	for i := range values {
		values[i] = r.value(i)
	}
	return values
}

// Equals reports whether SQL's "column = operand" holds for the row: the
// column's affinity is applied to the operand first, NUMERIC for a numeric
// column and TEXT for a text one, and text is compared under the column's
// collation; NULL on either side is never equal.
func (r Row) Equals(name string, operand Value) bool {
	order, ok := r.Cmp(name, operand)
	return ok && order == 0
}

// Cmp compares the column called name with operand as SQL's <, = and >
// do, after the conversions Equals describes, and returns -1, 0 or 1. The
// second result is false, and the comparison false in SQL, when either
// value is NULL or the row holds no such column.
func (r Row) Cmp(name string, operand Value) (int, bool) {
	position := r.lookup(name)
	if position < 0 {
		return 0, false
	}
	value, column := r.value(position), r.shape.columns[position]
	switch {
	case column.Affinity.numeric():
		operand = applyAffinity(operand, NumericAffinity)
	case column.Affinity == TextAffinity:
		operand = applyAffinity(operand, TextAffinity)
	}
	if value.Type == Null || operand.Type == Null {
		return 0, false
	}
	return compareCollated(value, operand, column.Collation), true
}
