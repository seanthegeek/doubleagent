package tomldoc

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Parse reads a TOML 1.1 document. The error, an *Error, says where the
// document breaks the TOML rules, with a line and column, or that it is
// larger or nests deeper than this package reads.
func Parse(text string) (*Table, error) {
	if err := checkShape(text); err != nil {
		return nil, err
	}
	// Every CR LF becomes LF before reading, so a multi-line string holds LF
	// however the file ended its lines; TOML lets a reader normalise
	// newlines so.
	mapping, keys, err := decode(strings.ReplaceAll(text, "\r\n", "\n"))
	if err != nil {
		return nil, err
	}
	return newBuilder(mapping, keys).build(), nil
}

// decode runs the reader. The reader turns its own parse failures into
// errors but lets a panic from a defect of its own through; that panic
// becomes an error here, so one malformed file cannot stop a run.
func decode(text string) (mapping map[string]any, keys []toml.Key, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			mapping, keys, err = nil, nil, &Error{Kind: InvalidSyntax,
				text: fmt.Sprintf("invalid TOML: the reader failed on this input: %v", recovered)}
		}
	}()
	meta, err := toml.Decode(text, &mapping)
	if err != nil {
		return nil, nil, newSyntaxError(err)
	}
	return mapping, meta.Keys(), nil
}

// newSyntaxError words a reader failure as this package's own error, with
// the line and column when the reader gave them, keeping the reader's error
// as its cause.
func newSyntaxError(err error) error {
	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		return &Error{Kind: InvalidSyntax, Line: parseErr.Position.Line,
			Column: parseErr.Position.Col, cause: err,
			text: fmt.Sprintf("invalid TOML at line %d, column %d: %s", parseErr.Position.Line,
				parseErr.Position.Col, parseErr.Message)}
	}
	return &Error{Kind: InvalidSyntax, text: "invalid TOML: " + err.Error(), cause: err}
}

// builder rebuilds the document's order. The reader gives every key path in
// document order and the finished values as maps; walking the paths in order
// and creating each table at its first appearance gives the document's
// order.
type builder struct {
	mapping map[string]any
	keys    []toml.Key
	root    *Table
}

// newBuilder starts a builder over the reader's values and key paths.
func newBuilder(mapping map[string]any, keys []toml.Key) *builder {
	return &builder{mapping: mapping, keys: keys, root: &Table{}}
}

// build walks the key paths in order and then adds anything they missed.
func (b *builder) build() *Table {
	for i := 0; i < len(b.keys); i++ {
		i += b.addKey(i)
	}
	fillMissing(b.root, b.mapping)
	return b.root
}

// descend returns the table and the map that hold the last part of a path,
// creating the tables on the way, or false when the path runs into an array
// of plain values.
func (b *builder) descend(path toml.Key) (*Table, map[string]any, bool) {
	table, node := b.root, b.mapping
	for _, part := range path[:len(path)-1] {
		if _, ok := table.Get(part); !ok {
			if _, isTable := node[part].(map[string]any); !isTable {
				return nil, nil, false
			}
			table.set(part, Value{kind: KindTable, table: &Table{}})
		}
		child, _ := table.Get(part)
		switch child.kind {
		case KindTable:
			table, node = child.table, node[part].(map[string]any)
		case KindArray:
			elements, isTables := node[part].([]map[string]any)
			if !isTables || len(child.items) == 0 || len(child.items) > len(elements) {
				return nil, nil, false
			}
			table, node = child.items[len(child.items)-1].table, elements[len(child.items)-1]
		default:
			return nil, nil, false
		}
	}
	return table, node, true
}

// addKey adds the key at keys[position] and returns how many following keys
// it consumed (those that belong to an inline table inside an array).
func (b *builder) addKey(position int) int {
	path := b.keys[position]
	table, node, ok := b.descend(path)
	if !ok {
		return 0
	}
	last := path[len(path)-1]
	switch value := node[last].(type) {
	case []map[string]any:
		b.addTableArrayElement(table, last)
	case map[string]any:
		if _, exists := table.Get(last); !exists {
			table.set(last, Value{kind: KindTable, table: &Table{}})
		}
	default:
		run := b.followers(position)
		table.set(last, convert(value, relativeRun(run, len(path))))
		return len(run)
	}
	return 0
}

// addTableArrayElement appends a new empty table to the `[[name]]` array.
func (b *builder) addTableArrayElement(table *Table, name string) {
	element := Value{kind: KindTable, table: &Table{}}
	current, exists := table.Get(name)
	if !exists {
		current = Value{kind: KindArray}
	}
	current.items = append(current.items, element)
	table.set(name, current)
}

// followers returns the keys after position that lie inside the key at
// position, which are the keys of inline tables in its array value.
func (b *builder) followers(position int) []toml.Key {
	path := b.keys[position]
	end := position + 1
	for end < len(b.keys) && len(b.keys[end]) > len(path) && hasPrefix(b.keys[end], path) {
		end++
	}
	return b.keys[position+1 : end]
}

// hasPrefix says whether path starts with every part of prefix; path is
// at least as long.
func hasPrefix(path, prefix toml.Key) bool {
	for i, part := range prefix {
		if path[i] != part {
			return false
		}
	}
	return true
}

