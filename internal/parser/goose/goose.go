// Package goose reads the Goose CLI's and desktop app's sessions.
//
// It reads, relative to the home (<data> is .local/share/goose or
// AppData/Roaming/Block/goose/data, <state> is .local/state/goose or the
// Windows data directory, <config> is .config/goose or
// AppData/Roaming/Block/goose/config): <data>/sessions/sessions.db, the
// legacy <data>/sessions/<YYYYMMDD_HHMMSS>.jsonl, the request logs
// <state>/logs/llm_request.<N>.jsonl and the prompt histories
// <state>/history.txt and .config/goose/history.txt (both secondary), and
// the standing files <config>/permission.yaml, <config>/roaming_trust.json
// and <data>/schedule.json.
//
// It yields session start rows, one row per content block, model,
// permission, relay and scheduled-task system rows, and `parser:` rows for
// stored values that do not decode. Content blocks are read by type: text,
// image, document, toolRequest, toolResponse, thinking, redactedThinking,
// toolConfirmationRequest, actionRequired, systemNotification and error.
// See research/goose.md.
package goose

import (
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The directories a Goose path starts in.
const (
	dataDirectory   = `(?:\.local/share/goose|AppData/Roaming/Block/goose/data)`
	stateDirectory  = `(?:\.local/state/goose|AppData/Roaming/Block/goose/data)`
	configDirectory = `(?:\.config/goose|AppData/Roaming/Block/goose/config)`
)

// llmTolerance is the matching window of a request log's rows, in seconds:
// the log stamps each streamed chunk when it arrives, while sessions.db
// stamps the assembled message with the later of its first chunk's time and
// the session's newest stored message.
const llmTolerance = 600.0

// Parser reads Goose's session database, legacy sessions, request logs,
// histories and settings.
type Parser struct {
	parser.Base
	database, legacy, llm, history, permission, trust, schedule *regexp.Regexp
	gateway                                                     *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:       parser.NewBase("goose", "goose"),
		database:   regexp.MustCompile(`\A` + dataDirectory + `/sessions/sessions\.db\z`),
		legacy:     regexp.MustCompile(`\A` + dataDirectory + `/sessions/[^/]+\.jsonl\z`),
		llm:        regexp.MustCompile(`\A` + stateDirectory + `/logs/llm_request\.[^/]+\.jsonl\z`),
		history:    regexp.MustCompile(`\A(?:` + stateDirectory + `|\.config/goose)/history\.txt\z`),
		permission: regexp.MustCompile(`\A` + configDirectory + `/permission\.yaml\z`),
		trust:      regexp.MustCompile(`\A` + configDirectory + `/roaming_trust\.json\z`),
		schedule:   regexp.MustCompile(`\A` + dataDirectory + `/schedule\.json\z`),
		// A gateway session's working directory,
		// <config>/gateway/<platform>/<user_id>; the end may follow a line feed.
		gateway: regexp.MustCompile(`[/\\]gateway[/\\]([^/\\]+)[/\\]([^/\\]+)[/\\]?\n?\z`),
	}
}

// Marks says the parser records sessions relayed through a gateway.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Relay: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	for _, pattern := range []*regexp.Regexp{p.database, p.legacy, p.llm, p.history, p.permission,
		p.trust, p.schedule} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
}

// Secondary reports whether the artifact is a prompt history or a request
// log, which repeat sessions.db.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.history.MatchString(artifact.Rel) || p.llm.MatchString(artifact.Rel)
}

// SecondaryTolerance returns the request log's wider window, else the
// default one.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if p.llm.MatchString(artifact.Rel) {
		return llmTolerance
	}
	return parser.DefaultTolerance
}

// Parse emits the rows of one file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch rel := artifact.Rel; {
	case p.database.MatchString(rel):
		p.parseDatabase(artifact, options, emit)
	case p.legacy.MatchString(rel):
		p.parseLegacy(artifact, options, emit)
	case p.llm.MatchString(rel):
		p.parseLLMLog(artifact, options, emit)
	case p.permission.MatchString(rel):
		p.parsePermissionYAML(artifact, emit)
	case p.trust.MatchString(rel):
		p.parseRoamingTrust(artifact, emit)
	case p.schedule.MatchString(rel):
		p.parseSchedule(artifact, emit)
	default:
		p.parseHistory(artifact, emit)
	}
	return nil
}

