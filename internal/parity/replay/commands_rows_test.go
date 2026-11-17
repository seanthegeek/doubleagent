//go:build parity

package replay

import (
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// pythonParser is one parser of the analyzer's list, in its order: its
// agent, the session marks it records, whether it reads a secondary
// source, and the other agents whose artifacts it is offered.
type pythonParser struct {
	agent                        string
	sync, relay, origin, reading bool
	readsAgents                  []string
}

// pythonParsers returns the analyzer's parser list (parsers/__init__.py)
// with each parser's flags, as its classes declare them.
func pythonParsers() []pythonParser {
	editors := []string{"vscode", "cursor", "windsurf", "pearai", "kiro", "antigravity"}
	return []pythonParser{
		{"claude-code", true, false, true, true, nil}, {"claude-desktop", false, false, false, true, nil},
		{"codex-cli", true, false, false, true, nil}, {"antigravity", false, false, false, true, nil},
		{"qwen-code", false, true, false, true, nil}, {"kiro", false, false, false, true, nil},
		{"gemini-cli", false, false, false, true, nil}, {"crush", false, true, false, false, nil},
		{"goose", false, true, false, true, nil}, {"continue", false, false, false, true, nil},
		{"aider", false, false, false, true, nil}, {"zed", false, false, false, false, nil},
		{"vscode", true, false, true, true, nil}, {"opencode", true, false, false, true, nil},
		{"kilo-code", true, false, false, true, nil}, {"cline", true, false, true, true, nil},
		{"roo-code", false, false, false, true, editors}, {"tabby", false, false, false, true, editors},
		{"openhands", false, false, false, true, nil}, {"shellgpt", false, false, false, true, nil},
		{"pi", true, false, false, true, nil}, {"little-coder", true, true, false, true, nil},
		{"letta", true, true, false, true, nil}, {"hermes", false, true, false, true, nil},
		{"agent-zero", true, true, true, true, nil}, {"open-interpreter", true, false, false, true, nil},
		{"openclaw", true, true, false, true, nil}, {"nanobot", false, true, false, true, nil},
		{"cody", false, false, false, true, editors}, {"twinny", false, false, false, true, editors},
		{"pearai", false, false, false, true, nil}, {"muse-code", false, false, false, true, nil},
		{"ollama", false, false, false, false, []string{"claude-desktop"}},
		{"copilot-cli", true, false, false, true, nil}, {"copilot-desktop", false, false, false, false, nil},
		{"chatgpt-desktop", true, false, true, false, []string{"codex-cli"}},
		{"cursor", true, false, true, true, nil},
	}
}

// callKey names one artifact a parser was offered: the parser's agent and
// the artifact's original path and home-relative path.
type callKey struct {
	agent, original, rel string
}

// recordedCalls holds a node's recorded parser calls by artifact: the
// parses of one command run in their order, and the secondary and
// tolerance answers of the whole node, which the recorder writes once.
type recordedCalls struct {
	parses     map[callKey][]ParserCase
	secondary  map[callKey]bool
	tolerances map[callKey]float64
	root       func(Text) string
	// parsed holds the sequence numbers of the recorded parses in the
	// order the run made them.
	parsed []int
}

// recordedParser answers as the analyzer's parser did in the recording:
// it wants an artifact it parsed, and emits the rows the parse gave.
type recordedParser struct {
	parser.Base
	flags pythonParser
	calls *recordedCalls
}

// key returns the call key of an artifact for this parser.
func (p recordedParser) key(artifact *input.Artifact) callKey {
	return callKey{p.Agent(), artifact.Original, artifact.Rel}
}

// Wants reports whether the recording parsed the artifact with this
// parser.
func (p recordedParser) Wants(artifact *input.Artifact) bool {
	return len(p.calls.parses[p.key(artifact)]) > 0
}

// Marks returns the marks the analyzer's parser records.
func (p recordedParser) Marks() parser.Marks {
	return parser.Marks{Sync: p.flags.sync, Relay: p.flags.relay, Origin: p.flags.origin}
}

// ReadsAgents returns the other agents the analyzer's parser is offered.
func (p recordedParser) ReadsAgents() []string { return p.flags.readsAgents }

// Parse emits the rows of the next recorded parse of the artifact, and an
// operating-system error the parse raised as the error its reason names.
func (p recordedParser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	key := p.key(artifact)
	queue := p.calls.parses[key]
	if len(queue) == 0 {
		return nil
	}
	recorded := queue[0]
	p.calls.parses[key] = queue[1:]
	p.calls.parsed = append(p.calls.parsed, recorded.Seq)
	for _, row := range recorded.Result.Rows {
		emit(p.calls.row(row))
	}
	if failure := recorded.Result.Error; failure != nil {
		return recordedError(string(failure.Text))
	}
	return nil
}

// secondaryParser is a recorded parser whose analyzer parser reads a
// secondary source.
type secondaryParser struct {
	recordedParser
}

// Secondary gives the recorded answer.
func (p secondaryParser) Secondary(artifact *input.Artifact) bool {
	return p.calls.secondary[p.key(artifact)]
}

// SecondaryTolerance gives the recorded answer.
func (p secondaryParser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if tolerance, ok := p.calls.tolerances[p.key(artifact)]; ok {
		return tolerance
	}
	return parser.DefaultTolerance
}

// hookedAgents are the agents whose analyzer parser declares deduplication
// hooks that change a recorded run's result, which a recorded parser does
// not have. claude-code's hooks change none.
func hookedAgents() []string {
	return []string{"qwen-code"}
}

// archivedRoot reports whether a run reads an archive whose artifacts'
// paths hold the recording's root: compressed, the archive kept the
// recording's own path, which the run's root cannot stand in for.
func (c *recordedCalls) archivedRoot(arguments []Text, root string) bool {
	if !slices.ContainsFunc(arguments, func(argument Text) bool {
		return strings.HasSuffix(string(argument), ".tar.gz") || strings.HasSuffix(string(argument), ".zip")
	}) {
		return false
	}
	for key := range c.parses {
		if strings.HasPrefix(key.original, root) {
			return true
		}
	}
	return false
}

// needsHooks reports whether a run has secondary rows of an agent whose
// deduplication needs its parser's hooks.
func (c *recordedCalls) needsHooks() bool {
	for key, secondary := range c.secondary {
		if secondary && slices.Contains(hookedAgents(), key.agent) && len(c.parses[key]) > 0 {
			return true
		}
	}
	return false
}

// recordedError returns the error a Python OSError's text stands for, so
// its problem line gives the analyzer's reason.
func recordedError(text string) error {
	switch {
	case strings.Contains(text, "[Errno 13]"):
		return fs.ErrPermission
	case strings.Contains(text, "[Errno 2]"):
		return fs.ErrNotExist
	}
	return errors.New(text)
}

// row returns a recorded row as a timeline row, its paths under the run's
// root.
func (c *recordedCalls) row(row Row) timeline.Row {
	return timeline.Row{TimestampUTC: c.root(row.TimestampUTC), Host: c.root(row.Host),
		User: c.root(row.User), Agent: c.root(row.Agent), SessionID: c.root(row.SessionID),
		ProjectPath: c.root(row.ProjectPath), GitBranch: c.root(row.GitBranch),
		TurnType: c.root(row.TurnType), Model: c.root(row.Model), ToolName: c.root(row.ToolName),
		ToolUseID: c.root(row.ToolUseID), Text: c.root(row.Text), SourceFile: c.root(row.SourceFile),
		SourceLine: row.SourceLine, Recovered: c.root(row.Recovered)}
}

// recordedRegistry returns the analyzer's parser list answering from the
// recorded calls.
func recordedRegistry(calls *recordedCalls) *parsers.Registry {
	var list []parser.Parser
	for _, flags := range pythonParsers() {
		each := recordedParser{Base: parser.NewBase(flags.agent, "recorded"), flags: flags, calls: calls}
		// Pointers, since the registry compares parser values.
		if flags.reading {
			list = append(list, &secondaryParser{each})
		} else {
			list = append(list, &each)
		}
	}
	return parsers.NewOf(list)
}

// nodeCalls holds every recorded parser call by node, and the sequence
// numbers of each node's command runs.
type nodeCalls struct {
	calls    map[string][]ParserCase
	commands map[string][]int
}

// readNodeCalls reads the parser calls of the nodes that ran timeline.
func readNodeCalls(t *testing.T, corpus *Corpus, nodes map[string]bool) nodeCalls {
	found := nodeCalls{calls: map[string][]ParserCase{}, commands: map[string][]int{}}
	for _, command := range caseFiles(t, corpus, "commands") {
		for _, recorded := range corpus.CommandCases(t, command) {
			found.commands[recorded.Node] = append(found.commands[recorded.Node], recorded.Seq)
		}
	}
	for _, agent := range caseFiles(t, corpus, "parsers") {
		for _, recorded := range corpus.ParserCases(t, agent) {
			if nodes[recorded.Node] {
				found.calls[recorded.Node] = append(found.calls[recorded.Node], recorded)
			}
		}
	}
	return found
}

// forRun returns the calls of one command run: its node's parses after
// the node's previous command run, and the node's secondary answers.
func (n nodeCalls) forRun(recorded CommandCase, root func(Text) string) *recordedCalls {
	previous := -1
	for _, seq := range n.commands[recorded.Node] {
		if seq < recorded.Seq {
			previous = max(previous, seq)
		}
	}
	calls := &recordedCalls{parses: map[callKey][]ParserCase{}, secondary: map[callKey]bool{},
		tolerances: map[callKey]float64{}, root: root}
	runCalls := slices.Clone(n.calls[recorded.Node])
	slices.SortStableFunc(runCalls, func(left, right ParserCase) int { return left.Seq - right.Seq })
	for _, each := range runCalls {
		key := callKey{each.Agent, root(each.Artifact.Original), string(each.Artifact.Rel)}
		switch each.Method {
		case "parse":
			// A test may parse an artifact itself before the run; the run's
			// own parse of it is the last one.
			if each.Seq > previous && each.Seq < recorded.Seq {
				calls.parses[key] = []ParserCase{each}
			}
		case "secondary":
			calls.secondary[key] = string(each.Result.Value) == "true"
		case "secondary_tolerance":
			var tolerance float64
			if _, err := fmt.Sscan(string(each.Result.Value), &tolerance); err == nil {
				calls.tolerances[key] = tolerance
			}
		}
	}
	return calls
}

func TestTimelineCommandsOverRecordedParserRows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command cases print the POSIX paths of the recording; they run on Linux and macOS")
	}
	corpus := Open(t)
	replay := newCommandReplay(t, corpus)
	cases := corpus.CommandCases(t, "timeline")
	nodes := map[string]bool{}
	for _, recorded := range cases {
		nodes[recorded.Node] = true
	}
	calls := readNodeCalls(t, corpus, nodes)
	for _, recorded := range cases {
		t.Run(fmt.Sprint(recorded.Seq), func(t *testing.T) {
			node := fmt.Sprintf("%s (seq %d)", recorded.Node, recorded.Seq)
			if reason := replay.notComparable(t, node, recorded); reason != "" {
				replay.skipped[reason]++
				return
			}
			replay.replayRecordedRows(t, node, recorded, calls)
		})
	}
	t.Logf("%d timeline cases compared over the recorded parser rows", replay.full["timeline"])
	replay.report(t)
}

