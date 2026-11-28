package letta

import (
	"cmp"
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/pi"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The files read at fixed paths relative to the home.
const (
	sessionsRel       = ".letta/sessions.jsonl"
	settingsRel       = ".letta/settings.json"
	localSettingsRel  = ".letta/settings.local.json"
	legacySettingsRel = ".config/letta/settings.json"
	cronsRel          = ".letta/crons.json"
)

// The prefix of a local agent's id, and the server an agent without a base
// URL talks to.
const (
	localAgentPrefix = "agent-local-"
	lettaCloud       = "api.letta.com"
)

// turnTolerance is the window, in seconds, a transcript line may lag its
// messages.jsonl twin: a turn's whole duration, since the transcript is
// stamped at the turn's end.
const turnTolerance = 3600.0

// Parser reads Letta Code's local files.
type Parser struct {
	parserkit.Base
	transcript, local, routing, runs, listener, localAgent *parserkit.RelPattern
	// sessions and servers hold what sessions.jsonl and settings.json give
	// each agent, read once per state of the file.
	sessions map[fileState]map[string][]projectTime
	servers  map[fileState]map[string]string
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:       parserkit.NewBase("letta", "letta"),
		transcript: parserkit.MustRelPattern(`\A\.letta/transcripts/([^/]+)/([^/]+)/transcript\.jsonl\z`),
		local:      parserkit.MustRelPattern(`\A\.letta/lc-local-backend/conversations/([^/]+)/messages\.jsonl\z`),
		routing:    parserkit.MustRelPattern(`\A\.letta/channels/([^/]+)/routing\.(?:json|yaml)\z`),
		runs:       parserkit.MustRelPattern(`\A\.letta/runs/[^/]+\.jsonl\z`),
		listener:   parserkit.MustRelPattern(`\A\.letta/listener-state/[0-9a-f]{24}/[^/]+\.json\z`),
		localAgent: parserkit.MustRelPattern(`\A` + localAgentPrefix),
		sessions:   map[fileState]map[string][]projectTime{},
		servers:    map[fileState]map[string]string{},
	}
}

// Marks says the parser records synced and relayed sessions.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Relay: true}
}

// Kinds returns the session list, the three settings files, the crons, the
// transcripts, the local backend's conversations, the channel routing,
// the runs and the listener state.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelKind("sessions.jsonl", sessionsRel), parserkit.RelKind("settings.json", settingsRel),
		parserkit.RelKind("settings.local.json", localSettingsRel), parserkit.RelKind("legacy settings.json", legacySettingsRel),
		parserkit.RelKind("crons.json", cronsRel), parserkit.PatternKind("transcript", p.transcript),
		parserkit.PatternKind("local backend messages", p.local), parserkit.PatternKind("channel routing", p.routing),
		parserkit.PatternKind("run", p.runs), parserkit.PatternKind("listener state", p.listener)}
}

// Wants reports whether the artifact is one of the Letta files read.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is a transcript, which repeats the
// local backend's messages.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.transcript.MatchString(artifact.Rel)
}

// SecondaryTolerance returns the window a transcript line may lag its
// messages.jsonl twin.
func (p *Parser) SecondaryTolerance(*input.Artifact) float64 {
	return turnTolerance
}

// DecodeKey returns the conversation key a local-backend directory name
// holds as base64url ("conversation:<id>" or "default:<agent id>"), or ""
// when the name is not base64url of UTF-8 text.
func DecodeKey(name string) string {
	padded := name + strings.Repeat("=", (4-len(name)%4)%4)
	decoded, err := base64.URLEncoding.DecodeString(padded)
	if err != nil || !utf8.Valid(decoded) {
		return ""
	}
	return string(decoded)
}

// isBareMessages reports whether a session is a schema 1 file: no header,
// and every record a bare message with a role and no message type.
func isBareMessages(session *pi.Session) bool {
	if session.HasHeader() || len(session.Entries) == 0 {
		return false
	}
	for _, entry := range session.Entries {
		kind := entry.Record.Get("type")
		if !entry.Record.Has("role") || (kind.IsString() && kind.StringValue() == "message") {
			return false
		}
	}
	return true
}

// bareMessages wraps the bare messages of a schema 1 file as message
// entries timed by their metadata.created_at.
func bareMessages(session *pi.Session) *pi.Session {
	out := &pi.Session{Header: session.Header, Problems: session.Problems}
	for _, entry := range session.Entries {
		meta := entry.Record.Get("metadata")
		out.Entries = append(out.Entries, pi.Entry{Line: entry.Line, Record: jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("message")},
			jsonvalue.Member{Key: "timestamp", Value: meta.Get("created_at")},
			jsonvalue.Member{Key: "message", Value: entry.Record})})
	}
	return out
}

