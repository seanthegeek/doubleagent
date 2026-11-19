//go:build parity

package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/analyze"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// The reasons a recorded command case is not compared in full, as the test
// log names them.
const (
	commandPatched  = "the test patched the analyzer while the command ran"
	commandExcluded = "an excluding documented difference shows in its arguments"
	commandDenied   = "its tree denies reading, which this platform does not enforce"
	commandCrashed  = "the Python command raised an exception"
	commandMissing  = "the binary has no such command yet"
)

// commandNames maps the command a case recorded to the binary's command:
// the analyzer's inventory is the binary's fleet.
var commandNames = map[string]string{"detect": "detect", "timeline": "timeline",
	"inventory": "fleet", "catalog": "catalog"}

// commandReplay is the state of the command replay: the agents whose
// Python parser has no Go parser yet, and the tallies the log prints.
type commandReplay struct {
	corpus       *Corpus
	pending      map[string]bool
	pythonAgents []string
	deniesAccess bool
	trees        *WorkTree
	full         map[string]int
	partial      map[string]int
	skipped      map[string]int
	waiting      map[string][]string
	// blocks holds, by name, what became of the generated blocks of
	// analyzer/docs/output.md.
	blocks map[string]string
	// registryOverride, when set, gives the run's parsers in place of
	// registry.
	registryOverride func() *parsers.Registry
	// looseTimelines counts the loose timeline cases whose order was
	// checked; byKey lists those whose rows differ in order from the
	// recording's and are compared sorted by key, and equalRows those where
	// difference 107 took the other of two equal rows, with the count.
	looseTimelines int
	byKey          []string
	equalRows      []string
	// needed holds the cases that needed each difference, for the log;
	// used gathers the same across every replay of the run.
	needed, used neededTally
	// reordered counts the cases whose options difference 121 moved ahead
	// of the input.
	reordered int
	// calls holds the recorded parser calls of the nodes that ran timeline,
	// which tell difference 107 which files held a record.
	calls nodeCalls
	// withheld counts, by entry, the cases that needed difference 230.
	withheld map[string]int
	// added takes the rows of difference 231 from the parses of the run
	// in progress.
	added *addedRowFilter
}

// replayCommands runs every recorded command case, noting in used the
// differences each needed, and reports whether every case ran: a run
// filtered to some of them, or skipped on this platform, gives false.
func replayCommands(t *testing.T, corpus *Corpus, used neededTally) bool {
	if runtime.GOOS == "windows" {
		t.Skip("command cases print the POSIX paths of the recording; they run on Linux and macOS")
	}
	replay := newCommandReplay(t, corpus)
	replay.used = used
	replay.calls = readTimelineCalls(t, corpus)
	total, ran := 0, 0
	for _, command := range caseFiles(t, corpus, "commands") {
		for _, recorded := range corpus.CommandCases(t, command) {
			total++
			name := fmt.Sprintf("%s/%d", command, recorded.Seq)
			t.Run(name, func(t *testing.T) {
				ran++
				replay.replay(t, command, recorded)
			})
		}
	}
	replay.report(t)
	checkEveryWithheldFormUsed(t, commandScope, replay.withheld, ran == total)
	return ran == total
}

// newCommandReplay returns the replay's state: the Python parser agents
// are those with recorded parser cases, and a pending one is any of them
// with no registered parser, which fails the test.
func newCommandReplay(t *testing.T, corpus *Corpus) *commandReplay {
	replay := &commandReplay{corpus: corpus, pending: map[string]bool{},
		pythonAgents: caseFiles(t, corpus, "parsers"), deniesAccess: deniesReading(t),
		trees: corpus.NewWorkTree(t), full: map[string]int{}, partial: map[string]int{},
		skipped: map[string]int{}, waiting: map[string][]string{}, blocks: map[string]string{},
		needed: neededTally{}, used: neededTally{}, withheld: map[string]int{}}
	for _, agent := range checkEveryAgentHasAParser(t, replay.pythonAgents, parsers.All()) {
		replay.pending[agent] = true
	}
	return replay
}

// checkEveryAgentHasAParser fails the test for each agent with recorded
// cases that list has no parser for, since every parser of the analyzer
// has one in the binary, and returns those agents.
func checkEveryAgentHasAParser(t testing.TB, agents []string, list []parser.Parser) []string {
	t.Helper()
	registered := parsers.ByAgent(list)
	var missing []string
	for _, agent := range agents {
		if len(registered[agent]) == 0 {
			missing = append(missing, agent)
		}
	}
	if len(missing) > 0 {
		t.Errorf("agents with recorded cases and no parser: %s", strings.Join(missing, ", "))
	}
	return missing
}

