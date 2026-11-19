//go:build parity

package replay

import (
	"bufio"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// recordedText returns a file of testdata/recorded: outputs and rows the
// recorder wrote, the recording's root written as /evidence/root. Its line
// ends are LF, as recorded, also where a Windows checkout made them CRLF.
func recordedText(t *testing.T, name string) string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", "recorded", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// recordedRows returns the recorded rows of testdata/recorded/rows.jsonl,
// the rows of a few parses of seven agents.
func recordedRows(t *testing.T) []Row {
	t.Helper()
	var rows []Row
	scanner := bufio.NewScanner(strings.NewReader(recordedText(t, "rows.jsonl")))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var row Row
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

// rowHolding returns the index of the first row whose text holds part.
func rowHolding(t *testing.T, rows []Row, part string) int {
	t.Helper()
	index := slices.IndexFunc(rows, func(row Row) bool { return strings.Contains(string(row.Text), part) })
	if index < 0 {
		t.Fatalf("no recorded row holds %q", part)
	}
	return index
}

// rowsAgree compares two row lists as the parser replay does and returns
// whether they agree and the differences of format they needed.
func rowsAgree(t *testing.T, want, got []Row) (bool, []int) {
	return rowsAgreeCut(t, want, got, nil)
}

// rowsAgreeCut is rowsAgree for a case whose sources hold the times of
// cut, which difference 119 may find one millisecond earlier.
func rowsAgreeCut(t *testing.T, want, got []Row, cut *cutTimes) (bool, []int) {
	reporter := &recordingTB{TB: t}
	needed := compareRowInformation(reporter, "n", want, got, cut)
	return len(reporter.errors) == 0, needed
}

// textChanged returns a copy of rows with the text of row index rewritten.
func textChanged(rows []Row, index int, rewrite func(string) string) []Row {
	rows = slices.Clone(rows)
	rows[index].Text = Text(rewrite(string(rows[index].Text)))
	return rows
}

// replaced returns a function that replaces old by new once.
func replaced(old, replacement string) func(string) string {
	return func(text string) string { return strings.Replace(text, old, replacement, 1) }
}

func TestEachFormatOfTheBinaryPassesUnderItsOwnDifference(t *testing.T) {
	rows := recordedRows(t)
	session := rowHolding(t, rows, "interactive=True")
	sandbox := rowHolding(t, rows, `sandbox={"type":"workspace-write"}`)
	mutation := rowHolding(t, rows, `{"kind": 1, "k": ["nowhere", "x"], "v": 1}`)
	tests := []struct {
		name   string
		want   []Row
		got    []Row
		needed []int
	}{
		{"a formatted True is true", rows, textChanged(rows, session, replaced("=True", "=true")),
			[]int{differenceBoolean}},
		{"embedded JSON is compact in the evidence's order", rows,
			textChanged(rows, mutation, replaced(`{"kind": 1, "k": ["nowhere", "x"], "v": 1}`,
				`{"v":1,"kind":1,"k":["nowhere","x"]}`)), []int{differenceEmbeddedJSON}},
		{"embedded JSON's spacing is not compared", rows,
			textChanged(rows, sandbox, replaced(`{"type":"workspace-write"}`, `{ "type": "workspace-write" }`)),
			[]int{differenceEmbeddedJSON}},
		{"a timestamp cut from a time below the millisecond is not rounded up", rows, timeMoved(rows, session, -1),
			[]int{differenceRoundedTime}},
	}
	cut := fixedCutTimes(movedTimestamp(string(rows[session].TimestampUTC), -1))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agree, needed := rowsAgreeCut(t, test.want, test.got, cut)
			if !agree || !slices.Equal(needed, test.needed) {
				t.Errorf("agree %v needing %v, want agreement needing %v", agree, needed, test.needed)
			}
		})
	}
}

// movedTimestamp returns a timestamp moved by milliseconds.
func movedTimestamp(text string, milliseconds int) string {
	moment, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return text
	}
	return moment.Add(time.Duration(milliseconds) * time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z")
}

// timeMoved returns a copy of rows with the timestamp of row index moved
// by milliseconds.
func timeMoved(rows []Row, index, milliseconds int) []Row {
	rows = slices.Clone(rows)
	rows[index].TimestampUTC = Text(movedTimestamp(string(rows[index].TimestampUTC), milliseconds))
	return rows
}

