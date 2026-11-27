package parsers

import (
	"context"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/agentzero"
	"github.com/seanthegeek/doubleagent/internal/parsers/aider"
	"github.com/seanthegeek/doubleagent/internal/parsers/antigravity"
	"github.com/seanthegeek/doubleagent/internal/parsers/chatgptdesktop"
	"github.com/seanthegeek/doubleagent/internal/parsers/claudecode"
	"github.com/seanthegeek/doubleagent/internal/parsers/claudedesktop"
	"github.com/seanthegeek/doubleagent/internal/parsers/cline"
	"github.com/seanthegeek/doubleagent/internal/parsers/codex"
	"github.com/seanthegeek/doubleagent/internal/parsers/cody"
	"github.com/seanthegeek/doubleagent/internal/parsers/continuedev"
	"github.com/seanthegeek/doubleagent/internal/parsers/copilotcli"
	"github.com/seanthegeek/doubleagent/internal/parsers/copilotdesktop"
	"github.com/seanthegeek/doubleagent/internal/parsers/crush"
	"github.com/seanthegeek/doubleagent/internal/parsers/cursor"
	"github.com/seanthegeek/doubleagent/internal/parsers/geminicli"
	"github.com/seanthegeek/doubleagent/internal/parsers/goose"
	"github.com/seanthegeek/doubleagent/internal/parsers/hermes"
	"github.com/seanthegeek/doubleagent/internal/parsers/kilocode"
	"github.com/seanthegeek/doubleagent/internal/parsers/kiro"
	"github.com/seanthegeek/doubleagent/internal/parsers/letta"
	"github.com/seanthegeek/doubleagent/internal/parsers/littlecoder"
	"github.com/seanthegeek/doubleagent/internal/parsers/musecode"
	"github.com/seanthegeek/doubleagent/internal/parsers/nanobot"
	"github.com/seanthegeek/doubleagent/internal/parsers/ollama"
	"github.com/seanthegeek/doubleagent/internal/parsers/openclaw"
	"github.com/seanthegeek/doubleagent/internal/parsers/opencode"
	"github.com/seanthegeek/doubleagent/internal/parsers/openhands"
	"github.com/seanthegeek/doubleagent/internal/parsers/openinterpreter"
	"github.com/seanthegeek/doubleagent/internal/parsers/pearai"
	"github.com/seanthegeek/doubleagent/internal/parsers/pi"
	"github.com/seanthegeek/doubleagent/internal/parsers/qwencode"
	"github.com/seanthegeek/doubleagent/internal/parsers/roocode"
	"github.com/seanthegeek/doubleagent/internal/parsers/shellgpt"
	"github.com/seanthegeek/doubleagent/internal/parsers/tabby"
	"github.com/seanthegeek/doubleagent/internal/parsers/twinny"
	"github.com/seanthegeek/doubleagent/internal/parsers/vscode"
	"github.com/seanthegeek/doubleagent/internal/parsers/zed"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// constructors returns the constructor of every parser, in registry order,
// which decides the order of rows whose sort keys tie and so is part of the
// output: a new parser adds its line at its place, and no line moves.
func constructors() []func() parserkit.Parser {
	return []func() parserkit.Parser{
		func() parserkit.Parser { return claudecode.New() },
		func() parserkit.Parser { return claudedesktop.New() },
		func() parserkit.Parser { return codex.New() },
		func() parserkit.Parser { return antigravity.New() },
		func() parserkit.Parser { return qwencode.New() },
		func() parserkit.Parser { return kiro.New() },
		func() parserkit.Parser { return geminicli.New() },
		func() parserkit.Parser { return crush.New() },
		func() parserkit.Parser { return goose.New() },
		func() parserkit.Parser { return continuedev.New() },
		func() parserkit.Parser { return aider.New() },
		func() parserkit.Parser { return zed.New() },
		func() parserkit.Parser { return vscode.New() },
		func() parserkit.Parser { return opencode.New() },
		func() parserkit.Parser { return kilocode.New() },
		func() parserkit.Parser { return cline.New() },
		func() parserkit.Parser { return roocode.New() },
		func() parserkit.Parser { return tabby.New() },
		func() parserkit.Parser { return openhands.New() },
		func() parserkit.Parser { return shellgpt.New() },
		func() parserkit.Parser { return pi.New() },
		func() parserkit.Parser { return littlecoder.New() },
		func() parserkit.Parser { return letta.New() },
		func() parserkit.Parser { return hermes.New() },
		func() parserkit.Parser { return agentzero.New() },
		func() parserkit.Parser { return openinterpreter.New() },
		func() parserkit.Parser { return openclaw.New() },
		func() parserkit.Parser { return nanobot.New() },
		func() parserkit.Parser { return cody.New() },
		func() parserkit.Parser { return twinny.New() },
		func() parserkit.Parser { return pearai.New() },
		func() parserkit.Parser { return musecode.New() },
		func() parserkit.Parser { return ollama.New() },
		func() parserkit.Parser { return copilotcli.New() },
		func() parserkit.Parser { return copilotdesktop.New() },
		func() parserkit.Parser { return chatgptdesktop.New() },
		func() parserkit.Parser { return cursor.New() },
	}
}

// All returns a new value of every parser, in registry order.
func All() []parserkit.Parser {
	makers := constructors()
	all := make([]parserkit.Parser, 0, len(makers))
	for _, construct := range makers {
		all = append(all, construct())
	}
	return all
}

// ByAgent returns the parsers of list by their agent, each agent's in list
// order.
func ByAgent(list []parserkit.Parser) map[string][]parserkit.Parser {
	byAgent := map[string][]parserkit.Parser{}
	for _, each := range list {
		byAgent[each.Agent()] = append(byAgent[each.Agent()], each)
	}
	return byAgent
}

// NoTranscripts returns the catalog agents whose collected files hold
// nothing to parse, which the analysis summary reports as collected only
// rather than as parsers still to write: the "collected" rows of the agent
// table in docs/supported-agents.md.
func NoTranscripts() []string {
	return []string{
		"camel-ai", // an OAuth token and config; its chats are under the working directory
		"copilot",  // the language server's token store; Copilot Chat parses as vscode
		"metagpt",  // model configuration only; its workspace is under the working directory
		"shared",   // instruction and skill directories several agents share
	}
}

// Registry is the parsers of one run.
type Registry struct {
	all     []parserkit.Parser
	byAgent map[string][]parserkit.Parser
}

// NewOf returns a registry of the given parsers, in their order.
func NewOf(list []parserkit.Parser) *Registry {
	return &Registry{all: list, byAgent: ByAgent(list)}
}

// New returns a registry of new parser values.
func New() *Registry {
	return NewOf(All())
}

// All returns the run's parsers in registry order.
func (r *Registry) All() []parserkit.Parser {
	return slices.Clone(r.all)
}

// For returns the parsers offered an artifact of the agent: every parser
// for a project file no single agent owns (input.ProjectAgent), else the
// agent's own parsers followed by the others that read its artifacts
// (parserkit.AgentReader), in registry order. A parser is told apart from the
// agent's own by its agent, never by comparing parser values, which panics
// for a parser type Go cannot compare.
func (r *Registry) For(agent string) []parserkit.Parser {
	if agent == input.ProjectAgent {
		return r.All()
	}
	offered := slices.Clone(r.byAgent[agent])
	for _, each := range r.all {
		if each.Agent() != agent && slices.Contains(parserkit.ReadsAgentsOf(each), agent) {
			offered = append(offered, each)
		}
	}
	return offered
}

// Hooks returns the deduplication hooks the parsers declare, by agent.
func (r *Registry) Hooks() dedup.Hooks {
	hooks := dedup.Hooks{}
	for _, each := range r.all {
		if declarer, ok := each.(parserkit.HooksDeclarer); ok {
			hooks[each.Agent()] = declarer.DedupHooks()
		}
	}
	return hooks
}

// addWhen adds agent to the set when holds is true.
func addWhen(set map[string]bool, agent string, holds bool) {
	if holds {
		set[agent] = true
	}
}

// Marks returns, for each session mark, the agents whose parser records it,
// and the agents that read a secondary source.
func (r *Registry) Marks() timeline.MarkAgents {
	marks := timeline.MarkAgents{Synced: map[string]bool{}, Relayed: map[string]bool{},
		Origin: map[string]bool{}, Secondary: map[string]bool{}}
	for _, each := range r.all {
		recorded, agent := parserkit.MarksOf(each), each.Agent()
		addWhen(marks.Synced, agent, recorded.Sync)
		addWhen(marks.Relayed, agent, recorded.Relay)
		addWhen(marks.Origin, agent, recorded.Origin)
		addWhen(marks.Secondary, agent, parserkit.HasSecondary(each))
	}
	return marks
}

// Result is what Collect gathers: the rows of primary sources and of
// secondary sources, each in parse order, and the problem lines, the
// collection's own first. ProblemSecrets holds, for each problem line,
// what the parse that failed identified (nil for the collection's own),
// which the output replaces in the line.
type Result struct {
	Primary        []timeline.Row
	Secondary      []dedup.Secondary
	Problems       []string
	ProblemSecrets []*secrets.Found
}

// collectOne runs one parser over one artifact as one parse (parserkit.Collect)
// and adds its rows, or its problem, to result.
func (r *Registry) collectOne(each parserkit.Parser, artifact *input.Artifact,
	options parserkit.Options, result *Result) {
	secondary := parserkit.IsSecondary(each, artifact)
	tolerance := 0.0
	if secondary {
		tolerance = parserkit.Tolerance(each, artifact)
	}
	found, err := parserkit.Collect(each, artifact, options, func(row timeline.Row) {
		row.ProjectPath = timeline.CanonicalProjectPath(row.ProjectPath)
		if secondary {
			result.Secondary = append(result.Secondary, dedup.Secondary{Row: row, Tolerance: tolerance})
		} else {
			result.Primary = append(result.Primary, row)
		}
	})
	if err != nil {
		result.Problems = append(result.Problems, artifact.Original+": "+reason.For(err))
		result.ProblemSecrets = append(result.ProblemSecrets, found)
	}
}

// Collect parses every artifact of the collection with every parser that is
// offered it and wants it, in artifact order then registry order. agents,
// when not empty, keeps only the parsers of those agents (a parser's own
// agent, so a project file another agent's parser reads is kept under that
// parser's agent). Each row's project path gets its one spelling
// (timeline.CanonicalProjectPath). A parse that fails keeps the rows it
// emitted and adds the problem "<original path>: <reason>". Collect stops
// with the context's error when it is cancelled, checked per artifact.
func (r *Registry) Collect(ctx context.Context, collection *input.Collection,
	options parserkit.Options, agents []string) (Result, error) {
	result := Result{Problems: slices.Clone(collection.Problems),
		ProblemSecrets: make([]*secrets.Found, len(collection.Problems))}
	for _, artifact := range collection.Artifacts {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		for _, each := range r.For(artifact.Agent) {
			if len(agents) > 0 && !slices.Contains(agents, each.Agent()) {
				continue
			}
			if each.Wants(artifact) {
				r.collectOne(each, artifact, options, &result)
			}
		}
	}
	return result, nil
}
