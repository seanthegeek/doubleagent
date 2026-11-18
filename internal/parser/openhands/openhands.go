// Package openhands reads OpenHands conversations: Agent Canvas, the Agent
// Server and the openhands CLI.
//
// Files read, under .openhands/ (see research/openhands.md):
//
//   - [agent-canvas/[dev_]]conversations/<hex id>/events/event-<index>-<id>.json,
//     one event object per file. The conversation's meta.json (title,
//     created_at, parent and fork ids, workspace.working_dir) and
//     base_state.json (agent.llm.model, confirmation_policy,
//     security_analyzer, blocked_actions, token usage) are read once per
//     conversation; their secret fields are never read.
//   - base_state.json as its own file: the standing "permission setting:" rows
//     and a "model:" row for a switch made over REST, at the file's time.
//   - projects/<sha256 of cwd>/prompt_history.json, the CLI's prompts, a
//     secondary source (a file repeating the conversations' prompts).
//   - Legacy 0.x sessions/<sid>/events/<n>.json: one row per session, not
//     parsed.
//
// Event times are naive host local time. When meta.json has a UTC
// created_at, the host's offset is the first event's time less created_at,
// floored to the quarter hour after a minute of skew; without it (the CLI
// writes none) the times are emitted as if UTC, and the header row says
// which. The session id is the directory name; the project path is
// workspace.working_dir; no git branch is recorded; the source line is 1.
package openhands

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// cacheLimit is how many conversations the parser keeps before it drops them
// all.
const cacheLimit = 256

// conversationGlobs are the files, under .openhands, that name a project's
// working directory, as directory lists and a file name.
var conversationGlobs = []struct {
	directory []string
	file      string
}{
	{[]string{"agent-canvas", "dev_conversations"}, "base_state.json"},
	{[]string{"agent-canvas", "conversations"}, "base_state.json"},
	{[]string{"conversations"}, "base_state.json"},
	{[]string{"agent-canvas", "dev_conversations"}, "meta.json"},
	{[]string{"agent-canvas", "conversations"}, "meta.json"},
}

// Parser reads OpenHands files. It keeps the conversations it has loaded.
type Parser struct {
	parser.Base
	event, eventName, state, history, legacy *regexp.Regexp
	cache                                    map[cacheKey]*conversation
}

// cacheKey names a conversation as loaded: its directory and the
// modification times of the files and directory it was read from, so a
// changed conversation is read again.
type cacheKey struct {
	directory           string
	meta, state, events int64
}

// New returns the parser.
func New() *Parser {
	const conversations = `\.openhands/(?:agent-canvas/(?:dev_)?conversations|conversations)/` +
		`([0-9a-fA-F]{32})/`
	return &Parser{
		Base:      parser.NewBase("openhands", "openhands"),
		event:     regexp.MustCompile(`\A` + conversations + `events/event-([0-9]{5,})-[0-9a-fA-F-]{8,}\.json\z`),
		eventName: regexp.MustCompile(`\Aevent-([0-9]{5,})-[0-9a-fA-F-]{8,}\.json\z`),
		state:     regexp.MustCompile(`\A` + conversations + `base_state\.json\z`),
		history:   regexp.MustCompile(`\A\.openhands/projects/([0-9a-f]{64})/prompt_history\.json\z`),
		legacy: regexp.MustCompile(
			`\A\.openhands/(?:sessions|users/[^/]+/conversations)/([^/]+)/events/([0-9]+)\.json\z`),
		cache: map[cacheKey]*conversation{},
	}
}

// Wants reports whether the artifact is an event file, base_state.json, a
// prompt history or a legacy event.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.event.MatchString(rel) || p.legacy.MatchString(rel) || p.state.MatchString(rel) ||
		p.history.MatchString(rel)
}

// Secondary reports whether the artifact is the CLI's prompt history.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.history.MatchString(artifact.Rel)
}

