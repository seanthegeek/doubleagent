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
// The retired v0.0.x releases, a separate implementation, kept their state
// in .little-coder/:
// saved sessions sessions/**/*.json (saved_at in host local time, written
// as if UTC), sessions/history.json and sessions/mr_sessions/
// session_latest.json (secondary copies), input_history.txt (secondary),
// config.json (the gist id, the Telegram chat id, the permission mode and
// the provider keys and tokens, these through parser.SecretText) and
// checkpoints/<session
// id>/snapshots.json. A file cut mid-write keeps the messages before the
// cut. See research/little-coder.md.
package littlecoder

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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

// The reasons an entry of a JSON array file is reported: a prompt that is
// not a string, a snapshot that is not an object.
var (
	errNotString = errors.New("not a JSON string")
	errNotObject = errors.New("not a JSON object")
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
	// firstCheckpoint is the first file by name of each checkpoint
	// directory listed, so its other files are passed over without a
	// listing of their own.
	firstCheckpoint map[checkpointKey]string
	// sessionIndexes holds each home's .pi/agent/sessions, listed once,
	// by the home's path on disk, since every checkpoint set of a pi
	// session looks for its session file there.
	sessionIndexes map[string]*sessionIndex
}

// checkpointKey names a checkpoint directory: the home's path on disk and
// the directory's name under .little-coder/checkpoints.
type checkpointKey struct {
	home, set string
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
		firstCheckpoint: map[checkpointKey]string{},
		sessionIndexes:  map[string]*sessionIndex{},
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

// parseHistory emits one user row per prompt of the prompt history; an
// entry that is not a string is a problem.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for position, prompt := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !prompt.IsString() {
			problems.AddRaw(position, parser.UnparseableRecord(position, errNotString), parser.JSONText(prompt))
			continue
		}
		if prompt.StringValue() != "" {
			row := p.BaseRow(artifact)
			row.SourceLine = position
			row.TurnType = timeline.TurnUser
			row.Text = timeline.Compact(prompt.StringValue())
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
		if strings.TrimSpace(text) == "" {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine, row.TurnType, row.Text = line, timeline.TurnUser, timeline.Compact(text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseLegacyConfig emits the gist, Telegram chat and permission mode rows
// of config.json, at the file's modification time, then one "stored
// credentials:" row of its <provider>_api_key, telegram_token and
// gist_token values in file order, each through parser.SecretText.
func (p *Parser) parseLegacyConfig(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := jsonvalue.Decode(data)
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
	if gist := document.Get("cloudsave_last_gist_id"); parser.Present(gist) {
		put(timeline.Compact("session synced: gist " + parser.SettingOrDash(gist) + source))
	}
	if chat := document.Get("telegram_chat_id"); parser.Present(chat) && !isZero(chat) {
		put(timeline.Compact("session relayed: telegram chat " + parser.SettingOrDash(chat) + source))
	}
	if mode := document.Get("permission_mode"); parser.Present(mode) {
		put(timeline.Compact("permission setting: permission_mode=" + parser.SettingOrDash(mode) + source))
	}
	var stored []string
	for _, member := range document.Members() {
		credential := strings.HasSuffix(member.Key, "_api_key") || member.Key == "telegram_token" ||
			member.Key == "gist_token"
		if value := parser.Text(member.Value); credential && value != "" {
			stored = append(stored, member.Key+"="+parser.SecretText(artifact, value))
		}
	}
	if len(stored) > 0 {
		put(timeline.Compact("stored credentials: " + strings.Join(stored, " ") + source))
	}
}

// isZero reports whether value is a number equal to zero or false: a
// Telegram chat id of 0 or false names no chat.
func isZero(value jsonvalue.Value) bool {
	return (value.IsNumber() || value.IsBool()) && value.IsEmpty()
}

// parseLegacySnapshots emits one row per snapshot of a v0.0.x
// checkpoints/<session id>/snapshots.json; a snapshot's file_backups that is
// not an object, or a backup in it that is neither an object nor null (no
// backup, as a null backup_filename), adds a problem.
func (p *Parser) parseLegacySnapshots(artifact *input.Artifact, emit parser.Emit) {
	directorySession := strings.Split(artifact.Rel, "/")[2]
	var problems parser.Problems
	for position, snapshot := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !snapshot.IsObject() {
			problems.AddRaw(position, parser.UnparseableRecord(position, errNotObject), parser.JSONText(snapshot))
			continue
		}
		var files []string
		for _, backup := range parser.ObjectMember(snapshot.Get("file_backups"), "file_backups", position, &problems) {
			if !backup.Value.IsObject() && !backup.Value.IsNull() {
				problems.Add(position, parser.WrongKind("file_backups."+backup.Key, jsonvalue.Object))
				continue
			}
			name := backup.Value.Get("backup_filename")
			state := "did not exist"
			if !name.IsEmpty() {
				state = "backup " + name.Text()
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
		row.SessionID = timeutil.FirstNonEmpty(parser.Text(snapshot.Get("session_id")), directorySession)
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
