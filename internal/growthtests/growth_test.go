package growthtests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The growth test holds every parser, for every kind of file it lists
// (parserkit.KindLister), to output and work that grow in step with its
// input: each case builds a home at a size n and at 2n in one of the
// shapes below and parses it, and what the larger home costs must stay
// within growthLimit times what the smaller one does. The measures are
// counts, never times: the rows, the bytes of every string of every row,
// the memory allocations and the bytes allocated (runtime.MemStats), so
// the result does not depend on the machine. A parse that reads a file
// again for every record, or writes a value again into every row,
// allocates or writes in proportion to the square of the input and fails.

// The shapes of input a case builds.
const (
	shapeLargeShared = "many records naming one large value"
	shapeSharedID    = "many records sharing one id"
	shapeCarried     = "one long value carried by many short records"
	shapeNested      = "deep nesting and many damaged siblings"
	shapeManyFiles   = "many small files that each look at the others"
	shapeJoins       = "equal join keys and a large text column joined many times"
	shapeUnlisted    = "many directories that cannot be listed"
)

// growthLimit is the most any measure at 2n may be, as a multiple of the
// same measure at n. Linear work doubles; work that also has a fixed part
// grows by less; work that grows with the square of the input grows by
// about 4. 2.5 leaves room for the steps of slices and maps that grow by
// doubling, and for work of n log n (a sort), which grows by 2.2 at the
// sizes the cases use.
const growthLimit = 2.5

// outputPerInput and rowAllowance bound the bytes of row text (every
// string of a row but the four that name where it came from: host, user,
// agent and source file) a parse may write, at either size: outputPerInput
// per byte of input, and rowAllowance more per row. The allowance covers
// the fixed wording a row writes about its file ("(from <rel>, file
// mtime)") and up to four values carried from elsewhere in the file, each
// cut to parserkit.CarriedLimit as the kit cuts them: a row may name its
// project, session, model and branch. A case's long value (longValue) is
// longer than the allowance, so a parse that writes it whole into each of
// n rows breaks the bound by far.
const (
	outputPerInput = 4
	rowAllowance   = 4*parserkit.CarriedLimit + 512
)

// defaultGrowthSize is the smaller size of a case that names none: large
// enough that work growing with the square of n outweighs any fixed cost.
const defaultGrowthSize = 64

// growthFile is one file a case writes, by its path relative to the home.
type growthFile struct {
	rel  string
	data []byte
}

// growthCase is one shape of input for one kind of file a parser reads:
// files builds the home's files at size n. agent, when set, is the agent
// the artifacts belong to, for a parser that reads another agent's files.
// known, when set, records a defect the case finds on the current code:
// the measure that grows, its ratio and the site, until a fix removes it.
// unlisted, when set, names the directories of the home (relative to it)
// that the case takes every permission from once its files are written,
// so they cannot be listed (shapeUnlisted). first, when set, is the path
// of the file parsed before the others, which otherwise go in the order of
// their paths, as a collection may list them.
type growthCase struct {
	kind, shape string
	n           int
	agent       string
	known       string
	first       string
	files       func(t *testing.T, n int) []growthFile
	unlisted    func(n int) []string
}

// growthSet is one parser's growth cases, and the kinds it reads that
// need none, each with the reason.
type growthSet struct {
	parser string
	cases  []growthCase
	exempt map[string]string
}

// size returns the case's smaller size.
func (c growthCase) size() int {
	if c.n == 0 {
		return defaultGrowthSize
	}
	return c.n
}

// growthMeasure is what one parse of a home cost.
type growthMeasure struct {
	rows, rowBytes, textBytes, inputBytes int
	allocations, allocatedBytes           uint64
}

// measures returns the measures that must grow by growthLimit at most, by
// name: the allocations only where they were counted, without the race
// detector.
func (m growthMeasure) measures() map[string]float64 {
	measures := map[string]float64{"rows": float64(m.rows), "row bytes": float64(m.rowBytes)}
	if !raceDetector {
		measures["allocations"] = float64(m.allocations)
		measures["allocated bytes"] = float64(m.allocatedBytes)
	}
	return measures
}

