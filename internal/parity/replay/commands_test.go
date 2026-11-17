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
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
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
	// checked; multisets lists those whose rows differ in order from the
	// recording's and are compared as multisets, and equalRows those where
	// difference 107 took the other of two equal rows, with the count.
	looseTimelines int
	multisets      []string
	equalRows      []string
}

func TestCommandsReplayEveryRecordedCase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command cases print the POSIX paths of the recording; they run on Linux and macOS")
	}
	corpus := Open(t)
	replay := newCommandReplay(t, corpus)
	for _, command := range caseFiles(t, corpus, "commands") {
		for _, recorded := range corpus.CommandCases(t, command) {
			name := fmt.Sprintf("%s/%d", command, recorded.Seq)
			t.Run(name, func(t *testing.T) { replay.replay(t, command, recorded) })
		}
	}
	replay.report(t)
}

// newCommandReplay returns the replay's state: the Python parser agents
// are those with recorded parser cases, and a pending one is any of them
// with no registered parser.
func newCommandReplay(t *testing.T, corpus *Corpus) *commandReplay {
	registered := parsers.ByAgent(parsers.All())
	replay := &commandReplay{corpus: corpus, pending: map[string]bool{},
		pythonAgents: caseFiles(t, corpus, "parsers"), deniesAccess: deniesReading(t),
		trees: corpus.NewWorkTree(t), full: map[string]int{}, partial: map[string]int{},
		skipped: map[string]int{}, waiting: map[string][]string{}, blocks: map[string]string{}}
	for _, agent := range replay.pythonAgents {
		if len(registered[agent]) == 0 {
			replay.pending[agent] = true
		}
	}
	return replay
}

