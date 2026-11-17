package tomldoc

import (
	"github.com/seanthegeek/doubleagent/internal/pyjson"
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
	return pyjson.Sorted(jsonValue(value))
}

// jsonValue returns a value as the JSON value Python's encoder sees: a
// table as an object in document order, a date or time as its text.
func jsonValue(value Value) pyjson.Value {
	switch value.kind {
	case KindInteger:
		return pyjson.NewInt(value.integer)
	case KindFloat:
		return pyjson.NewFloat(value.float)
	case KindBoolean:
		return pyjson.NewBool(value.boolean)
	case KindArray:
		items := make([]pyjson.Value, len(value.items))
		for i, item := range value.items {
			items[i] = jsonValue(item)
		}
		return pyjson.NewArray(items...)
	case KindTable:
		members := make([]pyjson.Member, len(value.table.entries))
		for i, entry := range value.table.entries {
			members[i] = pyjson.Member{Key: entry.Key, Value: jsonValue(entry.Value)}
		}
		return pyjson.NewObject(members...)
	}
	return pyjson.NewString(value.text)
}
