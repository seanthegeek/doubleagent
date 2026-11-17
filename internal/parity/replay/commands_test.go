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
	// proposed lists the cases compared after the proposed difference
	// proposedStrerror.
	proposed []string
	// blocks holds, by name, what became of the generated blocks of
	// analyzer/docs/output.md.
	blocks map[string]string
	// registryOverride, when set, gives the run's parsers in place of
	// registry.
	registryOverride func() *parsers.Registry
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
	if len(r.proposed) > 0 {
		t.Logf("compared after the proposed difference of a repository file's system message: %s",
			strings.Join(r.proposed, ", "))
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
	if strerrorShape.MatchString(want.stdout + strings.Join(slices.Collect(maps.Values(want.outputs)), "")) {
		r.proposed = append(r.proposed, node)
	}
	compareFull(t, node, want, got)
	checkNoOtherOutput(t, node, root, before, want.outputs)
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
// after the documented differences their recorded text shows.
func compareFull(t *testing.T, node string, want, got commandOutput) {
	if got.exit != want.exit {
		t.Errorf("%s: exit %d, want %d", node, got.exit, want.exit)
	}
	compareText(t, node, "stdout", want.stdout, got.stdout, want.loose)
	compareText(t, node, "stderr", want.stderr, got.stderr, false)
	for _, path := range slices.Sorted(maps.Keys(want.outputs)) {
		compareText(t, node, path, want.outputs[path], got.outputs[path], want.loose)
	}
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
// recorded form shows, and the proposed ones; for a loose input the listing
// order is normalised.
func compareText(t *testing.T, node, label, want, got string, loose bool) {
	numbers := textDifferences(want)
	want, got = NormaliseTexts(t, node, numbers, want, got)
	want = proposedStrerror(want)
	if loose {
		want, got = listingOrder(t, want), listingOrder(t, got)
	}
	DiffText(t, node, label, want, got)
}

// strerrorShape is a repository file's problem as the analyzer words it:
// the system's message with its first letter capitalised, at the end of a
// problem line or of a JSON string.
var strerrorShape = regexp.MustCompile(`: (Permission denied|No such file or directory|` +
	`Is a directory|Not a directory|Too many levels of symbolic links|File name too long)` +
	`("?,?\n|\z)`)

// proposedStrerror rewrites the analyzer's capitalised system message of an
// unreadable repository file to the binary's lower-case reason, which every
// other problem line of both already uses. It is a proposed documented
// difference, not yet numbered: a case it rewrites is logged.
func proposedStrerror(text string) string {
	return strerrorShape.ReplaceAllStringFunc(text, func(found string) string {
		return ": " + strings.ToLower(found[2:3]) + found[3:]
	})
}

// textDifferences returns the numbers of the rewriting differences whose
// shape shows in text.
func textDifferences(text string) []int {
	var numbers []int
	for _, entry := range Differences() {
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
