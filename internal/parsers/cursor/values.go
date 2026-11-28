package cursor

import (
	"iter"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

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
	text := value.UTF8Text()
	decoded, err := parserkit.DecodeJSON(artifact, text)
	if err != nil {
		return jsonvalue.NewString(text)
	}
	return decoded
}

// decodeRecord returns the JSON value of a row, or null after adding the
// problem "unparseable record <rowid> (<key>: <reason>)" with the text.
func decodeRecord(artifact *input.Artifact, rowid int64, key string, value sqlitefile.Value,
	problems *parserkit.Problems) jsonvalue.Value {
	text := value.UTF8Text()
	decoded, err := parserkit.DecodeJSON(artifact, text)
	if err != nil {
		problems.AddRaw(int(rowid), "unparseable record "+strconv.FormatInt(rowid, 10)+" ("+key+": "+err.Error()+")", text)
		return jsonvalue.NewNull()
	}
	return decoded
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
		found[i] = keyed{rowid: row.Rowid, key: row.Get("key").UTF8Text(), value: row.Get("value")}
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
		found[i] = keyed{rowid: row.Rowid, key: row.Get("key").UTF8Text()}
	}
	return found, nil
}

// keyStarts returns a filter for the rows whose key starts with prefix, an
// ASCII text: the key is at least prefix and below prefix with its last
// byte incremented, compared under the key column's collation as keyEquals
// compares, so only a text key can match.
func keyStarts(prefix string) func(sqlitefile.Row) bool {
	start := sqlitefile.TextValue(prefix)
	end := sqlitefile.TextValue(prefix[:len(prefix)-1] + string(prefix[len(prefix)-1]+1))
	return func(row sqlitefile.Row) bool {
		low, ok := row.Cmp("key", start)
		high, okHigh := row.Cmp("key", end)
		return ok && okHigh && low >= 0 && high < 0
	}
}

// prefixed returns the rows of a key-value table whose key starts with
// prefix (keyStarts), in the order given.
func prefixed(table *sqlitefile.Table, prefix string, order ...string) ([]keyed, error) {
	return keyedRows(table, keyStarts(prefix), order...)
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
func objectEntries(artifact *input.Artifact, row keyed, problems *parserkit.Problems) iter.Seq2[int, jsonvalue.Value] {
	return parserkit.EntriesOfKind(decodeRecord(artifact, row.rowid, row.key, row.value, problems), row.key, jsonvalue.Object,
		int(row.rowid), problems)
}

// objectRecord returns the value of a key-value row whose value the format
// makes a JSON object, an empty object when it is not one. A value that
// does not decode is a problem through decodeRecord, and one of another
// kind adds the problem that the row's key is not a JSON object.
func objectRecord(artifact *input.Artifact, row keyed, problems *parserkit.Problems) jsonvalue.Value {
	value := decodeRecord(artifact, row.rowid, row.key, row.value, problems)
	parserkit.ObjectMember(value, row.key, int(row.rowid), problems)
	return parserkit.ObjectOrEmpty(value)
}
