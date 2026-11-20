// Package openclaw reads OpenClaw (formerly Clawdbot and Moltbot): its
// agent databases, its Gateway state database and its JSONL transcripts,
// under the state directory .openclaw, .openclaw-<profile>, .clawdbot or
// .moltbot. Files read:
//
//   - agents/<id>/agent/openclaw-agent.sqlite: the entries of
//     transcript_events (event_json, or a zstd frame in event_zstd), the
//     generations of session_windows, the reset and deleted generations of
//     session_transcript_archives, the cold archives stored in the database
//     (or their chunks), session_nodes.entry_json, session_conversations
//     with conversations, session_pending_inputs, the full-text index's
//     content table (joined on the entry id, keeping what the transcript
//     lost), conversation_deliveries and memory_session_tombstones.
//   - state/openclaw.sqlite, the Gateway's state (stateQueries).
//   - [agents/<id>/]sessions/<id>.jsonl legacy transcripts, with the session
//     key from the sibling sessions.json, and sessions/cold/<sha256>.jsonl.zst.
//   - Secondary sources: the published .jsonl.reset.* and .jsonl.deleted.*
//     archives, .checkpoint.<uuid>.jsonl snapshots and
//     .pre-doctor-*-repair-*.bak copies.
//
// Entries share pi's format and model rows (pi.ModelText, pi.SwitchText).
// A PostgreSQL anchor database is one `parser:` row; credential tables are
// never read. ToolArgsText is shared with the nanobot parser. See
// research/openclaw.md.
package openclaw

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/zed"
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
	agentDatabase *regexp.Regexp
	stateDatabase *regexp.Regexp
	transcript    *regexp.Regexp
	cold          *regexp.Regexp
}

// newPatterns compiles the patterns.
func newPatterns() patterns {
	return patterns{
		agentDatabase: regexp.MustCompile(`\A` + state + `/agents/[^/]+/agent/openclaw-agent\.sqlite\z`),
		stateDatabase: regexp.MustCompile(`\A` + state + `/state/openclaw\.sqlite\z`),
		transcript: regexp.MustCompile(sessions + `(?P<name>[^/]+?)(?:(?P<archive>` + archiveSuffix +
			`)|(?P<copy>` + copySuffix + `)|\.jsonl)\z`),
		cold: regexp.MustCompile(sessions + `cold/[0-9a-f]{64}\.jsonl\.zst\z`),
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
	parser.Base
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
	return &Parser{Base: parser.NewBase("openclaw", "openclaw"), files: newPatterns(),
		budgetFloor: zed.BudgetFloor}
}

// Marks says the parser records synced and relayed sessions.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Relay: true}
}

// Wants reports whether the artifact is an agent or state database, a cold
// archive or a transcript file; a trajectory transcript is not read.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if p.files.agentDatabase.MatchString(rel) || p.files.cold.MatchString(rel) ||
		p.files.stateDatabase.MatchString(rel) {
		return true
	}
	name, ok := p.matchTranscript(rel)
	return ok && !strings.HasSuffix(name.name, ".trajectory")
}

// Secondary reports whether the artifact is a published archive, a
// checkpoint or a pre-doctor copy; the databases and the plain legacy
// transcript are primary.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	name, ok := p.matchTranscript(artifact.Rel)
	return ok && (name.archive != "" || name.copy != "")
}

// Parse emits the rows of one OpenClaw file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
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
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
