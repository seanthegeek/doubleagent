package cline

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
)

// The patterns of the other files read.
var (
	statePattern     = parserkit.MustRelPattern(`^` + root + `/state/taskHistory\.json\z`)
	manifestPattern  = parserkit.MustRelPattern(`^\.cline/data/sessions/([^/]+)/([^/]+)\.json\z`)
	messagesPattern  = parserkit.MustRelPattern(`^\.cline/data/sessions/([^/]+)/([^/]+)\.messages\.json\z`)
	syncQueuePattern = parserkit.MustRelPattern(
		`^(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev/cache/sync-queue\.json\z`)
	mcpSettingsPattern  = parserkit.MustRelPattern(`^` + root + `/settings/cline_mcp_settings\.json\z`)
	pendingStartPattern = parserkit.MustRelPattern(`^\.cline/data/cloud-pending-starts/[0-9]+-[0-9a-f-]+\.json\z`)
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
	match := manifestPattern.FindStringSubmatch(rel)
	return match != nil && match[2] == match[1]
}

// Parser reads Cline's tasks, SDK sessions, logs and settings.
type Parser struct {
	*clinelegacy.TaskParser
	// manifests holds, per home, what each session manifest a messages
	// file reads gave, by the entry its path opens.
	manifests *parserkit.PerHome[map[parserkit.EntryKey]sessionManifest]
}

// New returns the parser.
func New() *Parser {
	created := &Parser{manifests: parserkit.NewPerHome(func() map[parserkit.EntryKey]sessionManifest {
		return map[parserkit.EntryKey]sessionManifest{}
	})}
	created.TaskParser = clinelegacy.NewTaskParser(parserkit.NewBase("cline", "cline"), root,
		"cwdOnTaskInitialization", created)
	return created
}

// Marks says the parser records synced sessions (an enterprise blob upload
// or Cline Cloud) and sessions of remote origin (Cline Cloud).
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Origin: true}
}

// Kinds returns the databases, logs and settings of the SDK, the task
// files, the task index, the SDK sessions' files, the sync queue, the MCP
// settings and the pending cloud starts.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := []parserkit.Kind{parserkit.RelKind("sessions.db", sessionsDatabaseRel),
		parserkit.RelKind("session-search.db", searchDatabaseRel),
		parserkit.RelKind("user_input_history.jsonl", inputHistoryRel), parserkit.RelKind("hooks.jsonl", hooksRel),
		parserkit.RelKind("globalState.json", globalStateRel),
		parserkit.RelKind("global-settings.json", globalSettingsRel)}
	return append(append(kinds, p.TaskKinds()...), parserkit.PatternKind("taskHistory.json", statePattern),
		parserkit.RelFuncKind("session manifest", isManifest),
		parserkit.PatternKind("session messages.json", messagesPattern),
		parserkit.PatternKind("sync-queue.json", syncQueuePattern),
		parserkit.PatternKind("cline_mcp_settings.json", mcpSettingsPattern),
		parserkit.PatternKind("cloud pending start", pendingStartPattern))
}

// Wants reports whether the artifact is a task, session, database,
// history, log or settings file of Cline.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is the search index, the input
// history or the hook log.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	for _, rel := range []string{searchDatabaseRel, inputHistoryRel, hooksRel} {
		if textlines.EqualFoldASCII(artifact.Rel, rel) {
			return true
		}
	}
	return false
}

// SecondaryTolerance returns an hour for the hook log, else the default
// window.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if textlines.EqualFoldASCII(artifact.Rel, hooksRel) {
		return hooksTolerance
	}
	return parserkit.DefaultTolerance
}

// HistoryItem returns the task's entry in state/taskHistory.json, or null,
// and the group of its id when the file repeats it.
func (p *Parser) HistoryItem(task clinelegacy.Task) (jsonvalue.Value, *timeline.Group) {
	history, found := task.InRoot("state/taskHistory.json")
	if !found {
		return jsonvalue.NewNull(), nil
	}
	return p.CachedEntry(history, "", task.TaskID), p.EntryGroup(history, "", task.TaskID)
}

// settingsFile emits the rows of a standing settings file.
func (p *Parser) settingsFile(artifact *input.Artifact, emit parserkit.Emit) {
	rel := artifact.Rel
	switch {
	case textlines.EqualFoldASCII(rel, globalStateRel):
		p.globalState(artifact, emit)
	case textlines.EqualFoldASCII(rel, globalSettingsRel):
		p.globalSettings(artifact, emit)
	case mcpSettingsPattern.MatchString(rel):
		p.mcpSettings(artifact, emit)
	case pendingStartPattern.MatchString(rel):
		p.pendingStarts(artifact, emit)
	}
}

// Parse emits the rows of one Cline file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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
	case textlines.EqualFoldASCII(rel, sessionsDatabaseRel):
		p.sessionsDatabase(artifact, emit)
	case textlines.EqualFoldASCII(rel, searchDatabaseRel):
		p.searchDatabase(artifact, emit)
	case textlines.EqualFoldASCII(rel, inputHistoryRel):
		p.inputHistory(artifact, emit)
	case syncQueuePattern.MatchString(rel):
		p.syncQueue(artifact, emit)
	case textlines.EqualFoldASCII(rel, hooksRel):
		p.hooksLog(artifact, emit)
	default:
		p.settingsFile(artifact, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
