//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The reasons a recorded parser case is not replayed, as the test log names
// them.
const (
	skipPatched  = "the test patched the analyzer while the call ran"
	skipPosixURI = "a file URI the test built from the recording's POSIX root, which has no Windows form"
	skipDenied   = "its tree denies reading, which this platform does not enforce"
	skipCrash    = "the Python call raised an exception that is not an operating-system error"
	skipExcluded = "an excluding documented difference shows in its rows"
)

// parserReplay is the state of the parser replay across agents: the one
// work tree, whether this platform enforces a mode that denies reading, the
// cases not replayed by reason, and the cases that needed each difference,
// for the log (needed) and across the run (used).
type parserReplay struct {
	corpus       *Corpus
	work         *WorkTree
	deniesAccess bool
	skipped      map[string]int
	needed, used neededTally
	// wrongKind counts, by agent, the parses difference 170 took a row of.
	wrongKind map[string]int
	// withheld counts, by entry, the parses that needed difference 230.
	withheld map[string]int
	// shown counts, by entry, the parses that needed difference 232.
	shown map[string]int
}

func TestReplayEveryRecordedCase(t *testing.T) {
	corpus := Open(t)
	used := neededTally{}
	complete := map[string]bool{}
	replays := []struct {
		name string
		run  func(t *testing.T, corpus *Corpus, used neededTally) bool
	}{
		{"parsers", replayParsers},
		{"commands", replayCommands},
		{"timelines over recorded parser rows", replayTimelinesOverRecordedRows},
	}
	for _, each := range replays {
		t.Run(each.name, func(t *testing.T) { complete[each.name] = each.run(t, corpus, used) })
	}
	whole := true
	for _, each := range replays {
		whole = whole && complete[each.name]
	}
	checkEveryDifferenceUsed(t, Differences(), used, whole)
}

// replayParsers runs every recorded parser case, noting in used the
// differences each needed, and reports whether every agent's cases ran.
func replayParsers(t *testing.T, corpus *Corpus, used neededTally) bool {
	started := time.Now()
	replay := &parserReplay{corpus: corpus, work: corpus.NewWorkTree(t),
		deniesAccess: deniesReading(t), skipped: map[string]int{}, needed: neededTally{}, used: used,
		wrongKind: map[string]int{}, withheld: map[string]int{}, shown: map[string]int{}}
	registered := parsers.ByAgent(parsers.All())
	agents := caseFiles(t, corpus, "parsers")
	missing := checkEveryAgentHasAParser(t, agents, parsers.All())
	replayed, ran := 0, 0
	for _, agent := range agents {
		if slices.Contains(missing, agent) {
			t.Errorf("%s: %d recorded cases not replayed", agent, countCases(t, corpus, agent))
			ran++
			continue
		}
		if len(registered[agent]) > 1 {
			t.Fatalf("%d parsers are registered for %s; a case names its parser by agent",
				len(registered[agent]), agent)
		}
		t.Run(agent, func(t *testing.T) {
			ran++
			replayed += replay.agent(t, agent)
		})
	}
	t.Logf("%d parser cases replayed in %v", replayed, time.Since(started).Round(time.Millisecond))
	for _, reason := range slices.Sorted(maps.Keys(replay.skipped)) {
		t.Logf("not replayed, %s: %d cases", reason, replay.skipped[reason])
	}
	logNeeded(t, "information comparison", replay.needed, rowFormatNumbers())
	checkEveryWrongKindReportUsed(t, replay.wrongKind, ran == len(agents))
	checkEveryWithheldFormUsed(t, parserScope, replay.withheld, ran == len(agents))
	checkEveryShownFormUsed(t, replay.shown, ran == len(agents))
	return ran == len(agents)
}

// checkEveryDifferenceUsed fails the test for each difference of table,
// retired ones aside, that no case of the run used, so a rule that loosens
// the comparison stays only while a recording needs it. It checks only a
// whole run: a run filtered to some cases, or with a replay this platform
// skips, cannot tell an unused difference from one its cases left out.
func checkEveryDifferenceUsed(t testing.TB, table []Difference, used neededTally, whole bool) {
	t.Helper()
	if !whole {
		t.Log("not checked whether every difference is used: the run did not replay every case")
		return
	}
	for _, entry := range table {
		if entry.Retired != "" || entry.Unchecked != "" {
			continue
		}
		if len(used[entry.Number]) == 0 {
			t.Errorf("difference %d (%s) is used by no recorded case: retire it", entry.Number, entry.Summary)
			continue
		}
		t.Logf("difference %d used by %d cases", entry.Number, len(used[entry.Number]))
	}
}

