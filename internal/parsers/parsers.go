// Package parsers is the registry of the agent parsers: the list in the
// order that decides ties in the output, the routing of an artifact to the
// parsers offered it, and the run of every parser over a collection.
//
// A parser is a value created for one run, since some keep a cache of the
// files they have read; All returns new values each time, and a Registry
// holds one run's values so that every artifact of the run reaches the same
// ones.
package parsers

import (
	"context"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/chatgptdesktop"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// constructors returns the constructor of every parser, in registry order:
// the order of the analyzer's parser list, which decides the order of rows
// whose sort keys tie. A new parser adds its line at its place in that
// order.
func constructors() []func() parser.Parser {
	return []func() parser.Parser{
		func() parser.Parser { return claudecode.New() },
		func() parser.Parser { return chatgptdesktop.New() },
	}
}

// All returns a new value of every parser, in registry order.
func All() []parser.Parser {
	makers := constructors()
	all := make([]parser.Parser, 0, len(makers))
	for _, construct := range makers {
		all = append(all, construct())
	}
	return all
}

// ByAgent returns the parsers of list by their agent, each agent's in list
// order.
func ByAgent(list []parser.Parser) map[string][]parser.Parser {
	byAgent := map[string][]parser.Parser{}
	for _, each := range list {
		byAgent[each.Agent()] = append(byAgent[each.Agent()], each)
	}
	return byAgent
}

// NoTranscripts returns the catalog agents whose collected files hold
// nothing to parse, which the analyzer reports as collected only rather
// than as parsers still to write: the "collected" rows of the agent table
// in docs/agents.md.
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
	all     []parser.Parser
	byAgent map[string][]parser.Parser
}

// New returns a registry of new parser values.
func New() *Registry {
	return NewOf(All())
}

// NewOf returns a registry of the given parsers, in their order.
func NewOf(list []parser.Parser) *Registry {
	return &Registry{all: list, byAgent: ByAgent(list)}
}

// All returns the run's parsers in registry order.
func (r *Registry) All() []parser.Parser {
	return slices.Clone(r.all)
}

// For returns the parsers offered an artifact of the agent: every parser
// for a project file no single agent owns (input.ProjectAgent), else the
// agent's own parsers followed by the others that read its artifacts
// (parser.AgentReader), in registry order.
func (r *Registry) For(agent string) []parser.Parser {
	if agent == input.ProjectAgent {
		return r.All()
	}
	offered := slices.Clone(r.byAgent[agent])
	own := len(offered)
	for _, each := range r.all {
		if slices.Contains(parser.ReadsAgentsOf(each), agent) && !slices.Contains(offered[:own], each) {
			offered = append(offered, each)
		}
	}
	return offered
}

// Hooks returns the deduplication hooks the parsers declare, by agent.
func (r *Registry) Hooks() dedup.Hooks {
	hooks := dedup.Hooks{}
	for _, each := range r.all {
		if declarer, ok := each.(parser.HooksDeclarer); ok {
			hooks[each.Agent()] = declarer.DedupHooks()
		}
	}
	return hooks
}

// Marks returns, for each session mark, the agents whose parser records it,
// and the agents that read a secondary source.
func (r *Registry) Marks() timeline.MarkAgents {
	marks := timeline.MarkAgents{Synced: map[string]bool{}, Relayed: map[string]bool{},
		Origin: map[string]bool{}, Secondary: map[string]bool{}}
	for _, each := range r.all {
		recorded, agent := parser.MarksOf(each), each.Agent()
		addWhen(marks.Synced, agent, recorded.Sync)
		addWhen(marks.Relayed, agent, recorded.Relay)
		addWhen(marks.Origin, agent, recorded.Origin)
		addWhen(marks.Secondary, agent, parser.HasSecondary(each))
	}
	return marks
}

// addWhen adds agent to the set when holds is true.
func addWhen(set map[string]bool, agent string, holds bool) {
	if holds {
		set[agent] = true
	}
}

// Secondary is one row of a secondary source, with the matching window of
// the source it came from in seconds: the deduplicator's input row.
type Secondary = dedup.Secondary

// Result is what Collect gathers: the rows of primary sources and of
// secondary sources, each in parse order, and the problem lines, the
// collection's own first.
type Result struct {
	Primary   []timeline.Row
	Secondary []Secondary
	Problems  []string
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
	options parser.Options, agents []string) (Result, error) {
	result := Result{Problems: slices.Clone(collection.Problems)}
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

// collectOne runs one parser over one artifact and adds its rows, or its
// problem, to result.
func (r *Registry) collectOne(each parser.Parser, artifact *input.Artifact,
	options parser.Options, result *Result) {
	secondary := parser.IsSecondary(each, artifact)
	tolerance := 0.0
	if secondary {
		tolerance = parser.Tolerance(each, artifact)
	}
	err := each.Parse(artifact, options, func(row timeline.Row) {
		row.ProjectPath = timeline.CanonicalProjectPath(row.ProjectPath)
		if secondary {
			result.Secondary = append(result.Secondary, Secondary{Row: row, Tolerance: tolerance})
		} else {
			result.Primary = append(result.Primary, row)
		}
	})
	if err != nil {
		result.Problems = append(result.Problems, artifact.Original+": "+reason.For(err))
	}
}
