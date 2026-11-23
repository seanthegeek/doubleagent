package codex

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Settings are what tell one agent of the Codex format from another.
type Settings struct {
	// Agent is the catalog agent the rows carry, and the parser's name.
	Agent string
	// HomePrefix is the directory under the home that holds the stores.
	HomePrefix string
	// LedgerName is the import ledger's file name under HomePrefix, "" when
	// the agent has none.
	LedgerName string
	// Automations reads automations/<slug>/automation.toml.
	Automations bool
	// GlobalState reads .codex-global-state.json.
	GlobalState bool
}

// globalStateName is the desktop app's state file under the Codex home.
const globalStateName = ".codex-global-state.json"

// Parser reads the stores of one Codex-format agent.
type Parser struct {
	parserkit.Base
	settings Settings
	paths    paths
	// guardians holds the guardian indexes of each Codex home's log
	// databases, by home, threads the threads of each state database
	// read, by path, and groups the group of each thread id a state
	// database repeats, by the database's path and the id.
	guardians map[string][]*guardianIndex
	threads   map[string]*threadsFile
	groups    map[[2]string]*timeline.Group
	// zst reads every compressed rollout of the run (zstRecords).
	zst *zstReader
}

// paths are the relative paths and patterns of the files the parser reads,
// built from its home prefix.
type paths struct {
	rollout, automation, logs, rules, memory, backupState      *regexp.Regexp
	config, history, state, threadHistory, ledger, globalState string
}

// New returns the parser of Codex CLI and the Codex Desktop app.
func New() *Parser {
	return NewRollout(Settings{Agent: "codex-cli", HomePrefix: ".codex", Automations: true,
		GlobalState: true})
}

// NewRollout returns a parser of the Codex format with the given settings.
func NewRollout(settings Settings) *Parser {
	prefix := settings.HomePrefix
	pattern := func(rest string) *regexp.Regexp {
		return regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + rest + `\z`)
	}
	built := paths{
		rollout:       pattern(`/(?:archived_)?sessions/(?:.*/)?rollout-[^/]*\.jsonl(?:\.zst)?`),
		automation:    pattern(`/automations/[^/]+/automation\.toml`),
		logs:          pattern(`/logs_[0-9]+\.sqlite`),
		rules:         pattern(`/rules/[^/]+\.rules`),
		memory:        pattern(`/memories/rollout_summaries/[^/]+\.md`),
		backupState:   pattern(`/db-backups/[^/]+/state_5\.sqlite`),
		config:        prefix + "/config.toml",
		history:       prefix + "/history.jsonl",
		state:         prefix + "/state_5.sqlite",
		threadHistory: prefix + "/thread_history_1.sqlite",
	}
	if settings.LedgerName != "" {
		built.ledger = prefix + "/" + settings.LedgerName
	}
	if settings.GlobalState {
		built.globalState = prefix + "/" + globalStateName
	}
	return &Parser{Base: parserkit.NewBase(settings.Agent, settings.Agent), settings: settings,
		paths: built}
}

// Marks says the parser records synced sessions: a /feedback report in
// logs_<N>.sqlite marks a thread sent to the vendor's error tracker.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the files the parser reads: the import ledger, the
// automations and the desktop app's state only where the settings name
// them.
func (p *Parser) Kinds() []parserkit.Kind {
	built := p.paths
	kinds := []parserkit.Kind{parserkit.RelKind("history.jsonl", built.history),
		parserkit.RelKind("state_5.sqlite", built.state), parserkit.RelKind("thread_history_1.sqlite", built.threadHistory),
		parserkit.RelFuncKind("rollout", func(rel string) bool {
			return built.rollout.MatchString(rel) && !strings.HasSuffix(rel, ".zst")
		}),
		parserkit.RelFuncKind("compressed rollout", func(rel string) bool {
			return built.rollout.MatchString(rel) && strings.HasSuffix(rel, ".zst")
		})}
	if built.ledger != "" {
		kinds = append(kinds, parserkit.RelKind("import ledger", built.ledger))
	}
	if p.settings.Automations {
		kinds = append(kinds, parserkit.PatternKind("automation.toml", built.automation))
	}
	kinds = append(kinds, parserkit.PatternKind("logs database", built.logs))
	if built.globalState != "" {
		kinds = append(kinds, parserkit.RelKind(globalStateName, built.globalState))
	}
	return append(kinds, parserkit.RelKind("config.toml", built.config), parserkit.PatternKind("rules", built.rules),
		parserkit.PatternKind("rollout summary", built.memory), parserkit.PatternKind("state backup", built.backupState))
}

// Secondary reports whether the artifact repeats events the rollouts
// record: the prompt history, the SQLite projections and their backups, and
// the desktop app's state.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	rel, built := artifact.Rel, p.paths
	return rel == built.history || rel == built.state || rel == built.threadHistory ||
		(built.globalState != "" && rel == built.globalState) ||
		built.backupState.MatchString(rel)
}

// Parse emits the rows of one artifact, read as the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel, built := artifact.Rel, p.paths
	switch {
	case rel == built.history:
		p.parseHistory(artifact, emit)
	case rel == built.state || built.backupState.MatchString(rel):
		p.parseState(artifact, emit)
	case rel == built.threadHistory:
		p.parseThreadHistory(artifact, options, emit)
	case p.settings.Automations && built.automation.MatchString(rel):
		p.parseAutomation(artifact, emit)
	case built.logs.MatchString(rel):
		p.parseLogs(artifact, emit)
	case built.ledger != "" && rel == built.ledger:
		p.parseLedger(artifact, emit)
	case built.globalState != "" && rel == built.globalState:
		p.parseGlobalState(artifact, emit)
	case rel == built.config:
		p.parseConfig(artifact, emit)
	case built.rules.MatchString(rel):
		p.parseRules(artifact, emit)
	case built.memory.MatchString(rel):
		p.parseMemorySummary(artifact, emit)
	default:
		p.parseRollout(artifact, options, emit)
	}
	return nil
}

// homeDepth returns how many directories the home prefix spans.
func (p *Parser) homeDepth() int {
	return strings.Count(p.settings.HomePrefix, "/") + 1
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