// growthHome writes the files in a new home and returns its artifacts, in
// the order of their paths, and the bytes written.
func growthHome(t *testing.T, agent string, files []growthFile) ([]*input.Artifact, int) {
	t.Helper()
	home := t.TempDir()
	owner := &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}
	var artifacts []*input.Artifact
	written := 0
	for _, file := range files {
		path := filepath.Join(home, filepath.FromSlash(file.rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, file.data, 0o600); err != nil {
			t.Fatal(err)
		}
		written += len(file.data)
		artifacts = append(artifacts, &input.Artifact{DiskPath: path, Original: "/home/u/" + file.rel,
			Rel: file.rel, Agent: agent, Home: owner, Manifest: jsonvalue.NewNull()})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Rel < artifacts[j].Rel })
	return artifacts, written
}

// newParser returns a new value of the registered parser called name.
func newParser(t *testing.T, name string) parserkit.Parser {
	t.Helper()
	for _, each := range parsers.All() {
		if each.Name() == name {
			return each
		}
	}
	t.Fatalf("no parser is called %s", name)
	return nil
}

// parseHome parses the artifacts with the parser as a run does
// (Registry.Collect), settles the parent each group of a repeated id goes
// with (timeline.Attach), its first or, with later set, its last, as a
// filter that keeps only that parent's rows would choose, writes out the
// shared texts the rows name as a run writes its output
// (timeline.Resolve), and returns the rows' measures; a problem line
// counts as a row. Every row is kept, the secondary rows after the primary
// ones, so a text the run's deduplication would drop still counts.
func parseHome(t *testing.T, each parserkit.Parser, artifacts []*input.Artifact, later bool) growthMeasure {
	t.Helper()
	result, err := parsers.NewOf([]parserkit.Parser{each}).Collect(t.Context(), &input.Collection{Artifacts: artifacts},
		parserkit.Options{IncludeThinking: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var measure growthMeasure
	identity := 0
	add := func(row timeline.Row) {
		measure.rows++
		for _, field := range row.Strings() {
			measure.rowBytes += len(field)
		}
		identity += len(row.Host) + len(row.User) + len(row.Agent) + len(row.SourceFile)
	}
	written := slices.Clone(result.Primary)
	for _, row := range result.Secondary {
		written = append(written, row.Row)
	}
	var choice timeline.Choice
	if later {
		choice = timeline.LastParents(written)
	}
	written = timeline.Attach(written, choice)
	timeline.Resolve(written)
	for _, row := range written {
		add(row)
	}
	for _, line := range result.Problems {
		add(timeline.Row{Text: line})
	}
	measure.textBytes = measure.rowBytes - identity
	return measure
}

// measureGrowth builds the case's home at size n and parses it with a new
// parser value, each group going with its last parent when later is set
// (parseHome), counting the allocations of the parse alone. Under the race
// detector the cases run in parallel and the allocations are not counted,
// since the counts are of the whole process.
func measureGrowth(t *testing.T, name string, test growthCase, n int, later bool) growthMeasure {
	t.Helper()
	each := newParser(t, name)
	agent := test.agent
	if agent == "" {
		agent = each.Agent()
	}
	artifacts, written := growthHome(t, agent, test.files(t, n))
	if first := slices.IndexFunc(artifacts, func(artifact *input.Artifact) bool {
		return test.first != "" && artifact.Rel == test.first
	}); first > 0 {
		artifacts = slices.Concat(artifacts[first:first+1], artifacts[:first], artifacts[first+1:])
	}
	if test.unlisted != nil {
		unlist(t, artifacts[0].Home.DiskPath, test.unlisted(n))
	}
	if !slices.ContainsFunc(artifacts, func(artifact *input.Artifact) bool {
		return kindWants(each, test.kind, artifact)
	}) {
		t.Fatalf("no file of the case is of the kind %q", test.kind)
	}
	if raceDetector {
		measure := parseHome(t, each, artifacts, later)
		measure.inputBytes = written
		return measure
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	measure := parseHome(t, each, artifacts, later)
	runtime.ReadMemStats(&after)
	measure.inputBytes = written
	measure.allocations = after.Mallocs - before.Mallocs
	measure.allocatedBytes = after.TotalAlloc - before.TotalAlloc
	return measure
}

// unlist takes every permission from the directories of home at rels, so
// that they cannot be listed, and gives them back when the test ends. Where
// the first can still be listed (a user whom permissions do not bind, or a
// system that keeps no such permission on a directory) the test is
// skipped: it is decided by what the directory does, not by the platform.
func unlist(t *testing.T, home string, rels []string) {
	t.Helper()
	for _, rel := range rels {
		directory := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.Chmod(directory, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
	}
	if _, err := osfile.ReadDir(filepath.Join(home, filepath.FromSlash(rels[0]))); err == nil {
		t.Skip("a directory without permissions can still be listed here")
	}
}

// kindWants reports whether the parser's kind called name wants the
// artifact.
func kindWants(each parserkit.Parser, name string, artifact *input.Artifact) bool {
	for _, kind := range parserkit.KindsOf(each) {
		if kind.Name == name {
			return kind.Wants(artifact)
		}
	}
	return false
}

// growthSlack is how far each measure at 2n may pass growthLimit times its
// value at n before it counts as growth: none for the rows and their bytes,
// which are exact, and for the allocations what the runtime's own work
// (garbage collection, timers, other goroutines) adds to the counts of the
// process during a small parse, tens of kilobytes. Work that grows with the square
// of the input passes the slack by far at the cases' sizes.
func growthSlack() map[string]float64 {
	return map[string]float64{"rows": 0, "row bytes": 0, "allocations": 2048, "allocated bytes": 256 << 10}
}

// growthFailures returns what breaks the bounds between the measures at n
// and at 2n, one line each.
func growthFailures(small, large growthMeasure) []string {
	var failures []string
	smallMeasures, largeMeasures, slack := small.measures(), large.measures(), growthSlack()
	for _, name := range []string{"rows", "row bytes", "allocations", "allocated bytes"} {
		if _, counted := smallMeasures[name]; !counted {
			continue
		}
		if largeMeasures[name] > growthLimit*smallMeasures[name]+slack[name] {
			failures = append(failures, fmt.Sprintf("%s grew %.2f times (%.0f to %.0f)", name,
				largeMeasures[name]/max(smallMeasures[name], 1), smallMeasures[name], largeMeasures[name]))
		}
	}
	for _, each := range []growthMeasure{small, large} {
		if each.textBytes > outputPerInput*each.inputBytes+rowAllowance*each.rows {
			failures = append(failures, fmt.Sprintf("%d bytes of row text from %d bytes of input", each.textBytes,
				each.inputBytes))
		}
	}
	return failures
}

// Every parser, for every kind of file it reads, writes rows and does work
// that grow in step with its input in every shape its cases build, read as
// a run writes them: each group of a repeated id going with its first
// parent, as without a filter, and with its last, so its members take
// another parent's fields. A case marked known is a defect not yet fixed:
// it must still break a bound where every measure is counted (without the
// race detector), so its fix also removes the mark. Work that allocates
// nothing is not measured: a parser's own tests bound it through counters
// (parserkit.Work). Under the race detector only rows and their bytes count.
func TestEveryParserGrowsInStepWithItsInput(t *testing.T) {
	for _, set := range growthSets() {
		for _, test := range set.cases {
			t.Run(set.parser+"/"+test.kind+"/"+test.shape, func(t *testing.T) {
				if raceDetector {
					t.Parallel()
				}
				n := test.size()
				var failures []string
				for _, later := range []bool{false, true} {
					small, large := measureGrowth(t, set.parser, test, n, later),
						measureGrowth(t, set.parser, test, 2*n, later)
					for _, failure := range growthFailures(small, large) {
						failures = append(failures, choiceName(later)+": "+failure)
					}
					t.Logf("%s, n=%d: %+v; 2n: %+v", choiceName(later), n, small, large)
				}
				reportGrowth(t, test, n, failures)
			})
		}
	}
}

// choiceName names the parent the groups go with.
func choiceName(later bool) string {
	if later {
		return "each group with its last parent"
	}
	return "each group with its first parent"
}

// reportGrowth fails the case for the failures at n and 2n, or for a known
// growth no longer seen.
func reportGrowth(t *testing.T, test growthCase, n int, failures []string) {
	t.Helper()
	known := test.known
	switch {
	case known != "" && len(failures) == 0 && raceDetector:
		t.Logf("known growth (%s) not seen: allocations are not counted under the race detector", known)
	case known != "" && len(failures) == 0:
		t.Errorf("the known growth (%s) is gone: remove the mark", known)
	case known != "":
		t.Logf("known growth (%s): %s", known, strings.Join(failures, "; "))
	case len(failures) > 0:
		t.Errorf("at %d and %d units: %s", n, 2*n, strings.Join(failures, "; "))
	}
}

// Every registered parser lists its kinds of file and has a growth set,
// and every kind has at least one growth case or a reason it needs none;
// every case and every exemption names a kind the parser lists, so a
// parser or kind added without its growth cases fails here.
func TestEveryFileKindHasGrowthCases(t *testing.T) {
	sets := map[string]growthSet{}
	for _, set := range growthSets() {
		if _, twice := sets[set.parser]; twice {
			t.Errorf("two growth sets name %s", set.parser)
		}
		sets[set.parser] = set
	}
	for _, each := range parsers.All() {
		set, ok := sets[each.Name()]
		if !ok {
			t.Errorf("%s has no growth set", each.Name())
		}
		delete(sets, each.Name())
		checkKinds(t, each, set)
	}
	for name := range sets {
		t.Errorf("a growth set names %s, which is not registered", name)
	}
}

// checkKinds fails for a kind of the parser that no case builds and no
// exemption names, for a kind listed twice, and for a case or exemption
// naming a kind the parser does not list.
func checkKinds(t *testing.T, each parserkit.Parser, set growthSet) {
	t.Helper()
	kinds := parserkit.KindsOf(each)
	if len(kinds) == 0 {
		t.Errorf("%s lists no kinds of file (parserkit.KindLister)", each.Name())
	}
	covered := map[string]bool{}
	for _, test := range set.cases {
		covered[test.kind] = true
	}
	listed := map[string]bool{}
	for _, kind := range kinds {
		if listed[kind.Name] {
			t.Errorf("%s lists the kind %q twice", each.Name(), kind.Name)
		}
		listed[kind.Name] = true
		if !covered[kind.Name] && set.exempt[kind.Name] == "" {
			t.Errorf("%s: no growth case builds the kind %q", each.Name(), kind.Name)
		}
	}
	for name := range covered {
		if !listed[name] {
			t.Errorf("%s: a growth case names the kind %q, which the parser does not list", each.Name(), name)
		}
	}
	for name := range set.exempt {
		if !listed[name] || covered[name] {
			t.Errorf("%s: the exemption of %q names a kind not listed or one a case builds", each.Name(), name)
		}
	}
}

// Over every file the growth cases write, each parser's Wants agrees with
// its kinds, so every file a parser is given is of a kind a growth case
// builds. Every Wants is the kit's union of the kinds today; this fails for
// one written another way. A neighbour a parser opens itself (Beside,
// Above) is no kind: the case of the file it is beside reaches it.
func TestWantsIsTheUnionOfTheKinds(t *testing.T) {
	for _, set := range growthSets() {
		t.Run(set.parser, func(t *testing.T) {
			t.Parallel()
			all := parsers.All()
			for _, test := range set.cases {
				agent := test.agent
				if agent == "" {
					agent = newParser(t, set.parser).Agent()
				}
				artifacts, _ := growthHome(t, agent, test.files(t, 2))
				for _, artifact := range artifacts {
					for _, other := range all {
						if other.Wants(artifact) != parserkit.WantsAny(parserkit.KindsOf(other), artifact) {
							t.Errorf("%s: Wants and the kinds disagree on %s", other.Name(), artifact.Rel)
						}
					}
				}
			}
		})
	}
}

// growthSets returns every parser's growth set, in registry order.
func growthSets() []growthSet {
	return []growthSet{claudecodeGrowth(), claudedesktopGrowth(), codexGrowth(), antigravityGrowth(),
		qwencodeGrowth(), kiroGrowth(), geminicliGrowth(), crushGrowth(), gooseGrowth(), continuedevGrowth(),
		aiderGrowth(), zedGrowth(), vscodeGrowth(), opencodeGrowth(), kilocodeGrowth(), clineGrowth(),
		roocodeGrowth(), tabbyGrowth(), openhandsGrowth(), shellgptGrowth(), piGrowth(), littlecoderGrowth(),
		lettaGrowth(), hermesGrowth(), agentzeroGrowth(), openinterpreterGrowth(), openclawGrowth(),
		nanobotGrowth(), codyGrowth(), twinnyGrowth(), pearaiGrowth(), musecodeGrowth(), ollamaGrowth(),
		copilotcliGrowth(), copilotdesktopGrowth(), chatgptdesktopGrowth(), cursorGrowth()}
}
