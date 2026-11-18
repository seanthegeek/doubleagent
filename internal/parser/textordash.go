package parser

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// TextOrDash returns a value as row text (Text), "-" when that is empty: a
// value that jsonvalue.Value.IsEmpty calls empty, so false and zero are "-"
// too, where SettingOrDash shows them.
func TextOrDash(value jsonvalue.Value) string {
	return cmp.Or(Text(value), "-")
}
