package growthtests

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The repeat test holds every parser, over the files of every growth case,
// to the rule for a JSON object that repeats a member name: the rows are
// those of the same file without the repeat, the value used being the
// last, plus one problem row per file and reader that says so. A case's
// JSON files are written a second time with each object's first member
// given twice, first with the placeholder 0 and then with its own value,
// so every value the parser reads is the same and only the repeat is new.

// firstMember is the start of an object up to its first member's colon, in
// text outside a JSON string: inside one a quote is escaped, so an opening
// brace is never directly followed by a bare quote there.
var firstMember = regexp.MustCompile(`\{(\s*)("(?:[^"\\]|\\.)*")(\s*):`)

// isJSONText reports whether data is one JSON document, or lines that are
// each blank or a JSON document.
func isJSONText(data []byte) bool {
	if _, err := jsonvalue.Decode(data); err == nil {
		return true
	}
	any := false
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if _, err := jsonvalue.Decode(bytes.TrimSpace(line)); err != nil {
			return false
		}
		any = true
	}
	return any
}

// carriedCount is the count a problem row of cut values gives, which the
// repeat row adds one to when it carries a cut field.
var carriedCount = regexp.MustCompile(`cut to 4096 bytes \(\d+ times?\)`)

// repeatFirstMembers returns data with every object's first member given
// copies more times ahead of itself with the value 0, and whether data is a
// JSON document or JSON lines that held an object; anything else is
// returned as it is.
func repeatFirstMembers(data []byte, copies int) ([]byte, bool) {
	if !isJSONText(data) {
		return data, false
	}
	repeated := firstMember.ReplaceAll(data, []byte("{$1"+strings.Repeat("$2$3:0,", copies)+"$2$3:"))
	return repeated, !bytes.Equal(repeated, data)
}

// unrepeat returns text with each first member repeatFirstMembers added
// taken out again, for a row that quotes a line as read.
func unrepeat(text string) string {
	var out strings.Builder
	last := 0
	for _, match := range firstMember.FindAllStringSubmatchIndex(text, -1) {
		name, space := text[match[4]:match[5]], text[match[6]:match[7]]
		rest := text[match[1]:]
		added := "0," + name + space + ":"
		if !strings.HasPrefix(rest, added) {
			continue
		}
		out.WriteString(text[last:match[1]])
		last = match[1] + len(added)
		for strings.HasPrefix(text[last:], added) {
			last += len(added)
		}
	}
	out.WriteString(text[last:])
	return out.String()
}

// isRepeatRow reports whether a row is the problem row of repeated member
// names.
func isRepeatRow(row timeline.Row) bool {
	return strings.HasPrefix(row.Text, parserkit.ProblemPrefix) &&
		(strings.Contains(row.Text, " member names repeat within their JSON objects (") ||
			strings.Contains(row.Text, " member name repeats within its JSON object ("))
}