// report logs, per command, the cases compared in full and in part, the
// cases not compared by reason, and the agents the partial cases wait for.
func (r *commandReplay) report(t *testing.T) {
	for _, command := range slices.Sorted(maps.Keys(commandNames)) {
		if r.full[command]+r.partial[command] == 0 {
			continue
		}
		t.Logf("%s (recorded as %s): %d cases compared in full, %d in part", commandNames[command],
			command, r.full[command], r.partial[command])
	}
	for _, reason := range slices.Sorted(maps.Keys(r.skipped)) {
		t.Logf("not compared, %s: %d cases", reason, r.skipped[reason])
	}
	if r.looseTimelines > 0 {
		t.Logf("listing order: %d loose timeline cases in the documented order; compared as row "+
			"multisets: %s; the other of two equal rows kept (difference 107): %s", r.looseTimelines,
			namesOrNone(r.multisets), namesOrNone(r.equalRows))
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
	got := r.run(t, command, recorded, root)
	want := r.expected(t, recorded, root)
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
	r.compareFull(t, node, root, want, got)
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

// commandOutput is what a run gave, or what the recording says it gave:
// the exit code, the two streams and each output file's bytes by its
// root-relative path.
type commandOutput struct {
	exit           int
	stdout, stderr string
	outputs        map[string]string
	loose          bool
}

// run runs the case's command line in this process, in the recorded
// working directory, with the recorded clock and the run's parsers, and
// returns what it gave and the bytes of each file the recording wrote.
func (r *commandReplay) run(t *testing.T, command string, recorded CommandCase,
	root string) commandOutput {
	args := make([]string, len(recorded.Argv))
	for i, argument := range recorded.Argv {
		args[i] = r.corpus.Expand(argument, root)
	}
	args[0] = commandNames[command]
	t.Chdir(r.corpus.Expand(recorded.Cwd, root))
	registry := r.registry
	if r.registryOverride != nil {
		registry = r.registryOverride
	}
	env := analyze.Environment{Now: recordedClock(t, recorded.Clock), Parsers: registry,
		Version: r.recordedVersion(t, recorded, root)}
	var stdout, stderr bytes.Buffer
	exit := analyze.Run(context.Background(), args, env, &stdout, &stderr)
	got := commandOutput{exit: exit, stdout: stdout.String(), stderr: stderr.String(),
		outputs: map[string]string{}}
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

// registry returns the run's parsers: every registered parser, and for
// each Python parser agent without one a stand-in that wants no artifact,
// so the parser columns and the agents line say what the analyzer says
// while that agent's rows are still missing.
func (r *commandReplay) registry() *parsers.Registry {
	list := parsers.All()
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

// compareFull compares the exit code, both streams and every output file,
// after the documented differences their recorded text shows. root is the
// run's root, which stands where the recording's stood. A loose tree's
// timeline and sessions must be in the documented order and are compared
// after difference 107, as multisets of lines when their order differs,
// since rows of equal sort keys keep the listing order.
func (r *commandReplay) compareFull(t *testing.T, node, root string, want, got commandOutput) {
	if want.loose && holdsTimeline(want.outputs) {
		checkDocumentedOrder(t, node, got.outputs)
		want.outputs, got.outputs = maps.Clone(want.outputs), maps.Clone(got.outputs)
		r.looseTimelines++
		if rows := difference(t, 107).Outputs(want.outputs, got.outputs); rows > 0 {
			r.equalRows = append(r.equalRows, fmt.Sprintf("%s (%d rows)", node, rows))
		}
		if rowOrderDiffers(t, node, want.outputs, got.outputs) {
			r.multisets = append(r.multisets, node)
			sortRowLines(want.outputs)
			sortRowLines(got.outputs)
		}
	}
	if got.exit != want.exit {
		t.Errorf("%s: exit %d, want %d", node, got.exit, want.exit)
	}
	compareText(t, node, "stdout", want.stdout, got.stdout, want.loose)
	compareText(t, node, "stderr", want.stderr, got.stderr, false)
	for _, path := range slices.Sorted(maps.Keys(want.outputs)) {
		wanted, gotten := want.outputs[path], got.outputs[path]
		if strings.HasSuffix(path, analyze.InventoryFile) {
			wanted, gotten = rootKeyOrder(t, wanted, root), rootKeyOrder(t, gotten, root)
		}
		compareText(t, node, path, wanted, gotten, want.loose)
	}
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
	document, err := pyjson.DecodeString(text)
	if err != nil {
		t.Fatalf("inventory.json: %v", err)
	}
	var members []pyjson.Member
	for _, entry := range document.Members() {
		if slices.Contains(rootSortedObjects(), entry.Key) {
			sorted := slices.Clone(entry.Value.Members())
			slices.SortStableFunc(sorted, func(left, right pyjson.Member) int {
				return strings.Compare(left.Key, right.Key)
			})
			entry.Value = pyjson.NewObject(sorted...)
		}
		members = append(members, entry)
	}
	return pyjson.Indented(pyjson.NewObject(members...))
}

// comparePart compares what does not depend on the parsers of a case that
// waits for some: stderr, the stdout lines naming the input, host, notes and
// filters, and which files were written.
func comparePart(t *testing.T, node string, want, got commandOutput) {
	if want.exit == analyze.ExitError && got.exit != want.exit {
		t.Errorf("%s: exit %d, want %d", node, got.exit, want.exit)
	}
	compareText(t, node, "stderr", want.stderr, got.stderr, false)
	compareText(t, node, "stdout header", headerLines(want.stdout), headerLines(got.stdout), false)
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

// compareText compares one text after the documented differences its
// recorded form shows; for a loose input the listing order is normalised.
func compareText(t *testing.T, node, label, want, got string, loose bool) {
	want, got = normalisedPair(t, node, want, got)
	if loose {
		want, got = listingOrder(t, want), listingOrder(t, got)
	}
	DiffText(t, node, label, want, got)
}

// normalisedPair returns a recorded text and the run's after the
// documented differences the recorded one shows.
func normalisedPair(t *testing.T, node, want, got string) (string, string) {
	return normaliseCommandTexts(t, node, textDifferences(want), want, got)
}

// commandDifferences returns the table of documented differences as the
// command replay applies them: difference 102 also rewrites the system
// message of a file a command could not read, which Python printed as
// its capitalised strerror at the end of a problem line or a JSON string
// and the binary prints as the shared lower-case reason.
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
// that rewrite text to both texts, and fails the test for a named one that
// finds nothing in want.
func normaliseCommandTexts(t testing.TB, node string, numbers []int, want, got string) (string, string) {
	t.Helper()
	table := commandDifferences()
	for _, number := range numbers {
		index := slices.IndexFunc(table, func(entry Difference) bool { return entry.Number == number })
		if index < 0 {
			t.Fatalf("no documented difference %d", number)
		}
		if table[index].Text == nil {
			continue
		}
		var found int
		want, found = table[index].Text(want)
		got, _ = table[index].Text(got)
		if found == 0 {
			t.Errorf("%s: difference %d names this case but its text holds nothing to normalise",
				node, number)
		}
	}
	return want, got
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

// textDifferences returns the numbers of the rewriting differences whose
// shape shows in text.
func textDifferences(text string) []int {
	var numbers []int
	for _, entry := range commandDifferences() {
		if entry.Text != nil {
			if _, found := entry.Text(text); found > 0 {
				numbers = append(numbers, entry.Number)
			}
		}
	}
	return numbers
}

// listingOrder returns the text of a loose input's output with the order of
// its homes, files and problems, which is the order a file system lists a
// directory in, made one: a JSON document's homes, files and problems
// arrays sorted by their text, and in plain text the lines of each block
// between blank lines sorted.
func listingOrder(t *testing.T, text string) string {
	if strings.HasPrefix(text, "{") {
		return sortedJSONArrays(t, text)
	}
	blocks := strings.Split(text, "\n\n")
	for i, block := range blocks {
		lines := strings.Split(block, "\n")
		slices.Sort(lines)
		blocks[i] = strings.Join(lines, "\n")
	}
	return strings.Join(blocks, "\n\n")
}

// sortedJSONArrays returns an indented JSON document with its top-level
// homes, files and problems arrays sorted by each element's text.
func sortedJSONArrays(t *testing.T, text string) string {
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &document); err != nil {
		return text
	}
	var sorted []string
	for _, key := range []string{"homes", "files", "problems"} {
		var items []json.RawMessage
		if json.Unmarshal(document[key], &items) != nil {
			continue
		}
		texts := make([]string, len(items))
		for i, item := range items {
			texts[i] = string(item)
		}
		slices.Sort(texts)
		sorted = append(sorted, key+": "+strings.Join(texts, "\n"))
		delete(document, key)
	}
	rest, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return string(rest) + "\n" + strings.Join(sorted, "\n")
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

func TestDifference52RewritesACommandsSystemMessage(t *testing.T) {
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
		{"a JSON string's message is compared under 102", pythonJSON, binaryJSON, []int{102}, false, true},
		{"the shape names difference 102", python, binary, textDifferences(python), false, true},
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
