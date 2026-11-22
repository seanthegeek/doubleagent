package parser

import (
	"iter"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// EntriesOfKind returns the entries of value, a member of a record that the
// format makes an array of entries of kind want, each with its index,
// passing over the entries of another kind. Before the first is given it
// adds, at line, one problem for all those passed over (WrongEntries), so a
// parser that ranges over it cannot lose an entry in silence. A member that
// is not an array gives nothing and adds the problem ArrayMember adds.
func EntriesOfKind(value jsonvalue.Value, member string, want jsonvalue.Kind, line int,
	problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	items := ArrayMember(value, member, line, problems)
	reportWrongEntries(member, want, line, problems, func(yield func(jsonvalue.Value) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	})
	return func(yield func(int, jsonvalue.Value) bool) {
		for i, item := range items {
			if item.Kind() == want && !yield(i, item) {
				return
			}
		}
	}
}

// MembersOfKind returns the members of value, a member of a record that the
// format makes an object of values of kind want, passing over the members
// of another kind under the same rule as EntriesOfKind; a member that is
// not an object gives nothing and adds the problem ObjectMember adds.
func MembersOfKind(value jsonvalue.Value, member string, want jsonvalue.Kind, line int,
	problems *Problems) iter.Seq2[string, jsonvalue.Value] {
	members := ObjectMember(value, member, line, problems)
	reportWrongEntries(member, want, line, problems, func(yield func(jsonvalue.Value) bool) {
		for _, each := range members {
			if !yield(each.Value) {
				return
			}
		}
	})
	return func(yield func(string, jsonvalue.Value) bool) {
		for _, each := range members {
			if each.Value.Kind() == want && !yield(each.Key, each.Value) {
				return
			}
		}
	}
}

// reportWrongEntries adds, at line, WrongEntries for the values of values
// that are not of kind want, when there are any.
func reportWrongEntries(member string, want jsonvalue.Kind, line int, problems *Problems,
	values iter.Seq[jsonvalue.Value]) {
	wrong := 0
	for value := range values {
		if value.Kind() != want {
			wrong++
		}
	}
	if wrong > 0 {
		problems.Add(line, WrongEntries(member, want, wrong))
	}
}

// WrongEntries returns the problem text of a member whose entries include
// some that are not of the kind the agent's format gives them: "<member>
// holds <n> entries that are not JSON <kind>s", or "holds 1 entry that is
// not a JSON <kind>", the one wording every parser uses for it. It is one
// problem however many entries it counts.
func WrongEntries(member string, want jsonvalue.Kind, n int) string {
	if n == 1 {
		return member + " holds 1 entry that is not a JSON " + kindName(want)
	}
	return member + " holds " + strconv.Itoa(n) + " entries that are not JSON " + kindName(want) + "s"
}

// ListOfKind returns the entries EntriesOfKind gives, in order, for a
// parser that keeps them as a list, under the same rule.
func ListOfKind(value jsonvalue.Value, member string, want jsonvalue.Kind, line int,
	problems *Problems) []jsonvalue.Value {
	var entries []jsonvalue.Value
	for _, entry := range EntriesOfKind(value, member, want, line, problems) {
		entries = append(entries, entry)
	}
	return entries
}

// EntriesOfKindFrom gives the entries of entries, an array read as it is
// walked (JSONArray), that are of kind want, passing over the others, and
// adds one problem for those passed over at line once the walk ends, or
// when the loop leaves it, for the entries walked by then (WrongEntries).
func EntriesOfKindFrom(entries iter.Seq2[int, jsonvalue.Value], member string, want jsonvalue.Kind, line int,
	problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		wrong := 0
		defer func() {
			if wrong > 0 {
				problems.Add(line, WrongEntries(member, want, wrong))
			}
		}()
		for i, entry := range entries {
			if entry.Kind() != want {
				wrong++
				continue
			}
			if !yield(i, entry) {
				return
			}
		}
	}
}
