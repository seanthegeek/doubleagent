//go:build parity

package replay

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

func TestTheTableNumbersEachAnalysisDifferenceOnceInOrderFrom101(t *testing.T) {
	previous := 100
	for i, entry := range Differences() {
		if entry.Number <= previous {
			t.Errorf("entry %d is numbered %d, after %d", i, entry.Number, previous)
		}
		previous = entry.Number
		if (entry.Text == nil) != (entry.Binary == nil) {
			t.Errorf("difference %d rewrites one side's text only", entry.Number)
		}
		kinds := 0
		for _, set := range []bool{entry.Text != nil, entry.Excludes != nil,
			entry.Outputs != nil, entry.Arguments != nil, entry.Scope != "", entry.Added != nil} {
			if set {
				kinds++
			}
		}
		switch {
		case entry.Retired != "" && kinds != 0:
			t.Errorf("retired difference %d still has a normaliser", entry.Number)
		case entry.Retired == "" && kinds != 1:
			t.Errorf("difference %d needs exactly one kind of normaliser", entry.Number)
		}
	}
}

func TestExcludingDifferencesSeeOnlyTheirShape(t *testing.T) {
	tests := []struct {
		name   string
		number int
		text   string
		want   bool
	}{
		{"103 sees a lookahead", 103, "timeline\n/in\n--match\nfoo(?=bar)", true},
		{"103 sees a backreference in the joined form", 103, "--match=(a)\\1", true},
		{"103 sees a pattern Go cannot compile", 103, "--match\n[z-a]", true},
		{"103 passes a pattern both read alike", 103, "--match\nfoo|bar", false},
		{"150 sees an unreadable -shm file", 150,
			"parser: state.vscdb-shm could not be read ({system error})", true},
		{"150 passes an unreadable -wal file", 150,
			"parser: state.vscdb-wal could not be read ({system error})", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := difference(t, test.number).Excludes(test.text); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestANamedDifferenceThatFindsNothingFailsTheCase(t *testing.T) {
	tests := []struct {
		name  string
		check func(reporter *recordingTB)
		fails bool
	}{
		{"a text difference with its shape passes", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{120}, "error: 'x' is bad\n", "error: \"x\" is bad\n")
		}, false},
		{"a text difference without its shape fails", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{120}, "x\n", "x\n")
		}, true},
		{"a row difference without its shape fails", func(reporter *recordingTB) {
			NormaliseRows(reporter, "n", []int{102}, []Row{{Text: "hello"}}, nil)
		}, true},
		{"an excluding difference that shows passes", func(reporter *recordingTB) {
			Excluded(reporter, "n", []int{103}, "--match\nfoo(?=bar)")
		}, false},
		{"an excluding difference that does not show fails", func(reporter *recordingTB) {
			Excluded(reporter, "n", []int{103}, "--match\nfoo|bar")
		}, true},
		{"a difference of another kind is left to its own call", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{103}, "x", "x")
		}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			test.check(reporter)
			failed := len(reporter.errors) > 0
			if failed != test.fails {
				t.Errorf("failed %v, want %v: %s", failed, test.fails,
					strings.Join(reporter.errors, "; "))
			}
		})
	}
}

func TestNormaliseRowsRewritesBothSides(t *testing.T) {
	want := []Row{{Text: "parser: unparseable line (Expecting value: line 1 column 1 (char 0)): x"}}
	got := []Row{{Text: "parser: unparseable line (invalid JSON at line 1, column 1: invalid character 'x' " +
		"looking for beginning of value): x"}}
	reporter := &recordingTB{TB: t}
	normalWant, normalGot := NormaliseRows(reporter, "n", []int{102}, want, got)
	if !DiffRows(reporter, "n", normalWant, normalGot) || len(reporter.errors) > 0 {
		t.Errorf("rows still differ: %v", reporter.errors)
	}
	if want[0].Text != "parser: unparseable line (Expecting value: line 1 column 1 (char 0)): x" {
		t.Error("the caller's rows were changed")
	}
}