// report logs, per command, the cases compared in full and in part, the
// cases not compared by reason, and the agents the partial cases wait for;
// a case compared only in part fails the test.
func (r *commandReplay) report(t testing.TB) {
	for _, command := range slices.Sorted(maps.Keys(commandNames)) {
		if r.full[command]+r.partial[command] == 0 {
			continue
		}
		t.Logf("%s (recorded as %s): %d cases compared in full, %d in part", commandNames[command],
			command, r.full[command], r.partial[command])
		if r.partial[command] > 0 {
			t.Errorf("%d %s cases were compared only in part", r.partial[command], commandNames[command])
		}
	}
	for _, reason := range slices.Sorted(maps.Keys(r.skipped)) {
		t.Logf("not compared, %s: %d cases", reason, r.skipped[reason])
	}
	logNeeded(t, "information comparison", r.needed, commandFormatNumbers())
	t.Logf("difference 121: options moved ahead of the input in %d cases", r.reordered)
	if r.looseTimelines > 0 {
		t.Logf("listing order: %d loose timeline cases in the documented order; compared "+
			"sorted by their keys: %s; the other of two equal rows kept (difference 107): %s", r.looseTimelines,
			namesOrNone(r.byKey), namesOrNone(r.equalRows))
	}
	for _, block := range slices.Sorted(maps.Keys(r.blocks)) {
		t.Logf("analyzer/docs/output.md block %s %s", block, r.blocks[block])
	}
	for _, agents := range slices.Sorted(maps.Keys(r.waiting)) {
		t.Logf("compared in part, waiting for %s: %s", agents, strings.Join(r.waiting[agents], ", "))
	}
}

// replay runs one recorded command case and compares what it gave.
func (r *commandReplay) replay(t *testing.T, command string, recorded CommandCase) {
	node := fmt.Sprintf("%s (seq %d)", recorded.Node, recorded.Seq)
	if reason := r.notComparable(t, node, recorded); reason != "" {
		r.skipped[reason]++
		return
	}
	root := r.corpus.Materialise(t, recorded.Tree, recorded.Times)
	before := filesUnder(root)
	got := r.run(t, node, command, recorded, root)
	want := r.expected(t, recorded, root)
	want.held = heldBy(r.calls.forRun(recorded, func(text Text) string { return r.corpus.Expand(text, root) }))
	want.cut = r.corpus.cutTimesOf(recorded.Tree, recorded.Times)
	want.inputs = treeHolds(root, before)
	if archiveHoldsRoot(recorded, want, root) {
		r.skipped[commandArchivedRoot]++
		return
	}
	waiting := r.waitingAgents(t, command, recorded, want)
	defer r.noteBlocks(t, recorded, waiting)
	if len(waiting) > 0 {
		key := strings.Join(waiting, ", ")
		r.waiting[key] = append(r.waiting[key], node)
		r.partial[command]++
		comparePart(t, node, want, got)
		return
	}
	r.full[command]++
	r.compareFull(t, node, root, recorded, want, got)
	checkNoOtherOutput(t, node, root, before, want.outputs)
}

// commandArchivedRoot is why a run over an archive is not compared: the
// archive's own manifest names paths under the recording's root, which sit
// inside compressed bytes where the replay's root cannot be put.
const commandArchivedRoot = "an archive holding the recording's own root in its paths"

// archiveHoldsRoot reports whether a recorded run read a tar.gz or zip
// archive whose rows carry paths under the recording's root.
func archiveHoldsRoot(recorded CommandCase, want commandOutput, root string) bool {
	if !slices.ContainsFunc(recorded.Argv, func(argument Text) bool {
		return strings.HasSuffix(string(argument), ".tar.gz") || strings.HasSuffix(string(argument), ".zip")
	}) {
		return false
	}
	for name, content := range want.outputs {
		if strings.HasSuffix(name, "timeline.jsonl") && strings.Contains(content, root) {
			return true
		}
	}
	return false
}

// exampleNode is the node of the recorder's runs over the example
// collection, whose outputs are the generated blocks of
// analyzer/docs/output.md.
const exampleNode = "recorder::example-collection"

// noteBlocks records, for a run over the example collection, whether the
// generated blocks it gives passed in full or wait for parsers.
func (r *commandReplay) noteBlocks(t *testing.T, recorded CommandCase, waiting []string) {
	if recorded.Node != exampleNode {
		return
	}
	state := "passes"
	switch {
	case len(waiting) > 0:
		state = "waits for " + strings.Join(waiting, ", ")
	case t.Failed():
		state = "fails"
	}
	for _, block := range outputBlocks(ArgumentsText(recorded)) {
		r.blocks[block] = state
	}
}

// outputBlocks returns the generated blocks of analyzer/docs/output.md a
// run over the example collection gives, from its arguments.
func outputBlocks(arguments string) []string {
	switch {
	case strings.HasPrefix(arguments, "detect\n") && strings.HasSuffix(arguments, "--json"):
		return []string{"detect-json"}
	case strings.HasPrefix(arguments, "detect\n") && !strings.Contains(arguments, "--files"):
		return []string{"detect-text"}
	case strings.HasPrefix(arguments, "timeline\n") && strings.Contains(arguments, "--match"):
		return []string{"timeline-jsonl"}
	case strings.HasPrefix(arguments, "timeline\n"):
		return []string{"timeline-stdout", "inventory-json", "sessions-jsonl"}
	}
	return nil
}

