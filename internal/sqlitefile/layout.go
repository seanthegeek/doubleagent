package sqlitefile

import (
	"fmt"
	"unicode/utf8"
)

// recordLayout is how a table's stored records become its rows. A row keeps
// the values of its record up to the last one a column reads, and reads a
// column through its source when asked, so a row costs no more than its
// record holds, however many columns the table declares, and no more than
// the declared columns, however many values the record holds.
type recordLayout struct {
	sources []columnSource
	// read marks the record indexes some column's value is taken from; its
	// length is how many of a record's values a row keeps.
	read []bool
	// failsUpTo is the smallest record index of a column whose default
	// cannot be evaluated, and invalidUpTo of one whose default is text that
	// is not valid UTF-8; a record of that many values or fewer needs the
	// default. Each is -1 when no column has such a default.
	failsUpTo   int
	invalidUpTo int
}

// columnSource is where one column's value comes from: the rowid, or the
// value at index in the stored record, an integer read as a float for a
// REAL column; and what a record too short to hold it reads as. index is
// -1 for a virtual generated column, which is never stored and reads as
// NULL.
type columnSource struct {
	rowid       bool
	index       int
	real        bool
	fallback    Value
	fallbackErr error
	// table and column name the column in the error of its default.
	table, column string
}

// read returns the column's value in the row with rowid and record.
func (s *columnSource) read(rowid int64, record []Value) Value {
	switch {
	case s.rowid:
		return IntValue(rowid)
	case s.index < 0 || s.index >= len(record):
		return s.fallback
	}
	value := record[s.index]
	if s.real && value.Type == Integer {
		return FloatValue(float64(value.Int))
	}
	return value
}

// newLayout returns the layout of a table whose columns sit in its records
// as recordPositions places them.
func newLayout(shape *tableShape) *recordLayout {
	positions := recordPositions(shape)
	sources := make([]columnSource, len(shape.columns))
	for i := range shape.columns {
		column := &shape.columns[i]
		sources[i] = columnSource{rowid: column.RowidAlias, index: positions[i],
			real: column.Affinity == RealAffinity, table: shape.name, column: column.Name}
		if positions[i] >= 0 {
			sources[i].fallback, sources[i].fallbackErr = column.Default, column.defaultErr
		}
	}
	return layoutOf(sources)
}

// layoutOf completes a layout from its columns' sources.
func layoutOf(sources []columnSource) *recordLayout {
	layout := &recordLayout{sources: sources, failsUpTo: -1, invalidUpTo: -1}
	lowest := func(current, index int) int {
		if current < 0 {
			return index
		}
		return min(current, index)
	}
	for _, source := range sources {
		if source.rowid || source.index < 0 {
			continue
		}
		if source.index >= len(layout.read) {
			layout.read = append(layout.read, make([]bool, source.index+1-len(layout.read))...)
		}
		layout.read[source.index] = true
		if source.fallbackErr != nil {
			layout.failsUpTo = lowest(layout.failsUpTo, source.index)
		}
		if source.fallback.Type == Text && !utf8.Valid(source.fallback.Bytes) {
			layout.invalidUpTo = lowest(layout.invalidUpTo, source.index)
		}
	}
	return layout
}

// checkDefaults returns the error of the first column, in declared order,
// that a record too short to hold it gives a default that cannot be
// evaluated.
func (l *recordLayout) checkDefaults(rowid int64, record []Value) error {
	if len(record) > l.failsUpTo {
		return nil
	}
	for i := range l.sources {
		source := &l.sources[i]
		if !source.rowid && source.index >= len(record) && source.fallbackErr != nil {
			return fmt.Errorf("table %s row %d: column %s: %w", source.table, rowid, source.column,
				source.fallbackErr)
		}
	}
	return nil
}

// hasInvalidText reports whether a column of the row reads text that is
// not valid UTF-8, looking only at the values the record holds and, when
// the record is short, at the defaults.
func (l *recordLayout) hasInvalidText(record []Value) bool {
	if len(record) <= l.invalidUpTo {
		return true
	}
	for i, value := range record[:min(len(record), len(l.read))] {
		if l.read[i] && value.Type == Text && !utf8.Valid(value.Bytes) {
			return true
		}
	}
	return false
}
