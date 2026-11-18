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
		kinds := 0
		for _, set := range []bool{entry.Text != nil, entry.Rows != nil, entry.Excludes != nil,
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

func TestTextDifferencesRewriteOnlyTheirShape(t *testing.T) {
	tests := []struct {
		name   string
		number int
		text   string
		want   string
		found  int
	}{
		{"102 rewrites a JSON decoder detail in a parser row", 102,
			"parser: unparseable line (Expecting ',' delimiter: line 1 column 5 (char 4)): {\"a\" 1",
			"parser: unparseable line ({json error}): {\"a\" 1", 1},
		{"102 rewrites the binary's JSON decoder detail in a parser row", 102,
			"parser: unparseable record 2 (invalid JSON at line 1, column 4: trailing comma before ']'): ]",
			"parser: unparseable record 2 ({json error}): ]", 1},
		{"102 rewrites a SQLite error up to a JSON quote", 102,
			`{"text":"parser: SQLite error: file is not a database","source_line":0}`,
			`{"text":"parser: SQLite error: {sqlite error}","source_line":0}`, 1},
		{"102 rewrites a system error of a problem line", 102,
			"problem:   /x/y: could not be read (Permission denied)\n",
			"problem:   /x/y: could not be read ({system error})\n", 1},
		{"102 rewrites the binary's TOML reader detail in a parser row", 102,
			"parser: unreadable automation.toml: invalid TOML at line 2, column 20: strings cannot contain newlines",
			"parser: unreadable automation.toml: {toml error}", 1},
		{"102 rewrites a zip reader's detail in a parser row", 102,
			"parser: report.zip is not a zip archive (File is not a zip file)",
			"parser: report.zip is not a zip archive ({zip error})", 1},
		{"102 rewrites a base64 decoder's detail in a parser row", 102,
			"parser: session data is not base64 (Only base64 data is allowed)",
			"parser: session data is not base64 ({base64 error})", 1},
		{"102 rewrites the database detail of an unreadable OpenCode store", 102,
			"parser: database could not be read: file is not a database",
			"parser: database could not be read: {sqlite error}", 1},
		{"102 leaves a lead-in in capitals before Python's JSON detail", 102,
			"parser: unparseable record 4 (tool_calls is not valid JSON: Expecting value: line 1 column 1 (char 0)): nope",
			"parser: unparseable record 4 (tool_calls is not valid JSON: {json error}): nope", 1},
		{"102 leaves a record's key before Python's JSON detail", 102,
			"parser: unparseable record 14 (bubbleId:Ab: Expecting value: line 1 column 1 (char 0)): x",
			"parser: unparseable record 14 (bubbleId:Ab: {json error}): x", 1},
		{"102 rewrites the database detail of an unreadable editor state store", 102,
			"parser: state.vscdb unreadable: database disk image is malformed",
			"parser: state.vscdb unreadable: {sqlite error}", 1},
		{"102 leaves a note after the binary's JSON detail", 102,
			"parser: unparseable record 2 (invalid JSON at line 1, column 1: expected a value; stored zstd blob as hex): 28b5",
			"parser: unparseable record 2 ({json error}; stored zstd blob as hex): 28b5", 1},
		{"the note for an unreadable persisted output keeps its form", 102,
			"output\n[persisted output could not be read (Permission denied)]",
			"output\n[persisted output could not be read ({system error})]", 1},
		{"102 rewrites an XML parse error", 102,
			"parser: cody_history.xml did not parse; read 0 chat(s) (not well-formed (invalid token): line 1, column 5)",
			"parser: cody_history.xml did not parse; read 0 chat(s) ({xml error})", 1},
		{"102 rewrites a TOML error", 102,
			"parser: unreadable config.toml: Invalid value (at line 2, column 7)",
			"parser: unreadable config.toml: {toml error}", 1},
		{"102 rewrites Python's JSON error with its class name", 102,
			"parser: unparseable record 2 (JSONDecodeError: Expecting value: line 1 column 1 (char 0); stored zstd blob as hex): 28b5",
			"parser: unparseable record 2 ({json error}; stored zstd blob as hex): 28b5", 1},
		{"102 rewrites Python's JSON error with its class name after a key", 102,
			"parser: unparseable record 5 (agent_draft_prompts d: JSONDecodeError: Unterminated string starting at: line 1 column 24 (char 23)): [",
			"parser: unparseable record 5 (agent_draft_prompts d: {json error}): [", 1},
		{"102 rewrites the binary's JSON error after a key", 102,
			"parser: unparseable record 5 (agent_draft_prompts d: invalid JSON at line 1, column 24: unterminated string): [",
			"parser: unparseable record 5 (agent_draft_prompts d: {json error}): [", 1},
		{"102 rewrites Python's thread that is not an object", 102,
			`parser: unparseable record 2 (ValueError: not a JSON object): ["not"]`,
			`parser: unparseable record 2 ({thread not an object}): ["not"]`, 1},
		{"102 rewrites the binary's thread that is not an object", 102,
			`parser: unparseable record 2 (the thread is not a JSON object): ["not"]`,
			`parser: unparseable record 2 ({thread not an object}): ["not"]`, 1},
		{"102 rewrites Python's worktree record without its seconds", 102,
			`parser: unparseable record 6 (created_git_worktrees: KeyError: 'created_at_seconds'): {}`,
			`parser: unparseable record 6 (created_git_worktrees: {no creation time}): {}`, 1},
		{"102 rewrites the binary's worktree record without its seconds", 102,
			`parser: unparseable record 6 (created_git_worktrees: no created_at_seconds field): {}`,
			`parser: unparseable record 6 (created_git_worktrees: {no creation time}): {}`, 1},
		{"102 rewrites Python's UTF-8 codec error on one byte", 102,
			"parser: unparseable line ('utf-8' codec can't decode byte 0xc3 in position 0: invalid continuation byte): x",
			"parser: unparseable line ({utf-8 error}): x", 1},
		{"102 rewrites Python's UTF-8 codec error on several bytes", 102,
			"parser: unparseable line ('utf-8' codec can't decode bytes in position 2-3: unexpected end of data): x",
			"parser: unparseable line ({utf-8 error}): x", 1},
		{"102 rewrites the binary's UTF-8 error", 102,
			"parser: unparseable line (not valid UTF-8 at byte offset 10): x",
			"parser: unparseable line ({utf-8 error}): x", 1},
		{"102 leaves a UTF-8 error that names no offset alone", 102,
			"parser: unparseable line (not valid UTF-8): x", "parser: unparseable line (not valid UTF-8): x", 0},
		{"102 rewrites Python's path of a mutation not applied", 102,
			`parser: mutation not applied (path ['nowhere'] not found): {"kind": 1}`,
			`parser: mutation not applied (path {mutation path} not found): {"kind": 1}`, 1},
		{"102 rewrites the binary's path of a mutation not applied", 102,
			`parser: mutation not applied (path ["nowhere"] not found): {"kind": 1}`,
			`parser: mutation not applied (path {mutation path} not found): {"kind": 1}`, 1},
		{"102 rewrites Python's unclosed quote", 102,
			"parser: unparseable line (No closing quotation): permissions allow 'x",
			"parser: unparseable line ({unclosed quote}): permissions allow 'x", 1},
		{"102 rewrites the binary's unclosed quote", 102,
			"parser: unparseable line (a quote is not closed): permissions allow 'x",
			"parser: unparseable line ({unclosed quote}): permissions allow 'x", 1},
		{"102 leaves the analyzer's own wording alone", 102,
			"parser: unparseable line (not a JSON object): [1]",
			"parser: unparseable line (not a JSON object): [1]", 0},
		{"102 leaves an assistant row that quotes an error alone", 102,
			"SQLite error: file is not a database", "SQLite error: file is not a database", 0},
		{"120 writes Python's quoted value in one form", 120,
			"error: cannot read 'last friday' as a time; accepted: 'ago'\n",
			"error: cannot read {quoted last friday} as a time; accepted: 'ago'\n", 1},
		{"120 writes Go's quoted value in the same form", 120,
			"error: cannot read \"last friday\" as a time; accepted: 'ago'\n",
			"error: cannot read {quoted last friday} as a time; accepted: 'ago'\n", 1},
		{"120 reads a value that starts the reason", 120, "error: '2026-13' is out of range\n",
			"error: {quoted 2026-13} is out of range\n", 1},
		{"120 leaves a value holding a quote alone", 120, "error: cannot read \"it's\" as a time\n",
			"error: cannot read \"it's\" as a time\n", 0},
		{"130 gives a TOML config's recorded reason one token", 130,
			"parser: config.toml is not valid TOML (Invalid value (at line 2, column 25))",
			"parser: config.toml is not valid TOML ({position})", 1},
		{"130 gives a TOML config's position the same token", 130,
			"parser: config.toml is not valid TOML (invalid TOML at line 2, column 25)",
			"parser: config.toml is not valid TOML ({position})", 1},
		{"130 gives a JSON config's reason one token", 130,
			"parser: config.json did not parse: invalid JSON at line 1, column 11",
			"parser: config.json did not parse: {position}", 1},
		{"130 drops the position of a crush.json", 130,
			"parser: .config/crush/crush.json is not a JSON object (invalid JSON at line 1, column 3)",
			"parser: .config/crush/crush.json is not a JSON object", 1},
		{"130 gives an exec config's reason one token and keeps its note", 130,
			"parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 60; text withheld, it",
			"parser: exec_approvals_config raw_json unparseable ({position}; text withheld, it", 1},
		{"130 leaves another file's reason alone", 130,
			"parser: projects.json did not parse: invalid JSON at line 1, column 2",
			"parser: projects.json did not parse: invalid JSON at line 1, column 2", 0},
		{"120 leaves a quote inside a row's text alone", 120, "parser: cannot read 'x' here",
			"parser: cannot read 'x' here", 0},
		{"122 writes the plural note of one manifest row in the singular", 122,
			"note:      1 manifest rows did not parse\n", "note:      1 manifest row did not parse\n", 1},
		{"122 finds the singular note too", 122, `"notes": ["1 manifest row did not parse"]`,
			`"notes": ["1 manifest row did not parse"]`, 1},
		{"122 leaves a note of eleven rows alone", 122, "note: 11 manifest rows did not parse",
			"note: 11 manifest rows did not parse", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := difference(t, test.number).Text(test.text)
			if got != test.want || found != test.found {
				t.Errorf("got %q (%d places), want %q (%d)", got, found, test.want, test.found)
			}
		})
	}
}

