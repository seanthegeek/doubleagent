package openclaw

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/zed"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// The state directory and the sessions directory below a home, and the
// suffixes of an archived transcript and of a copy of one.
const (
	state         = `(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)`
	sessions      = `\A` + state + `/(?:agents/[^/]+/)?sessions/`
	archiveSuffix = `\.jsonl\.(reset|deleted)\.[0-9]{4}-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]-[0-9][0-9]-` +
		`[0-9][0-9](?:\.[0-9]{3})?Z(?:\.[0-9a-f]{32})?(?:\.zst)?`
	// copySuffix is a pre-compaction checkpoint or a pre-doctor repair
	// backup.
	copySuffix = `\.checkpoint\.[0-9a-fA-F-]{36}\.jsonl` +
		`|\.jsonl\.pre-doctor-(?:branch|openai-codex)-repair-` +
		`[0-9]{4}-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]-[0-9][0-9]-[0-9][0-9]-[0-9]{3}Z\.bak`
)

// patterns holds the files the parser reads, by their path relative to the
// home.
type patterns struct {
	agentDatabase *parserkit.RelPattern
	stateDatabase *parserkit.RelPattern
	transcript    *parserkit.RelPattern
	cold          *parserkit.RelPattern
}

// newPatterns compiles the patterns.
func newPatterns() patterns {
	return patterns{
		agentDatabase: parserkit.MustRelPattern(`\A` + state + `/agents/[^/]+/agent/openclaw-agent\.sqlite\z`),
		stateDatabase: parserkit.MustRelPattern(`\A` + state + `/state/openclaw\.sqlite\z`),
		transcript: parserkit.MustRelPattern(sessions + `(?P<name>[^/]+?)(?:(?P<archive>` + archiveSuffix +
			`)|(?P<copy>` + copySuffix + `)|\.jsonl)\z`),
		cold: parserkit.MustRelPattern(sessions + `cold/[0-9a-f]{64}\.jsonl\.zst\z`),
	}
}

// transcriptName holds the parts of a transcript file's name: the session
// id it names, and its archive or copy suffix when it has one.
type transcriptName struct {
	name, archive, copy string
}

// matchTranscript returns the parts of a transcript file's name, and false
// for any other file.
func (p *Parser) matchTranscript(rel string) (transcriptName, bool) {
	match := p.files.transcript.FindStringSubmatch(rel)
	if match == nil {
		return transcriptName{}, false
	}
	pattern := p.files.transcript
	return transcriptName{name: match[pattern.SubexpIndex("name")],
		archive: match[pattern.SubexpIndex("archive")],
		copy:    match[pattern.SubexpIndex("copy")]}, true
}

// Parser reads OpenClaw's databases and transcripts.
type Parser struct {
	parserkit.Base
	files patterns
	// budgetFloor is the least decompression budget of a database or file
	// (zed.BudgetFloor; a test lowers it).
	budgetFloor int
	// decoder decodes the zstd blobs of every database and file the parser
	// reads, so the window a frame announces is reserved once for the run.
	decoder zed.SharedDecoder
	// legacyIndexes holds each legacy sessions.json read, by its state.
	legacyIndexes map[fileState]*legacyIndex
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("openclaw", "openclaw"), files: newPatterns(),
		budgetFloor: zed.BudgetFloor}
}

// Marks says the parser records synced and relayed sessions.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Relay: true}
}

// Kinds returns the agent and state databases, the cold transcripts, and
// the transcripts other than trajectories.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("openclaw-agent.sqlite", p.files.agentDatabase),
		parserkit.PatternKind("cold transcript", p.files.cold),
		parserkit.PatternKind("openclaw.sqlite", p.files.stateDatabase),
		parserkit.RelFuncKind("transcript", func(rel string) bool {
			name, ok := p.matchTranscript(rel)
			return ok && !textlines.HasSuffixFoldASCII(name.name, ".trajectory")
		})}
}

// Wants reports whether the artifact is an agent or state database, a cold
// archive or a transcript file; a trajectory transcript is not read.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is a published archive, a
// checkpoint or a pre-doctor copy; the databases and the plain legacy
// transcript are primary.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	name, ok := p.matchTranscript(artifact.Rel)
	return ok && (name.archive != "" || name.copy != "")
}

// Parse emits the rows of one OpenClaw file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	switch {
	case p.files.agentDatabase.MatchString(artifact.Rel):
		p.parseAgentDatabase(artifact, options, emit)
	case p.files.stateDatabase.MatchString(artifact.Rel):
		p.parseStateDatabase(artifact, emit)
	default:
		p.parseFile(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
