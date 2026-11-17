// Package littlecoder reads little-coder's own files. little-coder is a
// launcher and extension set for pi, so its transcripts are pi session
// files that the pi parser reads; this parser reads what little-coder
// writes itself:
//
//   - .pi/agent/little-coder-prompt-history.json, a JSON array of prompts,
//     oldest first, with no time or session; a row's source line is the
//     array position.
//   - .little-coder/checkpoints/<session file name>/<sanitised path>, the
//     pre-edit copy of each file a session first edited, or an empty
//     <name>.absent marker. One row per directory, emitted on its first file
//     by name, at the earliest file modification time; the session id comes
//     from the directory name and the project from the pi session's header
//     when it was collected. Only the directory's listing and the files'
//     times are read, never their content.
//
// The retired v0.0.x Python releases kept their state in .little-coder/:
// saved sessions sessions/**/*.json (saved_at in host local time, written
// as if UTC), sessions/history.json and sessions/mr_sessions/
// session_latest.json (secondary copies), input_history.txt (secondary),
// config.json (only the gist id, the Telegram chat id and the permission
// mode are read, never a token or a key) and checkpoints/<session
// id>/snapshots.json. A file cut mid-write keeps the messages before the
// cut. See research/little-coder.md.
package littlecoder

import (
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The files read at fixed paths relative to the home.
const (
	historyRel       = ".pi/agent/little-coder-prompt-history.json"
	legacyHistoryRel = ".little-coder/sessions/history.json"
	legacyLatestRel  = ".little-coder/sessions/mr_sessions/session_latest.json"
	legacyInputRel   = ".little-coder/input_history.txt"
	legacyConfigRel  = ".little-coder/config.json"
)

// legacyDenied is the tool result of an operation the user refused in the
// v0.0.x releases.
const legacyDenied = "Denied: user rejected this operation"

// Parser reads little-coder's own files.
type Parser struct {
	parser.Base
	checkpoint, sessionFile, legacySession, legacySnapshots *regexp.Regexp
	// The keys a session file cut mid-write is searched for.
	sessionsKey, messagesKey, sessionID, savedAt *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:            parser.NewBase("little-coder", "little-coder"),
		checkpoint:      regexp.MustCompile(`\A\.little-coder/checkpoints/([^/]+)/[^/]+\z`),
		sessionFile:     regexp.MustCompile(`_([^_/]+)\.jsonl\z`),
		legacySession:   regexp.MustCompile(`\A\.little-coder/sessions/(?:[^/]+/)*[^/]+\.json\z`),
		legacySnapshots: regexp.MustCompile(`\A\.little-coder/checkpoints/([^/]+)/snapshots\.json\z`),
		sessionsKey:     regexp.MustCompile(`"sessions"[ \t\n\r]*:[ \t\n\r]*`),
		messagesKey:     regexp.MustCompile(`"messages"[ \t\n\r]*:[ \t\n\r]*`),
		sessionID:       regexp.MustCompile(`"session_id"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"`),
		savedAt:         regexp.MustCompile(`"saved_at"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"`),
	}
}

// Marks says the parser records synced and relayed sessions.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Relay: true}
}

// Wants reports whether the artifact is one of little-coder's files.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return slices.Contains([]string{historyRel, legacyInputRel, legacyConfigRel}, rel) ||
		p.checkpoint.MatchString(rel) || p.legacySession.MatchString(rel)
}

// Secondary reports whether the artifact is a v0.0.x file that repeats a
// saved session or a prompt.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return slices.Contains([]string{legacyHistoryRel, legacyLatestRel, legacyInputRel}, artifact.Rel)
}

// Parse emits the rows of one little-coder file.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case rel == historyRel:
		p.parseHistory(artifact, emit)
	case rel == legacyInputRel:
		p.parseInputHistory(artifact, emit)
	case rel == legacyConfigRel:
		p.parseLegacyConfig(artifact, emit)
	case rel == legacyHistoryRel:
		p.parseLegacyHistory(artifact, emit)
	case p.legacySession.MatchString(rel):
		p.parseLegacySession(artifact, emit)
	case p.legacySnapshots.MatchString(rel) && !p.sessionFile.MatchString(strings.Split(rel, "/")[2]):
		p.parseLegacySnapshots(artifact, emit)
	default:
		p.parseCheckpoints(artifact, emit)
	}
	return nil
}