// notComparable returns why a case is not compared at all, or "".
func (r *commandReplay) notComparable(t *testing.T, node string, recorded CommandCase) string {
	switch {
	case !slices.Contains(analyze.Commands(), commandNames[string(recorded.Argv[0])]):
		return commandMissing
	case len(recorded.Patched) > 0:
		return commandPatched
	case recorded.Error != nil:
		return commandCrashed
	case Excluded(t, node, excludingNumbers(ArgumentsText(recorded)), ArgumentsText(recorded)):
		r.note(node, excludingNumbers(ArgumentsText(recorded)))
		return commandExcluded
	}
	if recorded.Tree != "" && !r.deniesAccess {
		denied, err := r.trees.DeniesReading(recorded.Tree)
		if err != nil {
			t.Fatal(err)
		}
		if denied {
			return commandDenied
		}
	}
	return ""
}

// excludingNumbers returns the numbers of the excluding differences that
// show in text.
func excludingNumbers(text string) []int {
	var numbers []int
	for _, entry := range Differences() {
		if entry.Excludes != nil && entry.Excludes(text) {
			numbers = append(numbers, entry.Number)
		}
	}
	return numbers
}

// note records the differences a case needed, for the log and the run.
func (r *commandReplay) note(node string, numbers []int) {
	r.needed.note(node, numbers)
	r.used.note(node, numbers)
}

// commandOutput is what a run gave, or what the recording says it gave:
// the exit code, the two streams and each output file's bytes by its
// root-relative path.
type commandOutput struct {
	exit           int
	stdout, stderr string
	outputs        map[string]string
	loose          bool
	// held is, for a recorded run, the records its recorded parses read.
	held heldRecords
	// cut is, for a recorded run, the times difference 119 may find one
	// millisecond earlier.
	cut *cutTimes
	// source is, for the binary's side, where the run's texts come from.
	source runSource
	// inputs is, for a recorded run, whether a file of the case's input
	// tree holds a match of a pattern; nil when the tree is not known.
	inputs func(pattern *regexp.Regexp) bool
}

// runSource is where the texts of the binary's side of a command case come
// from, which decides how the differences of text read them.
type runSource int

const (
	// parsedRun is a run of the binary over the case's tree.
	parsedRun runSource = iota
	// recordedRowsRun is a timeline run over the recording's parser rows,
	// which hold what the recording printed, so the entries of
	// differences 230 and 232 for a parser's rows apply on neither side.
	recordedRowsRun
	// comparedRun is a run a test built from a recording with every text
	// already in the compared form, which the binary's side of a
	// difference of text then leaves as it is.
	comparedRun
)

// caseRun is what decides how one command case's texts are rewritten: the
// source of the binary's side, and what the case's input tree holds (nil
// when not known).
type caseRun struct {
	source runSource
	inputs func(pattern *regexp.Regexp) bool
}

// applies reports whether an entry of difference 230 or 232 rewrites the
// case's texts: an entry for a parser's rows only where the binary's
// parsers gave them, and an entry with a cause only in a run of the
// binary's parsers over an input tree that holds the cause.
func (c caseRun) applies(form withheldForm) bool {
	if form.cause != nil && (c.source != parsedRun || c.inputs == nil || !c.inputs(form.cause)) {
		return false
	}
	return c.source != recordedRowsRun || form.scope != parserScope
}

// caseRunOf returns the caseRun of comparing the recorded run want with
// got.
func caseRunOf(want, got commandOutput) caseRun {
	return caseRun{source: got.source, inputs: want.inputs}
}

// treeHolds returns whether a file of a case's input tree, the files under
// root by root-relative path, holds a match of a pattern; each pattern is
// looked for once.
func treeHolds(root string, files map[string]bool) func(pattern *regexp.Regexp) bool {
	held := map[string]bool{}
	return func(pattern *regexp.Regexp) bool {
		found, seen := held[pattern.String()]
		if !seen {
			found = slices.ContainsFunc(slices.Sorted(maps.Keys(files)), func(name string) bool {
				data, err := osfile.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
				return err == nil && pattern.Match(data)
			})
			held[pattern.String()] = found
		}
		return found
	}
}

// run runs the case's command line in this process, in the recorded
// working directory, with the recorded clock and the run's parsers, secrets
// withheld as every recorded run withheld them (withSecretsWithheld), and
// returns what it gave and the bytes of each file the recording wrote.
func (r *commandReplay) run(t *testing.T, node, command string, recorded CommandCase,
	root string) commandOutput {
	args := make([]string, len(recorded.Argv))
	for i, argument := range recorded.Argv {
		args[i] = r.corpus.Expand(argument, root)
	}
	args[0] = commandNames[command]
	rest, moved := difference(t, 121).Arguments(args[0], args[1:])
	if moved > 0 {
		r.reordered++
		r.note(node, []int{121})
	}
	args = append([]string{args[0]}, withSecretsWithheld(args[0], rest)...)
	t.Chdir(r.corpus.Expand(recorded.Cwd, root))
	r.added = &addedRowFilter{}
	registry := r.registry
	if r.registryOverride != nil {
		registry = r.registryOverride
	}
	env := analyze.Environment{Now: recordedClock(t, recorded.Clock), Parsers: registry,
		Version: r.recordedVersion(t, recorded, root)}
	var stdout, stderr bytes.Buffer
	exit := analyze.Run(context.Background(), args, env, &stdout, &stderr)
	if r.added.taken > 0 {
		r.note(node, []int{withheldRowsDifference().Number})
	}
	got := commandOutput{exit: exit, stdout: stdout.String(), stderr: stderr.String(),
		outputs: map[string]string{}, source: parsedRun}
	for path := range recorded.Outputs {
		data, err := osfile.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			got.outputs[path] = "(not written: " + err.Error() + ")"
			continue
		}
		got.outputs[path] = string(data)
	}
	return got
}

