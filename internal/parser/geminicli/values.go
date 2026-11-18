package geminicli

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// orDash returns a value as row text (parser.Text), "-" when it is empty.
func orDash(value jsonvalue.Value) string {
	return cmp.Or(parser.Text(value), "-")
}