// relativeRun cuts the first depth parts off every path.
func relativeRun(run []toml.Key, depth int) [][]string {
	relative := make([][]string, len(run))
	for i, path := range run {
		relative[i] = path[depth:]
	}
	return relative
}

// convert turns a value the reader returned into a Value. The run is the key
// paths of the inline tables inside an array value, in document order.
func convert(raw any, run [][]string) Value {
	switch value := raw.(type) {
	case string:
		return Value{kind: KindString, text: value}
	case int64:
		return Value{kind: KindInteger, integer: value}
	case float64:
		return Value{kind: KindFloat, float: value}
	case bool:
		return Value{kind: KindBoolean, boolean: value}
	case time.Time:
		return temporal(value)
	case map[string]any:
		return Value{kind: KindTable, table: sortedTable(value)}
	case []map[string]any:
		items := make([]any, len(value))
		for i, element := range value {
			items[i] = element
		}
		return convertArray(items, nil)
	case []any:
		return convertArray(value, run)
	}
	return Value{kind: KindString, text: fmt.Sprint(raw)}
}

// convertArray converts the elements of an array, giving each inline table
// its keys in document order when the run can be matched to the tables.
func convertArray(items []any, run [][]string) Value {
	ordered := orderedKeys(items, run)
	array := Value{kind: KindArray, items: make([]Value, len(items))}
	for i, item := range items {
		if table, isTable := item.(map[string]any); isTable && ordered != nil {
			array.items[i] = Value{kind: KindTable, table: tableInOrder(table, ordered[i])}
			continue
		}
		array.items[i] = convert(item, nil)
	}
	return array
}

// orderedKeys splits the run among the inline tables of an array, one key
// list per element, or returns nil when that cannot be done exactly: when a
// table has a table inside it, or the run does not match the tables' keys.
func orderedKeys(items []any, run [][]string) [][]string {
	if len(run) == 0 {
		return nil
	}
	ordered := make([][]string, len(items))
	position := 0
	for i, item := range items {
		table, isTable := item.(map[string]any)
		if !isTable {
			if containsTable(item) {
				return nil
			}
			continue
		}
		if position+len(table) > len(run) {
			return nil
		}
		for _, path := range run[position : position+len(table)] {
			if len(path) != 1 {
				return nil
			}
			ordered[i] = append(ordered[i], path[0])
		}
		position += len(table)
		if !sameKeys(ordered[i], table) {
			return nil
		}
	}
	if position != len(run) {
		return nil
	}
	return ordered
}

// containsTable says whether an array element is, or holds, a table.
func containsTable(raw any) bool {
	switch value := raw.(type) {
	case map[string]any, []map[string]any:
		return true
	case []any:
		for _, item := range value {
			if containsTable(item) {
				return true
			}
		}
	}
	return false
}

// sameKeys says whether keys are exactly the keys of table, none twice.
func sameKeys(keys []string, table map[string]any) bool {
	seen := map[string]bool{}
	for _, key := range keys {
		if _, ok := table[key]; !ok || seen[key] {
			return false
		}
		seen[key] = true
	}
	return len(seen) == len(table)
}

// tableInOrder builds a table whose values hold no table, with keys in the
// given order.
func tableInOrder(raw map[string]any, keys []string) *Table {
	table := &Table{}
	for _, key := range keys {
		table.set(key, convert(raw[key], nil))
	}
	return table
}

// sortedTable builds a table with its keys in sorted order, for a table whose
// document order is not known.
func sortedTable(raw map[string]any) *Table {
	table := &Table{}
	for _, key := range sortedKeys(raw) {
		table.set(key, convert(raw[key], nil))
	}
	return table
}

// sortedKeys returns a map's keys in byte order.
func sortedKeys(raw map[string]any) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// fillMissing adds, in sorted order, any key of the maps that the walk over
// the key paths did not reach. It is a safety net: every key of a document
// has a path.
func fillMissing(table *Table, node map[string]any) {
	for _, key := range sortedKeys(node) {
		existing, ok := table.Get(key)
		if !ok {
			table.set(key, convert(node[key], nil))
			continue
		}
		switch child := node[key].(type) {
		case map[string]any:
			if existing.kind == KindTable {
				fillMissing(existing.table, child)
			}
		case []map[string]any:
			for i, element := range child {
				if i < len(existing.items) && existing.items[i].kind == KindTable {
					fillMissing(existing.items[i].table, element)
				}
			}
		}
	}
}

// temporal converts a date or time the reader returned to its RFC 3339
// text, in the forms the reader's own encoder writes: a fraction only when
// it is not zero, without trailing zeros, and Z for a zero offset. The
// reader marks a value without an offset by a zone named for its kind.
func temporal(moment time.Time) Value {
	switch moment.Location().String() {
	case "date-local":
		return Value{kind: KindLocalDate, text: moment.Format(time.DateOnly)}
	case "time-local":
		return Value{kind: KindLocalTime, text: moment.Format("15:04:05.999999999")}
	case "datetime-local":
		return Value{kind: KindLocalDatetime, text: moment.Format("2006-01-02T15:04:05.999999999")}
	}
	return Value{kind: KindDatetime, text: moment.Format(time.RFC3339Nano)}
}