// registry returns the run's parsers: every registered parser, each whose
// agent has a row of difference 231 taking that row from its parses
// (withoutAddedRows), and for each Python parser agent without one a
// stand-in that wants no artifact, so the parser columns and the agents
// line say what the analyzer says while that agent's rows are still
// missing.
func (r *commandReplay) registry() *parsers.Registry {
	list := withoutAddedRows(parsers.All(), r.added)
	for _, agent := range r.pythonAgents {
		if r.pending[agent] {
			list = append(list, pendingParser{parser.NewBase(agent, "pending")})
		}
	}
	return parsers.NewOf(list)
}

// pendingParser stands for a Python parser not yet ported: it has the
// agent and wants nothing.
type pendingParser struct {
	parser.Base
}

// Wants reports that the stand-in reads no artifact.
func (pendingParser) Wants(*input.Artifact) bool { return false }

// Parse emits nothing.
func (pendingParser) Parse(*input.Artifact, parser.Options, parser.Emit) error { return nil }

// recordedClock returns a clock that gives the times the recorded run read,
// in order, the last one again when the run reads more.
func recordedClock(t *testing.T, clock []string) func() time.Time {
	times := make([]time.Time, len(clock))
	for i, text := range clock {
		moment, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			t.Fatalf("clock %q: %v", text, err)
		}
		times[i] = moment
	}
	read := 0
	return func() time.Time {
		if len(times) == 0 {
			t.Error("the run read the clock, the recording did not")
			return time.Now()
		}
		moment := times[min(read, len(times)-1)]
		read++
		return moment
	}
}

// versionKey starts the line of inventory.json that names the version.
const versionKey = `"doubleagent_version": "`

// recordedVersion returns the analyzer version the recorded inventory.json
// names, which the run writes in place of the binary's own, or "".
func (r *commandReplay) recordedVersion(t *testing.T, recorded CommandCase, root string) string {
	for path, digest := range recorded.Outputs {
		if !strings.HasSuffix(path, "inventory.json") {
			continue
		}
		data := r.blobText(t, digest, root)
		if _, rest, ok := strings.Cut(data, versionKey); ok {
			version, _, _ := strings.Cut(rest, `"`)
			return version
		}
	}
	return ""
}

// blobText returns an output's recorded bytes, a blob hash in JSON, with
// the root in place of the placeholder; a link output gives its JSON text.
func (r *commandReplay) blobText(t *testing.T, digest json.RawMessage, root string) string {
	var hash string
	if err := json.Unmarshal(digest, &hash); err != nil {
		return string(digest)
	}
	data, err := r.corpus.ReadBlob(hash, root)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// expected returns what the recorded run gave, its paths under root.
func (r *commandReplay) expected(t *testing.T, recorded CommandCase, root string) commandOutput {
	var exit int
	if err := json.Unmarshal(recorded.Exit, &exit); err != nil {
		t.Fatalf("%s: exit %s: %v", recorded.Node, recorded.Exit, err)
	}
	want := commandOutput{exit: exit, stdout: r.corpus.Expand(recorded.Stdout, root),
		stderr: r.corpus.Expand(recorded.Stderr, root), outputs: map[string]string{}}
	for path, digest := range recorded.Outputs {
		want.outputs[path] = r.blobText(t, digest, root)
	}
	want.loose = strings.Contains(want.stdout, "\nkind:    loose") ||
		strings.Contains(want.stdout, `"kind": "loose"`) ||
		strings.HasPrefix(want.stdout, "input:     ") &&
			strings.Contains(strings.SplitN(want.stdout, "\n", 2)[0], " (loose)")
	return want
}

// waitingAgents returns the pending agents a timeline case's output
// depends on: those whose rows or files the recorded outputs show, among
// the --agent values when the case names some.
func (r *commandReplay) waitingAgents(t *testing.T, command string, recorded CommandCase,
	want commandOutput) []string {
	if command != "timeline" {
		return nil
	}
	shown := map[string]bool{}
	for path, data := range want.outputs {
		for _, agent := range agentsShown(t, path, data) {
			shown[agent] = true
		}
	}
	chosen := agentArguments(recorded)
	var waiting []string
	for agent := range shown {
		if r.pending[agent] && (len(chosen) == 0 || chosen[agent]) {
			waiting = append(waiting, agent)
		}
	}
	slices.Sort(waiting)
	return waiting
}

// agentArguments returns the values of a case's --agent options.
func agentArguments(recorded CommandCase) map[string]bool {
	chosen := map[string]bool{}
	for i, argument := range recorded.Argv {
		if value, ok := strings.CutPrefix(string(argument), "--agent="); ok {
			chosen[value] = true
		} else if argument == "--agent" && i+1 < len(recorded.Argv) {
			chosen[string(recorded.Argv[i+1])] = true
		}
	}
	return chosen
}

// agentPattern finds the agents an output file names: the agent field of a
// JSONL record and the agent key of inventory.json.
var agentPatterns = []string{`"agent":"`, `"agent": "`}

// agentsShown returns the agents an output file names as an agent value, or
// as a key of an "agents" object of inventory.json.
func agentsShown(t *testing.T, path, data string) []string {
	var agents []string
	for _, pattern := range agentPatterns {
		for _, part := range strings.Split(data, pattern)[1:] {
			agent, _, _ := strings.Cut(part, `"`)
			agents = append(agents, agent)
		}
	}
	if strings.HasSuffix(path, "inventory.json") {
		agents = append(agents, inventoryAgentKeys(t, data)...)
	}
	return agents
}

// inventoryAgentKeys returns the keys of every "agents" object in an
// inventory.json.
func inventoryAgentKeys(t *testing.T, data string) []string {
	var document any
	if err := json.Unmarshal([]byte(data), &document); err != nil {
		t.Fatalf("inventory.json: %v", err)
	}
	var keys []string
	var walk func(value any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if agents, ok := child.(map[string]any); ok && key == "agents" {
					keys = append(keys, slices.Collect(maps.Keys(agents))...)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(document)
	return keys
}

// filesUnder returns the root-relative paths of the files and links under
// root that can be listed.
func filesUnder(root string) map[string]bool {
	found := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			relative, _ := filepath.Rel(root, path)
			found[filepath.ToSlash(relative)] = true
		}
		return nil
	})
	return found
}