// parseHistory emits one user row per prompt of the prompt history.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for position, prompt := range parser.JSONArray(artifact.DiskPath, &problems) {
		if prompt.IsString() && prompt.Text() != "" {
			row := p.BaseRow(artifact)
			row.SourceLine = position
			row.TurnType = timeline.TurnUser
			row.Text = timeline.Compact(prompt.Text())
			emit(row)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseInputHistory emits one user row per line of input_history.txt, with
// no time.
func (p *Parser) parseInputHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, raw := range parser.Lines(artifact.DiskPath, &problems) {
		text := parser.LineText(raw)
		if pytext.Strip(text) == "" {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine, row.TurnType, row.Text = line, timeline.TurnUser, timeline.Compact(text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseLegacyConfig emits the gist, Telegram chat and permission mode rows
// of config.json, at the file's modification time.
func (p *Parser) parseLegacyConfig(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := pyjson.Decode(data)
	timestamp := parser.FileMtime(artifact)
	put := func(text string) {
		row := p.SystemRow(artifact, text)
		row.SourceLine, row.TimestampUTC = 1, timestamp
		emit(row)
	}
	if err != nil || !document.IsObject() {
		put("parser: " + artifact.Rel + " is not a JSON object")
		return
	}
	source := " (from " + artifact.Rel + ", file mtime)"
	if gist := document.Get("cloudsave_last_gist_id"); !isNullOrEmpty(gist) {
		put(timeline.Compact("session synced: gist " + parser.SettingOrDash(gist) + source))
	}
	if chat := document.Get("telegram_chat_id"); !isNullOrEmpty(chat) && !isZero(chat) {
		put(timeline.Compact("session relayed: telegram chat " + parser.SettingOrDash(chat) + source))
	}
	if mode := document.Get("permission_mode"); !isNullOrEmpty(mode) {
		put(timeline.Compact("permission setting: permission_mode=" + parser.SettingOrDash(mode) + source))
	}
}

// isNullOrEmpty reports whether value is null or the empty string.
func isNullOrEmpty(value pyjson.Value) bool {
	return value.IsNull() || (value.IsString() && value.Text() == "")
}

// isZero reports whether value equals zero as Python compares it, false
// included.
func isZero(value pyjson.Value) bool {
	return (value.IsNumber() || value.IsBool()) && !value.Truthy()
}

// parseLegacySnapshots emits one row per snapshot of a v0.0.x
// checkpoints/<session id>/snapshots.json.
func (p *Parser) parseLegacySnapshots(artifact *input.Artifact, emit parser.Emit) {
	directorySession := strings.Split(artifact.Rel, "/")[2]
	var problems parser.Problems
	for position, snapshot := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !snapshot.IsObject() {
			continue
		}
		var files []string
		for _, backup := range snapshot.Get("file_backups").Members() {
			name := backup.Value.Get("backup_filename")
			state := "did not exist"
			if name.Truthy() {
				state = "backup " + name.Str()
			}
			files = append(files, backup.Key+" ("+state+")")
		}
		listed := strings.Join(files, "; ")
		if listed == "" {
			listed = "-"
		}
		row := p.SystemRow(artifact, timeline.Compact("little-coder v0.0.x checkpoint "+
			parser.SettingOrDash(snapshot.Get("id"))+": turn_count="+
			parser.SettingOrDash(snapshot.Get("turn_count"))+" prompt="+
			parser.SettingOrDash(snapshot.Get("user_prompt_preview"))+" files="+listed+
			" (created_at host local time)"))
		row.SourceLine = position
		row.TimestampUTC = timeutil.FromValue(snapshot.Get("created_at"))
		row.SessionID = timeutil.FirstNonEmpty(parser.Str(snapshot.Get("session_id")), directorySession)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
