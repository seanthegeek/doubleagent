//go:build parity

package replay

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// rowMutation is a change of what recorded rows say, which the
// information comparison must refuse; apply reports false when the rows
// give it nothing to change.
type rowMutation struct {
	name  string
	apply func(rows []Row) ([]Row, bool)
}

// rowMutations returns the changes made to every recorded parse.
func rowMutations() []rowMutation {
	return []rowMutation{
		{"a changed letter in a text", textMutation(func(character rune) bool {
			return unicode.IsLetter(character) && character < unicode.MaxASCII
		})},
		{"a changed digit in a text", textMutation(func(character rune) bool {
			return character >= '0' && character <= '9'
		})},
		{"a row dropped", func(rows []Row) ([]Row, bool) { return rows[:len(rows)-1], true }},
		{"two rows swapped", rowsSwapped},
		{"a source_line changed", func(rows []Row) ([]Row, bool) {
			rows = slices.Clone(rows)
			rows[0].SourceLine++
			return rows, true
		}},
		{"a timestamp moved by two milliseconds", func(rows []Row) ([]Row, bool) {
			index := slices.IndexFunc(rows, func(row Row) bool { return timestampText.MatchString(string(row.TimestampUTC)) })
			if index < 0 {
				return nil, false
			}
			return timeMoved(rows, index, 2), true
		}},
		{"a formatted boolean flipped", textMutation(nil)},
	}
}

// textMutation changes the first character of a row's text that changes
// accepts to another such character; with no test it turns the first
// formatted True or true into False.
func textMutation(changes func(rune) bool) func(rows []Row) ([]Row, bool) {
	return func(rows []Row) ([]Row, bool) {
		for i, row := range rows {
			text := string(row.Text)
			if changes == nil {
				if flipped := formattedTrue.ReplaceAllString(text, "${1}False"); flipped != text {
					return textChanged(rows, i, func(string) string { return flipped }), true
				}
				continue
			}
			if position := strings.IndexFunc(text, changes); position >= 0 {
				other := byte('a')
				if text[position] == 'a' || text[position] == '0' {
					other++
				} else if text[position] >= '0' && text[position] <= '9' {
					other = '0'
				}
				return textChanged(rows, i, func(string) string { return text[:position] + string(other) + text[position+1:] }), true
			}
		}
		return nil, false
	}
}

// formattedTrue is a formatted True after "=".
var formattedTrue = regexp.MustCompile(`(=)(?:True|true)\b`)

// rowsSwapped swaps the first two neighbouring rows that differ.
func rowsSwapped(rows []Row) ([]Row, bool) {
	for i := 1; i < len(rows); i++ {
		if rows[i] != rows[i-1] {
			rows = slices.Clone(rows)
			rows[i], rows[i-1] = rows[i-1], rows[i]
			return rows, true
		}
	}
	return nil, false
}

// binaryRows returns rows with each text as the binary is to write it.
func binaryRows(rows []Row) []Row {
	rows = slices.Clone(rows)
	for i := range rows {
		rows[i].Text = Text(binaryRowText(string(rows[i].TurnType), string(rows[i].Text)))
	}
	return rows
}

func TestEveryRecordedParseFailsEachMutationAndPassesTheBinarysForm(t *testing.T) {
	corpus := Open(t)
	refused, formatted, parses, formNeeded := map[string]int{}, 0, 0, neededTally{}
	for _, agent := range caseFiles(t, corpus, "parsers") {
		for _, recorded := range corpus.ParserCases(t, agent) {
			rows := recorded.Result.Rows
			if recorded.Method != "parse" || len(rows) == 0 {
				continue
			}
			parses++
			node := fmt.Sprintf("%s (seq %d)", recorded.Node, recorded.Seq)
			binary := binaryRows(rows)
			if !slices.Equal(binary, rows) {
				formatted++
			}
			agree, needed := rowsAgree(t, rows, binary)
			if !agree {
				t.Errorf("%s: the binary's form of the rows is refused", node)
			}
			formNeeded.note(node, needed)
			for _, mutation := range rowMutations() {
				mutated, ok := mutation.apply(rows)
				if !ok {
					continue
				}
				if agree, _ := rowsAgree(t, rows, binaryRows(mutated)); agree {
					t.Errorf("%s: %s is accepted", node, mutation.name)
					continue
				}
				refused[mutation.name]++
			}
		}
	}
	t.Logf("the binary's form changes the rows of %d of %d recorded parses:", formatted, parses)
	logNeeded(t, "the binary's form", formNeeded, rowFormatNumbers())
	t.Logf("mutations of the recorded parses:")
	for _, mutation := range rowMutations() {
		t.Logf("  %s: refused in %d parses", mutation.name, refused[mutation.name])
	}
}

// outputMutation is a change of what a recorded run's output says, which
// the information comparison must refuse; it names the text it changes by
// a label's suffix (stdout, or an output path's ending).
type outputMutation struct {
	name, suffix string
	rewrite      func(text string) string
}

// firstMatch returns a rewrite of the first match of pattern by change.
func firstMatch(pattern string, change func(match []string) string) func(string) string {
	shape := regexp.MustCompile(pattern)
	return func(text string) string {
		location := shape.FindStringSubmatchIndex(text)
		if location == nil {
			return text
		}
		var parts []string
		for i := 0; i < len(location); i += 2 {
			parts = append(parts, text[location[i]:location[i+1]])
		}
		return text[:location[0]] + change(parts) + text[location[1]:]
	}
}