// checkNoOtherOutput fails the case for a file the run left under root
// that was neither there before nor written by the recorded run.
func checkNoOtherOutput(t *testing.T, node, root string, before map[string]bool,
	outputs map[string]string) {
	for path := range filesUnder(root) {
		if !before[path] && !hasKey(outputs, path) {
			t.Errorf("%s: the run wrote %s, the recording did not", node, path)
		}
	}
}

// hasKey reports whether a map holds key.
func hasKey(entries map[string]string, key string) bool {
	_, ok := entries[key]
	return ok
}

// compareFull compares the exit code, and both streams and every output
// file as information. The run's timeline and sessions must be in the
// documented order (checkRunOrder). root is the run's root, which stands
// where the recording's stood.
func (r *commandReplay) compareFull(t *testing.T, node, root string, recorded CommandCase,
	want, got commandOutput) {
	checkRunOrder(t, node, want, got)
	if got.exit != want.exit {
		t.Errorf("%s: exit %d, want %d", node, got.exit, want.exit)
	}
	notes, needed := compareInformation(t, node, root, want, got)
	r.note(node, needed)
	if slices.Contains(needed, withheldDifference().Number) {
		r.noteWithheld(want, caseRunOf(want, got))
	}
	if !notes.checked {
		return
	}
	r.looseTimelines++
	if notes.equalRows > 0 {
		r.equalRows = append(r.equalRows, fmt.Sprintf("%s (%d rows)", node, notes.equalRows))
	}
	if notes.byKey {
		r.byKey = append(r.byKey, node)
	}
}

// noteWithheld counts, for each command entry of difference 230 that
// applies in run and whose recorded form shows in the recorded streams or
// outputs, one case.
func (r *commandReplay) noteWithheld(want commandOutput, run caseRun) {
	texts := []string{want.stdout, want.stderr}
	for _, path := range slices.Sorted(maps.Keys(want.outputs)) {
		texts = append(texts, want.outputs[path])
	}
	for _, label := range withheldEntriesIn(commandScope, strings.Join(texts, "\n"), run) {
		r.withheld[label]++
	}
}

// looseNotes is what the comparison of a loose tree's timeline found:
// whether it was checked, whether its rows were compared sorted by key, and
// how many rows difference 107 rewrote.
type looseNotes struct {
	checked, byKey bool
	equalRows      int
}

// compareInformation compares a run's streams and outputs with the
// recorded ones as information, under every difference of format, reports
// what still differs, and returns the loose tree's notes and the
// differences the case needed: of format, those it fails without; of text,
// those whose shape its recorded texts show; and 107 where it rewrote a
// row.
func compareInformation(t testing.TB, node, root string, want, got commandOutput) (looseNotes, []int) {
	t.Helper()
	normaliser := newTextNormaliser(caseRunOf(want, got), nil)
	compare := func(reporter testing.TB, enabled enabledSet) looseNotes {
		return compareOutputs(reporter, node, root, outputForms{enabled}, normaliser, want, got)
	}
	reporter := &recordingTB{TB: t}
	notes := compare(reporter, setOf(commandFormatNumbers()))
	for _, message := range reporter.errors {
		t.Errorf("%s", message)
	}
	if len(reporter.errors) > 0 {
		return notes, nil
	}
	needed := neededDifferences(commandFormatNumbers(), func(enabled enabledSet) bool {
		quiet := &recordingTB{TB: t}
		compare(quiet, enabled)
		return len(quiet.errors) == 0
	})
	needed = append(needed, neededAmong(slices.Sorted(maps.Keys(normaliser.applied)), func(numbers []int) bool {
		quiet := &recordingTB{TB: t}
		without := newTextNormaliser(caseRunOf(want, got), setOf(numbers))
		compareOutputs(quiet, node, root, outputForms{setOf(commandFormatNumbers())}, without, want, got)
		return len(quiet.errors) == 0
	})...)
	if notes.equalRows > 0 {
		needed = append(needed, 107)
	}
	slices.Sort(needed)
	return notes, needed
}

