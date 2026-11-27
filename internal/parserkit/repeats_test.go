package parserkit

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeatsOf returns what decoding text noted of its repeated member names.
func repeatsOf(t *testing.T, text string) jsonvalue.Repeats {
	t.Helper()
	document := jsonvalue.NewDocument(text)
	if _, err := document.Decode(); err != nil {
		t.Fatal(err)
	}
	return document.TakeRepeats()
}

func TestRepeatedMembersNamesTheFirstNamesAndCountsAll(t *testing.T) {
	long := strings.Repeat("k", CarriedLimit+1)
	tests := []struct{ name, text, want string }{
		{"one repeat", `{"id":1,"id":2}`,
			`1 member name repeats within its JSON object ("id"); the last value is used`},
		{"one name repeated twice", `{"id":1,"id":2,"id":3}`,
			`2 member names repeat within their JSON objects ("id" 2 times); the last value of each is used`},
		{"two names", `{"a":1,"b":1,"a":2,"b":2,"b":3}`,
			`3 member names repeat within their JSON objects ("a" 1 time, "b" 2 times); the last value of each is used`},
		{"more names than are kept", `{"a":1,"a":1,"b":1,"b":1,"c":1,"c":1,"d":1,"d":1}`,
			`4 member names repeat within their JSON objects ("a" 1 time, "b" 1 time, "c" 1 time, ...); ` +
				`the last value of each is used`},
		{"a name is written as a JSON string", `{"a\"\u0001":1,"a\"\u0001":2}`,
			`1 member name repeats within its JSON object ("a\"\u0001"); the last value is used`},
		{"a name longer than the bound is given by its length", `{"` + long + `":1,"` + long + `":2}`,
			`1 member name repeats within its JSON object (a name of 4097 bytes); the last value is used`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := repeatedMembers(repeatsOf(t, test.text)); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestTheValueARepeatReplacedIsNeverWritten(t *testing.T) {
	got := repeatedMembers(repeatsOf(t, `{"token":"hidden-value","token":"shown"}`))
	if strings.Contains(got, "hidden-value") || strings.Contains(got, "shown") {
		t.Errorf("the problem %q quotes a value", got)
	}
}

// repeatParse runs one parse of a new artifact as a run does (Within):
// it decodes each text with DecodeJSON and emits one row per text with the
// session id that sessions gives it, and returns every row emitted.
func repeatParse(t *testing.T, texts, sessions []string) []timeline.Row {
	t.Helper()
	artifact := &input.Artifact{Original: "/home/u/a.json", Agent: "agent",
		Home: &input.Home{Host: "h", User: "u"}}
	var rows []timeline.Row
	_, err := Within(artifact, func(emit Emit) error {
		for i, text := range texts {
			if _, err := DecodeJSON(artifact, text); err != nil {
				return err
			}
			emit(timeline.Row{Agent: "parser-agent", SessionID: sessions[i], ProjectPath: "/p", Text: text})
		}
		return nil
	}, func(row timeline.Row) { rows = append(rows, row) })
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Home.Repeats != nil {
		t.Error("the parse left its record of repeats on the home")
	}
	return rows
}

func TestAParseEndsWithOneRowForTheRepeatsItsValuesNoted(t *testing.T) {
	texts := []string{`{"a":1,"a":2}`, `{"b":1}`, `{"a":1,"a":2,"c":1,"c":2}`}
	rows := repeatParse(t, texts, []string{"s", "s", "s"})
	want := timeline.Row{Host: "h", User: "u", Agent: "parser-agent", SessionID: "s", ProjectPath: "/p",
		TurnType: timeline.TurnSystem, SourceFile: "/home/u/a.json", Text: "parser: 3 member names repeat " +
			`within their JSON objects ("a" 2 times, "c" 1 time); the last value of each is used`}
	if len(rows) != 4 {
		t.Fatalf("%d rows, want the three rows and one problem row", len(rows))
	}
	last := rows[3]
	last.Secrets = nil
	if last != want {
		t.Errorf("problem row %+v, want %+v", last, want)
	}
}

func TestTheRepeatsRowGivesOnlyTheSessionEveryRowShares(t *testing.T) {
	rows := repeatParse(t, []string{`{"a":1,"a":2}`, `{}`}, []string{"s1", "s2"})
	if len(rows) != 3 || rows[2].SessionID != "" || rows[2].ProjectPath != "/p" {
		t.Errorf("rows %+v, want a problem row with no session id and the shared project", rows)
	}
}

func TestAParseWithoutARepeatGivesNoRepeatsRow(t *testing.T) {
	rows := repeatParse(t, []string{`{"a":{"a":1}}`, `[{"a":1},{"a":2}]`}, []string{"s", "s"})
	if len(rows) != 2 {
		t.Errorf("%d rows, want 2", len(rows))
	}
}

// repeatingLines returns n JSON lines that each repeat the member "id",
// with a line that does not decode in the middle.
func repeatingLines(n int) []byte {
	var text strings.Builder
	for i := range n {
		text.WriteString(`{"id":` + strconv.Itoa(i) + `,"id":"last"}` + "\n")
		if i == n/2 {
			text.WriteString("{\"cut\":\n")
		}
	}
	return []byte(text.String())
}

func TestJSONLinesGiveOneRepeatProblemPerFileAtTheFirstLineThatRepeats(t *testing.T) {
	for _, n := range []int{10, 1000} {
		data := append([]byte("{\"a\":1}\n"), repeatingLines(n)...)
		var problems Problems
		records := 0
		for _, record := range JSONLines(writeFile(t, "lines.jsonl", data), &problems) {
			records++
			if record.Get("id").Text() != "last" && record.Has("id") {
				t.Fatalf("a repeat changed the value kept: %s", record.JSON())
			}
		}
		want := Problems{
			{Line: n/2 + 3, Message: "unparseable line (invalid JSON at line 1, column 8: unexpected end of JSON input)",
				Raw: `{"cut":`},
			{Line: 2, Message: strconv.Itoa(n) +
				` member names repeat within their JSON objects ("id" ` + strconv.Itoa(n) +
				` times); the last value of each is used`, note: true},
		}
		if records != n+1 || !slices.Equal(problems, want) {
			t.Errorf("%d lines: %d records and %+v, want %d and %+v", n, records, problems, n+1, want)
		}
	}
}

func TestJSONLinesReportRepeatsWhenTheLoopStopsEarly(t *testing.T) {
	var problems Problems
	for range JSONRecords(SplitLines(repeatingLines(4)), &problems) {
		break
	}
	if len(problems) != 1 || problems[0].Line != 1 {
		t.Errorf("problems %+v, want the one repeat of line 1", problems)
	}
}

func TestJSONArrayGivesOneRepeatProblemAtThePositionOfTheFirstElementThatRepeats(t *testing.T) {
	text := `[{"a":1}, {"b":1,"b":2}, {"c":1}, {"b":1,"b":2,"d":1,"d":2}, {"x":`
	var problems Problems
	elements := ArrayElements(text, &problems)
	want := Problems{
		{Line: 5, Message: "unparseable record 5 (invalid JSON at line 1, column 67: unexpected end of JSON input)",
			Raw: `{"x":`},
		{Line: 2, Message: `3 member names repeat within their JSON objects ("b" 2 times, "d" 1 time); ` +
			`the last value of each is used`, note: true},
	}
	if len(elements) != 4 || !slices.Equal(problems, want) {
		t.Errorf("%d elements and %+v, want 4 and %+v", len(elements), problems, want)
	}
}

func TestANoteIsNotDamage(t *testing.T) {
	var problems Problems
	problems.addRepeats(1, repeatsOf(t, `{"a":1,"a":2}`))
	if problems.Damaged() {
		t.Error("a note of repeated names counts as damage")
	}
	problems.Add(2, "unparseable line")
	if !problems.Damaged() {
		t.Error("a bad line beside a note is not damage")
	}
}
