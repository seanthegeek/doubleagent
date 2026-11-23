package parserkit

import (
	"iter"
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// EntryTally gathers the wrong-kind problems of the members that each entry
// of a list holds, for a parser that reads every entry at one line, so a
// member of the wrong kind in many entries is one problem rather than one
// per entry. Its methods read a member as the kit's functions of the same
// name do and count what those would report; Report then adds one problem
// per member, in the order first met.
type EntryTally struct {
	tallies []tally
	// position maps a member's key to its place in tallies, so finding the
	// tally of a member costs the same however many members were met.
	position map[tallyKey]int
}

// tallyKey names one tally: the member, the kind wanted and whether it
// counts entries of a list.
type tallyKey struct {
	member  string
	want    jsonvalue.Kind
	entries bool
}

// tally is the count of one member: of the entries whose member is not of
// kind want, or, when entries is set, of the entries of the wrong kind its
// arrays or objects hold, summed over every entry.
type tally struct {
	member  string
	want    jsonvalue.Kind
	entries bool
	count   int
}

// add counts n more for a member.
func (t *EntryTally) add(member string, want jsonvalue.Kind, entries bool, n int) {
	key := tallyKey{member: member, want: want, entries: entries}
	if i, found := t.position[key]; found {
		t.tallies[i].count += n
		return
	}
	if t.position == nil {
		t.position = make(map[tallyKey]int)
	}
	t.position[key] = len(t.tallies)
	t.tallies = append(t.tallies, tally{member: member, want: want, entries: entries, count: n})
}

// Check reports whether value, a member of an entry, is of kind want or
// absent or null, counting it for Report when it is present and of another
// kind.
func (t *EntryTally) Check(value jsonvalue.Value, member string, want jsonvalue.Kind) bool {
	if value.Kind() == want || value.IsNull() {
		return true
	}
	t.add(member, want, false, 1)
	return false
}

// ArrayMember returns the items of value under the rule of ArrayMember.
func (t *EntryTally) ArrayMember(value jsonvalue.Value, member string) []jsonvalue.Value {
	t.Check(value, member, jsonvalue.Array)
	return value.Items()
}

// ObjectMember returns the members of value under the rule of ObjectMember.
func (t *EntryTally) ObjectMember(value jsonvalue.Value, member string) []jsonvalue.Member {
	t.Check(value, member, jsonvalue.Object)
	return value.Members()
}

// ObjectOrEmptyMember returns value, or an empty object when it is not
// one, under the rule of ObjectOrEmptyMember.
func (t *EntryTally) ObjectOrEmptyMember(value jsonvalue.Value, member string) jsonvalue.Value {
	t.Check(value, member, jsonvalue.Object)
	return ObjectOrEmpty(value)
}

// EntriesOfKind returns the entries of value of kind want, with their
// indexes, under the rule of EntriesOfKind.
func (t *EntryTally) EntriesOfKind(value jsonvalue.Value, member string,
	want jsonvalue.Kind) iter.Seq2[int, jsonvalue.Value] {
	items := t.ArrayMember(value, member)
	t.countWrong(member, want, slices.Values(items))
	return func(yield func(int, jsonvalue.Value) bool) {
		for i, item := range items {
			if item.Kind() == want && !yield(i, item) {
				return
			}
		}
	}
}

// ListOfKind returns the entries EntriesOfKind gives, in order.
func (t *EntryTally) ListOfKind(value jsonvalue.Value, member string, want jsonvalue.Kind) []jsonvalue.Value {
	var entries []jsonvalue.Value
	for _, entry := range t.EntriesOfKind(value, member, want) {
		entries = append(entries, entry)
	}
	return entries
}

// MembersOfKind returns the members of value of kind want under the rule
// of MembersOfKind.
func (t *EntryTally) MembersOfKind(value jsonvalue.Value, member string,
	want jsonvalue.Kind) iter.Seq2[string, jsonvalue.Value] {
	members := t.ObjectMember(value, member)
	t.countWrong(member, want, func(yield func(jsonvalue.Value) bool) {
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

// countWrong counts, for Report, the values that are not of kind want.
func (t *EntryTally) countWrong(member string, want jsonvalue.Kind, values iter.Seq[jsonvalue.Value]) {
	wrong := 0
	for value := range values {
		if value.Kind() != want {
			wrong++
		}
	}
	if wrong > 0 {
		t.add(member, want, true, wrong)
	}
}

// Report adds, at line, one problem per member counted since the last
// Report, in the order first met, and starts the count afresh: WrongKindIn
// for a member of the wrong kind, and WrongEntries for the entries of the
// wrong kind a member held.
func (t *EntryTally) Report(line int, problems *Problems) {
	for _, each := range t.tallies {
		if each.entries {
			problems.Add(line, WrongEntries(each.member, each.want, each.count))
		} else {
			problems.Add(line, WrongKindIn(each.member, each.want, each.count))
		}
	}
	t.tallies = nil
	t.position = nil
}

// WrongKindIn returns the problem text of a member that is not of the kind
// the agent's format gives it in n entries of a list: WrongKind's text for
// one entry, and "<member> is not a JSON <kind> in <n> entries" for more.
func WrongKindIn(member string, want jsonvalue.Kind, n int) string {
	if n == 1 {
		return WrongKind(member, want)
	}
	return WrongKind(member, want) + " in " + strconv.Itoa(n) + " entries"
}
