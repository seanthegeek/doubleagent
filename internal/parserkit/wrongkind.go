package parserkit

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// kindName returns the JSON name of a kind of value.
func kindName(kind jsonvalue.Kind) string {
	switch kind {
	case jsonvalue.Bool:
		return "boolean"
	case jsonvalue.Number:
		return "number"
	case jsonvalue.String:
		return "string"
	case jsonvalue.Array:
		return "array"
	case jsonvalue.Object:
		return "object"
	}
	return "null"
}

// WrongKind returns the problem text of a member that is not of the kind
// the agent's format gives it: "<member> is not a JSON <kind>", the one
// wording every parser uses for it.
func WrongKind(member string, want jsonvalue.Kind) string {
	return member + " is not a JSON " + kindName(want)
}

// ArrayMember returns the items of value, a member of a record that the
// format makes an array. An absent or null member gives nothing in
// silence, as does an empty array; a member of any other kind gives
// nothing and adds the problem WrongKind(member, Array) at line, so the
// records it would have held are not lost without a trace.
func ArrayMember(value jsonvalue.Value, member string, line int, problems *Problems) []jsonvalue.Value {
	if !value.IsArray() && !value.IsNull() {
		problems.Add(line, WrongKind(member, jsonvalue.Array))
	}
	return value.Items()
}

// ObjectMember returns the members of value, a member of a record that the
// format makes an object, under the same rule as ArrayMember.
func ObjectMember(value jsonvalue.Value, member string, line int, problems *Problems) []jsonvalue.Member {
	if !value.IsObject() && !value.IsNull() {
		problems.Add(line, WrongKind(member, jsonvalue.Object))
	}
	return value.Members()
}

// ObjectOrEmptyMember returns value, a member of a record that the format
// makes an object, or an empty object when it is not one, adding the
// problem WrongKind(member, Object) at line for a present member of
// another kind; an absent or null member is silent.
func ObjectOrEmptyMember(value jsonvalue.Value, member string, line int, problems *Problems) jsonvalue.Value {
	ObjectMember(value, member, line, problems)
	return ObjectOrEmpty(value)
}
