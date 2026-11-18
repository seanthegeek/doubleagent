package parser

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// TextOr returns a value as row text (Text), and fallback when that is
// empty: for a value jsonvalue.Value.IsEmpty calls empty, so false and zero
// give fallback too, where SettingOrDash shows them.
func TextOr(value jsonvalue.Value, fallback string) string {
	return cmp.Or(Text(value), fallback)
}