func TestEachMutationOfRecordedRowsFails(t *testing.T) {
	rows := recordedRows(t)
	session := rowHolding(t, rows, "interactive=True")
	prose := rowHolding(t, rows, "fix tests | [1 image(s)]")
	sandbox := rowHolding(t, rows, `sandbox={"type":"workspace-write"}`)
	mutation := rowHolding(t, rows, `{"kind": 1, "k": ["nowhere", "x"], "v": 1}`)
	array := rowHolding(t, rows, `["oops",{"id":"call_8"`)
	cost := rowHolding(t, rows, "cost=0.0041")
	swapped := slices.Clone(rows)
	swapped[cost-1], swapped[cost] = swapped[cost], swapped[cost-1]
	userTrue := textChanged(rows, prose, replaced("fix tests", "True"))
	tests := []struct {
		name      string
		want, got []Row
	}{
		{"a changed word in a text", rows, textChanged(rows, session, replaced("fix tests", "fix bests"))},
		{"a changed digit in a number inside embedded JSON", rows, textChanged(rows, mutation, replaced(`"v": 1`, `"v": 2`))},
		{"a changed number after =", rows, textChanged(rows, cost, replaced("cost=0.0041", "cost=0.0042"))},
		{"a removed key in embedded JSON", rows, textChanged(rows, mutation, replaced(`, "v": 1`, ""))},
		{"an added key in embedded JSON", rows, textChanged(rows, sandbox, replaced(`"workspace-write"}`, `"workspace-write","x":0}`))},
		{"a changed value of a key", rows, textChanged(rows, sandbox, replaced("workspace-write", "read-only"))},
		{"two array elements swapped in embedded JSON", rows, textChanged(rows, mutation, replaced(`["nowhere", "x"]`, `["x", "nowhere"]`))},
		{"two array elements swapped in a parser row", rows, textChanged(rows, array, func(text string) string {
			head, rest, _ := strings.Cut(text, `["oops",`)
			return head + "[" + strings.TrimSuffix(rest, "]") + `,"oops"]`
		})},
		{"a row dropped", rows, slices.Delete(slices.Clone(rows), cost, cost+1)},
		{"two rows swapped", rows, swapped},
		{"true turned into false", rows, textChanged(rows, session, replaced("=True", "=False"))},
		{"a timestamp two milliseconds later", rows, timeMoved(rows, cost, 2)},
		{"a source_line changed", rows, func() []Row {
			changed := slices.Clone(rows)
			changed[cost].SourceLine++
			return changed
		}()},
		{"True turned into true in a user's own words", userTrue,
			textChanged(userTrue, prose, replaced("True", "true"))},
		{"a number in embedded JSON written another way", textChanged(rows, sandbox, replaced(`{"type"`, `{"n":1.5,"type"`)),
			textChanged(rows, sandbox, replaced(`{"type"`, `{"n":1.50,"type"`))},
		{"a float after = written as an integer", textChanged(rows, cost, replaced("0.0041", "1.0")),
			textChanged(rows, cost, replaced("0.0041", "1"))},
		{"None after = turned into nothing", textChanged(rows, cost, replaced("cost=0.0041", "cost=None")),
			textChanged(rows, cost, replaced("cost=0.0041", "cost="))},
		{"a key written twice once", textChanged(rows, sandbox, replaced(`{"type"`, `{"type":"x","type"`)), rows},
		{"the first of two values of a key changed",
			textChanged(rows, sandbox, replaced(`{"type"`, `{"type":"x","type"`)),
			textChanged(rows, sandbox, replaced(`{"type"`, `{"type":"y","type"`))},
		{"a list repr in a user's words turned into JSON", textChanged(rows, prose, replaced("fix tests", "{'a': 1}")),
			textChanged(rows, prose, replaced("fix tests", `{"a":1}`))},
		{"a separator at the end trimmed", textChanged(rows, cost, func(text string) string { return text + " \x1c" }),
			rows},
		{"a lone surrogate turned into U+FFFD", textChanged(rows, cost, replaced("cost", "co\xed\xa0\x80st")),
			textChanged(rows, cost, replaced("cost", "co\uFFFDst"))},
		{"a time quoted in a row's text moved one millisecond earlier",
			textChanged(rows, cost, func(text string) string { return text + " at 2026-09-10T08:00:00.500Z" }),
			textChanged(rows, cost, func(text string) string { return text + " at 2026-09-10T08:00:00.499Z" })},
		{"a timestamp moved one millisecond later", rows, timeMoved(rows, cost, 1)},
		{"a session id's leading zero dropped", fieldChanged(rows, cost, func(row *Row) { row.SessionID = "007" }),
			fieldChanged(rows, cost, func(row *Row) { row.SessionID = "7" })},
		{"None in a formatted field turned into nothing", fieldChanged(rows, cost, func(row *Row) { row.GitBranch = "None" }),
			fieldChanged(rows, cost, func(row *Row) { row.GitBranch = "" })},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree, _ := rowsAgree(t, test.want, test.got); agree {
				t.Error("the comparison accepted the mutation")
			}
		})
	}
}

// fieldChanged returns a copy of rows with row index changed by change.
func fieldChanged(rows []Row, index int, change func(row *Row)) []Row {
	rows = slices.Clone(rows)
	change(&rows[index])
	return rows
}
