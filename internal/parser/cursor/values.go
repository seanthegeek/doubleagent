package cursor

import (
	"iter"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// asText returns a database value as text: a blob decoded as UTF-8 with
// replacement, "" for NULL, anything else as its row text
// (sqlitefile.Value.String).
func asText(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Blob:
		return textlines.ValidUTF8(value.Bytes)
	case sqlitefile.Null:
		return ""
	}
	return value.String()
}

// firstOrLast returns the first of values that is not empty, else the
// last one: a time field falls back to the next, and the last is read even
// when it is empty, so a zero time still reads as the epoch.
func firstOrLast(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values[:len(values)-1] {
		if !value.IsEmpty() {
			return value
		}
	}
	return values[len(values)-1]
}

// plain returns a short ItemTable value's JSON value when it decodes, else
// its text as a string.
func plain(artifact *input.Artifact, value sqlitefile.Value) jsonvalue.Value {
	text := asText(value)
	decoded, err := parser.DecodeJSON(artifact, text)
	if err != nil {
		return jsonvalue.NewString(text)
	}
	return decoded
}

// decodeRecord returns the JSON value of a row, or null after adding the
// problem "unparseable record <rowid> (<key>: <reason>)" with the text.
func decodeRecord(artifact *input.Artifact, rowid int64, key string, value sqlitefile.Value,
	problems *parser.Problems) jsonvalue.Value {
	text := asText(value)
	decoded, err := parser.DecodeJSON(artifact, text)
	if err != nil {
		problems.AddRaw(int(rowid), "unparseable record "+strconv.FormatInt(rowid, 10)+" ("+key+": "+err.Error()+")", text)
		return jsonvalue.NewNull()
	}
	return decoded
}

// hasTextPrefix reports whether SQL's substr(value, 1, len(prefix)) =
// prefix holds for an ASCII prefix: a text value that starts with it.
func hasTextPrefix(value sqlitefile.Value, prefix string) bool {
	cut := sqlitefile.Substr(value, 1, int64(len(prefix)))
	return cut.Type == sqlitefile.Text && string(cut.Bytes) == prefix
}

// keyed is a key-value row: its rowid, key text and value.
type keyed struct {
	rowid int64
	key   string
	value sqlitefile.Value
}

// keyedRows returns the rows of a key-value table the filter keeps, as
// keyed rows in the order given.
func keyedRows(table *sqlitefile.Table, keep func(sqlitefile.Row) bool, order ...string) ([]keyed, error) {
	query := table.Select("key", "value").Where(keep)
	if len(order) > 0 {
		query = query.OrderBy(order...)
	}
	rows, err := query.All()
	if err != nil {
		return nil, err
	}
	found := make([]keyed, len(rows))
	for i, row := range rows {
		found[i] = keyed{rowid: row.Rowid, key: asText(row.Get("key")), value: row.Get("value")}
	}
	return found, nil
}

// keyNames returns the keys of the rows of a key-value table the filter
// keeps, in rowid order, as keyed rows whose value is NULL: the value
// column is not selected, for values that must never reach a row.
func keyNames(table *sqlitefile.Table, keep func(sqlitefile.Row) bool) ([]keyed, error) {
	rows, err := table.Select("key").Where(keep).All()
	if err != nil {
		return nil, err
	}
	found := make([]keyed, len(rows))
	for i, row := range rows {
		found[i] = keyed{rowid: row.Rowid, key: asText(row.Get("key"))}
	}
	return found, nil
}

// prefixed returns the rows of a key-value table whose key starts with
// prefix, in key order: SQL's key >= prefix and key < the prefix with its
// last character incremented.
func prefixed(table *sqlitefile.Table, prefix string) ([]keyed, error) {
	end := sqlitefile.TextValue(prefix[:len(prefix)-1] + string(prefix[len(prefix)-1]+1))
	start := sqlitefile.TextValue(prefix)
	return keyedRows(table, func(row sqlitefile.Row) bool {
		low, ok := row.Cmp("key", start)
		high, okHigh := row.Cmp("key", end)
		return ok && okHigh && low >= 0 && high < 0
	}, "key")
}

// substrPrefixed returns the rows of a key-value table whose key starts
// with prefix as SQL's substr() compares it, in key order.
func substrPrefixed(table *sqlitefile.Table, prefix string) ([]keyed, error) {
	return keyedRows(table, func(row sqlitefile.Row) bool {
		return hasTextPrefix(row.Get("key"), prefix)
	}, "key")
}

// keyEquals returns a filter for the rows whose key is text.
func keyEquals(text string) func(sqlitefile.Row) bool {
	operand := sqlitefile.TextValue(text)
	return func(row sqlitefile.Row) bool { return row.Equals("key", operand) }
}

// selectPresent returns every row of a table, in rowid order, with the
// wanted columns it has; none when it has none of them.
func selectPresent(table *sqlitefile.Table, wanted []string) ([]sqlitefile.Row, error) {
	var columns []string
	for _, column := range wanted {
		if table.Has(column) {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return nil, nil
	}
	return table.Select(columns...).OrderBy("rowid").All()
}

// objectEntries returns the entries of a key-value row whose value the
// format makes a JSON array of objects. A value that does not decode is a
// problem through decodeRecord, one of another kind adds the problem that
// the row's key is not a JSON array, and entries that are not objects one
// problem, each at the row's rowid.
func objectEntries(artifact *input.Artifact, row keyed, problems *parser.Problems) iter.Seq2[int, jsonvalue.Value] {
	return parser.EntriesOfKind(decodeRecord(artifact, row.rowid, row.key, row.value, problems), row.key, jsonvalue.Object,
		int(row.rowid), problems)
}

// objectRecord returns the value of a key-value row whose value the format
// makes a JSON object, an empty object when it is not one. A value that
// does not decode is a problem through decodeRecord, and one of another
// kind adds the problem that the row's key is not a JSON object.
func objectRecord(artifact *input.Artifact, row keyed, problems *parser.Problems) jsonvalue.Value {
	value := decodeRecord(artifact, row.rowid, row.key, row.value, problems)
	parser.ObjectMember(value, row.key, int(row.rowid), problems)
	return parser.ObjectOrEmpty(value)
}
