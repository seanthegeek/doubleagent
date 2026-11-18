// Package tomldoc reads a TOML 1.0 document into values that keep the
// document's order: a parser walks tables and keys in the order the file
// wrote them.
//
// The reading itself is done by github.com/BurntSushi/toml, which returns
// tables as unordered maps; the order comes from the list of keys that module
// reports in document order. That list cannot always be matched to the
// inline tables inside an array, so their keys keep document order only
// when the array is a key's own value, not an element of another array, and
// nothing in the array holds a table except those inline tables themselves,
// whose values hold none; otherwise every inline table in the array has its
// keys in byte order.
//
// Parse refuses a document over a size limit, or whose keys and arrays nest
// deeper than a depth limit (maxDocumentBytes and maxDepth), before reading
// it, with an *Error that says which.
//
// The module's Decode into a map gives the values but not their order;
// rows list settings in the file's order, so this package rebuilds it from
// the key list Decode also returns.
package tomldoc

// Kind says what a Value holds.
type Kind uint8

// The kinds of value. The four temporal kinds hold their RFC 3339 text,
// as temporal writes it.
const (
	// KindString is a string.
	KindString Kind = iota
	// KindInteger is a 64-bit integer.
	KindInteger
	// KindFloat is a 64-bit float.
	KindFloat
	// KindBoolean is true or false.
	KindBoolean
	// KindDatetime is a date and time with a UTC offset.
	KindDatetime
	// KindLocalDatetime is a date and time without an offset.
	KindLocalDatetime
	// KindLocalDate is a date.
	KindLocalDate
	// KindLocalTime is a time of day.
	KindLocalTime
	// KindArray is an array, or the elements of an array of tables.
	KindArray
	// KindTable is a table, whose entries are in document order.
	KindTable
)

// Value is one TOML value.
type Value struct {
	kind    Kind
	text    string
	integer int64
	float   float64
	boolean bool
	items   []Value
	table   *Table
}

// Kind returns what the value holds.
func (v Value) Kind() Kind { return v.kind }

// Text returns a string's content, or a temporal value's text; it is "" for
// the other kinds.
func (v Value) Text() string { return v.text }

// Int returns an integer's value, or 0 for another kind.
func (v Value) Int() int64 { return v.integer }

// Float returns a float's value, or 0 for another kind.
func (v Value) Float() float64 { return v.float }

// Bool returns a boolean's value, or false for another kind.
func (v Value) Bool() bool { return v.boolean }

// Items returns an array's elements in order, or nil for another kind.
func (v Value) Items() []Value { return v.items }

// Table returns a table's entries, or nil for another kind.
func (v Value) Table() *Table { return v.table }

// Entry is one key of a table and its value.
type Entry struct {
	Key   string
	Value Value
}

// Table is a TOML table: its entries in the order the document first wrote
// them.
type Table struct {
	entries []Entry
	byKey   map[string]int
}

// Entries returns the table's entries in document order.
func (t *Table) Entries() []Entry {
	return append([]Entry(nil), t.entries...)
}

// Len returns the number of entries.
func (t *Table) Len() int { return len(t.entries) }

// Get returns the value of a key and whether the table has it.
func (t *Table) Get(key string) (Value, bool) {
	position, ok := t.byKey[key]
	if !ok {
		return Value{}, false
	}
	return t.entries[position].Value, true
}

// set adds a key, or replaces the value of one already there.
func (t *Table) set(key string, value Value) {
	if position, ok := t.byKey[key]; ok {
		t.entries[position].Value = value
		return
	}
	if t.byKey == nil {
		t.byKey = map[string]int{}
	}
	t.byKey[key] = len(t.entries)
	t.entries = append(t.entries, Entry{key, value})
}
