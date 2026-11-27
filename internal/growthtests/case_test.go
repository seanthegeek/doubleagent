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

// The case test holds every parser to reading a home whatever the ASCII
// case of its files' paths, as collect and the catalog match them: each
// growth case's files are written once as the case spells them and once
// with every ASCII letter of every path raised, and the two parses give as
// many rows, each equal to its partner once ASCII case is ignored. A
// case's keptCase marks the parts of its paths the second home keeps as
// written: a name a parser joins with a name the evidence holds, which a
// home spelled in another case does not change. A set's fixedNames are
// the names the parser opens by its own spelling, which only a disk that
// ignores case finds in another.

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
