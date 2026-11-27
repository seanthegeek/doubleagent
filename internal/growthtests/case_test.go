package growthtests

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The case test holds every parser to reading a home whatever the ASCII case
// of its files' paths: each growth case's files are written once as the case
// spells them and once with every ASCII letter of every path raised, and the
// two parses give as many rows, each equal to its partner once ASCII case is
// ignored, and none names a file, a session or a project half as the disk
// spells it; a value wholly in the parser's spelling, or one the files'
// contents also hold, it cannot tell from a right one. A case's keptCase
// marks the parts of its paths the second home keeps as written: a name a
// parser joins with a name the evidence holds. A set's fixedNames are the
// names the parser opens by its own spelling.

// caseSize is the size the case test builds each case at, the smallest at
// which a case of many files has more than one.
const caseSize = 2

// respelled returns files with every ASCII letter of each path raised but
// in the parts kept: each component named in fixed, and the groups of each
// match of each expression of kept. It fails the test when two paths would
// then be one.
func respelled(t *testing.T, files []growthFile, fixed []string, kept ...string) []growthFile {
	t.Helper()
	var keep []*regexp.Regexp
	for _, expression := range kept {
		if expression != "" {
			keep = append(keep, regexp.MustCompile(expression))
		}
	}
	raised := make([]growthFile, len(files))
	seen := map[string]string{}
	for i, file := range files {
		components := strings.Split(file.rel, "/")
		for k, component := range components {
			if !slices.Contains(fixed, component) {
				components[k] = textlines.UpperASCII(component)
			}
		}
		spelled := []byte(strings.Join(components, "/"))
		for _, expression := range keep {
			for _, match := range expression.FindAllStringSubmatchIndex(file.rel, -1) {
				for group := 2; group < len(match); group += 2 {
					if match[group] >= 0 {
						copy(spelled[match[group]:match[group+1]], file.rel[match[group]:match[group+1]])
					}
				}
			}
		}
		rel := string(spelled)
		if other, twice := seen[rel]; twice {
			t.Fatalf("%s and %s are both %s once raised", other, file.rel, rel)
		}
		seen[rel] = file.rel
		raised[i] = growthFile{rel: rel, data: file.data}
	}
	return raised
}

// caseHome writes files in a new home, gives every file one modification
// time, and returns the artifacts in order: by path with the case's first
// file ahead, or, when partners is set, in the order of the partner of
// each file (the file of the same place in partners' files).
func caseHome(t *testing.T, agent string, test growthCase, files []growthFile,
	partners []*input.Artifact, partnerFiles []growthFile) []*input.Artifact {
	t.Helper()
	artifacts, _ := growthHome(t, agent, files)
	pinTimes(t, artifacts)
	if partners == nil {
		return putFirst(artifacts, test.first)
	}
	position := map[string]int{}
	for i, artifact := range partners {
		position[artifact.Rel] = i
	}
	place := map[string]int{}
	for i, file := range files {
		place[file.rel] = position[partnerFiles[i].rel]
	}
	slices.SortStableFunc(artifacts, func(left, right *input.Artifact) int {
		return place[left.Rel] - place[right.Rel]
	})
	return artifacts
}

// foldedRow returns every string of a row and its source line, one per
// line, with ASCII letters lowered.
func foldedRow(row timeline.Row) string {
	return textlines.LowerASCII(strings.Join(append(row.Strings(), fmt.Sprint(row.SourceLine)), "\n"))
}

// checkCase parses the case's files as written and raised (respelled) and
// fails for rows that differ beyond the case of their letters.
func checkCase(t *testing.T, set growthSet, test growthCase) {
	t.Helper()
	name := set.parser
	first := newParser(t, name)
	agent := cmpAgent(test.agent, first.Agent())
	files := test.files(t, caseSize)
	plainHome := caseHome(t, agent, test, files, nil, nil)
	raised := respelled(t, files, set.fixedNames, set.keptCase, test.keptCase)
	raisedHome := caseHome(t, agent, test, raised, plainHome, files)
	plain, again := homeRows(t, first, plainHome, false), homeRows(t, newParser(t, name), raisedHome, false)
	var plainText, againText []string
	for i := range plain {
		plainText = append(plainText, foldedRow(plain[i]))
	}
	for i := range again {
		againText = append(againText, foldedRow(again[i]))
	}
	if len(plain) != len(again) {
		t.Errorf("%s: %d rows as written, %d raised", test.kind+"/"+test.shape, len(plain), len(again))
	}
	if difference := firstDifference(plainText, againText); difference != nil {
		t.Errorf("%s: raised paths change a row:\n%s", test.kind+"/"+test.shape, strings.Join(difference, "\n"))
	}
	reportMixed(t, test, plain, again, pathRuns(files, raised), cutValues(again, raised))
}