// replayRecordedRows runs one timeline case with the recorded parsers and
// compares everything it gave.
func (r *commandReplay) replayRecordedRows(t *testing.T, node string, recorded CommandCase,
	calls nodeCalls) {
	root := r.corpus.Materialise(t, recorded.Tree, recorded.Times)
	run := calls.forRun(recorded, func(text Text) string { return r.corpus.Expand(text, root) })
	switch {
	case run.needsHooks():
		r.skipped["secondary rows of an agent whose parser declares deduplication hooks"]++
		return
	case run.archivedRoot(recorded.Argv, root):
		r.skipped[commandArchivedRoot]++
		return
	}
	r.registryOverride = func() *parsers.Registry { return recordedRegistry(run) }
	defer func() { r.registryOverride = nil }()
	r.full["timeline"]++
	defer r.noteBlocks(t, recorded, nil)
	before := filesUnder(root)
	got := r.run(t, "timeline", recorded, root)
	want := r.expected(t, recorded, root)
	if !slices.IsSorted(run.parsed) {
		// A loose tree is listed in the file system's order, which decides
		// which of two equal secondary rows is kept.
		r.full["timeline"]--
		r.skipped["artifacts listed in another order than the recording's"]++
		return
	}
	compareFull(t, node, root, want, got)
	checkNoOtherOutput(t, node, root, before, want.outputs)
}