// timelineLine returns a timeline record with the given type, text, source
// file and line, at one time.
func timelineLine(turnType, text, sourceFile string, sourceLine int) string {
	return `{"timestamp_utc":"2026-09-10T08:00:00.000Z","agent":"tabby","session_id":"3",` +
		`"turn_type":"` + turnType + `","text":"` + text + `","source_file":"` + sourceFile +
		`","source_line":` + strconv.Itoa(sourceLine) + `,"recovered":"yes"}`
}

// sessionLine returns a session record of session 3 with a source file.
func sessionLine(sourceFile string) string {
	return `{"agent":"tabby","session_id":"3","first_timestamp_utc":"2026-09-10T08:00:00.000Z",` +
		`"user_turns":1,"source_file":"` + sourceFile + `"}`
}

func TestListingOrderTakesOnlyTheOtherOfTwoEqualRows(t *testing.T) {
	early, late := "/a/db.backup-1.sqlite", "/a/db.backup-2.sqlite"
	tests := []struct {
		name                      string
		want, got                 []string
		wantSessions, gotSessions string
		rows                      int
		rowsAfter                 []string
		sessionsAfter             string
	}{
		{"a row kept from the other backup is the recorded one",
			[]string{timelineLine("user", "hi", early, 1)}, []string{timelineLine("user", "hi", late, 5)},
			sessionLine(early), sessionLine(late), 1,
			[]string{timelineLine("user", "hi", early, 1)}, sessionLine(early)},
		{"a row that differs in its text is left as it is",
			[]string{timelineLine("user", "hi", early, 1)}, []string{timelineLine("user", "ho", late, 1)},
			sessionLine(early), sessionLine(late), 0,
			[]string{timelineLine("user", "ho", late, 1)}, sessionLine(late)},
		{"a row of the same file on another line is left as it is",
			[]string{timelineLine("user", "hi", early, 1)}, []string{timelineLine("user", "hi", early, 2)},
			sessionLine(early), sessionLine(early), 0,
			[]string{timelineLine("user", "hi", early, 2)}, sessionLine(early)},
		{"a session whose file no row moved is left as it is",
			[]string{timelineLine("user", "hi", early, 1)}, []string{timelineLine("user", "hi", early, 1)},
			sessionLine(early), sessionLine(late), 0,
			[]string{timelineLine("user", "hi", early, 1)}, sessionLine(late)},
		{"a row from a file whose recorded parse did not read the record is left as it is",
			[]string{timelineLine("user", "hi", early, 1)}, []string{timelineLine("user", "hi", "/a/other.sqlite", 5)},
			sessionLine(early), sessionLine("/a/other.sqlite"), 0,
			[]string{timelineLine("user", "hi", "/a/other.sqlite", 5)}, sessionLine("/a/other.sqlite")},
		{"rows both sides hold are not paired",
			[]string{timelineLine("system", "start", early, 3), timelineLine("user", "hi", early, 1)},
			[]string{timelineLine("system", "start", early, 3), timelineLine("user", "hi", late, 5)},
			sessionLine(early), sessionLine(late), 1,
			[]string{timelineLine("system", "start", early, 3), timelineLine("user", "hi", early, 1)},
			sessionLine(early)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := map[string]string{"out/timeline.jsonl": strings.Join(test.want, "\n") + "\n",
				"out/sessions.jsonl": test.wantSessions + "\n"}
			got := map[string]string{"out/timeline.jsonl": strings.Join(test.got, "\n") + "\n",
				"out/sessions.jsonl": test.gotSessions + "\n"}
			held := heldRecords{late: {identity("2026-09-10T08:00:00.000Z", "3", "user", "hi"): true}}
			rows := difference(t, 107).Outputs(want, got, held)
			if rows != test.rows {
				t.Errorf("rewrote %d rows, want %d", rows, test.rows)
			}
			if after := strings.Join(test.rowsAfter, "\n") + "\n"; got["out/timeline.jsonl"] != after {
				t.Errorf("timeline\n got  %q\n want %q", got["out/timeline.jsonl"], after)
			}
			if got["out/sessions.jsonl"] != test.sessionsAfter+"\n" {
				t.Errorf("sessions\n got  %q\n want %q", got["out/sessions.jsonl"], test.sessionsAfter)
			}
		})
	}
}