// rowsOf parses the artifacts with the parser as a run does and returns
// each row, primary then secondary, and each problem line, as text that
// leaves out nothing the output writes.
func rowsOf(t *testing.T, each parserkit.Parser, artifacts []*input.Artifact) []timeline.Row {
	t.Helper()
	result, err := parsers.NewOf([]parserkit.Parser{each}).Collect(t.Context(), &input.Collection{Artifacts: artifacts},
		parserkit.Options{IncludeThinking: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := slices.Clone(result.Primary)
	for _, row := range result.Secondary {
		rows = append(rows, row.Row)
	}
	for _, line := range result.Problems {
		rows = append(rows, timeline.Row{Text: "problem line: " + line})
	}
	return rows
}

// rowText returns every string of a row and its source line, one per
// line, for a comparison of two parses: without the members the repeat
// added to a line a row quotes, and without the count of cut values, which
// the repeat row adds to by carrying a cut field.
func rowText(row timeline.Row) string {
	text := strings.Join(append(row.Strings(), fmt.Sprint(row.SourceLine)), "\n")
	return carriedCount.ReplaceAllString(unrepeat(text), "cut to 4096 bytes")
}

// repeatOutcome is what repeating the first members of a case's files did:
// the files changed, the problem rows of repeats, and the rows that differ
// from the parse of the files as written, other than those.
type repeatOutcome struct {
	files, repeatedFiles, repeatRows int
	changed                          []string
}

// pinTimes gives every artifact's file one modification time, so two homes
// written a moment apart give their rows the same file times.
func pinTimes(t *testing.T, artifacts []*input.Artifact) {
	t.Helper()
	pinned := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, artifact := range artifacts {
		if err := os.Chtimes(artifact.DiskPath, pinned, pinned); err != nil {
			t.Fatal(err)
		}
	}
}

// cmpAgent returns agent, or fallback when it is empty.
func cmpAgent(agent, fallback string) string {
	if agent == "" {
		return fallback
	}
	return agent
}

// plainRows parses files, a case's files at one size as written, and
// returns the texts of the rows other than repeat rows.
func plainRows(t *testing.T, name string, test growthCase, files []growthFile) []string {
	t.Helper()
	written, _ := growthHome(t, cmpAgent(test.agent, newParser(t, name).Agent()), files)
	pinTimes(t, written)
	var plain []string
	for _, row := range rowsOf(t, newParser(t, name), written) {
		if !isRepeatRow(row) {
			plain = append(plain, rowText(row))
		}
	}
	return plain
}

// firstDifference returns the fields that differ in the first row where
// the two lists differ, each shortened.
func firstDifference(plain, repeated []string) []string {
	for i := range max(len(plain), len(repeated)) {
		var left, right string
		if i < len(plain) {
			left = plain[i]
		}
		if i < len(repeated) {
			right = repeated[i]
		}
		if left == right {
			continue
		}
		leftFields, rightFields := strings.Split(left, "\n"), strings.Split(right, "\n")
		var shown []string
		for k := range max(len(leftFields), len(rightFields)) {
			var one, other string
			if k < len(leftFields) {
				one = leftFields[k]
			}
			if k < len(rightFields) {
				other = rightFields[k]
			}
			if one != other {
				shown = append(shown, fmt.Sprintf("row %d field %d: %.200q / %.200q", i, k, one, other))
			}
		}
		return shown
	}
	return nil
}

// parseRepeated parses files, a case's files at one size, with their first
// members given copies more times, and compares the rows with plain, the
// rows of the files as written (plainRows).
func parseRepeated(t *testing.T, name string, test growthCase, files []growthFile, plain []string,
	copies int) repeatOutcome {
	t.Helper()
	outcome := repeatOutcome{files: len(files)}
	repeated := make([]growthFile, len(files))
	for i, file := range files {
		data, changed := repeatFirstMembers(file.data, copies)
		if changed {
			outcome.repeatedFiles++
		}
		repeated[i] = growthFile{rel: file.rel, data: data}
	}
	again, _ := growthHome(t, cmpAgent(test.agent, newParser(t, name).Agent()), repeated)
	pinTimes(t, again)
	var withRepeats []string
	for _, row := range rowsOf(t, newParser(t, name), again) {
		if isRepeatRow(row) {
			outcome.repeatRows++
			continue
		}
		withRepeats = append(withRepeats, rowText(row))
	}
	if !slices.Equal(plain, withRepeats) {
		outcome.changed = firstDifference(plain, withRepeats)
	}
	return outcome
}

// checkRepeatOutcomes holds the outcomes of one kind of repeat at the
// smaller and the larger size to checkRepeats's rule.
func checkRepeatOutcomes(t *testing.T, small, large repeatOutcome) {
	t.Helper()
	for _, outcome := range []repeatOutcome{small, large} {
		if outcome.changed != nil {
			t.Errorf("repeating a member changed a row:\n%s", strings.Join(outcome.changed, "\n"))
		}
		if outcome.repeatRows > 2*outcome.files {
			t.Errorf("%d files gave %d repeat rows", outcome.files, outcome.repeatRows)
		}
	}
	if small.files == large.files && small.repeatRows != large.repeatRows {
		t.Errorf("%d files gave %d repeat rows at the smaller size and %d at the larger", small.files,
			small.repeatRows, large.repeatRows)
	}
	if large.repeatedFiles > 0 && large.repeatRows == 0 {
		t.Errorf("%d files with repeats gave no repeat row", large.repeatedFiles)
	}
}

// checkRepeats parses the case's files at two sizes as written and with
// their first members repeated, once (every record then repeats a name)
// and as many times as the size (one name repeats many times): the repeat
// changes no other row, gives the repeat rows of a file (at most one per
// reader, two at most) at either size, and as many at both when the case
// has as many files, so no row is written per record or per repeat. The
// files as written are parsed once per size, for both kinds of repeat.
func checkRepeats(t *testing.T, name string, test growthCase) {
	outcomes := map[int][]repeatOutcome{}
	for _, n := range []int{4, 8} {
		files := test.files(t, n)
		plain := plainRows(t, name, test, files)
		for _, copies := range []int{1, 0} {
			outcome := parseRepeated(t, name, test, files, plain, cmp.Or(copies, n))
			outcomes[copies] = append(outcomes[copies], outcome)
		}
	}
	for _, copies := range []int{1, 0} {
		checkRepeatOutcomes(t, outcomes[copies][0], outcomes[copies][1])
	}
}

func TestEveryParserReportsARepeatedMemberNameOncePerFile(t *testing.T) {
	for _, set := range growthSets() {
		for _, test := range set.cases {
			if test.shape == shapeUnlisted {
				continue
			}
			t.Run(set.parser+"/"+test.kind+"/"+test.shape, func(t *testing.T) {
				t.Parallel()
				checkRepeats(t, set.parser, test)
			})
		}
	}
}
