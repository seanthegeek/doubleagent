package openhands

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The host offset is rounded to a quarter hour after skew seconds are
// added, and an offset beyond maxOffset seconds is not believed.
const (
	quarter   = 900
	skew      = 60
	maxOffset = 14 * 3600
)

// worktreeRoots are the Agent Server's default conversation_worktree_root;
// macOS resolves /tmp to /private/tmp.
var worktreeRoots = []string{"/tmp/conversation-worktrees/", "/private/tmp/conversation-worktrees/"}

// switchKinds are the observations that switch the model.
var switchKinds = []string{"SwitchLLMObservation", "ClassifyAndSwitchLLMObservation"}

// setting is the text of a "permission setting:" row, with the tool call it
// is joined to.
type setting struct {
	text, toolName, toolUseID string
}

// modelStep is a "model:" row written after an event, with the model in force
// from then on.
type modelStep struct {
	text, model string
}

// conversation is what one conversation's directory gives every event of it.
type conversation struct {
	sessionID, projectPath, model string
	offset                        int
	hasOffset                     bool
	firstEvent, header, created   string
	// problems are the whole-file entries of a meta.json or base_state.json
	// that exists but cannot be read or did not decode.
	problems parserkit.Problems
	// metaRepeats and stateRepeats are the member names meta.json and
	// base_state.json repeat, which the parse of each file reports.
	metaRepeats, stateRepeats jsonvalue.Repeats
	// initialModel is the model in force at the conversation's start: the
	// model of the first response before any switch, else base_state.json's
	// agent.llm.model.
	initialModel string
	// modelBefore and modelAfter are, per event file name, a "model:" row
	// written before or after the event's own rows; eventModel is the model
	// in force at the event.
	modelBefore map[string]string
	modelAfter  map[string]modelStep
	eventModel  map[string]string
	// finalModelRow is the row for the base_state.json artifact when its
	// agent.llm.model differs from the last model the events show.
	finalModelRow string
	// actions maps an ActionEvent id to its tool name and call id, for the
	// security analyzer's verdicts and the blocked actions, which name
	// action ids.
	actions  map[string][2]string
	settings []setting
	scratch  string
}

// ignoredLookupError reports whether a failed look at a path is as good as
// the path being absent: it does not exist, or a part of it is not a
// directory or is a link loop.
func ignoredLookupError(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) ||
		errors.Is(err, syscall.ELOOP)
}

// errFileNotObject is the problem of a JSON file that holds something other
// than an object.
var errFileNotObject = errors.New("is not a JSON object")

// loadJSON returns the JSON in the regular file at path and whether it
// decoded, or the error that says why it did not: "could not be decoded
// (<reason>)". A file that exists but cannot be read adds its unreadable
// problem to problems. A missing file, a link or a directory gives nothing
// and no error, since the CLI writes no meta.json and that is not an error.
func loadJSON(repeats *jsonvalue.Repeats, path string, problems *parserkit.Problems) (jsonvalue.Value, bool, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		if !ignoredLookupError(err) {
			problems.AddUnreadable(0, path, err)
		}
		return jsonvalue.NewNull(), false, nil
	}
	if !info.Mode().IsRegular() {
		return jsonvalue.NewNull(), false, nil
	}
	data, ok := parserkit.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false, nil
	}
	document := jsonvalue.NewDocument(string(data))
	value, err := document.Decode()
	if err != nil {
		return jsonvalue.NewNull(), false, fmt.Errorf("could not be decoded (%w)", err)
	}
	repeats.Merge(document.TakeRepeats())
	return value, true, nil
}

// loadObject returns the JSON object in the file at path, else null. A file
// that exists but cannot be read adds its whole-file problem to problems;
// one that does not decode or is not an object adds "<name> <reason>" at
// line 1.
func loadObject(repeats *jsonvalue.Repeats, path string, problems *parserkit.Problems) jsonvalue.Value {
	var unread parserkit.Problems
	value, decoded, err := loadJSON(repeats, path, &unread)
	if len(unread) > 0 {
		*problems = append(*problems, unread...)
		return jsonvalue.NewNull()
	}
	if err == nil && decoded && !value.IsObject() {
		err = errFileNotObject
	}
	if err != nil {
		problems.Add(1, filepath.Base(path)+" "+err.Error())
	}
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewNull()
}

