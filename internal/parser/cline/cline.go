// Package cline reads Cline's legacy task layout, its SDK sessions and the
// stores beside them.
//
// The legacy tasks live under the VS Code extension's
// <editor>/User/globalStorage/saoudrizwan.claude-dev/ or ~/.cline/data/ and
// are read through package clinelegacy, with state/taskHistory.json as the
// task index and the extension's cache/sync-queue.json (only taskId, key,
// timestamp, status and retryCount; data and lastError are never read).
// Under ~/.cline/data/ are the SDK sessions: sessions/<id>/<id>.json
// manifests and *.messages.json transcripts, db/sessions.db for sessions
// whose manifest is gone, and the secondary sources db/session-search.db,
// cache/user_input_history.jsonl and logs/hooks.jsonl. The standing
// settings of globalState.json, settings/global-settings.json,
// settings/cline_mcp_settings.json (only autoApprove and disabled per
// server) and cloud-pending-starts/*.json give permission setting, sync and
// origin rows.
//
// The API history is a fallback, not a secondary source: it words a turn
// differently from ui_messages.json. See research/cline.md.
package cline

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
)

// root matches the storage roots: the extension's global storage in any
// editor, and the CLI's data directory.
const root = `(?:(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev|\.cline/data)`

// The files read, by path relative to the home.
const (
	sessionsDatabaseRel = ".cline/data/db/sessions.db"
	searchDatabaseRel   = ".cline/data/db/session-search.db"
	inputHistoryRel     = ".cline/data/cache/user_input_history.jsonl"
	hooksRel            = ".cline/data/logs/hooks.jsonl"
	globalStateRel      = ".cline/data/globalState.json"
	globalSettingsRel   = ".cline/data/settings/global-settings.json"
	sessionsPrefix      = ".cline/data/sessions/"
)

// The patterns of the other files read.
var (
	statePattern     = regexp.MustCompile(`(?i)^` + root + `/state/taskHistory\.json\z`)
	messagesPattern  = regexp.MustCompile(`^\.cline/data/sessions/[^/]+/[^/]+\.messages\.json\z`)
	syncQueuePattern = regexp.MustCompile(
		`(?i)^(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev/cache/sync-queue\.json\z`)
	mcpSettingsPattern  = regexp.MustCompile(`(?i)^` + root + `/settings/cline_mcp_settings\.json\z`)
	pendingStartPattern = regexp.MustCompile(`^\.cline/data/cloud-pending-starts/[0-9]+-[0-9a-f-]+\.json\z`)
)

// hooksTolerance is the matching window of a hook log line, in seconds: a
// tool_call or tool_result line is written when the call starts or ends,
// the transcript's message when the whole turn is persisted, which can be a
// long command later; the call id pairs them, so the wide window is safe.
const hooksTolerance = 3600.0

// isManifest reports whether rel is a session manifest,
// .cline/data/sessions/<id>/<id>.json, a file named after its own
// directory. A pattern for it needs a backreference
// (`^\.cline/data/sessions/([^/]+)/\1\.json$`), which Go's regexp lacks.
func isManifest(rel string) bool {
	rest, ok := strings.CutPrefix(rel, sessionsPrefix)
	if !ok {
		return false
	}
	directory, name, ok := strings.Cut(rest, "/")
	return ok && directory != "" && name == directory+".json"
}

// Parser reads Cline's tasks, SDK sessions, logs and settings.
type Parser struct {
	*clinelegacy.TaskParser
}

// New returns the parser.
func New() *Parser {
	created := &Parser{}
	created.TaskParser = clinelegacy.NewTaskParser(parser.NewBase("cline", "cline"), root,
		"cwdOnTaskInitialization", created)
	return created
}

// Marks says the parser records synced sessions (an enterprise blob upload
// or Cline Cloud) and sessions of remote origin (Cline Cloud).
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Origin: true}
}

// Wants reports whether the artifact is a task, session, database,
// history, log or settings file of Cline.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parser.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the databases, logs and settings of the SDK, the task
// files, the task index, the SDK sessions' files, the sync queue, the MCP
// settings and the pending cloud starts.
func (p *Parser) Kinds() []parser.Kind {
	kinds := []parser.Kind{parser.RelKind("sessions.db", sessionsDatabaseRel),
		parser.RelKind("session-search.db", searchDatabaseRel),
		parser.RelKind("user_input_history.jsonl", inputHistoryRel), parser.RelKind("hooks.jsonl", hooksRel),
		parser.RelKind("globalState.json", globalStateRel),
		parser.RelKind("global-settings.json", globalSettingsRel)}
	return append(append(kinds, p.TaskKinds()...), parser.PatternKind("taskHistory.json", statePattern),
		parser.RelFuncKind("session manifest", isManifest),
		parser.PatternKind("session messages.json", messagesPattern),
		parser.PatternKind("sync-queue.json", syncQueuePattern),
		parser.PatternKind("cline_mcp_settings.json", mcpSettingsPattern),
		parser.PatternKind("cloud pending start", pendingStartPattern))
}

// Secondary reports whether the artifact is the search index, the input
// history or the hook log.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	switch artifact.Rel {
	case searchDatabaseRel, inputHistoryRel, hooksRel:
		return true
	}
	return false
}

// SecondaryTolerance returns an hour for the hook log, else the default
// window.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if artifact.Rel == hooksRel {
		return hooksTolerance
	}
	return parser.DefaultTolerance
}

// HistoryItem returns the task's entry in state/taskHistory.json, or null.
func (p *Parser) HistoryItem(task clinelegacy.Task) jsonvalue.Value {
	history, found := task.InRoot("state/taskHistory.json")
	if !found {
		return jsonvalue.NewNull()
	}
	return p.CachedEntry(history, "", task.TaskID)
}

// Parse emits the rows of one Cline file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case p.TaskMatch(artifact):
		p.ParseTaskFile(artifact, options, emit)
	case statePattern.MatchString(rel):
		p.taskHistory(artifact, emit)
	case isManifest(rel):
		p.manifest(artifact, emit)
	case messagesPattern.MatchString(rel):
		p.messages(artifact, options, emit)
	case rel == sessionsDatabaseRel:
		p.sessionsDatabase(artifact, emit)
	case rel == searchDatabaseRel:
		p.searchDatabase(artifact, emit)
	case rel == inputHistoryRel:
		p.inputHistory(artifact, emit)
	case syncQueuePattern.MatchString(rel):
		p.syncQueue(artifact, emit)
	case rel == hooksRel:
		p.hooksLog(artifact, emit)
	default:
		p.settingsFile(artifact, emit)
	}
	return nil
}

// settingsFile emits the rows of a standing settings file.
func (p *Parser) settingsFile(artifact *input.Artifact, emit parser.Emit) {
	rel := artifact.Rel
	switch {
	case rel == globalStateRel:
		p.globalState(artifact, emit)
	case rel == globalSettingsRel:
		p.globalSettings(artifact, emit)
	case mcpSettingsPattern.MatchString(rel):
		p.mcpSettings(artifact, emit)
	case pendingStartPattern.MatchString(rel):
		p.pendingStarts(artifact, emit)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.ToleranceSetter = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
