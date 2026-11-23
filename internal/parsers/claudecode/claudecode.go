// Package claudecode reads Claude Code's transcripts and the files beside
// them, under ~/.claude or a CLAUDE_CONFIG_DIR home ~/.claude-<name>.
//
// The transcripts are projects/<slug>/<session>.jsonl and
// <session>/subagents/[<dir>/]agent-<id>.jsonl, with the full text of
// persisted tool outputs in <session>/tool-results/<name>.txt. Beside them:
// ccr-tip.json and bridge-pointer.json under projects/ (Remote Control),
// history.jsonl (a secondary source, the prompts as typed) with its
// paste-cache/, the process registry sessions/<pid>.json, the user and
// project settings[.local].json and ~/.claude.json. The rows are user,
// assistant, thinking, tool_use, tool_result and system rows, the system
// rows carrying surface, origin, sync, permission, model, prompt trigger,
// title, scratch directory and setting changes. Only the settings keys
// named here are read: several others can hold credentials.
//
// A history line and its transcript record differ in form for slash
// commands and pastes, and a command's record can come long after its line:
// the parser declares deduplication hooks (MatchForm, MatchWindow) for that.
//
// Reader holds the readers of these formats under any agent name, since the
// Claude Desktop app keeps the same transcripts; ToolSummary and ResultText
// are shared with the parsers of other agents that write Claude's tool
// blocks. See research/claude-code.md.
package claudecode

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The artifacts the parser reads, by path relative to the home. A
// Remote Control sidecar ("ccr" is Claude Code Remote) is ccr-tip.json in a
// session's directory or bridge-pointer.json in a project's.
var (
	sessionPattern       = regexp.MustCompile(`^\.claude(?:-[^/]+)?/projects/[^/]+/.*\.jsonl\z`)
	subagentPattern      = regexp.MustCompile(`/subagents/(?:[^/]+/)*agent-([^/]+)\.jsonl\z`)
	historyPattern       = regexp.MustCompile(`^\.claude(?:-[^/]+)?/history\.jsonl\z`)
	registryPattern      = regexp.MustCompile(`^\.claude(?:-[^/]+)?/sessions/[0-9]+\.json\z`)
	settingsPattern      = regexp.MustCompile(`^\.claude(?:-[^/]+)?/settings(?:\.local)?\.json\z`)
	globalConfigPattern  = regexp.MustCompile(`^(?:\.claude-[^/]+/)?\.claude\.json\z`)
	ccrTipPattern        = regexp.MustCompile(`^\.claude(?:-[^/]+)?/projects/[^/]+/([^/]+)/ccr-tip\.json\z`)
	bridgePointerPattern = regexp.MustCompile(`^\.claude(?:-[^/]+)?/projects/[^/]+/bridge-pointer\.json\z`)
)

// Reader reads Claude Code's file formats into rows of the agent its Base
// names. The claude-code parser is one; the claude-desktop parser holds one
// for the transcripts the desktop app keeps in the same format.
type Reader struct {
	parserkit.Base
	// outputs holds, per home, the persisted tool outputs its transcripts
	// named. They are kept for the pass over the home's files, since a
	// session's subagent transcripts are files of their own that share its
	// tool-results/.
	outputs *parserkit.PerHome[persistedFiles]
	// pastes holds, per home, the paste-cache files its history lines named
	// (pasteOf).
	pastes *parserkit.PerHome[pasteFiles]
}

// NewReader returns a Reader whose rows carry base's agent.
func NewReader(base parserkit.Base) *Reader {
	return &Reader{Base: base, outputs: parserkit.NewPerHome(func() persistedFiles { return persistedFiles{} }),
		pastes: parserkit.NewPerHome(func() pasteFiles { return pasteFiles{} })}
}

// Parser reads Claude Code's transcripts and the history, registry and
// settings beside them.
type Parser struct {
	Reader
}

// New returns the parser.
func New() *Parser {
	return &Parser{Reader: *NewReader(parserkit.NewBase("claude-code", "claude-code"))}
}

// Marks says the parser records synced sessions (a Remote Control bridge
// puts the session on claude.ai) and sessions of remote origin (a Remote
// Control spawn or a teleported cloud session began on claude.ai).
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Origin: true}
}

// Wants reports whether the artifact is a transcript, the history, a
// registry entry, a settings file, ~/.claude.json or a Remote Control
// sidecar.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the history, the session transcripts, the session
// registry, the settings, the global config, the remote tips and the
// bridge pointers.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("history.jsonl", historyPattern),
		parserkit.PatternKind("session transcript", sessionPattern),
		parserkit.PatternKind("session registry", registryPattern), parserkit.PatternKind("settings.json", settingsPattern),
		parserkit.PatternKind(".claude.json", globalConfigPattern), parserkit.PatternKind("ccr-tip.json", ccrTipPattern),
		parserkit.PatternKind("bridge-pointer.json", bridgePointerPattern)}
}

// Secondary reports whether the artifact is history.jsonl.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return historyPattern.MatchString(artifact.Rel)
}

// DedupHooks returns the match form and the forward window under which a
// history line meets its transcript record.
func (p *Parser) DedupHooks() parserkit.AgentHooks {
	return parserkit.AgentHooks{Form: MatchForm, Window: MatchWindow}
}

// Parse emits the rows of one Claude Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case historyPattern.MatchString(rel):
		p.history(artifact, emit)
	case registryPattern.MatchString(rel):
		p.registry(artifact, emit)
	case settingsPattern.MatchString(rel):
		p.settings(artifact, emit)
	case globalConfigPattern.MatchString(rel):
		p.globalConfig(artifact, emit)
	case ccrTipPattern.MatchString(rel):
		p.ccrTip(artifact, emit)
	case bridgePointerPattern.MatchString(rel):
		p.bridgePointer(artifact, emit)
	default:
		p.Session(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
	_ parserkit.HooksDeclarer   = (*Parser)(nil)
)
