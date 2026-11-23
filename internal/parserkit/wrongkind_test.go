package parserkit

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestWrongKindNamesTheMemberAndTheKindWanted(t *testing.T) {
	tests := []struct {
		name, member string
		kind         jsonvalue.Kind
		want         string
	}{
		{"an array", "history", jsonvalue.Array, "history is not a JSON array"},
		{"an object", "twinny.conversations", jsonvalue.Object, "twinny.conversations is not a JSON object"},
		{"a string", "title", jsonvalue.String, "title is not a JSON string"},
		{"a number", "ts", jsonvalue.Number, "ts is not a JSON number"},
		{"a boolean", "done", jsonvalue.Bool, "done is not a JSON boolean"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := WrongKind(test.member, test.kind); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAMemberOfTheWrongKindIsAProblemAndAnAbsentOneIsNot(t *testing.T) {
	tests := []struct {
		name, text   string
		arrayItems   int
		objectItems  int
		arrayReport  bool
		objectReport bool
	}{
		{"an absent member", `{}`, 0, 0, false, false},
		{"a null member", `{"m":null}`, 0, 0, false, false},
		{"an empty array", `{"m":[]}`, 0, 0, false, true},
		{"an empty object", `{"m":{}}`, 0, 0, true, false},
		{"an array of two", `{"m":[1,2]}`, 2, 0, false, true},
		{"an object of one", `{"m":{"a":1}}`, 0, 1, true, false},
		{"a string", `{"m":"oops"}`, 0, 0, true, true},
		{"a number", `{"m":0}`, 0, 0, true, true},
		{"false", `{"m":false}`, 0, 0, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := jsonvalue.DecodeString(test.text)
			if err != nil {
				t.Fatal(err)
			}
			var arrayProblems, objectProblems Problems
			items := ArrayMember(record.Get("m"), "m", 3, &arrayProblems)
			members := ObjectMember(record.Get("m"), "m", 3, &objectProblems)
			var orEmptyProblems Problems
			object := ObjectOrEmptyMember(record.Get("m"), "m", 3, &orEmptyProblems)
			if !object.IsObject() || object.Len() != len(members) || !slices.Equal(orEmptyProblems, objectProblems) {
				t.Errorf("ObjectOrEmptyMember gave %s and %v", object.JSON(), orEmptyProblems)
			}
			wantArray, wantObject := Problems(nil), Problems(nil)
			if test.arrayReport {
				wantArray = Problems{{Line: 3, Message: "m is not a JSON array"}}
			}
			if test.objectReport {
				wantObject = Problems{{Line: 3, Message: "m is not a JSON object"}}
			}
			if len(items) != test.arrayItems || len(members) != test.objectItems ||
				!slices.Equal(arrayProblems, wantArray) || !slices.Equal(objectProblems, wantObject) {
				t.Errorf("%d items %v and %d members %v", len(items), arrayProblems, len(members), objectProblems)
			}
		})
	}
}
