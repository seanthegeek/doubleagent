package parserkit

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ObjectsFrom returns the objects among the JSON values text holds from
// offset on, separated by JSON whitespace and commas, up to a closing
// bracket, the end of the text or the first value that does not decode:
// what a damaged array's elements still give. A value that is not an
// object is passed over. The values' repeated member names are noted in the
// parse of artifact (NoteRepeats).
func ObjectsFrom(artifact *input.Artifact, text string, offset int) []jsonvalue.Value {
	var objects []jsonvalue.Value
	document := jsonvalue.NewDocument(text)
	defer func() { NoteRepeats(artifact, document.TakeRepeats()) }()
	for {
		for offset < len(text) && strings.IndexByte(" \t\r\n,", text[offset]) >= 0 {
			offset++
		}
		if offset >= len(text) || text[offset] == ']' {
			return objects
		}
		value, end, err := document.DecodePrefix(offset)
		if err != nil {
			return objects
		}
		offset = end
		if value.IsObject() {
			objects = append(objects, value)
		}
	}
}