// cutValues returns a line for each session id or project path of a raised
// home's row that a file's original path holds once ASCII case is ignored,
// between two bytes that are not letters or digits, but none holds as
// spelled, unless a file's contents hold it too and so may have given it: a
// value cut from the path and then respelled, which its partner row, of
// the same text, cannot show.
func cutValues(rows []timeline.Row, files []growthFile) []string {
	var originals, contents strings.Builder
	for _, file := range files {
		originals.WriteString("\n/home/u/" + file.rel + "\n")
		contents.WriteString(textlines.LowerASCII(string(file.data)) + "\n")
	}
	spelled, held := originals.String(), contents.String()
	var found []string
	for i, row := range rows {
		for _, value := range []string{row.SessionID, row.ProjectPath} {
			sought := textlines.LowerASCII(value)
			if value != "" && boundedIn(textlines.LowerASCII(spelled), sought) && !strings.Contains(spelled, value) &&
				!strings.Contains(held, sought) {
				found = append(found, fmt.Sprintf("row %d holds %q, which the paths spell otherwise", i, value))
			}
		}
	}
	return found
}

// boundedIn reports whether text holds sought with a byte that is no ASCII
// letter or digit on each side, as a value cut from a path is.
func boundedIn(text, sought string) bool {
	for offset := 0; ; {
		place := strings.Index(text[offset:], sought)
		if place < 0 {
			return false
		}
		place += offset
		end := place + len(sought)
		if place > 0 && end < len(text) && !isWordByte(text[place-1]) && !isWordByte(text[end]) {
			return true
		}
		offset = place + 1
	}
}

// isWordByte reports whether char is an ASCII letter or digit.
func isWordByte(char byte) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || '0' <= char && char <= '9'
}

// reportMixed fails the case for each value of the raised home's rows that
// is cut from a path (found, from cutValues) or names a file (mixedSpellings)
// in a spelling other than the disk's.
func reportMixed(t *testing.T, test growthCase, plain, again []timeline.Row, runs []pathRun, found []string) {
	t.Helper()
	for i := range min(len(plain), len(again)) {
		for _, mixed := range mixedSpellings(plain[i], again[i], runs) {
			found = append(found, fmt.Sprintf("row %d %s", i, mixed))
		}
	}
	for _, line := range found {
		t.Errorf("%s: %s", test.kind+"/"+test.shape, line)
	}
}

// pathRun is the tail of a file's path, its last components, as the case
// writes it and as the raised home spells it.
type pathRun struct {
	written, raised string
}

// pathRuns returns each tail of whole components of each file's path, from
// its name alone to the whole path, that the raised home spells otherwise,
// once each. Only tails are sought: a row names its file by one, and a
// run inside a path is also a part of a path the agent spells itself, such
// as a container path built from a project's name.
func pathRuns(files, raised []growthFile) []pathRun {
	seen := map[pathRun]bool{}
	var runs []pathRun
	for i, file := range files {
		written, spelled := strings.Split(file.rel, "/"), strings.Split(raised[i].rel, "/")
		for start := range written {
			run := pathRun{strings.Join(written[start:], "/"), strings.Join(spelled[start:], "/")}
			if run.written != run.raised && !seen[run] {
				seen[run] = true
				runs = append(runs, run)
			}
		}
	}
	return runs
}

// mixedSpellings returns a line for each string of the raised home's row
// that holds a tail of a path, ignoring case, that differs from the same
// place of the partner row and yet is not the raised spelling: a name joined
// from the disk's spelling of one part of a path and the parser's of
// another. A place equal in both rows is text of the evidence, a kept part
// or a label wholly in the parser's spelling, none of which it can tell apart.
func mixedSpellings(plain, raised timeline.Row, runs []pathRun) []string {
	plainFields, raisedFields := plain.Strings(), raised.Strings()
	var found []string
fields:
	for k := range min(len(plainFields), len(raisedFields)) {
		before, after := plainFields[k], raisedFields[k]
		if before == after || len(before) != len(after) {
			continue
		}
		lowered := textlines.LowerASCII(after)
		for _, run := range runs {
			sought := textlines.LowerASCII(run.raised)
			for offset := 0; ; {
				place := strings.Index(lowered[offset:], sought)
				if place < 0 {
					break
				}
				place += offset
				held := after[place : place+len(sought)]
				if held != run.raised && held != before[place:place+len(sought)] {
					found = append(found, fmt.Sprintf("field %d holds %q where the disk spells %q: %.200q", k, held,
						run.raised, after))
					continue fields
				}
				offset = place + 1
			}
		}
	}
	return found
}

// Every parser reads each growth case's home with its paths raised as it
// reads the home as written, but for the parts the case keeps.
func TestEveryParserReadsAHomeSpelledInAnotherCase(t *testing.T) {
	for _, set := range growthSets() {
		t.Run(set.parser, func(t *testing.T) {
			t.Parallel()
			for _, test := range set.cases {
				checkCase(t, set, test)
			}
		})
	}
}
