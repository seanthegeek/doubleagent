package parser

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// IsText reports whether value is a JSON string equal to text; a number,
// boolean or null that would print as text is not.
func IsText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}
