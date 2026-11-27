package claudecode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The artifacts the parser reads, by path relative to the home. A
// Remote Control sidecar ("ccr" is Claude Code Remote) is ccr-tip.json in a
// session's directory or bridge-pointer.json in a project's.
var (
	sessionPattern       = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/projects/[^/]+/.*\.jsonl\z`)
	subagentPattern      = parserkit.MustRelPattern(`/subagents/(?:[^/]+/)*agent-([^/]+)\.jsonl\z`)
	historyPattern       = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/history\.jsonl\z`)
	registryPattern      = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/sessions/([0-9]+)\.json\z`)
	settingsPattern      = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/settings(?:\.local)?\.json\z`)
	globalConfigPattern  = parserkit.MustRelPattern(`^(?:\.claude-[^/]+/)?\.claude\.json\z`)
	ccrTipPattern        = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/projects/[^/]+/([^/]+)/ccr-tip\.json\z`)
	bridgePointerPattern = parserkit.MustRelPattern(`^\.claude(?:-[^/]+)?/projects/[^/]+/bridge-pointer\.json\z`)
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

// Wants reports whether the artifact is a transcript, the history, a
// registry entry, a settings file, ~/.claude.json or a Remote Control
// sidecar.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
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
