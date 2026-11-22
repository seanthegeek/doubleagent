package tomldoc

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// SettingText returns a value as the text of a setting row: `true` or
// `false` for a boolean, a string as it is, a date or time as its RFC 3339
// text, and anything else as CompactJSON.
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

// CompactJSON returns a value as compact JSON through jsonvalue: tables'
// keys in document order, non-ASCII characters kept, a float in its
// shortest form. A date or time inside an array or table is written as a
// JSON string of its text.
func CompactJSON(value Value) string {
	return jsonValue(value).JSON()
}

// jsonValue returns a value as a JSON value: a table as an object in
// document order, a date or time as its text.
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
