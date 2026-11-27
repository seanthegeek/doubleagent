package parserkit

import (
	"iter"
	"slices"
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
	items := slices.All(ArrayMember(value, member, line, problems))
	if wrong := countWrong(items, want); wrong > 0 {
		problems.Add(line, WrongEntries(member, want, wrong))
	}
	return ofKind(items, want)
}

// MembersOfKind returns the members of value, a member of a record that the
// format makes an object of values of kind want, passing over the members
// of another kind under the same rule as EntriesOfKind; a member that is
// not an object gives nothing and adds the problem ObjectMember adds.
func MembersOfKind(value jsonvalue.Value, member string, want jsonvalue.Kind, line int,
	problems *Problems) iter.Seq2[string, jsonvalue.Value] {
	members := memberPairs(ObjectMember(value, member, line, problems))
	if wrong := countWrong(members, want); wrong > 0 {
		problems.Add(line, WrongEntries(member, want, wrong))
	}
	return ofKind(members, want)
}

// memberPairs gives the key and value of each of members, in order.
func memberPairs(members []jsonvalue.Member) iter.Seq2[string, jsonvalue.Value] {
	return func(yield func(string, jsonvalue.Value) bool) {
		for _, each := range members {
			if !yield(each.Key, each.Value) {
				return
			}
		}
	}
}

// ofKind gives the keys and values of pairs whose value is of kind want,
// in order.
func ofKind[Key any](pairs iter.Seq2[Key, jsonvalue.Value], want jsonvalue.Kind) iter.Seq2[Key, jsonvalue.Value] {
	return func(yield func(Key, jsonvalue.Value) bool) {
		for key, value := range pairs {
			if value.Kind() == want && !yield(key, value) {
				return
			}
		}
	}
}

// countWrong returns how many values of pairs are not of kind want.
func countWrong[Key any](pairs iter.Seq2[Key, jsonvalue.Value], want jsonvalue.Kind) int {
	wrong := 0
	for _, value := range pairs {
		if value.Kind() != want {
			wrong++
		}
	}
	return wrong
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
