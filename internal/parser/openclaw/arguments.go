package openclaw

import (
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// ToolArgsText returns the text of a tool call's arguments: the call's
// command, path or url argument, the first of them that is a non-empty
// string, else the arguments as CompactJSON. Arguments given as a string
// are decoded as JSON first, and kept as they are when they are not JSON;
// null, an empty string and an empty object give "".
func ToolArgsText(arguments pyjson.Value) string {
	if arguments.IsString() {
		decoded, err := pyjson.DecodeString(arguments.Text())
		if err != nil {
			return arguments.Text()
		}
		arguments = decoded
	}
	if arguments.IsObject() {
		for _, key := range []string{"command", "path", "url"} {
			if value := arguments.Get(key); value.IsString() && value.Text() != "" {
				return value.Text()
			}
		}
	}
	empty := arguments.IsNull() || (arguments.IsString() && arguments.Text() == "") ||
		(arguments.IsObject() && arguments.Len() == 0)
	if empty {
		return ""
	}
	return parser.CompactJSON(arguments)
}
