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
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	parserkit.Base
	event, state, meta, history, legacy *parserkit.RelPattern
	eventName                           *regexp.Regexp
	cache                               map[cacheKey]*conversation
	// legacyLowest is the lowest event number of each legacy session's
	// events directory listed, so its other event files are passed over
	// without a listing of their own.
	legacyLowest map[legacyKey]string
	// projects holds, per .openhands directory, the working directory of
	// each digest its conversations name.
	projects map[string]map[string]string
}

// legacyKey names a legacy session's events directory: the home's path on
// disk and the event file's path from the home without its file name.
type legacyKey struct {
	home, events string
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
		Base:      parserkit.NewBase("openhands", "openhands"),
		event:     parserkit.MustRelPattern(`\A` + conversations + `events/event-([0-9]{5,})-[0-9a-fA-F-]{8,}\.json\z`),
		eventName: regexp.MustCompile(`\Aevent-([0-9]{5,})-[0-9a-fA-F-]{8,}\.json\z`),
		state:     parserkit.MustRelPattern(`\A` + conversations + `base_state\.json\z`),
		meta:      parserkit.MustRelPattern(`\A` + conversations + `meta\.json\z`),
		history:   parserkit.MustRelPattern(`\A\.openhands/projects/([0-9a-f]{64})/prompt_history\.json\z`),
		legacy: parserkit.MustRelPattern(
			`\A\.openhands/(?:sessions|users/[^/]+/conversations)/([^/]+)/events/([0-9]+)\.json\z`),
		cache:        map[cacheKey]*conversation{},
		legacyLowest: map[legacyKey]string{},
	}
}

// Wants reports whether the artifact is an event file, base_state.json,
// meta.json, a prompt history or a legacy event.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the conversation events, the legacy event files, the base
// state, the conversation metadata and the prompt history.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("event", p.event), parserkit.PatternKind("legacy event", p.legacy),
		parserkit.PatternKind("base_state.json", p.state), parserkit.PatternKind("meta.json", p.meta),
		parserkit.PatternKind("prompt_history.json", p.history)}
}

// Secondary reports whether the artifact is the CLI's prompt history.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.history.MatchString(artifact.Rel)
}

// Parse emits the rows of one OpenHands file; a conversation's first event
// adds its header.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	switch rel := artifact.Rel; {
	case p.legacy.MatchString(rel):
		p.parseLegacy(artifact, emit)
	case p.history.MatchString(rel):
		p.parseHistory(artifact, emit)
	case p.state.MatchString(rel):
		p.parseState(artifact, emit)
	case p.meta.MatchString(rel):
		p.parseMeta(artifact, emit)
	default:
		p.parseEvent(artifact, options, emit)
	}
	return nil
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
func (p *Parser) parseHistory(artifact *input.Artifact, emit parserkit.Emit) {
	digest := p.history.FindStringSubmatch(artifact.Rel)[1]
	project := p.projectForHash(artifact, digest)
	var problems parserkit.Problems
	for index, entry := range parserkit.JSONArray(artifact.DiskPath, &problems) {
		if !entry.IsObject() {
			problems.AddRaw(index, parserkit.UnparseableRecord(index, errNotObject), parserkit.JSONText(entry))
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// errNotObject is the reason a prompt history entry is skipped.
var errNotObject = errors.New("not a JSON object")

// projectForHash returns the working directory under the .openhands
// directory, two levels above the history file, whose SHA-256 is
// digest, else "". The directories are the
// conversations' workspace.working_dir, and the digest is the name the CLI
// gives a project's directory under projects/. The conversations are read
// once per .openhands directory for the run, since each project's history
// looks its digest up among all of them.
func (p *Parser) projectForHash(artifact *input.Artifact, digest string) string {
	base, found := parserkit.Above(artifact, 2)
	if !found {
		return ""
	}
	index, ok := p.projects[base]
	if !ok {
		index = p.projectIndex(artifact)
		if p.projects == nil {
			p.projects = map[string]map[string]string{}
		}
		p.projects[base] = index
	}
	return index[digest]
}

// projectIndex returns, by SHA-256, the first working directory each
// digest names among the conversations under the .openhands directory two
// levels above the history file, in the order of conversationGlobs and
// then of the conversations' names.
func (p *Parser) projectIndex(artifact *input.Artifact) map[string]string {
	index := map[string]string{}
	for _, glob := range conversationGlobs {
		directory, found := parserkit.Above(artifact, 2, glob.directory...)
		if !found {
			return index
		}
		entries, err := parserkit.ReadDir(directory)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			// A conversation is a real directory: a link in its place is
			// never followed.
			if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			conversation, reachable := parserkit.Under(directory, name, glob.file)
			if !reachable {
				continue
			}
			p.Work.Add("conversations read for projects", 1)
			// The conversation's file is parsed as its own artifact, whose
			// parse reports its repeated member names.
			var ignored jsonvalue.Repeats
			document, _, _ := loadJSON(&ignored, conversation, &parserkit.Problems{})
			dir, ok := nonEmptyText(document.Get("workspace").Get("working_dir"))
			if _, seen := index[sha256Hex(dir)]; ok && !seen {
				index[sha256Hex(dir)] = dir
			}
		}
	}
	return index
}

// parseState emits the standing permission rows of a base_state.json and the
// model row of a switch made over REST. A conversation with no event has no
// header rows, so this file reports the problems of its meta.json and
// base_state.json in their place.
func (p *Parser) parseState(artifact *input.Artifact, emit parserkit.Emit) {
	directory, ok := parserkit.Beside(artifact)
	if !ok {
		return
	}
	files, ok := filesOf(directory)
	if !ok {
		return
	}
	conv := p.conversation(artifact, files)
	parserkit.NoteRepeats(artifact, conv.stateRepeats)
	timestamp := parserkit.FileMtime(artifact)
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
	if conv.firstEvent == "" {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, conv.problems, conv.sessionID, conv.projectPath))
	}
}