// Parse emits the rows of one OpenHands file; a conversation's first event
// adds its header.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch rel := artifact.Rel; {
	case p.legacy.MatchString(rel):
		p.parseLegacy(artifact, emit)
	case p.history.MatchString(rel):
		p.parseHistory(artifact, emit)
	case p.state.MatchString(rel):
		p.parseState(artifact, emit)
	default:
		p.parseEvent(artifact, options, emit)
	}
	return nil
}

// orDefault returns a value as row text, or fallback when the value is
// empty (jsonvalue.Value.IsEmpty).
func orDefault(value jsonvalue.Value, fallback string) string {
	if !value.IsEmpty() {
		return value.Text()
	}
	return fallback
}

// isString reports whether value is the string want.
func isString(value jsonvalue.Value, want string) bool {
	return value.IsString() && value.StringValue() == want
}

// nonEmptyText returns a value's string when it is a non-empty string.
func nonEmptyText(value jsonvalue.Value) (string, bool) {
	return value.StringValue(), value.IsString() && value.StringValue() != ""
}

// mtimeOf returns a file's modification time in nanoseconds, 0 when it
// cannot be read.
func mtimeOf(path string) int64 {
	info, err := osfile.Lstat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

// sha256Hex returns the hex SHA-256 of text.
func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// parseHistory emits one user row per prompt of the CLI's prompt history.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	digest := p.history.FindStringSubmatch(artifact.Rel)[1]
	project := p.projectForHash(artifact, digest)
	var problems parser.Problems
	for index, entry := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !entry.IsObject() {
			problems.AddRaw(index, parser.UnparseableRecord(index, errNotObject), parser.JSONText(entry))
			continue
		}
		body, ok := nonEmptyText(entry.Get("text"))
		if !ok {
			continue
		}
		row := p.BaseRow(artifact)
		row.TurnType = timeline.TurnUser
		row.ProjectPath = project
		row.SourceLine = index
		if stamp := entry.Get("timestamp"); stamp.IsString() {
			row.TimestampUTC = timeutil.FromText(stamp.StringValue())
		}
		row.Text = body
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// errNotObject is the reason a prompt history entry is skipped.
var errNotObject = errors.New("not a JSON object")

// projectForHash returns the working directory under the .openhands
// directory, two levels above the history file, whose SHA-256 is
// digest, else "". The directories are the
// conversations' workspace.working_dir, and the digest is the name the CLI
// gives a project's directory under projects/.
func (p *Parser) projectForHash(artifact *input.Artifact, digest string) string {
	for _, glob := range conversationGlobs {
		directory, found := parser.Above(artifact, 2, glob.directory...)
		if !found {
			return ""
		}
		entries, err := parser.ReadDir(directory)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			conversation, reachable := parser.Under(directory, name, glob.file)
			if !reachable {
				continue
			}
			document, _, _ := loadJSON(conversation, &parser.Problems{})
			dir, ok := nonEmptyText(document.Get("workspace").Get("working_dir"))
			if ok && sha256Hex(dir) == digest {
				return dir
			}
		}
	}
	return ""
}

// parseState emits the standing permission rows of a base_state.json and the
// model row of a switch made over REST.
func (p *Parser) parseState(artifact *input.Artifact, emit parser.Emit) {
	directory, ok := parser.Beside(artifact)
	if !ok {
		return
	}
	conv := p.conversation(directory)
	timestamp := parser.FileMtime(artifact)
	texts := slices.Clone(conv.settings)
	if conv.finalModelRow != "" {
		texts = append(texts, setting{text: conv.finalModelRow})
	}
	for _, each := range texts {
		row := p.row(artifact, conv, timeline.TurnSystem)
		row.TimestampUTC = timestamp
		row.Text = each.text
		row.ToolName, row.ToolUseID = each.toolName, each.toolUseID
		emit(row)
	}
}

// row returns a row of the conversation with the given turn type.
func (p *Parser) row(artifact *input.Artifact, conv *conversation, turn string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SessionID = conv.sessionID
	row.ProjectPath = conv.projectPath
	row.Model = conv.model
	row.TurnType = turn
	row.SourceLine = 1
	return row
}

