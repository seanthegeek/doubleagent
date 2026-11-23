package parserkit

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestEntriesOfKindGiveTheRightKindAndReportTheRestOnce(t *testing.T) {
	tests := []struct {
		name, array string
		want        jsonvalue.Kind
		indexes     []int
		problems    []string
	}{
		{"objects only give every entry and no problem", `[{}, {"a": 1}]`, jsonvalue.Object, []int{0, 1}, nil},
		{"one string among objects is one problem", `[{}, "x", {}]`, jsonvalue.Object, []int{0, 2},
			[]string{"messages holds 1 entry that is not a JSON object"}},
		{"three entries of other kinds are one problem", `["x", 1, null, {}]`, jsonvalue.Object, []int{3},
			[]string{"messages holds 3 entries that are not JSON objects"}},
		{"strings wanted pass over an object", `["a", {}, "b"]`, jsonvalue.String, []int{0, 2},
			[]string{"messages holds 1 entry that is not a JSON string"}},
		{"an empty array is silent", `[]`, jsonvalue.Object, nil, nil},
		{"a member that is not an array gives nothing and is reported", `{"a": "x"}`, jsonvalue.Object, nil,
			[]string{"messages is not a JSON array"}},
		{"null gives nothing", `null`, jsonvalue.Object, nil, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.array)
			if err != nil {
				t.Fatal(err)
			}
			var problems Problems
			var indexes []int
			for i, entry := range EntriesOfKind(value, "messages", test.want, 7, &problems) {
				if entry.Kind() != test.want {
					t.Errorf("entry %d is of kind %v", i, entry.Kind())
				}
				indexes = append(indexes, i)
			}
			var messages []string
			for _, problem := range problems {
				if problem.Line != 7 {
					t.Errorf("problem at line %d, want 7", problem.Line)
				}
				messages = append(messages, problem.Message)
			}
			if !slices.Equal(indexes, test.indexes) || !slices.Equal(messages, test.problems) {
				t.Errorf("indexes %v and problems %q, want %v and %q", indexes, messages, test.indexes, test.problems)
			}
		})
	}
}

func TestMembersOfKindGiveTheRightKindAndReportTheRestOnce(t *testing.T) {
	value, err := jsonvalue.DecodeString(`{"a": {}, "b": "x", "c": {}, "d": [1]}`)
	if err != nil {
		t.Fatal(err)
	}
	var problems Problems
	var keys []string
	for key := range MembersOfKind(value, "projects", jsonvalue.Object, 0, &problems) {
		keys = append(keys, key)
	}
	want := Problems{{Message: "projects holds 2 entries that are not JSON objects"}}
	if !slices.Equal(keys, []string{"a", "c"}) || !slices.Equal(problems, want) {
		t.Errorf("keys %v and problems %v, want [a c] and %v", keys, problems, want)
	}
}

func TestMembersOfKindReportAMemberThatIsNotAnObject(t *testing.T) {
	var problems Problems
	for range MembersOfKind(jsonvalue.NewString("x"), "projects", jsonvalue.Object, 2, &problems) {
		t.Fatal("a string gave a member")
	}
	want := Problems{{Line: 2, Message: "projects is not a JSON object"}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %v, want %v", problems, want)
	}
}

// An array of a million entries of the wrong kind is one problem, not a
// million.
func TestAMillionWrongEntriesAreOneProblem(t *testing.T) {
	const n = 1_000_000
	value, err := jsonvalue.DecodeString("[" + strings.TrimSuffix(strings.Repeat(`"x",`, n), ",") + "]")
	if err != nil {
		t.Fatal(err)
	}
	var problems Problems
	for range EntriesOfKind(value, "history", jsonvalue.Object, 0, &problems) {
		t.Fatal("an entry of the wrong kind was given")
	}
	want := Problems{{Message: "history holds 1000000 entries that are not JSON objects"}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %v, want %v", problems, want)
	}
}

// Leaving the loop early still leaves the report of every entry of the
// wrong kind, made before the first entry is given.
func TestABreakStillLeavesTheReport(t *testing.T) {
	value, err := jsonvalue.DecodeString(`[{}, {}, 1, 2]`)
	if err != nil {
		t.Fatal(err)
	}
	var problems Problems
	for range EntriesOfKind(value, "history", jsonvalue.Object, 3, &problems) {
		break
	}
	want := Problems{{Line: 3, Message: "history holds 2 entries that are not JSON objects"}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %v, want %v", problems, want)
	}
}

func TestListOfKindKeepsTheEntriesInOrder(t *testing.T) {
	value, err := jsonvalue.DecodeString(`["a", 1, "b"]`)
	if err != nil {
		t.Fatal(err)
	}
	var problems Problems
	got := ListOfKind(value, "ids", jsonvalue.String, 4, &problems)
	want := Problems{{Line: 4, Message: "ids holds 1 entry that is not a JSON string"}}
	if len(got) != 2 || got[0].StringValue() != "a" || got[1].StringValue() != "b" || !slices.Equal(problems, want) {
		t.Errorf("got %v and %v, want [a b] and %v", got, problems, want)
	}
}

func TestEntriesOfKindFromReportTheEntriesWalkedOnce(t *testing.T) {
	value, err := jsonvalue.DecodeString(`[{}, "x", {}, 7, "y"]`)
	if err != nil {
		t.Fatal(err)
	}
	walk := func(yield func(int, jsonvalue.Value) bool) {
		for i, item := range value.Items() {
			if !yield(i, item) {
				return
			}
		}
	}
	tests := []struct {
		name     string
		stop     int
		indexes  []int
		problems Problems
	}{
		{"a whole walk reports every entry passed over", -1, []int{0, 2},
			Problems{{Line: 1, Message: "file.json holds 3 entries that are not JSON objects"}}},
		{"a loop that leaves early reports those walked", 1, []int{0, 2},
			Problems{{Line: 1, Message: "file.json holds 1 entry that is not a JSON object"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			var indexes []int
			for i := range EntriesOfKindFrom(walk, "file.json", jsonvalue.Object, 1, &problems) {
				indexes = append(indexes, i)
				if len(indexes) > test.stop && test.stop >= 0 {
					break
				}
			}
			if !slices.Equal(indexes, test.indexes) || !slices.Equal(problems, test.problems) {
				t.Errorf("indexes %v and problems %v, want %v and %v", indexes, problems, test.indexes, test.problems)
			}
		})
	}
}
