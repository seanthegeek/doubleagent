// Package codex reads the stores of Codex CLI and the Codex Desktop app
// under ~/.codex, and of the agents that fork Codex and keep its formats
// under a home of their own (Open Interpreter).
//
// The store of record is the rollout: one JSON line per event under
// sessions/ and archived_sessions/ (rollout-*.jsonl, or zstd-compressed
// .jsonl.zst, where a cut frame keeps the lines before the cut). Beside it
// the parser reads history.jsonl, the SQLite projections state_5.sqlite (and
// its copies under db-backups/) and thread_history_1.sqlite, the desktop
// app's .codex-global-state.json, all secondary sources that repeat what the
// rollout records; the tracing logs logs_<N>.sqlite (sent /feedback reports
// and approval decisions); and the standing settings of config.toml,
// rules/*.rules, memories/rollout_summaries/*.md, a fork's import ledger and
// Codex's automations/*/automation.toml.
//
// One package holds the rollout reader and the readers of the SQLite and
// JSON stores, which need each other: the store readers need the rollout's turn-item rows and guardian texts,
// and the rollout reader needs the log databases for a guardian review's
// governed call. A fork is the same parser built with other Settings. See
// research/codex-cli.md, Parser plan, for every row.
package codex

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	parser.Base
	settings Settings
	paths    paths
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
	return &Parser{Base: parser.NewBase(settings.Agent, settings.Agent), settings: settings,
		paths: built}
}

// Marks says the parser records synced sessions: a /feedback report in
// logs_<N>.sqlite marks a thread sent to the vendor's error tracker.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel, built := artifact.Rel, p.paths
	return rel == built.history || rel == built.state || rel == built.threadHistory ||
		built.rollout.MatchString(rel) ||
		(built.ledger != "" && rel == built.ledger) ||
		(p.settings.Automations && built.automation.MatchString(rel)) ||
		built.logs.MatchString(rel) ||
		(built.globalState != "" && rel == built.globalState) ||
		rel == built.config || built.rules.MatchString(rel) || built.memory.MatchString(rel) ||
		built.backupState.MatchString(rel)
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
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
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
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
