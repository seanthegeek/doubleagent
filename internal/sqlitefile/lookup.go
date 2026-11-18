package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
)

// Lookup holds a query's rows indexed by one column, for the joins
// parsers make: Find returns the rows a join's ON condition would pair
// with a row of the other table. A Lookup belongs to one goroutine.
type Lookup struct {
	column *Column
	keys   []Value
	rows   []Row
	// indexes caches, per comparison mode, the row numbers by key.
	indexes map[lookupMode]map[string][]int
}

// lookupMode is how a join compares the key: whether NUMERIC affinity is
// applied to both sides, and under which collation text is compared.
type lookupMode struct {
	numeric   bool
	collation string
}

// Lookup runs the query and indexes its rows by the column called column,
// which need not be selected.
func (q Query) Lookup(column string) (*Lookup, error) {
	position := q.table.shape.columnIndex(column)
	if position < 0 {
		return nil, errorNoColumn(q.table, column)
	}
	lookup := &Lookup{column: &q.table.shape.columns[position],
		indexes: map[lookupMode]map[string][]int{}}
	err := q.run(func(whole, selected Row) error {
		lookup.keys = append(lookup.keys, whole.value(position))
		lookup.rows = append(lookup.rows, selected)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lookup, nil
}

// errorNoColumn reports a column the table does not have.
func errorNoColumn(table *Table, column string) error {
	return errors.New("table " + table.shape.name + ": no such column: " + column)
}

// Find returns, in the query's order, the rows whose key equals the value
// of the column called column in row, compared as SQL compares two
// columns: NUMERIC affinity on both sides when either column is numeric,
// and text under the collation the probing column declares, else the one
// the key column declares. A NULL matches nothing.
func (l *Lookup) Find(row Row, column string) []Row {
	position := row.lookup(column)
	if position < 0 {
		return nil
	}
	probe, probeColumn := row.value(position), row.shape.columns[position]
	mode := lookupMode{numeric: probeColumn.Affinity.numeric() || l.column.Affinity.numeric(),
		collation: probeColumn.Collation}
	if mode.collation == "" {
		mode.collation = l.column.Collation
	}
	key, ok := lookupKey(probe, mode)
	if !ok {
		return nil
	}
	var found []Row
	for _, index := range l.index(mode)[key] {
		found = append(found, l.rows[index])
	}
	return found
}

// index returns, building it on first use, the row numbers by key in mode.
func (l *Lookup) index(mode lookupMode) map[string][]int {
	if index, ok := l.indexes[mode]; ok {
		return index
	}
	index := map[string][]int{}
	for i, value := range l.keys {
		if key, ok := lookupKey(value, mode); ok {
			index[key] = append(index[key], i)
		}
	}
	l.indexes[mode] = index
	return index
}

// lookupKey returns a key that two values share exactly when SQL finds
// them equal in mode: a number that is a whole 64-bit integer keys as that
// integer whatever its class, text keys by its collated bytes, and NULL
// has no key.
func lookupKey(value Value, mode lookupMode) (string, bool) {
	if mode.numeric {
		value = applyAffinity(value, NumericAffinity)
	}
	switch value.Type {
	case Integer:
		return "i" + strconv.FormatInt(value.Int, 10), true
	case Float:
		if exact := integerIfExact(value); exact.Type == Integer {
			return "i" + strconv.FormatInt(exact.Int, 10), true
		}
		bits := make([]byte, 8)
		binary.BigEndian.PutUint64(bits, math.Float64bits(value.Float))
		return "f" + string(bits), true
	case Text:
		switch mode.collation {
		case "NOCASE":
			return "t" + string(foldBytes(value.Bytes)), true
		case "RTRIM":
			return "t" + string(bytes.TrimRight(value.Bytes, " ")), true
		}
		return "t" + string(value.Bytes), true
	case Blob:
		return "b" + string(value.Bytes), true
	}
	return "", false
}
