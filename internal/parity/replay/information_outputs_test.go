//go:build parity

package replay

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The recorded runs testdata/recorded holds, by name: a timeline over a
// loose home, a detect and a detect --json over a collection, and a fleet.
const (
	recordedRoot   = "/evidence/root"
	recordedRemote = recordedRoot + "/cac-remotes-test-9wo1imx9/col"
)

// recordedRuns returns the recorded runs of testdata/recorded, each with
// its command line, compared as runs over a tree that is not loose so
// that the order of every line counts.
func recordedRuns(t *testing.T) map[string]commandOutput {
	t.Helper()
	return map[string]commandOutput{
		"timeline": {stdout: recordedText(t, "timeline.txt"), outputs: map[string]string{
			"out/timeline.jsonl": recordedText(t, "timeline.jsonl"),
			"out/sessions.jsonl": recordedText(t, "sessions.jsonl"),
			"out/inventory.json": recordedText(t, "inventory.json")}},
		"detect":      {stdout: recordedText(t, "detect.txt"), outputs: map[string]string{}},
		"detect-json": {stdout: recordedText(t, "detect.json"), outputs: map[string]string{}},
		"fleet": {stdout: recordedText(t, "fleet.txt"),
			outputs: map[string]string{"o.csv": recordedText(t, "fleet.csv")}},
	}
}

// changedOutput returns a copy of a run with the text of label (stdout or
// an output's path) rewritten.
func changedOutput(run commandOutput, label string, rewrite func(string) string) commandOutput {
	run.outputs = maps.Clone(run.outputs)
	if label == "stdout" {
		run.stdout = rewrite(run.stdout)
	} else {
		run.outputs[label] = rewrite(run.outputs[label])
	}
	return run
}

// outputsAgree compares two runs as the command replay does, and returns
// whether they agree and the differences of format they needed.
func outputsAgree(t *testing.T, want, got commandOutput) (bool, []int) {
	reporter := &recordingTB{TB: t}
	_, needed := compareInformation(reporter, "n", recordedRoot, want, got)
	return len(reporter.errors) == 0, slices.DeleteFunc(needed, func(number int) bool {
		return !slices.Contains(commandFormatNumbers(), number)
	})
}

// outputsRefused compares two runs once under every difference of format
// and reports whether they still differ.
func outputsRefused(t *testing.T, want, got commandOutput) bool {
	reporter := &recordingTB{TB: t}
	compareOutputs(reporter, "n", recordedRoot, outputForms{setOf(commandFormatNumbers())}, nil, want, got)
	return len(reporter.errors) > 0
}

// lineSwapped returns text with its lines first and second swapped, when
// both hold text.
func lineSwapped(first, second int) func(string) string {
	return func(text string) string {
		lines := strings.Split(text, "\n")
		if max(first, second) >= len(lines) || lines[first] == "" || lines[second] == "" {
			return text
		}
		lines[first], lines[second] = lines[second], lines[first]
		return strings.Join(lines, "\n")
	}
}

// lineDropped returns text without its line index.
func lineDropped(index int) func(string) string {
	return func(text string) string {
		lines := strings.Split(text, "\n")
		return strings.Join(slices.Delete(lines, index, index+1), "\n")
	}
}

func TestEachFormatOfTheBinaryPassesInTheOutputs(t *testing.T) {
	runs := recordedRuns(t)
	tests := []struct {
		name    string
		run     commandOutput
		label   string
		rewrite func(string) string
		needed  []int
	}{
		{"inventory.json written by encoding/json", runs["timeline"], "out/inventory.json",
			func(text string) string { return binaryJSON(text, "  ") }, []int{differenceJSONValues}},
		{"timeline.jsonl written by encoding/json", runs["timeline"], "out/timeline.jsonl", binaryJSONLines,
			[]int{differenceJSONValues}},
		{"sessions.jsonl written by encoding/json", runs["timeline"], "out/sessions.jsonl", binaryJSONLines,
			[]int{differenceJSONValues}},
		{"detect --json written by encoding/json", runs["detect-json"], "stdout",
			func(text string) string { return binaryJSON(text, "\t") }, []int{differenceJSONValues}},
		{"the CSV written by encoding/csv", changedOutput(runs["fleet"], "o.csv", func(text string) string {
			return strings.ReplaceAll(strings.Replace(text, ",ws01,", ", ws01,", 1), "\n", "\r\n")
		}), "o.csv", func(text string) string { return binaryCSV(strings.Replace(text, ", ws01,", ", ws01,", 1)) },
			[]int{differenceCSVRecords}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agree, needed := outputsAgree(t, test.run, changedOutput(test.run, test.label, test.rewrite))
			if !agree || !slices.Equal(needed, test.needed) {
				t.Errorf("agree %v needing %v, want agreement needing %v", agree, needed, test.needed)
			}
		})
	}
}