func TestListingOrderChecksTheDocumentedOrder(t *testing.T) {
	early, late := "/a/1.jsonl", "/a/2.jsonl"
	undated := strings.Replace(timelineLine("user", "x", early, 1), "2026-09-10T08:00:00.000Z", "", 1)
	tests := []struct {
		name  string
		path  string
		lines []string
		fails bool
	}{
		{"rows by source file then line pass", "out/timeline.jsonl",
			[]string{timelineLine("user", "b", early, 2), timelineLine("user", "a", late, 1), undated}, false},
		{"a later file before an earlier one fails", "out/timeline.jsonl",
			[]string{timelineLine("user", "a", late, 1), timelineLine("user", "b", early, 2)}, true},
		{"an undated row before a dated one fails", "out/timeline.jsonl",
			[]string{undated, timelineLine("user", "a", late, 1)}, true},
		{"sessions out of id order fail", "out/sessions.jsonl",
			[]string{sessionLine(early), strings.Replace(sessionLine(early), `"3"`, `"2"`, 1)}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			checkDocumentedOrder(reporter, "n", map[string]string{test.path: strings.Join(test.lines, "\n") + "\n"})
			if failed := len(reporter.errors) > 0; failed != test.fails {
				t.Errorf("failed %v, want %v: %v", failed, test.fails, reporter.errors)
			}
		})
	}
}

func TestADifferenceNoCaseUsedFailsAWholeRun(t *testing.T) {
	rewrite := quotedValueText(singleQuoted)
	table := []Difference{{Number: 101, Summary: "used", Text: rewrite, Binary: rewrite},
		{Number: 102, Summary: "unused", Text: rewrite, Binary: rewrite},
		{Number: 103, Summary: "retired", Retired: "no case needs it"},
		{Number: 104, Summary: "unchecked", Text: rewrite, Binary: rewrite, Unchecked: "needed only at some times"}}
	tests := []struct {
		name  string
		used  neededTally
		whole bool
		fails bool
	}{
		{"every difference used passes", neededTally{101: {"a"}, 102: {"b"}}, true, false},
		{"a difference no case used fails", neededTally{101: {"a"}}, true, true},
		{"a filtered run is not checked", neededTally{101: {"a"}}, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			checkEveryDifferenceUsed(reporter, table, test.used, test.whole)
			if failed := len(reporter.errors) > 0; failed != test.fails {
				t.Errorf("failed %v, want %v: %v", failed, test.fails, reporter.errors)
			}
		})
	}
}

func TestAnAgentWithoutAParserOrACaseComparedInPartFails(t *testing.T) {
	without := slices.DeleteFunc(parsers.All(), func(each parser.Parser) bool {
		return each.Agent() == "shellgpt"
	})
	tests := []struct {
		name    string
		check   func(reporter *recordingTB)
		failing bool
	}{
		{"every agent with a parser passes", func(reporter *recordingTB) {
			checkEveryAgentHasAParser(reporter, []string{"shellgpt", "aider"}, parsers.All())
		}, false},
		{"an agent whose parser is not registered fails", func(reporter *recordingTB) {
			checkEveryAgentHasAParser(reporter, []string{"shellgpt", "aider"}, without)
		}, true},
		{"cases compared in full pass", func(reporter *recordingTB) {
			(&commandReplay{full: map[string]int{"timeline": 2}, partial: map[string]int{}}).report(reporter)
		}, false},
		{"a case compared in part fails", func(reporter *recordingTB) {
			(&commandReplay{full: map[string]int{"timeline": 1}, partial: map[string]int{"timeline": 1}}).report(reporter)
		}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			test.check(reporter)
			if failed := len(reporter.errors) > 0; failed != test.failing {
				t.Errorf("failed %v, want %v: %v", failed, test.failing, reporter.errors)
			}
		})
	}
}
