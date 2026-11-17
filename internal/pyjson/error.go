package pyjson

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// SyntaxError is a document that is not JSON as Python's json module reads
// it. Offset counts bytes of the decoded text from its start; Line and
// Column count from 1, the column in code points as Python counts it.
type SyntaxError struct {
	Message string
	Offset  int
	Line    int
	Column  int
}

// Error returns the message with its position, for example "invalid JSON
// at line 1, column 31: expected a value".
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("invalid JSON at line %d, column %d: %s", e.Line, e.Column, e.Message)
}

// newSyntaxError returns the error message for the byte offset of text.
func newSyntaxError(text string, offset int, message string) *SyntaxError {
	before := text[:min(max(offset, 0), len(text))]
	lineStart := strings.LastIndexByte(before, '\n') + 1
	return &SyntaxError{
		Message: message,
		Offset:  offset,
		Line:    strings.Count(before, "\n") + 1,
		Column:  pytext.CodePointLen(before[lineStart:]) + 1,
	}
}
