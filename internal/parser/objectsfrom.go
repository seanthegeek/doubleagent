package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ObjectsFrom returns the objects among the JSON values text holds from
// offset on, separated by JSON whitespace and commas, up to a closing
// bracket, the end of the text or the first value that does not decode:
// what a damaged array's elements still give. A value that is not an
// object is passed over.
func ObjectsFrom(text string, offset int) []jsonvalue.Value {
	var objects []jsonvalue.Value
	for {
		for offset < len(text) && strings.IndexByte(" \t\r\n,", text[offset]) >= 0 {
			offset++
		}
		if offset >= len(text) || text[offset] == ']' {
			return objects
		}
		value, end, err := jsonvalue.DecodePrefix(text, offset)
		if err != nil {
			return objects
		}
		offset = end
		if value.IsObject() {
			objects = append(objects, value)
		}
	}
}