// compareOutputs compares both streams and every output file in their
// compared form, after the analysis differences their recorded text
// shows. A loose tree's timeline and sessions are compared after
// difference 107, and when their order differs, each sorted by its
// documented keys, which leaves the rows of one record in their order; the
// run's own order is checked against the documented one on its own.
func compareOutputs(t testing.TB, node, root string, forms outputForms, normaliser *textNormaliser,
	want, got commandOutput) looseNotes {
	if normaliser == nil {
		normaliser = newTextNormaliser(caseRunOf(want, got), nil)
	}
	wantTexts, gotTexts := comparedTexts(t, root, forms, want, want), comparedTexts(t, root, forms, got, want)
	var notes looseNotes
	if want.loose && holdsTimeline(want.outputs) {
		notes = looseNotes{checked: true, equalRows: difference(t, 107).Outputs(wantTexts, gotTexts, want.held)}
		if rowOrderDiffers(t, node, normaliser, wantTexts, gotTexts) {
			notes.byKey = true
			sortRowLinesByKey(wantTexts)
			sortRowLinesByKey(gotTexts)
		}
	}
	for _, label := range slices.Sorted(maps.Keys(wantTexts)) {
		compareText(t, node, normaliser, label, wantTexts[label], gotTexts[label],
			want.loose && label != "stderr", forms.enabled, want.cut)
	}
	return notes
}

// comparedTexts returns a run's streams, by the labels stdout and stderr,
// and the output files the recording wrote, by path, in their compared
// form; recorded is the recording, which decides each text's kind. An
// inventory.json first has its root-sorted objects sorted.
func comparedTexts(t testing.TB, root string, forms outputForms, run, recorded commandOutput) map[string]string {
	texts := map[string]string{"stdout": forms.form("stdout", recorded.stdout, run.stdout),
		"stderr": forms.form("stderr", recorded.stderr, run.stderr)}
	for path := range recorded.outputs {
		text := run.outputs[path]
		if strings.HasSuffix(path, analyze.InventoryFile) {
			text = rootKeyOrder(t, text, root)
		}
		texts[path] = forms.form(path, recorded.outputs[path], text)
	}
	return texts
}

// rootSortedObjects are the members of inventory.json whose keys are
// project paths in sorted order.
func rootSortedObjects() []string {
	return []string{"projects", "scratch_directories"}
}

// rootKeyOrder returns an inventory.json with the project paths of its
// projects and scratch_directories objects sorted, when one of them lies
// under root. Those objects are sorted by path, and a path under the root
// sorts by the root's own text, which the run's root and the recording's
// share only in length; any other inventory.json is returned unchanged, so
// its order is still compared.
func rootKeyOrder(t testing.TB, text, root string) string {
	if !strings.Contains(text, root) {
		return text
	}
	document, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatalf("inventory.json: %v", err)
	}
	var members []jsonvalue.Member
	for _, entry := range document.Members() {
		if slices.Contains(rootSortedObjects(), entry.Key) {
			sorted := slices.Clone(entry.Value.Members())
			slices.SortStableFunc(sorted, func(left, right jsonvalue.Member) int {
				return strings.Compare(left.Key, right.Key)
			})
			entry.Value = jsonvalue.NewObject(sorted...)
		}
		members = append(members, entry)
	}
	return jsonvalue.NewObject(members...).IndentedJSON("  ")
}

// comparePart compares what does not depend on the parsers of a case that
// waits for some: stderr, the stdout lines naming the input, host, notes and
// filters, and which files were written.
func comparePart(t *testing.T, node string, want, got commandOutput) {
	if want.exit == analyze.ExitError && got.exit != want.exit {
		t.Errorf("%s: exit %d, want %d", node, got.exit, want.exit)
	}
	compareText(t, node, nil, "stderr", want.stderr, got.stderr, false, nil, nil)
	compareText(t, node, nil, "stdout header", headerLines(want.stdout), headerLines(got.stdout), false, nil, nil)
	for path := range want.outputs {
		if strings.HasPrefix(got.outputs[path], "(not written: ") {
			t.Errorf("%s: %s %s", node, path, got.outputs[path])
		}
	}
}

// headerPrefixes start the stdout lines of timeline that no parser decides.
var headerPrefixes = []string{"input:", "host:", "note:", "window:", "match:"}

// headerLines returns the lines of a timeline stdout that start with one of
// headerPrefixes.
func headerLines(text string) string {
	var kept []string
	for _, line := range strings.SplitAfter(text, "\n") {
		for _, prefix := range headerPrefixes {
			if strings.HasPrefix(line, prefix) {
				kept = append(kept, line)
				break
			}
		}
	}
	return strings.Join(kept, "")
}

// compareText compares one text after the analysis differences its
// recorded form shows; for a loose input the listing order is normalised,
// and under difference 119 a timestamp the recording rounded up from a
// time of cut is let stand.
func compareText(t testing.TB, node string, normaliser *textNormaliser, label, want, got string,
	loose bool, enabled enabledSet, cut *cutTimes) {
	t.Helper()
	want, got = normaliser.pair(t, node, want, got)
	if loose {
		want, got = listingOrder(label, want), listingOrder(label, got)
	}
	if enabled[differenceRoundedTime] {
		want, got = rolledOverLines(want, got, cut)
	}
	DiffText(t, node, label, want, got)
}