func TestQuotedValuesStillDifferWhenTheValueDiffers(t *testing.T) {
	want, _ := quotedValueText("error: cannot read 'last friday' as a time\n")
	got, _ := quotedValueText("error: cannot read \"last monday\" as a time\n")
	if want == got {
		t.Errorf("two different values compare equal: %q", want)
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

func TestPlannerOrderSortsOnlyRunsOfItsRows(t *testing.T) {
	rows := []Row{{Text: "user prompt"}, {Text: "archived worktree: b"},
		{Text: "archived worktree: a"}, {Text: "assistant"}, {Text: "archived worktree: c"}}
	got, found := difference(t, 105).Rows(rows)
	texts := []string{}
	for _, row := range got {
		texts = append(texts, string(row.Text))
	}
	want := []string{"user prompt", "archived worktree: a", "archived worktree: b", "assistant",
		"archived worktree: c"}
	if !slices.Equal(texts, want) || found != 3 {
		t.Errorf("got %q (%d rows), want %q (3)", texts, found, want)
	}
	if rows[1].Text != "archived worktree: b" {
		t.Error("the input rows were reordered in place")
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
	want := []Row{{Text: "parser: SQLite error: file is not a database"}}
	got := []Row{{Text: "parser: SQLite error: malformed database: bad header"}}
	reporter := &recordingTB{TB: t}
	normalWant, normalGot := NormaliseRows(reporter, "n", []int{102}, want, got)
	if !DiffRows(reporter, "n", normalWant, normalGot) || len(reporter.errors) > 0 {
		t.Errorf("rows still differ: %v", reporter.errors)
	}
	if want[0].Text != "parser: SQLite error: file is not a database" {
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
	table := []Difference{{Number: 101, Summary: "used", Text: quotedValueText},
		{Number: 102, Summary: "unused", Text: libraryErrorText},
		{Number: 103, Summary: "retired", Retired: "no case needs it"},
		{Number: 104, Summary: "unchecked", Text: libraryErrorText, Unchecked: "needed only at some times"}}
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
