package openclaw

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// ToolArgsText returns the text of a tool call's arguments: the call's
// command, path or url argument, the first of them that is a non-empty
// string, else the arguments as CompactJSON. Arguments given as a string
// are decoded as JSON first, and kept as they are when they are not JSON;
// null, an empty string and an empty object give "".
func ToolArgsText(arguments jsonvalue.Value) string {
	if arguments.IsString() {
		decoded, err := jsonvalue.DecodeString(arguments.StringValue())
		if err != nil {
			return arguments.StringValue()
		}
		arguments = decoded
	}
	if arguments.IsObject() {
		for _, key := range []string{"command", "path", "url"} {
			if value := arguments.Get(key); value.IsString() && value.StringValue() != "" {
				return value.StringValue()
			}
		}
	}
	empty := arguments.IsNull() || (arguments.IsString() && arguments.StringValue() == "") ||
		(arguments.IsObject() && arguments.Len() == 0)
	if empty {
		return ""
	}
	return parser.JSONText(arguments)
}