// countCases returns the number of case lines in an agent's case file,
// without decoding them.
func countCases(t *testing.T, corpus *Corpus, agent string) int {
	t.Helper()
	data, err := osfile.ReadFile(corpus.caseFile(t, "parsers/"+agent))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(data, []byte("\n"))
}

// deniesReading reports whether this platform keeps the owner of a file
// whose mode is 000 from reading it: Windows does not, nor does a run as
// the superuser, and a case that relies on the denial is then not replayed.
func deniesReading(t *testing.T) bool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "denied")
	if err := osfile.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	file, err := osfile.Open(path)
	if err != nil {
		return true
	}
	_ = file.Close()
	return false
}

// agent replays every case of one agent and returns how many it compared.
// Each recorded parser object gets one new Go parser value, which sees that
// object's calls in their order, so a cache a parser keeps works as it did.
func (r *parserReplay) agent(t *testing.T, agent string) int {
	cases := r.corpus.ParserCases(t, agent)
	var order []string
	instances := map[string][]ParserCase{}
	for _, recorded := range cases {
		if _, ok := instances[recorded.Instance]; !ok {
			order = append(order, recorded.Instance)
		}
		instances[recorded.Instance] = append(instances[recorded.Instance], recorded)
	}
	compared := 0
	for _, instance := range order {
		calls := instances[instance]
		sort.SliceStable(calls, func(i, j int) bool { return calls[i].Call < calls[j].Call })
		subject := parsers.ByAgent(parsers.All())[agent][0]
		for _, recorded := range calls {
			if skip := r.replayCase(t, subject, recorded); skip != "" {
				r.skipped[skip]++
				continue
			}
			compared++
		}
	}
	t.Logf("%s: %d of %d cases compared", agent, compared, len(cases))
	return compared
}