// parseEvent emits the rows of one event file; the conversation's first event
// adds the header rows.
func (p *Parser) parseEvent(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	directory, found := parser.Above(artifact, 1)
	if !found {
		return
	}
	conv := p.conversation(directory)
	name := filepath.Base(artifact.DiskPath)
	if name == conv.firstEvent {
		p.headerRows(artifact, conv, emit)
	}
	var problems parser.Problems
	event, decoded, err := loadJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, conv.sessionID, conv.projectPath))
		return
	}
	if !decoded || !event.IsObject() {
		row := p.row(artifact, conv, timeline.TurnSystem)
		if err == nil {
			err = errFileNotObject
		}
		row.Text = "parser: event file " + err.Error()
		emit(row)
		return
	}
	timestamp := conv.timestamp(event.Get("timestamp"))
	model, found := conv.eventModel[name]
	if !found {
		model = cmp.Or(conv.initialModel, conv.model)
	}
	var rows []timeline.Row
	if before, ok := conv.modelBefore[name]; ok {
		row := p.row(artifact, conv, timeline.TurnSystem)
		row.Text = before
		rows = append(rows, row)
	}
	rows = append(rows, p.eventRows(artifact, conv, event, options)...)
	for _, row := range rows {
		row.TimestampUTC = timestamp
		row.Model = model
		emit(row)
	}
	if after, ok := conv.modelAfter[name]; ok {
		row := p.row(artifact, conv, timeline.TurnSystem)
		row.Text, row.Model = after.text, after.model
		row.TimestampUTC = timestamp
		emit(row)
	}
}

// headerRows emits the conversation's start row, its initial model and
// scratch rows, and the problems met reading its meta.json and
// base_state.json.
func (p *Parser) headerRows(artifact *input.Artifact, conv *conversation, emit parser.Emit) {
	row := p.row(artifact, conv, timeline.TurnSystem)
	row.TimestampUTC = conv.created
	row.Text = timeline.Compact(conv.header)
	emit(row)
	initial := ""
	if conv.initialModel != "" {
		initial = "model: " + conv.initialModel
	}
	for _, text := range []string{initial, conv.scratch} {
		if text == "" {
			continue
		}
		row := p.row(artifact, conv, timeline.TurnSystem)
		row.TimestampUTC = conv.created
		row.Model = cmp.Or(conv.initialModel, conv.model)
		row.Text = text
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, conv.problems, conv.sessionID, conv.projectPath))
}

// parseLegacy emits the one row of a legacy 0.x session: from its lowest
// numbered event file only.
func (p *Parser) parseLegacy(artifact *input.Artifact, emit parser.Emit) {
	match := p.legacy.FindStringSubmatch(artifact.Rel)
	var numbers []string
	directory, found := parser.Beside(artifact)
	if entries, err := parser.ReadDir(directory); found && err == nil {
		for _, entry := range entries {
			stem, isJSON := strings.CutSuffix(entry.Name(), ".json")
			if isJSON && stem != "" && strings.Trim(stem, "0123456789") == "" &&
				entry.Type()&fs.ModeSymlink == 0 {
				numbers = append(numbers, stem)
			}
		}
	}
	if len(numbers) > 0 && parser.CompareDigits(match[2], slices.MinFunc(numbers, parser.CompareDigits)) != 0 {
		return
	}
	var problems parser.Problems
	event, _, _ := loadJSON(artifact.DiskPath, &problems)
	row := p.BaseRow(artifact)
	row.SessionID = match[1]
	row.TurnType = timeline.TurnSystem
	row.SourceLine = 1
	if stamp := event.Get("timestamp"); stamp.IsString() {
		row.TimestampUTC = timeutil.FromText(stamp.StringValue())
	}
	row.Text = timeline.Compact("legacy OpenHands 0.x session, " + strconv.Itoa(len(numbers)) +
		" event files, not parsed; times are host local, emitted as if UTC")
	emit(row)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, row.SessionID, ""))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
