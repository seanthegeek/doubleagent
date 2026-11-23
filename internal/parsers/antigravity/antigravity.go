// Package antigravity reads the Antigravity CLI's (agy) and desktop app's
// conversations, histories and permissions.
//
// Under each data directory .gemini/<dir>/ (antigravity-cli for the CLI,
// antigravity for the desktop app, antigravity-ide, antigravity-backup) it
// reads the conversation databases conversations/<id>.db (SQLite holding
// protobuf blobs) and conversation_summaries.db; the brain mirror
// brain/<id>/.system_generated/logs/ (transcript_full.jsonl,
// transcript.jsonl or their chunks/, one copy per conversation; a secondary
// source); the CLI's history.jsonl (a secondary source); and the standing
// permission files .gemini/config/config.json, config/projects/<id>.json,
// config/sidecars/<name>/sidecar.json and antigravity-cli/settings.json.
//
// The protobuf messages come from the descriptors embedded in the agy
// binary; schemaText lists every field read. File: URIs are read as paths
// by evidencepath.FromFileURI. See research/antigravity.md.
package antigravity

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/protowire"
)

// dataDir matches the start of a path under one of the four data
// directories; its group is the directory's name.
const dataDir = `^\.gemini/(antigravity(?:-cli|-ide|-backup)?)/`

// The files read, by path relative to the home. A brain transcript's
// groups are the conversation id, then the file form, or the chunked form
// and the chunk number.
const (
	historyRel     = ".gemini/antigravity-cli/history.jsonl"
	configRel      = ".gemini/config/config.json"
	cliSettingsRel = ".gemini/antigravity-cli/settings.json"
)

// The patterns of the other files read.
var (
	conversationPattern = regexp.MustCompile(dataDir + `conversations/[^/]+\.db\z`)
	summariesPattern    = regexp.MustCompile(dataDir + `conversation_summaries\.db\z`)
	brainPattern        = regexp.MustCompile(dataDir + `brain/([^/]+)/\.system_generated/logs/` +
		`(?:(transcript(?:_full)?)\.jsonl|chunks/(transcript(?:_full)?)/([0-9]+)\.jsonl)\z`)
	projectPattern = regexp.MustCompile(`^\.gemini/config/projects/[^/]+\.json\z`)
	sidecarPattern = regexp.MustCompile(`^\.gemini/config/sidecars/[^/]+/sidecar\.json\z`)
)

// brainTolerance is how far, in seconds, a brain row's time may be from
// the database row's for the timeline to pair them: the mirror records only
// the creation time, the database reader stamps replies and tool rows with
// the completion time.
const brainTolerance = 600.0

// Parser reads Antigravity's conversations, brain mirror, history and
// permission files. It keeps the listings of the chunks directories it
// has read.
type Parser struct {
	parserkit.Base
	schema       *protowire.Schema
	schemaErr    error
	payloadKinds []string
	chunks       *chunkListings
}

// New returns the parser.
func New() *Parser {
	created := &Parser{Base: parserkit.NewBase("antigravity", "antigravity"), chunks: newChunkListings()}
	created.schema, created.schemaErr = newSchema()
	if created.schemaErr == nil {
		created.payloadKinds = payloadKinds(created.schema)
	}
	return created
}

// payloadKinds returns the step kinds, in declaration order: every Step
// field but the scalars, the metadata and the permissions that sit beside
// the step kind.
func payloadKinds(schema *protowire.Schema) []string {
	var kinds []string
	for _, field := range schema.Fields("Step") {
		switch field.Kind {
		case protowire.KindInt, "CortexStepMetadata", "TrajectoryPermissions":
		default:
			kinds = append(kinds, field.Name)
		}
	}
	return kinds
}

// standingFile reports whether rel is a standing permission file.
func standingFile(rel string) bool {
	return rel == configRel || rel == cliSettingsRel || projectPattern.MatchString(rel) ||
		sidecarPattern.MatchString(rel)
}

// Wants reports whether the artifact is a conversation, the summaries, a
// brain transcript, the history or a permission file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the history, the brain transcripts, the summaries, the
// conversations and the four permission files.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelKind("history.jsonl", historyRel),
		parserkit.PatternKind("brain transcript", brainPattern),
		parserkit.PatternKind("conversation_summaries.db", summariesPattern),
		parserkit.PatternKind("conversation database", conversationPattern),
		parserkit.RelKind("config.json", configRel), parserkit.RelKind("CLI settings.json", cliSettingsRel),
		parserkit.PatternKind("project permissions", projectPattern),
		parserkit.PatternKind("sidecar.json", sidecarPattern)}
}

// Secondary reports whether the artifact is the CLI history or the brain
// mirror.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return artifact.Rel == historyRel || brainPattern.MatchString(artifact.Rel)
}

// SecondaryTolerance returns the wider brain window, since the mirror
// stamps steps at creation, not completion.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if brainPattern.MatchString(artifact.Rel) {
		return brainTolerance
	}
	return parserkit.DefaultTolerance
}

// Parse emits the rows of one Antigravity file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	if p.schemaErr != nil {
		return p.schemaErr
	}
	rel := artifact.Rel
	switch {
	case rel == historyRel:
		p.history(artifact, emit)
	case brainPattern.MatchString(rel):
		p.brain(artifact, brainPattern.FindStringSubmatch(rel), options, emit)
	case standingFile(rel):
		p.standing(artifact, emit)
	case summariesPattern.MatchString(rel):
		p.summaries(artifact, emit)
	default:
		p.conversation(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
)
