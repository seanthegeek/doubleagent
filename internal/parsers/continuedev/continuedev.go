package continuedev

import (
	"path"
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The files read, by path relative to the home.
const (
	inputHistoryRel = ".continue/input_history.json"
	permissionsRel  = ".continue/permissions.yaml"
	// IndexName is the name of the session index beside the session files.
	IndexName = "sessions.json"
)

// The patterns of the other files read.
var (
	sessionPattern = regexp.MustCompile(`^(?:.*/)?\.continue/sessions/([^/]+)\.json\z`)
	devDataPattern = regexp.MustCompile(`^(?:.*/)?\.continue/dev_data/(?:[^/]+/)?` +
		`(chatInteraction|toolUsage|editInteraction|editOutcome|chatFeedback)\.jsonl\z`)
	cnLogPattern = regexp.MustCompile(`^\.continue/logs/cn[0-9]*\.log\z`)
)

// anyTime is the secondary sources' matching window, in seconds. Session
// rows carry the session's creation time, not the prompt's, and a rating
// can follow its reply by any time, so a history entry or a chatFeedback
// line pairs with its twin on text and scope whatever the time: ten years
// is in effect no bound.
const anyTime = 10 * 366 * 86400.0

// Parser reads Continue's session files, dev_data events, CLI state files
// and CLI logs.
type Parser struct {
	parserkit.Base
	indexes *Indexes
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("continue", "continue"), indexes: NewIndexes()}
}

// Wants reports whether the artifact is a session file (the index
// included), a dev_data file, a cn log, the input history or
// permissions.yaml.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the sessions, the five dev_data logs, the CLI logs, the
// input history and the permissions.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := []parserkit.Kind{parserkit.PatternKind("session", sessionPattern)}
	for _, name := range []string{"chatInteraction", "toolUsage", "editInteraction", "editOutcome",
		"chatFeedback"} {
		kinds = append(kinds, parserkit.RelFuncKind("dev_data "+name+".jsonl", func(rel string) bool {
			match := devDataPattern.FindStringSubmatch(rel)
			return match != nil && match[1] == name
		}))
	}
	return append(kinds, parserkit.PatternKind("cn log", cnLogPattern),
		parserkit.RelKind("input_history.json", inputHistoryRel), parserkit.RelKind("permissions.yaml", permissionsRel))
}

// Secondary reports whether the artifact is the CLI's prompt history or a
// chatFeedback file, each of which repeats a prompt of a session or a
// chatInteraction line.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	if artifact.Rel == inputHistoryRel {
		return true
	}
	match := devDataPattern.FindStringSubmatch(artifact.Rel)
	return match != nil && match[1] == "chatFeedback"
}

// SecondaryTolerance returns a window of ten years: a history entry is
// stamped when the prompt was submitted while the session's rows carry its
// creation time, and a rating when it was clicked, any time after the
// reply.
func (p *Parser) SecondaryTolerance(*input.Artifact) float64 {
	return anyTime
}

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case cnLogPattern.MatchString(rel):
		p.cnLog(artifact, emit)
		return nil
	case rel == inputHistoryRel:
		p.inputHistory(artifact, emit)
		return nil
	case rel == permissionsRel:
		p.permissions(artifact, emit)
		return nil
	}
	match := devDataPattern.FindStringSubmatch(rel)
	switch {
	case match != nil && (match[1] == "chatInteraction" || match[1] == "chatFeedback"):
		p.chatInteraction(artifact, match[1] == "chatFeedback", emit)
	case match != nil && match[1] == "editInteraction":
		p.editInteraction(artifact, emit)
	case match != nil && match[1] == "editOutcome":
		p.editOutcome(artifact, emit)
	case match != nil:
		p.toolUsage(artifact, emit)
	case path.Base(rel) == IndexName:
		parserkit.EmitAll(emit, p.ErrorRows(artifact, p.indexes.Errors(artifact.DiskPath), "", ""))
		if index, ok := parserkit.Beside(artifact, IndexName); ok {
			parserkit.EmitAll(emit, p.indexes.Notices(index, p.BaseRow(artifact)))
		}
	default:
		p.session(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
)