// parseMeta emits the problems of a conversation that has neither an event
// file nor a base_state.json, which would otherwise report them: a damaged
// meta.json is then the conversation's only trace. Otherwise it emits
// nothing, since meta.json gives no rows of its own.
func (p *Parser) parseMeta(artifact *input.Artifact, emit parserkit.Emit) {
	directory, ok := parserkit.Beside(artifact)
	if !ok {
		return
	}
	files, ok := filesOf(directory)
	if !ok {
		return
	}
	conv := p.conversation(artifact, files)
	parserkit.NoteRepeats(artifact, conv.metaRepeats)
	if parserkit.RegularFile(files.state) {
		return
	}
	if conv.firstEvent == "" {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, conv.problems, conv.sessionID, conv.projectPath))
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
func (p *Parser) parseEvent(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	directory, found := parserkit.Above(artifact, 1)
	if !found {
		return
	}
	files, found := filesOf(directory)
	if !found {
		return
	}
	conv := p.conversation(artifact, files)
	name := filepath.Base(artifact.DiskPath)
	if name == conv.firstEvent {
		p.headerRows(artifact, conv, emit)
	}
	var problems parserkit.Problems
	var repeats jsonvalue.Repeats
	event, decoded, err := loadJSON(&repeats, artifact.DiskPath, &problems)
	parserkit.NoteRepeats(artifact, repeats)
	if len(problems) > 0 {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, conv.sessionID, conv.projectPath))
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
func (p *Parser) headerRows(artifact *input.Artifact, conv *conversation, emit parserkit.Emit) {
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, conv.problems, conv.sessionID, conv.projectPath))
}

// parseLegacy emits the one row of a legacy 0.x session: from its lowest
// numbered event file only, with that file's problem when it does not
// decode as an object.
func (p *Parser) parseLegacy(artifact *input.Artifact, emit parserkit.Emit) {
	match := p.legacy.FindStringSubmatch(artifact.Rel)
	key := legacyKey{events: artifact.Rel[:len(artifact.Rel)-len(match[2]+".json")]}
	if artifact.Home != nil {
		key.home = artifact.Home.DiskPath
	}
	if lowest, held := p.legacyLowest[key]; held && parserkit.CompareDigits(match[2], lowest) != 0 {
		return
	}
	p.Work.Add("legacy listings", 1)
	var numbers []string
	directory, found := parserkit.Beside(artifact)
	if entries, err := parserkit.ReadDir(directory); found && err == nil {
		for _, entry := range entries {
			stem, isJSON := strings.CutSuffix(entry.Name(), ".json")
			if isJSON && stem != "" && strings.Trim(stem, "0123456789") == "" &&
				entry.Type()&fs.ModeSymlink == 0 {
				numbers = append(numbers, stem)
			}
		}
	}
	if len(numbers) > 0 {
		lowest := slices.MinFunc(numbers, parserkit.CompareDigits)
		p.legacyLowest[key] = lowest
		if parserkit.CompareDigits(match[2], lowest) != 0 {
			return
		}
	}
	var problems parserkit.Problems
	var repeats jsonvalue.Repeats
	event := loadObject(&repeats, artifact.DiskPath, &problems)
	parserkit.NoteRepeats(artifact, repeats)
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, row.SessionID, ""))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