// readObject returns the JSON object in a file this parser also parses in
// full, else null: a pre-scan, whose problems are reported when the file is
// parsed as an artifact or loaded as a conversation's state.
func readObject(path string) jsonvalue.Value {
	var ignored jsonvalue.Repeats
	value, _, _ := loadJSON(&ignored, path, &parserkit.Problems{})
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewNull()
}

// conversationFiles are the paths of a conversation directory's own files.
type conversationFiles struct {
	directory, meta, state, events string
}

// filesOf returns the paths of the files of the conversation directory at
// path, or false when path is not a directory or is a link: the directory
// is looked at once here, not once per file read from it.
func filesOf(path string) (conversationFiles, bool) {
	directory, ok := parserkit.DirectoryAt(path)
	if !ok {
		return conversationFiles{}, false
	}
	files := conversationFiles{directory: path}
	files.meta, _ = directory.Child("meta.json")
	files.state, _ = directory.Child("base_state.json")
	files.events, _ = directory.Child("events")
	return files, true
}

// conversation returns the conversation in a directory, loaded once for as
// long as its files keep their times, by the parse of artifact that first
// asks for it.
func (p *Parser) conversation(artifact *input.Artifact, files conversationFiles) *conversation {
	key := cacheKey{directory: files.directory, meta: mtimeOf(files.meta), state: mtimeOf(files.state),
		events: mtimeOf(files.events)}
	if held, ok := p.cache[key]; ok {
		return held
	}
	loaded := p.load(artifact, files)
	if len(p.cache) > cacheLimit {
		clear(p.cache)
	}
	p.cache[key] = loaded
	return loaded
}

// eventFile is an event file of a conversation: its index, name and path.
type eventFile struct {
	index, name, path string
}

// eventFiles returns the event files of the events directory at path in
// index order; none when it is not a directory or is a link.
func (p *Parser) eventFiles(path string) []eventFile {
	directory, ok := parserkit.DirectoryAt(path)
	if !ok {
		return nil
	}
	entries, err := parserkit.ReadDir(path)
	if err != nil {
		return nil
	}
	var files []eventFile
	for _, entry := range entries {
		match := p.eventName.FindStringSubmatch(entry.Name())
		if match == nil || entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if file, ok := directory.Child(entry.Name()); ok {
			files = append(files, eventFile{match[1], entry.Name(), file})
		}
	}
	slices.SortStableFunc(files, func(left, right eventFile) int {
		if order := parserkit.CompareDigits(left.index, right.index); order != 0 {
			return order
		}
		return strings.Compare(left.name, right.name)
	})
	return files
}

// load reads a conversation's files in the parse of artifact.
func (p *Parser) load(artifact *input.Artifact, paths conversationFiles) *conversation {
	conv := &conversation{sessionID: filepath.Base(paths.directory), modelBefore: map[string]string{},
		modelAfter: map[string]modelStep{}, eventModel: map[string]string{},
		actions: map[string][2]string{}}
	// Each file of the conversation is parsed as its own artifact: the
	// repeats of meta.json and base_state.json are kept for their parses to
	// report, an event's are its own parse's.
	meta := loadObject(&conv.metaRepeats, paths.meta, &conv.problems)
	state := loadObject(&conv.stateRepeats, paths.state, &conv.problems)
	conv.projectPath = parserkit.Text(parserkit.FirstNonEmpty(meta.Get("workspace").Get("working_dir"),
		state.Get("workspace").Get("working_dir")))
	conv.model = parserkit.Text(state.Get("agent").Get("llm").Get("model"))
	files := p.eventFiles(paths.events)
	if len(files) > 0 {
		conv.firstEvent = files[0].name
	}
	loaded := make([]loadedEvent, len(files))
	for i, file := range files {
		loaded[i] = loadedEvent{file.name, readObject(file.path)}
	}
	first := jsonvalue.NewNull()
	if len(loaded) > 0 {
		first = loaded[0].event
	}
	conv.trackModels(artifact, state, loaded)
	conv.settings = settingRows(state, conv.actions, &conv.problems)
	conv.scratch = scratchText(conv.projectPath)
	conv.anchor(meta, first)
	conv.header = header(conv, meta, state, len(files), first)
	return conv
}