// textNormaliser applies normalisedPair under the differences of one case
// (table), keeping each result, with what it reported, for the next
// comparison of the same case under other differences of format, and
// gathers in applied the differences it applied; a nil one keeps nothing
// and applies commandDifferences.
type textNormaliser struct {
	cache   map[[2]string]normalisedTexts
	applied map[int]bool
	// only, when set, holds the differences of text it may apply; the
	// others are left out, to learn whether a case needs them.
	only  enabledSet
	table []Difference
}

// newTextNormaliser returns a textNormaliser of the differences run calls
// for, among only when it is set.
func newTextNormaliser(run caseRun, only enabledSet) *textNormaliser {
	return &textNormaliser{cache: map[[2]string]normalisedTexts{}, applied: map[int]bool{}, only: only,
		table: caseDifferences(run)}
}

// normalisedTexts is one result of normalisedPair and what it reported.
type normalisedTexts struct {
	want, got string
	numbers   []int
	errors    []string
}

// pair returns normalisedPair of two texts, reporting what it reported.
func (n *textNormaliser) pair(t testing.TB, node, want, got string) (string, string) {
	t.Helper()
	if n == nil {
		want, got, _ = normalisedPair(t, node, commandDifferences(), want, got, nil)
		return want, got
	}
	key := [2]string{want, got}
	result, ok := n.cache[key]
	if !ok {
		reporter := &recordingTB{TB: t}
		result.want, result.got, result.numbers = normalisedPair(reporter, node, n.table, want, got, n.only)
		result.errors = reporter.errors
		n.cache[key] = result
	}
	for _, message := range result.errors {
		t.Errorf("%s", message)
	}
	for _, number := range result.numbers {
		n.applied[number] = true
	}
	return result.want, result.got
}

// normalisedPair returns a recorded text and the run's after the
// differences of table the recorded one shows, among only when it is set,
// and their numbers.
func normalisedPair(t testing.TB, node string, table []Difference, want, got string,
	only enabledSet) (string, string, []int) {
	numbers := textDifferences(table, want)
	if only != nil {
		numbers = slices.DeleteFunc(numbers, func(number int) bool { return !only[number] })
	}
	want, got = normaliseTextsOf(t, table, node, numbers, want, got)
	return want, got, numbers
}

// caseDifferences returns commandDifferences as one case calls for:
// differences 230 and 232 by the entries that apply in run, and for a run
// already in the compared form, no rewrite of the binary's side.
func caseDifferences(run caseRun) []Difference {
	table := commandDifferences()
	for i := range table {
		switch table[i].Number {
		case withheldDifference().Number:
			table[i].Text, table[i].Binary = formsText(withheldForms(), recordedSide, run),
				formsText(withheldForms(), binarySide, run)
		case shownDifference().Number:
			table[i].Text, table[i].Binary = formsText(shownForms(), recordedSide, run),
				formsText(shownForms(), binarySide, run)
		}
		if run.source == comparedRun && table[i].Binary != nil {
			table[i].Binary = asWritten
		}
	}
	return table
}

// commandDifferences returns the table of documented differences as the
// command replay applies them: difference 102 also rewrites the system
// message of a file a command could not read, which Python printed as its
// capitalised strerror at the end of a problem line or a JSON string and
// the binary prints as the shared lower-case reason, compared as written.
func commandDifferences() []Difference {
	table := Differences()
	for i := range table {
		if library := table[i].Text; table[i].Number == 102 && library != nil {
			table[i].Text = func(text string) (string, int) {
				text, found := library(text)
				text, system := systemMessageText(text)
				return text, found + system
			}
		}
	}
	return table
}

// normaliseCommandTexts applies the named differences of commandDifferences
// that rewrite text, each side's rewrite to its own text, and fails the
// test for a named one that finds nothing in want.
func normaliseCommandTexts(t testing.TB, node string, numbers []int, want, got string) (string, string) {
	t.Helper()
	return normaliseTextsOf(t, commandDifferences(), node, numbers, want, got)
}

// systemMessageShape is a system message as Python's strerror words it,
// capitalised, at the end of a problem line or of a JSON string.
var systemMessageShape = regexp.MustCompile(`: (Permission denied|No such file or directory|` +
	`Is a directory|Not a directory|Too many levels of symbolic links|File name too long)` +
	`("?,?\n|\z)`)

// systemMessageText lowers the first letter of each system message of
// systemMessageShape, the form internal/reason gives, and returns the count.
func systemMessageText(text string) (string, int) {
	found := 0
	text = systemMessageShape.ReplaceAllStringFunc(text, func(match string) string {
		found++
		return ": " + strings.ToLower(match[2:3]) + match[3:]
	})
	return text, found
}

// textDifferences returns the numbers of the rewriting differences of
// table whose recorded shape shows in text, each looked for, in applying
// order, in what the ones before it left.
func textDifferences(table []Difference, text string) []int {
	var numbers []int
	for _, entry := range applyingOrder(table) {
		if entry.Text != nil {
			var found int
			if text, found = entry.Text(text); found > 0 {
				numbers = append(numbers, entry.Number)
			}
		}
	}
	slices.Sort(numbers)
	return numbers
}