func TestEachMutationOfRecordedOutputsFails(t *testing.T) {
	runs := recordedRuns(t)
	timeline, sessions, inventory := "out/timeline.jsonl", "out/sessions.jsonl", "out/inventory.json"
	tests := []struct {
		name    string
		run     string
		label   string
		rewrite func(string) string
	}{
		{"a changed word in a row's text", "timeline", timeline, replaced("fix tests", "fix bests")},
		{"a row dropped", "timeline", timeline, lineDropped(3)},
		{"two rows swapped", "timeline", timeline, lineSwapped(2, 3)},
		{"a source_line changed", "timeline", timeline, replaced(`"source_line":0`, `"source_line":1`)},
		{"a timestamp moved by two milliseconds", "timeline", timeline,
			replaced("10:00:01.000Z", "10:00:01.002Z")},
		{"a session's turn counts changed", "timeline", sessions, replaced(`"user_turns":1`, `"user_turns":2`)},
		{"true turned into false", "timeline", inventory, replaced(`"inferred": true`, `"inferred": false`)},
		{"a removed key", "timeline", inventory, replaced(`  "sessions": 3,`+"\n", "")},
		{"an added key", "timeline", inventory, replaced(`  "sessions": 3,`, `  "sessions": 3, "x": 0,`)},
		{"a changed number", "timeline", inventory, replaced(`"sessions": 3,`, `"sessions": 4,`)},
		{"a count written as a float", "timeline", inventory, replaced(`"sessions": 3,`, `"sessions": 3.0,`)},
		{"two array elements swapped", "detect-json", "stdout", func(text string) string {
			text = strings.Replace(text, `"git@github.com:alice/proj.git"`, "{first}", 1)
			text = strings.Replace(text, `"https://mirror.example.com/alice/proj.git"`, `"git@github.com:alice/proj.git"`, 1)
			return strings.Replace(text, "{first}", `"https://mirror.example.com/alice/proj.git"`, 1)
		}},
		{"a CSV field changed", "fleet", "o.csv", replaced(",318,", ",319,")},
		{"a field moved to another column in a table", "detect", "stdout",
			replaced("codex-cli                 2        10812", "codex-cli             10812            2")},
		{"two cells of the fleet table exchanged", "fleet", "stdout", replaced("1      2  2026", "2      1  2026")},
		{"two lines of the timeline summary swapped", "timeline", "stdout", lineSwapped(0, 1)},
		{"two lines of the detect header swapped", "detect", "stdout", lineSwapped(1, 2)},
		{"the fleet table's header and first row swapped", "fleet", "stdout", lineSwapped(2, 3)},
		{"two CSV rows swapped", "fleet", "o.csv", lineSwapped(1, 2)},
		{"the CSV header and a row swapped", "fleet", "o.csv", lineSwapped(0, 1)},
	}
	for _, loose := range []bool{false, true} {
		for _, test := range tests {
			if loose && test.name == "two rows swapped" {
				// A loose tree's rows of equal sort keys keep the listing
				// order, so their order is compared through the documented
				// order instead.
				continue
			}
			t.Run(fmt.Sprintf("%s, loose %v", test.name, loose), func(t *testing.T) {
				want := runs[test.run]
				want.loose = loose
				mutated := changedOutput(want, test.label, test.rewrite)
				if reflect.DeepEqual(mutated, want) {
					t.Fatal("the mutation changed nothing")
				}
				got := binaryForm(mutated)
				got.loose = loose
				if !outputsRefused(t, want, got) {
					t.Error("the comparison accepted the mutation in the binary's form")
				}
			})
		}
	}
}

