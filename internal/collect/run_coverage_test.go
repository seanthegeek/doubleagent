package collect_test

import (
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"runtime"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
)

// newCoverageJudge returns the judge of the coverage runs, with their rows'
// paths folded as names are compared here.
func newCoverageJudge(t *testing.T, evidence coverageEvidence) coverageJudge {
	t.Helper()
	fold := caseFold(t, evidence.image.Work)
	for _, run := range evidence.runs {
		for index := range run.rows {
			run.rows[index].rel = fold(run.rows[index].rel)
		}
	}
	work := evidence.image.Work
	roots := runRoots(evidence.runs, work)
	manifest := func(name string) imageFixtures {
		entries := append(manifestEntries(t, name), evidence.builders...)
		return imageFixtures{entries: entries, roots: roots, anywhere: true}.folded(textlines.LowerASCII)
	}
	return coverageJudge{runs: evidence.runs, fold: fold,
		here:    imageFixtures{entries: evidence.entries, roots: roots, bases: runBases(evidence.runs, work)}.folded(fold),
		unix:    manifest("imagetest/testdata/image_unix.txt"),
		windows: manifest("imagetest/testdata/image_windows.txt")}
}

// coverageJudge holds what deciding a line needs: the runs, the image they
// collected with the bases they collected from, the two image manifests
// (each with the entries the shared builders add, its names lowered so
// every platform derives the same from them), and the fold the runs' names
// are compared after.
type coverageJudge struct {
	runs          []coverageRun
	here          imageFixtures
	unix, windows imageFixtures
	fold          func(string) string
}

// manifestReason returns the reason the two image manifests give a line
// with its globs lowered: none when neither holds a path it matches,
// unix-only or windows-only when one alone does, else "".
func (judge coverageJudge) manifestReason(inManifests coverageLine) string {
	onUnix, onWindows := inManifests.fixture(judge.unix), inManifests.fixture(judge.windows)
	switch {
	case !onUnix && !onWindows:
		return "none"
	case !onWindows:
		return "unix-only"
	case !onUnix:
		return "windows-only"
	}
	return ""
}

// unreachedReason is the reason of a line this platform's runs do not
// reach the fixture of: <GOOS>-unreached, since Linux and macOS differ
// there (macOS takes two names differing only in case for one).
func unreachedReason() string {
	return runtime.GOOS + "-unreached"
}

// reasons returns the reasons the uncovered list must give a line: the one
// the manifests give (manifestReason), when not "", and this platform's
// unreached when its image or its manifest holds a path the line matches
// and the runs leave no evidence of it. inManifests is the line with its
// globs lowered, for the manifests.
func (judge coverageJudge) reasons(line, inManifests coverageLine) []string {
	var reasons []string
	if manifests := judge.manifestReason(inManifests); manifests != "" {
		reasons = append(reasons, manifests)
	}
	own := judge.unix
	if runtime.GOOS == "windows" {
		own = judge.windows
	}
	if !line.evidence(judge.runs) && (line.fixture(judge.here) || inManifests.fixture(own)) {
		reasons = append(reasons, unreachedReason())
	}
	return reasons
}

// decidedHere reports whether this platform derives a reason: the
// manifests' reasons everywhere, and its own unreached.
func decidedHere(reason string) bool {
	return reason == "none" || reason == "unix-only" || reason == "windows-only" || reason == unreachedReason()
}

// check fails the test when the list's reasons for a line, of those this
// platform decides, are not the derived ones: so a line that loses its
// evidence or its fixture, or gains one, fails until the list says so.
func (judge coverageJudge) check(t *testing.T, derived, listed []string) {
	t.Helper()
	for _, reason := range append(slices.Clone(derived), listed...) {
		if decidedHere(reason) && slices.Contains(derived, reason) != slices.Contains(listed, reason) {
			t.Errorf("%s gives the line the reasons %q, where this platform derives %q of those it decides",
				uncoveredFile, listed, derived)
			return
		}
	}
}

// TestEveryCatalogLineLeavesItsEvidence runs the default, --no-secrets and
// --full runs over the main image, over it with the framework volumes, over
// the MSIX image and over the case-variant root, and needs each table line's
// evidence (coverageLines) unless testdata/coverage-uncovered.txt gives the
// reason this platform derives: none, unix-only or windows-only from the two
// image manifests, or <platform>-unreached for a fixture its runs do not
// reach. It also fails when the default run collects a path of
// forbiddenPaths, or when a run changes the image.
func TestEveryCatalogLineLeavesItsEvidence(t *testing.T) {
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	evidence := collectForCoverage(t)
	checkForbiddenPaths(t, evidence.runs[0], evidence.image.Root, evidence.entries)
	judge := newCoverageJudge(t, evidence)
	inManifests := coverageLines(tables, textlines.LowerASCII)
	lines := coverageLines(tables, judge.fold)
	derived := map[string][]string{}
	for index, line := range lines {
		derived[line.key()] = judge.reasons(line, inManifests[index])
	}
	if updating() {
		writeUncovered(t, derived, readUncovered(t))
	}
	listed := readUncovered(t)
	for _, line := range lines {
		t.Run(line.key(), func(t *testing.T) { judge.check(t, derived[line.key()], listed[line.key()]) })
	}
	for key := range listed {
		if _, found := derived[key]; !found {
			t.Errorf("%s lists %q, which is no table line", uncoveredFile, key)
		}
	}
}
