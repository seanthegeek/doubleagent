package littlecoder

import (
	"cmp"
	"errors"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
	parserkit.Base
	checkpoint, sessionFile, legacySession, legacySnapshots *parserkit.RelPattern
	// The keys a session file cut mid-write is searched for.
	sessionsKey, messagesKey, sessionID, savedAt *regexp.Regexp
	// firstCheckpoint is the first file by name of each checkpoint
	// directory listed, "" for one that could not be listed or holds
	// none, so its other files are passed over without a listing of their
	// own.
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
		Base:            parserkit.NewBase("little-coder", "little-coder"),
		checkpoint:      parserkit.MustRelPattern(`\A\.little-coder/checkpoints/([^/]+)/[^/]+\z`),
		sessionFile:     parserkit.MustRelPattern(`_([^_/]+)\.jsonl\z`),
		legacySession:   parserkit.MustRelPattern(`\A\.little-coder/sessions/(?:[^/]+/)*[^/]+\.json\z`),
		legacySnapshots: parserkit.MustRelPattern(`\A\.little-coder/checkpoints/([^/]+)/snapshots\.json\z`),
		sessionsKey:     regexp.MustCompile(`"sessions"[ \t\n\r]*:[ \t\n\r]*`),
		messagesKey:     regexp.MustCompile(`"messages"[ \t\n\r]*:[ \t\n\r]*`),
		sessionID:       regexp.MustCompile(`"session_id"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"`),
		savedAt:         regexp.MustCompile(`"saved_at"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"`),
		firstCheckpoint: map[checkpointKey]string{},
		sessionIndexes:  map[string]*sessionIndex{},
	}
}

// Marks says the parser records synced and relayed sessions.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Relay: true}
}

// Kinds returns the prompt history, the legacy input history and config,
// the checkpoints and the legacy sessions.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelKind("prompt history", historyRel),
		parserkit.RelKind("input_history.txt", legacyInputRel), parserkit.RelKind("config.json", legacyConfigRel),
		parserkit.PatternKind("checkpoint", p.checkpoint), parserkit.PatternKind("legacy session", p.legacySession)}
}

// Wants reports whether the artifact is one of little-coder's files.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is a v0.0.x file that repeats a
// saved session or a prompt.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return textlines.EqualFoldASCII(artifact.Rel, legacyHistoryRel) ||
		textlines.EqualFoldASCII(artifact.Rel, legacyLatestRel) || textlines.EqualFoldASCII(artifact.Rel, legacyInputRel)
}

// parseHistory emits one user row per prompt of the prompt history; an
// entry that is not a string is a problem.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for position, prompt := range parserkit.JSONArray(artifact.DiskPath, &problems) {
		if !prompt.IsString() {
			problems.AddRaw(position, parserkit.UnparseableRecord(position, errNotString), parserkit.JSONText(prompt))
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseInputHistory emits one user row per line of input_history.txt, with
// no time.
func (p *Parser) parseInputHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, raw := range parserkit.Lines(artifact.DiskPath, &problems) {
		text := parserkit.LineText(raw)
		if strings.TrimSpace(text) == "" {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine, row.TurnType, row.Text = line, timeline.TurnUser, timeline.Compact(text)
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// isZero reports whether value is a number equal to zero or false: a
// Telegram chat id of 0 or false names no chat.
func isZero(value jsonvalue.Value) bool {
	return (value.IsNumber() || value.IsBool()) && value.IsEmpty()
}

// parseLegacyConfig emits the gist, Telegram chat and permission mode rows
// of config.json, at the file's modification time, then one "stored
// credentials:" row of its <provider>_api_key, telegram_token and
// gist_token values in file order, each through parserkit.SecretText.
func (p *Parser) parseLegacyConfig(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.DecodeJSONBytes(artifact, data)
	timestamp := parserkit.FileMtime(artifact)
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
	if gist := document.Get("cloudsave_last_gist_id"); parserkit.Present(gist) {
		put(timeline.Compact("session synced: gist " + parserkit.SettingOrDash(gist) + source))
	}
	if chat := document.Get("telegram_chat_id"); parserkit.Present(chat) && !isZero(chat) {
		put(timeline.Compact("session relayed: telegram chat " + parserkit.SettingOrDash(chat) + source))
	}
	if mode := document.Get("permission_mode"); parserkit.Present(mode) {
		put(timeline.Compact("permission setting: permission_mode=" + parserkit.SettingOrDash(mode) + source))
	}
	var stored []string
	for _, member := range document.Members() {
		credential := strings.HasSuffix(member.Key, "_api_key") || member.Key == "telegram_token" ||
			member.Key == "gist_token"
		if value := parserkit.Text(member.Value); credential && value != "" {
			stored = append(stored, member.Key+"="+parserkit.SecretText(artifact, value))
		}
	}
	if len(stored) > 0 {
		put(timeline.Compact("stored credentials: " + strings.Join(stored, " ") + source))
	}
}

// parseLegacySnapshots emits one row per snapshot of a v0.0.x
// checkpoints/<session id>/snapshots.json; a snapshot's file_backups that is
// not an object, or a backup in it that is neither an object nor null (no
// backup, as a null backup_filename), adds a problem.
func (p *Parser) parseLegacySnapshots(artifact *input.Artifact, emit parserkit.Emit) {
	directorySession := strings.Split(artifact.Rel, "/")[2]
	var problems parserkit.Problems
	for position, snapshot := range parserkit.JSONArray(artifact.DiskPath, &problems) {
		if !snapshot.IsObject() {
			problems.AddRaw(position, parserkit.UnparseableRecord(position, errNotObject), parserkit.JSONText(snapshot))
			continue
		}
		var files []string
		for _, backup := range parserkit.ObjectMember(snapshot.Get("file_backups"), "file_backups", position, &problems) {
			if !backup.Value.IsObject() && !backup.Value.IsNull() {
				problems.Add(position, parserkit.WrongKind("file_backups."+backup.Key, jsonvalue.Object))
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
			parserkit.SettingOrDash(snapshot.Get("id"))+": turn_count="+
			parserkit.SettingOrDash(snapshot.Get("turn_count"))+" prompt="+
			parserkit.SettingOrDash(snapshot.Get("user_prompt_preview"))+" files="+listed+
			" (created_at host local time)"))
		row.SourceLine = position
		row.TimestampUTC = timeutil.FromValue(snapshot.Get("created_at"))
		row.SessionID = cmp.Or(parserkit.Text(snapshot.Get("session_id")), directorySession)
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// Parse emits the rows of one little-coder file.
func (p *Parser) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case textlines.EqualFoldASCII(rel, historyRel):
		p.parseHistory(artifact, emit)
	case textlines.EqualFoldASCII(rel, legacyInputRel):
		p.parseInputHistory(artifact, emit)
	case textlines.EqualFoldASCII(rel, legacyConfigRel):
		p.parseLegacyConfig(artifact, emit)
	case textlines.EqualFoldASCII(rel, legacyHistoryRel):
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

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