func TestALooseInputSortsOnlyWhatTheListingOrders(t *testing.T) {
	stdout := "input:   /in\nkind:    loose\nhost:    (unknown)\nproblem: /in/b: x\nproblem: /in/a: x\n\n" +
		"user  home  agent\nbob   /b    pi\nann   /a    pi\n\nann\tpi\t/a/2\nann\tpi\t/a/1\n"
	want := "input:   /in\nkind:    loose\nhost:    (unknown)\nproblem: /in/a: x\nproblem: /in/b: x\n\n" +
		"user  home  agent\nann   /a    pi\nbob   /b    pi\n\nann\tpi\t/a/1\nann\tpi\t/a/2\n"
	tests := []struct {
		name, label, text, want string
	}{
		{"stdout's problems, homes and files are sorted", "stdout", stdout, want},
		{"the other stdout lines keep their order", "stdout", "host: h\ninput: /in\n", "host: h\ninput: /in\n"},
		{"a CSV keeps its order", "o.csv", "agent,hosts\nb,1\na,1\n", "agent,hosts\nb,1\na,1\n"},
		{"JSON Lines keep their order", "out/timeline.jsonl", "{\"b\":1}\n{\"a\":1}\n", "{\"b\":1}\n{\"a\":1}\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := listingOrder(test.label, test.text); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// binaryForm returns a run with every text as the binary is to write it.
func binaryForm(run commandOutput) commandOutput {
	form := commandOutput{stdout: run.stdout, outputs: map[string]string{}}
	if strings.HasPrefix(run.stdout, "{") {
		form.stdout = binaryJSON(run.stdout, "  ")
	}
	for path, text := range run.outputs {
		switch {
		case strings.HasSuffix(path, ".jsonl"):
			form.outputs[path] = binaryJSONLines(text)
		case strings.HasSuffix(path, ".json"):
			form.outputs[path] = binaryJSON(text, "  ")
		case strings.HasSuffix(path, ".csv"):
			form.outputs[path] = binaryCSV(text)
		default:
			form.outputs[path] = text
		}
	}
	return form
}

func TestJSONAndCSVOutputsKeepWhatDecodingWouldLose(t *testing.T) {
	twice := `{"rows": 1, "rows": 2}` + "\n"
	csvText := "agent,hosts\r\nzed,1\r\n"
	tests := []struct {
		name, label, want, got string
		refused                bool
	}{
		{"a key written twice compares each value", "out/inventory.json", twice, `{"rows":1,"rows":2}`, false},
		{"a key written twice once is refused", "out/inventory.json", twice, `{"rows": 2}` + "\n", true},
		{"the first of two values changed is refused", "out/inventory.json", twice,
			`{"rows": 9, "rows": 2}` + "\n", true},
		{"the two values swapped are refused", "out/inventory.json", twice, `{"rows": 2, "rows": 1}` + "\n", true},
		{"a count written as a float is refused", "out/inventory.json", `{"rows": 3}`, `{"rows": 3.0}`, true},
		{"the CSV with its own line ends passes", "o.csv", csvText, "agent,hosts\nzed,1\n", false},
		{"an added blank CSV line is refused", "o.csv", csvText, "agent,hosts\n\nzed,1\n", true},
		{"an added blank line at the CSV's end is refused", "o.csv", csvText, "agent,hosts\nzed,1\n\n", true},
		{"a dropped blank CSV line is refused", "o.csv", "agent,hosts\r\n\r\nzed,1\r\n", "agent,hosts\nzed,1\n", true},
		{"a blank line inside a quoted field is the field's", "o.csv", "agent,note\r\nzed,\"a\r\n\r\nb\"\r\n",
			"agent,note\nzed,\"a\n\nb\"\n", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := commandOutput{outputs: map[string]string{test.label: test.want}}
			got := commandOutput{outputs: map[string]string{test.label: test.got}}
			if refused := outputsRefused(t, want, got); refused != test.refused {
				t.Errorf("refused %v, want %v", refused, test.refused)
			}
		})
	}
}