// outputMutations returns the changes made to every recorded run.
func outputMutations() []outputMutation {
	plusOne := func(match []string) string {
		number, _ := strconv.Atoi(match[2])
		return match[1] + strconv.Itoa(number+1)
	}
	return []outputMutation{
		{"a row dropped", "timeline.jsonl", lineDropped(0)},
		{"two rows swapped", "timeline.jsonl", lineSwapped(0, 1)},
		{"a source_line changed", "timeline.jsonl", firstMatch(`("source_line":)([0-9]+)`, plusOne)},
		{"a changed letter in a row's text", "timeline.jsonl", firstMatch(`("text":")([a-z])`, func(match []string) string {
			return match[1] + map[bool]string{true: "b", false: "a"}[match[2] == "a"]
		})},
		{"a session's turn counts changed", "sessions.jsonl", firstMatch(`("user_turns":)([0-9]+)`, plusOne)},
		{"true turned into false", ".json", firstMatch(`(: )(true)`, func(match []string) string { return match[1] + "false" })},
		{"true turned into false in detect --json", "stdout", firstMatch(`(: )(true)`, func(match []string) string {
			return match[1] + "false"
		})},
		{"a CSV field changed", ".csv", firstMatch(`(\n[^\n]*?,)([0-9]+)`, plusOne)},
		{"two CSV rows swapped", ".csv", lineSwapped(1, 2)},
		{"the CSV header and a row swapped", ".csv", lineSwapped(0, 1)},
		{"two stdout lines swapped", "stdout", lineSwapped(0, 1)},
		{"two cells of a table exchanged", "stdout", firstMatch(`(?m)^(\S+)( {2,})(\S+)( {2,})(\S+)`, func(match []string) string {
			return match[1] + match[2] + match[5] + match[4] + match[3]
		})},
	}
}

func TestEveryRecordedRunFailsEachMutationAndPassesTheBinarysForm(t *testing.T) {
	corpus := Open(t)
	replay := newCommandReplay(t, corpus)
	refused, runs, formNeeded := map[string]int{}, 0, neededTally{}
	for _, command := range caseFiles(t, corpus, "commands") {
		for _, recorded := range corpus.CommandCases(t, command) {
			if recorded.Error != nil {
				continue
			}
			runs++
			node := fmt.Sprintf("%s (seq %d)", recorded.Node, recorded.Seq)
			want := replay.expected(t, recorded, recordedRoot)
			agree, needed := outputsAgree(t, want, runForm(want, want))
			if !agree {
				t.Errorf("%s: the binary's form of the run is refused", node)
			}
			formNeeded.note(node, needed)
			for _, mutation := range outputMutations() {
				if mutatedRun(want, mutation, t, refused) {
					t.Errorf("%s: %s is accepted", node, mutation.name)
				}
			}
		}
	}
	t.Logf("the binary's form of %d recorded runs:", runs)
	logNeeded(t, "the binary's form", formNeeded, commandFormatNumbers())
	t.Logf("mutations of the recorded runs:")
	for _, mutation := range outputMutations() {
		t.Logf("  %s: refused in %d runs", mutation.name, refused[mutation.name])
	}
}

// recordedWordingForm returns text with the recorded shape of each
// difference of text in its compared form.
func recordedWordingForm(text string) string {
	for _, entry := range applyingOrder(commandDifferences()) {
		if entry.Text != nil {
			text, _ = entry.Text(text)
		}
	}
	return text
}

// mutatedRun applies a mutation to each text of a run it names, and
// reports whether the comparison accepted a changed run; each refusal is
// counted under the mutation's name.
func mutatedRun(want commandOutput, mutation outputMutation, t *testing.T, refused map[string]int) bool {
	labels := append([]string{"stdout"}, slices.Sorted(maps.Keys(want.outputs))...)
	for _, label := range labels {
		if !strings.HasSuffix(label, mutation.suffix) ||
			(mutation.name == "two rows swapped" && want.loose) ||
			(label == "stdout" && strings.HasPrefix(want.stdout, "{") != strings.Contains(mutation.name, "--json")) {
			continue
		}
		mutated := changedOutput(want, label, mutation.rewrite)
		if mutated.stdout == want.stdout && maps.Equal(mutated.outputs, want.outputs) {
			continue
		}
		if !outputsRefused(t, want, runForm(mutated, want)) {
			return true
		}
		refused[mutation.name]++
	}
	return false
}

// runForm returns a run in the binary's form, with the recorded run's
// exit, stderr and kind of input. The binary's own wording of a condition
// cannot be built from the recording, so each recorded wording of a
// difference of text is first put in the form the comparison gives both.
func runForm(run, recorded commandOutput) commandOutput {
	run.stdout = recordedWordingForm(run.stdout)
	run.outputs = maps.Clone(run.outputs)
	for path, text := range run.outputs {
		run.outputs[path] = recordedWordingForm(text)
	}
	form := binaryForm(run)
	form.exit, form.stderr, form.loose = recorded.exit, recordedWordingForm(recorded.stderr), recorded.loose
	return form
}