// listingOrder returns the text of a loose input's output with the order of
// its homes, files and problems, which is the order a file system lists a
// directory in, made one: a JSON document's homes, files and problems
// arrays sorted by their text, and in stdout the problem lines, the rows of
// the homes table and the --files lines sorted. Every other line, and every
// other output, keeps its order; the rows of timeline.jsonl and
// sessions.jsonl are compared in order unless rowOrderDiffers.
func listingOrder(label, text string) string {
	switch {
	case (label == "stdout" || strings.HasSuffix(label, ".json")) && strings.HasPrefix(text, "{"):
		return sortedJSONArrays(text)
	case label != "stdout":
		return text
	}
	blocks := strings.Split(text, "\n\n")
	for i, block := range blocks {
		all := strings.Split(block, "\n")
		lines := all[:len(strings.Split(strings.TrimRight(block, "\n"), "\n"))]
		switch {
		case strings.HasPrefix(block, "user "):
			slices.Sort(lines[1:])
		case !slices.ContainsFunc(lines, func(line string) bool { return strings.Count(line, "\t") != 2 }):
			slices.Sort(lines)
		default:
			sortProblemRuns(lines)
		}
		blocks[i] = strings.Join(all, "\n")
	}
	return strings.Join(blocks, "\n\n")
}

// sortProblemRuns sorts, in place, each run of neighbouring lines that
// start with "problem:".
func sortProblemRuns(lines []string) {
	problem := func(line string) bool { return strings.HasPrefix(line, "problem:") }
	for start := 0; start < len(lines); start++ {
		if !problem(lines[start]) {
			continue
		}
		end := start
		for end < len(lines) && problem(lines[end]) {
			end++
		}
		slices.Sort(lines[start:end])
		start = end
	}
}

// sortedJSONArrays returns a JSON document in its canonical form with its
// top-level homes, files and problems arrays sorted by each element's
// text; a text that does not decode as an object is kept as it is.
func sortedJSONArrays(text string) string {
	document, ok := readWholeJSON(text)
	if !ok || document.container != '{' {
		return text
	}
	rest := jsonValue{container: '{'}
	var sorted []string
	for _, member := range document.members {
		if member.value.container != '[' || !slices.Contains([]string{"homes", "files", "problems"}, member.key) {
			rest.members = append(rest.members, member)
			continue
		}
		texts := make([]string, len(member.value.items))
		for i, item := range member.value.items {
			texts[i] = item.canonical()
		}
		slices.Sort(texts)
		sorted = append(sorted, member.key+": "+strings.Join(texts, "\n"))
	}
	return indented(rest.canonical(), "  ") + "\n" + strings.Join(sorted, "\n")
}

func TestRootKeyOrderSortsOnlyAnInventoryWithPathsUnderTheRoot(t *testing.T) {
	root := "/run/root"
	tests := []struct {
		name, text, want string
	}{
		{"keys under the root are sorted",
			`{"projects": {"/z": 1, "/run/root/home": 2}, "scratch_directories": {"/y": 3, "/run/root/x": 4}, "rows": 0}`,
			"{\n  \"projects\": {\n    \"/run/root/home\": 2,\n    \"/z\": 1\n  },\n" +
				"  \"scratch_directories\": {\n    \"/run/root/x\": 4,\n    \"/y\": 3\n  },\n  \"rows\": 0\n}"},
		{"an inventory without the root keeps its order", `{"projects": {"/z": 1, "/a": 2}}`,
			`{"projects": {"/z": 1, "/a": 2}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rootKeyOrder(t, test.text, root); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDifference102RewritesACommandsSystemMessage(t *testing.T) {
	python := "problem:   /srv/proj/.git/config: Permission denied\nrows: 3\n"
	binary := "problem:   /srv/proj/.git/config: permission denied\nrows: 3\n"
	pythonJSON := `  "problems": [` + "\n" + `    "/srv/proj/.git/config: No such file or directory"` + "\n  ]"
	binaryJSON := `  "problems": [` + "\n" + `    "/srv/proj/.git/config: no such file or directory"` + "\n  ]"
	tests := []struct {
		name, want, got string
		numbers         []int
		fails, equal    bool
	}{
		{"a problem line's message is compared under 102", python, binary, []int{102}, false, true},
		{"the binary printing the capitalised message fails", python, python, []int{102}, false, false},
		{"a JSON string's message is compared under 102", pythonJSON, binaryJSON, []int{102}, false, true},
		{"the shape names difference 102", python, binary, textDifferences(commandDifferences(), python), false, true},
		{"another capitalised word is not a system message", "problem:   /x: Permission granted\n",
			"problem:   /x: permission granted\n", []int{102}, true, false},
		{"102 named on a text without its shape fails the case", "rows: 3\n", "rows: 3\n", []int{102},
			true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			want, got := normaliseCommandTexts(reporter, "n", test.numbers, test.want, test.got)
			if failed := len(reporter.errors) > 0; failed != test.fails {
				t.Errorf("failed %v, want %v: %s", failed, test.fails, strings.Join(reporter.errors, "; "))
			}
			if (want == got) != test.equal {
				t.Errorf("normalised texts %q and %q, want equal %v", want, got, test.equal)
			}
		})
	}
}
