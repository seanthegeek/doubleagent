package tomldoc

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// SettingText returns a value as the text of a setting row, as Python's
// `setting_value` writes it: `true` or `false` for a boolean, a string as it
// is, a date or time as Python writes it, and anything else as compact JSON
// with the keys of every table sorted.
func SettingText(value Value) string {
	switch value.kind {
	case KindBoolean:
		if value.boolean {
			return "true"
		}
		return "false"
	case KindString, KindDatetime, KindLocalDatetime, KindLocalDate, KindLocalTime:
		return value.text
	}
	return CompactJSON(value)
}

// CompactJSON returns a value as JSON with no spaces, the keys of every table
// sorted and non-ASCII characters kept, as Python's
// `json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True)`
// writes it. A date or time inside an array or table is written as a JSON
// string of its text.
func CompactJSON(value Value) string {
	return jsonValue(value).JSON()
}

// jsonValue returns a value as the JSON value Python's encoder sees: a
// table as an object in document order, a date or time as its text.
func jsonValue(value Value) jsonvalue.Value {
	switch value.kind {
	case KindInteger:
		return jsonvalue.NewInt(value.integer)
	case KindFloat:
		return jsonvalue.NewFloat(value.float)
	case KindBoolean:
		return jsonvalue.NewBool(value.boolean)
	case KindArray:
		items := make([]jsonvalue.Value, len(value.items))
		for i, item := range value.items {
			items[i] = jsonValue(item)
		}
		return jsonvalue.NewArray(items...)
	case KindTable:
		members := make([]jsonvalue.Member, len(value.table.entries))
		for i, entry := range value.table.entries {
			members[i] = jsonvalue.Member{Key: entry.Key, Value: jsonValue(entry.Value)}
		}
		return jsonvalue.NewObject(members...)
	}
	return jsonvalue.NewString(value.text)
}