// replayCase runs one recorded call on the Go parser and compares the
// result, or returns why the case is not replayed.
func (r *parserReplay) replayCase(t *testing.T, subject parser.Parser, recorded ParserCase) string {
	t.Helper()
	if len(recorded.Patched) > 0 {
		return skipPatched
	}
	if filepath.Separator == '\\' && posixRootURI(recorded.Node) {
		return skipPosixURI
	}
	if recorded.Tree != "" {
		denied, err := r.work.DeniesReading(recorded.Tree)
		if err != nil {
			t.Fatal(err)
		}
		if denied && !r.deniesAccess {
			return skipDenied
		}
		r.work.Switch(t, recorded.Tree, recorded.Times)
	}
	artifact := r.corpus.inputArtifact(t, recorded.Artifact, r.work.Root)
	node := fmt.Sprintf("%s (seq %d, %s call %d)", recorded.Node, recorded.Seq, recorded.Method,
		recorded.Call)
	if recorded.Method == "parse" {
		return r.compareParse(t, node, subject, artifact, recorded)
	}
	if recorded.Result.Error != nil {
		return skipCrash
	}
	var got any
	switch recorded.Method {
	case "wants":
		return r.compareWants(t, node, subject, artifact, recorded)
	case "secondary":
		got = parser.IsSecondary(subject, artifact)
	case "secondary_tolerance":
		got = parser.Tolerance(subject, artifact)
	default:
		t.Fatalf("%s: unknown method %q", node, recorded.Method)
	}
	var want any
	if err := json.Unmarshal(recorded.Result.Value, &want); err != nil {
		t.Fatalf("%s: %v", node, err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: %s gave %v, want %v", node, recorded.Method, got, want)
	}
	return ""
}

// compareWants runs one wants call and compares its answer with the
// recorded one, under the differences whose Wanted rule names the file.
func (r *parserReplay) compareWants(t *testing.T, node string, subject parser.Parser, artifact *input.Artifact,
	recorded ParserCase) string {
	t.Helper()
	var want bool
	if err := json.Unmarshal(recorded.Result.Value, &want); err != nil {
		t.Fatalf("%s: %v", node, err)
	}
	got := subject.Wants(artifact)
	agree, numbers := wantsAgree(subject.Agent(), artifact.Rel, got, want)
	if !agree {
		t.Errorf("%s: wants gave %v, want %v", node, got, want)
	}
	r.note(node, numbers)
	return ""
}

// compareParse runs one parse and compares its rows with the recorded
// ones as information, after the documented differences they show are
// normalised. A parse
// the recording did not run to its end is compared over the rows it gave.
func (r *parserReplay) compareParse(t *testing.T, node string, subject parser.Parser,
	artifact *input.Artifact, recorded ParserCase) string {
	t.Helper()
	result := recorded.Result
	if result.Error != nil && !osErrorTypes()[string(result.Error.Type)] {
		return skipCrash
	}
	var options parser.Options
	if recorded.Options != nil {
		options.IncludeThinking = recorded.Options.IncludeThinking
	}
	var got []Row
	err := subject.Parse(artifact, options, func(row timeline.Row) { got = append(got, replayRow(row)) })
	want := make([]Row, len(result.Rows))
	for i, row := range result.Rows {
		want[i] = r.corpus.expandRow(row, r.work.Root)
	}
	switch {
	case result.Error != nil && err == nil:
		t.Errorf("%s: Python raised %s (%s), the parser returned no error", node,
			result.Error.Type, result.Error.Text)
	case result.Error == nil && err != nil:
		t.Errorf("%s: the parser returned %v", node, err)
	}
	if !result.Completed {
		got = got[:min(len(got), len(want))]
	}
	numbers, excluding := rowDifferences(want)
	if excluding != 0 {
		r.note(node, []int{excluding})
		return skipExcluded
	}
	parse := parsedFile{subject.Agent(), artifact.Original, artifact.Rel}
	cut := r.corpus.cutTimesOf(recorded.Tree, recorded.Times)
	compare := func(reporter testing.TB, numbers []int) ([]int, []string) {
		normalWant, normalGot := NormaliseRows(reporter, node, numbers, want, got)
		normalGot, added, entries := addedRows(parse, normalWant, normalGot)
		return slices.Concat(added, compareRowInformation(reporter, node, normalWant, normalGot, cut)), entries
	}
	reporter := &recordingTB{TB: t}
	formats, entries := compare(reporter, numbers)
	for _, message := range reporter.errors {
		t.Errorf("%s", message)
	}
	for _, entry := range entries {
		if isWithheldRowLabel(entry) {
			r.withheld[entry]++
			continue
		}
		r.wrongKind[entry]++
	}
	if len(reporter.errors) == 0 {
		numbers = neededAmong(numbers, func(numbers []int) bool {
			quiet := &recordingTB{TB: t}
			compare(quiet, numbers)
			return len(quiet.errors) == 0
		})
	}
	r.note(node, slices.Concat(numbers, formats))
	if slices.Contains(numbers, withheldDifference().Number) {
		for _, label := range withheldEntriesIn(parserScope, RowsText(want)) {
			r.withheld[label]++
		}
	}
	if slices.Contains(numbers, shownDifference().Number) {
		for _, label := range shownEntriesIn(RowsText(want)) {
			r.shown[label]++
		}
	}
	return ""
}

// neededAmong returns the numbers of a comparison that agrees with all of
// them without which, each left out alone, it no longer agrees: the
// differences the case needs, not only those whose shape it shows.
func neededAmong(numbers []int, agree func(numbers []int) bool) []int {
	var needed []int
	for _, number := range numbers {
		if !agree(slices.DeleteFunc(slices.Clone(numbers), func(each int) bool { return each == number })) {
			needed = append(needed, number)
		}
	}
	return needed
}

// note records the differences a case needed, for the log and the run.
func (r *parserReplay) note(node string, numbers []int) {
	r.needed.note(node, numbers)
	r.used.note(node, numbers)
}

// osErrorTypes returns the qualified names of Python's OSError and the
// subclasses a file read raises: the exceptions the analyzer turns into a
// problem line rather than a crash.
func osErrorTypes() map[string]bool {
	names := map[string]bool{}
	for _, name := range []string{"OSError", "PermissionError", "FileNotFoundError",
		"IsADirectoryError", "NotADirectoryError", "FileExistsError", "InterruptedError",
		"BlockingIOError", "TimeoutError"} {
		names["builtins."+name] = true
	}
	return names
}

// rowDifferences returns the numbers of the documented differences whose
// shape shows in the recorded rows, each looked for, in applying order,
// in what the ones before it left, and the number of one that excludes the
// case from comparison, or 0.
func rowDifferences(want []Row) ([]int, int) {
	var numbers []int
	text := RowsText(want)
	rows := slices.Clone(want)
	for _, entry := range applyingOrder(Differences()) {
		switch {
		case entry.Excludes != nil:
			if entry.Excludes(text) {
				return nil, entry.Number
			}
		case entry.Text != nil:
			if rewriteRowTexts(rows, entry.Text) > 0 {
				numbers = append(numbers, entry.Number)
			}
		}
	}
	slices.Sort(numbers)
	return numbers, 0
}

// replayRow returns a Go row in the replay's row form.
func replayRow(row timeline.Row) Row {
	return Row{TimestampUTC: Text(row.TimestampUTC), Host: Text(row.Host), User: Text(row.User),
		Agent: Text(row.Agent), SessionID: Text(row.SessionID), ProjectPath: Text(row.ProjectPath),
		GitBranch: Text(row.GitBranch), TurnType: Text(row.TurnType), Model: Text(row.Model),
		ToolName: Text(row.ToolName), ToolUseID: Text(row.ToolUseID), Text: Text(row.Text),
		SourceFile: Text(row.SourceFile), SourceLine: row.SourceLine, Recovered: Text(row.Recovered)}
}

// expandRow returns a recorded row with the root token in each field
// replaced by root.
func (c *Corpus) expandRow(row Row, root string) Row {
	expand := func(text Text) Text { return Text(c.Expand(text, root)) }
	return Row{TimestampUTC: expand(row.TimestampUTC), Host: expand(row.Host),
		User: expand(row.User), Agent: expand(row.Agent), SessionID: expand(row.SessionID),
		ProjectPath: expand(row.ProjectPath), GitBranch: expand(row.GitBranch),
		TurnType: expand(row.TurnType), Model: expand(row.Model), ToolName: expand(row.ToolName),
		ToolUseID: expand(row.ToolUseID), Text: expand(row.Text), SourceFile: expand(row.SourceFile),
		SourceLine: row.SourceLine, Recovered: expand(row.Recovered)}
}

// inputArtifact returns the artifact a recorded parser call was given, with
// the root token replaced by root: in the paths the parser opens in this
// platform's separators, in the texts it copies into rows as recorded. Its
// home withholds secrets, as the recording's analyzer always did.
func (c *Corpus) inputArtifact(t *testing.T, recorded Artifact, root string) *input.Artifact {
	t.Helper()
	artifact := &input.Artifact{DiskPath: filepath.FromSlash(c.Expand(recorded.DiskPath, root)),
		Original: c.Expand(recorded.Original, root), Rel: string(recorded.Rel),
		Agent: string(recorded.Agent), Manifest: jsonvalue.NewNull()}
	if manifest := bytes.TrimSpace(recorded.Manifest); len(manifest) > 0 {
		decoded, err := jsonvalue.DecodeString(c.expandJSON(string(manifest), root))
		if err != nil {
			t.Fatalf("manifest of %s: %v", recorded.Original, err)
		}
		artifact.Manifest = decoded
	}
	if home := recorded.Home; home != nil {
		artifact.Home = &input.Home{DiskPath: filepath.FromSlash(c.Expand(home.DiskPath, root)),
			Original: c.Expand(home.Original, root), User: string(home.User),
			Host: string(home.Host), Inferred: home.Inferred, Project: home.Project,
			StagedPaths: map[string]string{}, Secrets: withheldSecrets()}
		for rel, staged := range home.StagedPaths {
			artifact.Home.StagedPaths[rel] = filepath.FromSlash(c.Expand(staged, root))
		}
	}
	return artifact
}

// expandJSON returns JSON text with the root and repository tokens, which
// stand inside its strings, replaced by the paths written as JSON string
// content.
func (c *Corpus) expandJSON(text, root string) string {
	escape := func(path string) string {
		quoted := jsonvalue.NewString(path).JSON()
		return quoted[1 : len(quoted)-1]
	}
	text = strings.ReplaceAll(text, c.Header.RootToken, escape(root))
	return strings.ReplaceAll(text, c.Header.RepoToken, escape(repositoryRoot()))
}

// posixRootURI reports whether a test writes a file URI made from its
// temporary root into the evidence. The recording's root is a POSIX path, so
// the URI and the row built from it cannot be given a Windows root.
func posixRootURI(node string) bool {
	return strings.HasSuffix(node, "::VsCodeTests::test_agents_window_project_is_working_directory")
}