// lastSnapshots keeps only the last entry of each message id.
func lastSnapshots(session *pi.Session) *pi.Session {
	idOf := func(record jsonvalue.Value) (string, bool) {
		identifier := record.Get("message").Get("id")
		return identifier.Text(), !identifier.IsEmpty()
	}
	last := map[string]int{}
	for i, entry := range session.Entries {
		if identifier, ok := idOf(entry.Record); ok {
			last[identifier] = i
		}
	}
	out := &pi.Session{Header: session.Header, HeaderLine: session.HeaderLine, Problems: session.Problems}
	for i, entry := range session.Entries {
		if identifier, ok := idOf(entry.Record); ok && last[identifier] != i {
			continue
		}
		out.Entries = append(out.Entries, entry)
	}
	return out
}

// parseLocal emits a local-backend conversation's start row, its entries'
// rows and its bad lines.
func (p *Parser) parseLocal(artifact *input.Artifact, directory string, options parserkit.Options,
	emit parserkit.Emit) {
	session := pi.ReadSession(artifact.DiskPath)
	key := DecodeKey(directory)
	sessionID := parserkit.Text(session.Header.Get("id"))
	if sessionID == "" {
		_, sessionID, _ = strings.Cut(key, ":")
	}
	cwd := parserkit.Text(session.Header.Get("cwd"))
	if isBareMessages(session) {
		session = bareMessages(session)
	} else {
		session = lastSnapshots(session)
	}
	if session.HasHeader() {
		row := p.SystemRow(artifact, timeline.Compact("letta local-backend conversation start: "+
			cmp.Or(key, sessionID)+" cwd="+cwd))
		row.SourceLine = session.HeaderLine
		row.TimestampUTC = timeutil.FromValue(session.Header.Get("timestamp"))
		row.SessionID, row.ProjectPath = sessionID, cwd
		emit(row)
	}
	pi.EntryRows(p.Base, artifact, session, options, sessionID, cwd, emit)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, session.Problems, sessionID, cwd))
}

// Parse emits the rows of one Letta file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case textlines.EqualFoldASCII(rel, sessionsRel):
		p.parseSessions(artifact, emit)
	case textlines.EqualFoldASCII(rel, settingsRel) || textlines.EqualFoldASCII(rel, localSettingsRel) ||
		textlines.EqualFoldASCII(rel, legacySettingsRel):
		p.parseSettings(artifact, emit)
	case textlines.EqualFoldASCII(rel, cronsRel):
		p.parseCrons(artifact, emit)
	case p.runs.MatchString(rel):
		p.parseRuns(artifact, emit)
	case p.listener.MatchString(rel):
		p.parseInterrupted(artifact, emit)
	case p.routing.MatchString(rel):
		p.parseRouting(artifact, p.routing.FindStringSubmatch(rel)[1], emit)
	case p.local.MatchString(rel):
		p.parseLocal(artifact, p.local.FindStringSubmatch(rel)[1], options, emit)
	default:
		match := p.transcript.FindStringSubmatch(rel)
		if match != nil {
			p.parseTranscript(artifact, match[1], match[2], options, emit)
		}
	}
	return nil
}

// keptInName reports whether sanitize keeps a character.
func keptInName(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-'
}

// sanitize returns the transcript directory name Letta derives from an id:
// each character other than an ASCII letter, a digit, ".", "_" and "-"
// becomes "_", and an empty result is "unknown".
func sanitize(segment string) string {
	var out strings.Builder
	for rest := segment; rest != ""; {
		char, width := utf8.DecodeRuneInString(rest)
		if keptInName(char) {
			out.WriteRune(char)
		} else {
			out.WriteByte('_')
		}
		rest = rest[width:]
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}

// argumentsText returns a transcript's argsText as the pi reader renders
// the same call's arguments: ArgsSummary of the text decoded as JSON, else
// the text itself.
func argumentsText(artifact *input.Artifact, arguments jsonvalue.Value) string {
	if !arguments.IsString() {
		return pi.ArgsSummary(arguments)
	}
	value, err := parserkit.DecodeJSON(artifact, arguments.StringValue())
	if err != nil {
		return arguments.StringValue()
	}
	return pi.ArgsSummary(value)
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
