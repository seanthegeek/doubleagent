package parser

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestAnEntryTallyReportsEachMemberOnceForEveryEntry(t *testing.T) {
	tests := []struct {
		name, entries string
		want          []string
	}{
		{"members of the right kind are silent", `[{"meta":{},"calls":[{}]},{"meta":null}]`, nil},
		{"one wrong member keeps the one-entry wording", `[{"meta":1},{"meta":{}}]`,
			[]string{"meta is not a JSON object"}},
		{"a member wrong in three entries is one problem", `[{"meta":1},{"meta":"x"},{"meta":[]}]`,
			[]string{"meta is not a JSON object in 3 entries"}},
		{"wrong entries of a list are summed over the entries", `[{"calls":[1,{}]},{"calls":["x","y"]}]`,
			[]string{"calls holds 3 entries that are not JSON objects"}},
		{"members are reported in the order first met", `[{"calls":1},{"meta":2},{"calls":3}]`,
			[]string{"calls is not a JSON array in 2 entries", "meta is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			list, err := jsonvalue.DecodeString(test.entries)
			if err != nil {
				t.Fatal(err)
			}
			var tally EntryTally
			for _, entry := range list.Items() {
				tally.ObjectOrEmptyMember(entry.Get("meta"), "meta")
				for range tally.EntriesOfKind(entry.Get("calls"), "calls", jsonvalue.Object) {
				}
			}
			var problems Problems
			tally.Report(4, &problems)
			var got []string
			for _, problem := range problems {
				if problem.Line != 4 {
					t.Errorf("problem %q at line %d, want 4", problem.Message, problem.Line)
				}
				got = append(got, problem.Message)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnEntryTallyStartsAfreshAfterAReport(t *testing.T) {
	var tally EntryTally
	tally.Check(jsonvalue.NewString("x"), "meta", jsonvalue.Object)
	var problems Problems
	tally.Report(1, &problems)
	tally.Report(2, &problems)
	if len(problems) != 1 {
		t.Errorf("got %v, want one problem", problems)
	}
}

func TestAHundredThousandEntriesOfTheWrongKindInAnEntryTallyAreOneProblem(t *testing.T) {
	list, err := jsonvalue.DecodeString("[" + strings.Repeat(`{"meta":1,"names":{"a":1}},`, 99_999) +
		`{"meta":1,"names":{"a":1}}]`)
	if err != nil {
		t.Fatal(err)
	}
	var tally EntryTally
	for _, entry := range list.Items() {
		tally.ObjectMember(entry.Get("meta"), "meta")
		for range tally.MembersOfKind(entry.Get("names"), "names", jsonvalue.String) {
		}
	}
	var problems Problems
	tally.Report(0, &problems)
	want := []Problem{{Message: "meta is not a JSON object in 100000 entries"},
		{Message: "names holds 100000 entries that are not JSON strings"}}
	if !slices.Equal(problems, want) {
		t.Errorf("got %v, want %v", problems, want)
	}
}

func TestAnEntryTallyLooksUpAMemberOnceWhateverNumberOfMembersWereMet(t *testing.T) {
	for _, members := range []int{1000, 4000} {
		var tally EntryTally
		for i := range members {
			tally.Check(jsonvalue.NewString("x"), "member"+strconv.Itoa(i), jsonvalue.Object)
		}
		// A second pass meets every member again.
		for i := range members {
			tally.Check(jsonvalue.NewString("x"), "member"+strconv.Itoa(i), jsonvalue.Object)
		}
		if want := 2 * members; tally.lookups != want {
			t.Errorf("%d members: %d lookups, want %d", members, tally.lookups, want)
		}
		var problems Problems
		tally.Report(1, &problems)
		if len(problems) != members || problems[0].Message != "member0 is not a JSON object in 2 entries" ||
			problems[members-1].Message != "member"+strconv.Itoa(members-1)+" is not a JSON object in 2 entries" {
			t.Errorf("%d members: %d problems, want one per member in the order first met", members, len(problems))
		}
	}
}
