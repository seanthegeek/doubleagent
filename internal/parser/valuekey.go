package parser

import (
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ValueKey returns a map key for a JSON value: two values have the same key
// exactly when jsonvalue.Equal holds for them, so numbers that are equal by
// value (1, 1.0, 1e0, 10e-1) share a key, as do objects with the same
// members in another order. Its cost grows in step with the value's size,
// however large a number's exponent is written.
func ValueKey(value jsonvalue.Value) string {
	var out strings.Builder
	writeValueKey(&out, value)
	return out.String()
}

// writeValueKey writes a value's key. Every piece is self-delimiting: a
// kind letter, then a length before text and a count before items.
func writeValueKey(out *strings.Builder, value jsonvalue.Value) {
	switch value.Kind() {
	case jsonvalue.Null:
		out.WriteByte('n')
	case jsonvalue.Bool:
		flag, _ := value.Bool()
		out.WriteString(strconv.FormatBool(flag)[:1])
	case jsonvalue.Number:
		out.WriteString("#" + jsonvalue.CanonicalNumber(string(value.Number())) + ";")
	case jsonvalue.String:
		writeTextKey(out, value.StringValue())
	case jsonvalue.Array:
		out.WriteString("[" + strconv.Itoa(value.Len()) + ":")
		for _, item := range value.Items() {
			writeValueKey(out, item)
		}
	case jsonvalue.Object:
		members := slices.SortedFunc(slices.Values(value.Members()), func(left, right jsonvalue.Member) int {
			return strings.Compare(left.Key, right.Key)
		})
		out.WriteString("{" + strconv.Itoa(len(members)) + ":")
		for _, member := range members {
			writeTextKey(out, member.Key)
			writeValueKey(out, member.Value)
		}
	}
}

// writeTextKey writes text with its length before it.
func writeTextKey(out *strings.Builder, text string) {
	out.WriteString("s" + strconv.Itoa(len(text)) + ":" + text)
}