// row returns a row of the artifact with its attribution.
func (p *Parser) row(artifact *input.Artifact, line int, sessionID, projectPath, model,
	timestamp string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = line
	row.SessionID = sessionID
	row.ProjectPath = projectPath
	row.Model = model
	row.TimestampUTC = timestamp
	return row
}

// systemCopy returns a system row with base's attribution, time and model,
// no tool, and the text as it is.
func systemCopy(base timeline.Row, text string) timeline.Row {
	base.TurnType = timeline.TurnSystem
	base.ToolName, base.ToolUseID = "", ""
	base.Text = text
	return base
}

// modelName returns "<provider>/<name>", or the name alone when the
// provider is unknown.
func modelName(provider, name string) string {
	if provider != "" && name != "" {
		return provider + "/" + name
	}
	return name
}

// epochNumber returns a created timestamp in seconds as a timeline time;
// some clients write milliseconds, so Goose divides anything above 1e10
// by 1000.
func epochNumber(value float64) string {
	if value > 10_000_000_000 {
		return timeutil.FromEpoch(value / 1000.0)
	}
	return timeutil.FromEpoch(value)
}

// epochJSON returns a decoded created time as epochNumber reads a number,
// any other value as timeutil reads it.
func epochJSON(value jsonvalue.Value) string {
	if number, ok := value.Float(); ok && value.IsNumber() && number > 10_000_000_000 {
		return timeutil.FromEpoch(number / 1000.0)
	}
	return timeutil.FromValue(value)
}

// unescapeHistory undoes the backslash escapes of a "#V2" history line: \n
// is a line feed and a backslash takes the character after it as it is.
func unescapeHistory(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) {
			i++
			if line[i] == 'n' {
				out.WriteByte('\n')
				continue
			}
		}
		out.WriteByte(line[i])
	}
	return out.String()
}

// parseHistory emits one user row per line of a prompt history.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		line := strings.TrimRight(textlines.ValidUTF8(raw), "\r\n")
		if line == "" || number == 1 && strings.HasPrefix(line, "#V") {
			continue
		}
		row := p.row(artifact, number, "", "", "", "")
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(unescapeHistory(line))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseLegacy emits the rows of a legacy session file: a header line
// gives the session start, every other line a message's blocks.
func (p *Parser) parseLegacy(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	sessionID := strings.TrimSuffix(path.Base(artifact.Rel), ".jsonl")
	projectPath := ""
	reader := &blockReader{parser: p, artifact: artifact, options: options,
		toolNames: map[string]string{}}
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		if !record.Has("role") && !record.Has("content") {
			if recorded := parser.Text(record.Get("id")); recorded != "" {
				sessionID = recorded
			}
			projectPath = parser.Text(record.Get("working_dir"))
			count := ""
			if value, ok := record.Lookup("message_count"); ok {
				count = value.Text() + " messages"
			}
			row := p.row(artifact, number, sessionID, projectPath, "",
				timeutil.FromValue(record.Get("created_at")))
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact(parser.JoinNonEmpty(" | ", "session start (legacy)",
				parser.Text(record.Get("description")), count))
			emit(row)
			continue
		}
		base := p.row(artifact, number, sessionID, projectPath, "", epochJSON(record.Get("created")))
		reader.blocks(base, parser.Text(record.Get("role")), record.Get("content"),
			userVisible(record.Get("metadata")), nil, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, projectPath))
}

// userVisible reports whether a message's metadata leaves it visible: only
// a userVisible of JSON false hides it.
func userVisible(metadata jsonvalue.Value) bool {
	flag, isBool := metadata.Get("userVisible").Bool()
	return !metadata.IsObject() || !isBool || flag
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.ToleranceSetter = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
