package parser

import (
	"path"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// JoinNonEmpty returns the parts that are not the empty string joined by
// separator, the form a row takes when it lists optional fields: an absent
// field leaves no doubled separator behind.
func JoinNonEmpty(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, separator)
}

// FirstNonEmpty returns the first of values that is not empty by
// jsonvalue.Value.IsEmpty (null, false, zero, an empty string, array or
// object), else the null value. It reads a field an agent has written
// under more than one name, or left empty in favour of another.
func FirstNonEmpty(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values {
		if !value.IsEmpty() {
			return value
		}
	}
	return jsonvalue.NewNull()
}

// Stem returns a file name without its last extension ("a.b.json" is
// "a.b", "x." is "x"). A name that would be left as nothing but dots has
// no extension and is returned whole, so a dot file such as ".json" or
// "..json" is its own stem.
func Stem(name string) string {
	stem := strings.TrimSuffix(name, path.Ext(name))
	if strings.Trim(stem, ".") == "" {
		return name
	}
	return stem
}
