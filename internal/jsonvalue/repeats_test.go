package jsonvalue

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestADocumentNotesEachMemberThatRepeatsANameOfItsObject(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		count int
		names []repeatedName
	}{
		{"an object without a repeat notes nothing", `{"a":1,"b":{"a":2}}`, 0, nil},
		{"a name repeated once is one repeat", `{"a":1,"a":2}`, 1, []repeatedName{{"a", 1}}},
		{"a name given three times is two repeats", `{"a":1,"a":2,"a":3}`, 2, []repeatedName{{"a", 2}}},
		{"a repeat inside a nested object is noted", `{"x":{"a":1,"a":2},"a":3}`, 1,
			[]repeatedName{{"a", 1}}},
		{"the same name in two objects is no repeat", `[{"a":1},{"a":2}]`, 0, nil},
		{"the names are kept in the order their first repeat is met", `{"b":1,"a":1,"a":2,"b":2,"a":3}`, 3,
			[]repeatedName{{"a", 2}, {"b", 1}}},
		{"names past the bound are counted but not kept",
			`{"a":1,"a":1,"b":1,"b":1,"c":1,"c":1,"d":1,"d":1,"a":1}`, 5,
			[]repeatedName{{"a", 2}, {"b", 1}, {"c", 1}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := NewDocument(test.text)
			if _, err := document.Decode(); err != nil {
				t.Fatal(err)
			}
			repeats := document.TakeRepeats()
			if repeats.Count() != test.count || !slices.Equal(repeats.names, test.names) {
				t.Errorf("got %d %v, want %d %v", repeats.Count(), repeats.names, test.count, test.names)
			}
		})
	}
}

func TestADocumentDecodesTheValueDecodeStringDoes(t *testing.T) {
	for _, text := range []string{`{"a":1,"b":2,"a":3}`, ` [{"k":[1],"k":{"x":1,"x":2}}] `, `"plain"`} {
		document := NewDocument(text)
		got, err := document.Decode()
		want, wantErr := DecodeString(text)
		if err != nil || wantErr != nil || got.JSON() != want.JSON() {
			t.Errorf("%s: got %s (%v), want %s (%v)", text, got.JSON(), err, want.JSON(), wantErr)
		}
	}
	if _, err := NewDocument(`{} x`).Decode(); err == nil {
		t.Error("text after the value decoded")
	}
}

func TestAValueThatDoesNotDecodeNotesNoRepeat(t *testing.T) {
	for _, text := range []string{`{"a":1,"a":2`, `{"a":1,"a":2} x`, `[{"a":1,"a":2},]`} {
		document := NewDocument(text)
		if _, err := document.Decode(); err == nil {
			t.Fatalf("%s decoded", text)
		}
		if repeats := document.TakeRepeats(); repeats.Count() != 0 {
			t.Errorf("%s: noted %d repeats of a value it did not return", text, repeats.Count())
		}
	}
}

func TestTakeRepeatsGivesWhatTheValuesSinceTheLastTakeRepeated(t *testing.T) {
	text := `{"a":1,"a":2} {"b":1} {"c":1,"c":2,"c":3}`
	document := NewDocument(text)
	wants := []int{1, 0, 2}
	for offset, i := 0, 0; i < len(wants); i++ {
		_, end, err := document.DecodePrefix(offset)
		if err != nil {
			t.Fatal(err)
		}
		offset = end
		if got := document.TakeRepeats().Count(); got != wants[i] {
			t.Errorf("value %d: %d repeats, want %d", i+1, got, wants[i])
		}
	}
	for offset := 0; offset < len(text); {
		_, end, err := document.DecodePrefix(offset)
		if err != nil {
			t.Fatal(err)
		}
		offset = end
	}
	if got := document.TakeRepeats(); got.Count() != 3 || len(got.names) != 2 {
		t.Errorf("three values read without a take noted %d repeats of %v, want 3 of a and c", got.Count(), got.names)
	}
}

// repeatedNames returns an object whose n distinct names each appear
// twice.
func repeatedNames(n int) string {
	var text strings.Builder
	text.WriteString("{")
	for i := range n {
		name := strconv.Quote("name" + strconv.Itoa(i))
		text.WriteString(name + ":1," + name + ":2,")
	}
	text.WriteString(`"end":0}`)
	return text.String()
}

func TestARepeatNotesAFixedNumberOfNamesHoweverManyRepeat(t *testing.T) {
	for _, n := range []int{10, 10_000} {
		document := NewDocument(repeatedNames(n))
		if _, err := document.Decode(); err != nil {
			t.Fatal(err)
		}
		repeats := document.TakeRepeats()
		if repeats.Count() != n || len(repeats.names) != repeatedNamesKept {
			t.Errorf("%d repeated names: counted %d, kept %d names, want %d and %d", n, repeats.Count(),
				len(repeats.names), n, repeatedNamesKept)
		}
	}
}

func TestMergeAddsTheCountsAndKeepsTheFirstNames(t *testing.T) {
	var total Repeats
	for _, text := range []string{`{"a":1,"a":2}`, `{}`, `{"b":1,"b":2,"a":3,"a":4}`, `{"c":1,"c":1,"d":1,"d":1}`} {
		document := NewDocument(text)
		if _, err := document.Decode(); err != nil {
			t.Fatal(err)
		}
		total.Merge(document.TakeRepeats())
	}
	want := []repeatedName{{"a", 2}, {"b", 1}, {"c", 1}}
	if total.Count() != 5 || !slices.Equal(total.names, want) {
		t.Errorf("got %d %v, want 5 %v", total.Count(), total.names, want)
	}
}