// anchor sets the host's offset from the first event's time and meta.json's
// created_at, and the conversation's start time.
func (c *conversation) anchor(meta, first jsonvalue.Value) {
	created, hasCreated := aware(meta.Get("created_at"))
	naiveFirst, hasNaive := naive(first.Get("timestamp"))
	if hasCreated && hasNaive {
		diff := float64(naiveFirst.Sub(created).Microseconds()) / 1e6
		offset := int(floorDivide(diff+skew, quarter)) * quarter
		if offset >= -maxOffset && offset <= maxOffset {
			c.offset, c.hasOffset = offset, true
		}
	}
	if hasCreated {
		c.created = timeutil.FromText(meta.Get("created_at").StringValue())
	} else {
		c.created = c.timestamp(first.Get("timestamp"))
	}
}

// floorDivide returns the floor of left over right, computed from the
// remainder (math.Mod) rather than from left/right, whose rounding can
// carry a quotient just below an integer up to it: 899.9999999999999 over
// 900 is 0 quarters, not 1. Once the remainder is taken away the quotient
// is a whole number up to rounding error, so it is rounded to the nearest.
func floorDivide(left, right float64) float64 {
	remainder := math.Mod(left, right)
	quotient := (left - remainder) / right
	if remainder != 0 && (right < 0) != (remainder < 0) {
		quotient--
	}
	if quotient == 0 {
		return math.Copysign(0, left/right)
	}
	floor := math.Floor(quotient)
	if quotient-floor > 0.5 {
		floor++
	}
	return floor
}

// header returns the text of the conversation's start row.
func header(conv *conversation, meta, state jsonvalue.Value, events int, first jsonvalue.Value) string {
	basis := ""
	if conv.hasOffset {
		basis = "event times are host local, corrected by " + formatOffset(conv.offset) +
			" from meta.json created_at"
	} else if _, ok := naive(first.Get("timestamp")); ok {
		basis = "event times are host local with no zone, emitted as if UTC"
	}
	labelled := func(label string, value jsonvalue.Value) string {
		if !value.IsEmpty() {
			return label + "=" + value.Text()
		}
		return ""
	}
	model := cmp.Or(conv.initialModel, conv.model)
	if model != "" {
		model = "model=" + model
	}
	return parserkit.JoinNonEmpty(" | ", "conversation start", parserkit.Text(meta.Get("title")), labelled("id", meta.Get("id")),
		labelled("parent", meta.Get("parent_conversation_id")),
		labelled("forked_from", meta.Get("forked_from_conversation_id")), model,
		labelled("status", state.Get("execution_status")), strconv.Itoa(events)+" events", basis)
}

// formatOffset returns an offset in seconds as "UTC+HH:MM".
func formatOffset(seconds int) string {
	sign := '+'
	if seconds < 0 {
		sign = '-'
		seconds = -seconds
	}
	return fmt.Sprintf("UTC%c%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

// aware returns the time of a text timeutil reads, else false.
func aware(value jsonvalue.Value) (time.Time, bool) {
	if !value.IsString() {
		return time.Time{}, false
	}
	stamp := timeutil.FromText(value.StringValue())
	if stamp == "" {
		return time.Time{}, false
	}
	moment, err := time.Parse("2006-01-02T15:04:05.000Z", stamp)
	return moment, err == nil
}

// naiveShape is the shape of a time written with no zone: a date, or a
// date and a time to the minute, second or fraction, as timeutil's forms
// without the Z or offset (a T, t or space before the time). time.Parse is
// given only text of this shape, since it would also take a one-digit hour.
var naiveShape = regexp.MustCompile(`\A[0-9]{4}-[0-9]{2}-[0-9]{2}` +
	`(?:[Tt ][0-9]{2}:[0-9]{2}(?::[0-9]{2}(?:[.,][0-9]+)?)?)?\z`)

// naive returns the time of a text holding a time with no zone, else false.
// OpenHands writes an event's timestamp in host local time with
// microseconds and no zone (research/openhands.md, "Joins and
// timestamps"); the fraction is cut to microseconds and ASCII whitespace
// around the text is ignored.
func naive(value jsonvalue.Value) (time.Time, bool) {
	text := strings.Trim(value.StringValue(), textlines.ASCIIWhitespace)
	if !value.IsString() || !naiveShape.MatchString(text) {
		return time.Time{}, false
	}
	if len(text) > 10 {
		text = text[:10] + "T" + text[11:]
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		moment, err := time.Parse(layout, text)
		if err == nil && moment.Year() >= 1 {
			return moment.Truncate(time.Microsecond), true
		}
	}
	return time.Time{}, false
}

// timestamp returns an event time as UTC: a naive time less the host's
// offset when the conversation has one, else the text as timeutil reads it.
func (c *conversation) timestamp(value jsonvalue.Value) string {
	if moment, ok := naive(value); ok && c.hasOffset {
		return timeutil.FormatUTC(moment.Add(-time.Duration(c.offset) * time.Second))
	}
	if value.IsString() {
		return timeutil.FromText(value.StringValue())
	}
	return ""
}

// scratchText returns the "scratch directory:" text for a directory under
// the default worktree root.
func scratchText(workingDir string) string {
	for _, root := range worktreeRoots {
		if rest, ok := strings.CutPrefix(workingDir, root); ok && strings.Trim(rest, "/") != "" {
			return "scratch directory: " + workingDir + " (conversation worktree under the Agent " +
				"Server's default conversation_worktree_root /tmp/conversation-worktrees)"
		}
	}
	return ""
}

// loadedEvent is an event file's name and its object, null when it has none.
type loadedEvent struct {
	name  string
	event jsonvalue.Value
}

// responseModels returns the model of each response id, from the token
// usages of base_state.json's stats; the first model of an id stands. Each
// event naming a response copies its model into its rows, so the model is
// cut ("token usage model") as it is filed. A usage_to_metrics or
// token_usages of the wrong kind adds a problem.
func responseModels(state jsonvalue.Value, carried *parserkit.Carried, problems *parserkit.Problems) map[string]string {
	byResponse := map[string]string{}
	usages := state.Get("stats").Get("usage_to_metrics")
	for _, metrics := range parserkit.ObjectMember(usages, "stats.usage_to_metrics", 1, problems) {
		member := "stats.usage_to_metrics." + metrics.Key + ".token_usages"
		for _, usage := range parserkit.ArrayMember(metrics.Value.Get("token_usages"), member, 1, problems) {
			response, model := usage.Get("response_id"), usage.Get("model")
			if responseID, ok := nonEmptyText(response); ok {
				if name, ok := nonEmptyText(model); ok {
					if _, set := byResponse[responseID]; !set {
						byResponse[responseID] = carried.Cut("token usage model", name)
					}
				}
			}
		}
	}
	return byResponse
}

// trackModels fills the conversation's model rows and ActionEvent map from
// its events in index order. Each response's model comes from the token
// usages joined on the event's llm_response_id, the agent's own switches
// from the switch observations' active_model, and a final row when
// agent.llm.model differs from the last model the events show, since a
// switch over REST leaves no event.
func (c *conversation) trackModels(artifact *input.Artifact, state jsonvalue.Value, loaded []loadedEvent) {
	carried := parserkit.NewCarried(artifact)
	byResponse := responseModels(state, carried, &c.problems)
	carried.Report(0, &c.problems)
	initial, switched := "", false
	for _, each := range loaded {
		if next, _ := switchOf(each.event); next != "" {
			switched = true
			break
		}
		initial = byResponse[parserkit.Text(each.event.Get("llm_response_id"))]
		if initial != "" {
			break
		}
	}
	if initial == "" && !switched {
		initial = c.model
	}
	c.initialModel = initial
	current := initial
	for _, each := range loaded {
		event := each.event
		if parserkit.IsText(event.Get("kind"), "ActionEvent") && !event.Get("id").IsEmpty() {
			c.actions[event.Get("id").Text()] = [2]string{parserkit.Text(event.Get("tool_name")),
				parserkit.Text(event.Get("tool_call_id"))}
		}
		if model := byResponse[parserkit.Text(event.Get("llm_response_id"))]; model != "" && model != current {
			c.modelBefore[each.name] = modelText(current, model)
			current = model
		}
		c.eventModel[each.name] = current
		if next, suffix := switchOf(event); next != "" && next != current {
			c.modelAfter[each.name] = modelStep{modelText(current, next) + suffix, next}
			current = next
		}
	}
	if c.model != "" && current != "" && c.model != current {
		c.finalModelRow = modelText(current, c.model) + " (from base_state.json agent.llm, file mtime)"
	}
}

// modelText returns "model: <old> -> <new>", or "model: <new>" when there is
// no old model.
func modelText(old, next string) string {
	if old != "" {
		return "model: " + old + " -> " + next
	}
	return "model: " + next
}

// switchOf returns the new model of a model switch and its " profile=<name>[
// class=<class>]" suffix, else two empty texts.
func switchOf(event jsonvalue.Value) (string, string) {
	observation := event.Get("observation")
	if !parserkit.IsText(event.Get("kind"), "ObservationEvent") ||
		!slices.ContainsFunc(switchKinds, func(kind string) bool {
			return parserkit.IsText(observation.Get("kind"), kind)
		}) {
		return "", ""
	}
	next, ok := nonEmptyText(observation.Get("active_model"))
	if !observation.Get("is_error").IsEmpty() || !ok {
		return "", ""
	}
	suffix := ""
	if profile, ok := nonEmptyText(parserkit.FirstNonEmpty(observation.Get("profile_name"), observation.Get("model"))); ok {
		suffix = " profile=" + profile
	}
	if class := observation.Get("chosen_class"); !class.IsEmpty() {
		suffix += " class=" + class.Text()
	}
	return next, suffix
}

// settingRows returns each "permission setting:" row of base_state.json:
// the confirmation policy, the security analyzer ("none" when the key is
// absent, since the file is written without null fields) and one per
// hook-blocked action, joined to its tool call when the events name it. A
// blocked_actions that is not an object adds a problem.
func settingRows(state jsonvalue.Value, actions map[string][2]string, problems *parserkit.Problems) []setting {
	if state.IsEmpty() {
		return nil
	}
	const tail = " (from base_state.json, file mtime)"
	var rows []setting
	policy := state.Get("confirmation_policy")
	if kind := policy.Get("kind"); !kind.IsEmpty() {
		text := "permission setting: confirmation_policy=" + kind.Text()
		if threshold := policy.Get("threshold"); !threshold.IsEmpty() {
			text += " threshold=" + threshold.Text()
		}
		if policy.Has("confirm_unknown") {
			text += " confirm_unknown=" + parserkit.SettingValue(policy.Get("confirm_unknown"))
		}
		rows = append(rows, setting{text: text + tail})
	}
	analyzer := parserkit.TextOr(state.Get("security_analyzer").Get("kind"), "none")
	rows = append(rows, setting{text: "permission setting: security_analyzer=" + analyzer + tail})
	for _, blocked := range parserkit.ObjectMember(state.Get("blocked_actions"), "blocked_actions", 1, problems) {
		held := actions[blocked.Key]
		rows = append(rows, setting{"permission setting: blocked_actions." + blocked.Key + "=" +
			parserkit.SettingValue(blocked.Value) + tail, held[0], held[1]})
	}
	return rows
}
