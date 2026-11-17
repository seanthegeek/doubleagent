package openhands

import (
	"errors"
	"io/fs"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	problems parser.Problems
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

// loadJSON returns the JSON in the regular file at path and whether it
// decoded, or why it did not: "could not be decoded (<reason>)". A file that
// exists but cannot be read adds its unreadable problem to problems. A
// missing file, a link or a directory gives nothing and no reason, since the
// CLI writes no meta.json and that is not an error.
func loadJSON(path string, problems *parser.Problems) (jsonvalue.Value, bool, string) {
	info, err := osfile.Lstat(path)
	if err != nil {
		if !ignoredLookupError(err) {
			problems.AddUnreadable(0, path, err)
		}
		return jsonvalue.NewNull(), false, ""
	}
	if !info.Mode().IsRegular() {
		return jsonvalue.NewNull(), false, ""
	}
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false, ""
	}
	value, err := jsonvalue.Decode(data)
	if err != nil {
		return jsonvalue.NewNull(), false, "could not be decoded (" + err.Error() + ")"
	}
	return value, true, ""
}

// loadObject returns the JSON object in the file at path, else null. A file
// that exists but cannot be read adds its whole-file problem to problems;
// one that does not decode or is not an object adds "<name> <reason>" at
// line 1.
func loadObject(path string, problems *parser.Problems) jsonvalue.Value {
	var unread parser.Problems
	value, decoded, reason := loadJSON(path, &unread)
	if len(unread) > 0 {
		*problems = append(*problems, unread...)
		return jsonvalue.NewNull()
	}
	if reason == "" && decoded && !value.IsObject() {
		reason = "is not a JSON object"
	}
	if reason != "" {
		problems.Add(1, filepath.Base(path)+" "+reason)
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
	value, _, _ := loadJSON(path, &parser.Problems{})
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewNull()
}

// conversation returns the conversation in a directory, loaded once for as
// long as its files keep their times.
func (p *Parser) conversation(directory string) *conversation {
	key := cacheKey{directory: directory,
		meta:   mtimeOf(filepath.Join(directory, "meta.json")),
		state:  mtimeOf(filepath.Join(directory, "base_state.json")),
		events: mtimeOf(filepath.Join(directory, "events"))}
	if held, ok := p.cache[key]; ok {
		return held
	}
	loaded := p.load(directory)
	if len(p.cache) > cacheLimit {
		clear(p.cache)
	}
	p.cache[key] = loaded
	return loaded
}

// eventFile is an event file of a conversation: its name and index.
type eventFile struct {
	index, name string
}

// eventFiles returns the conversation's event files in index order.
func (p *Parser) eventFiles(directory string) []eventFile {
	entries, err := parser.ReadDir(filepath.Join(directory, "events"))
	if err != nil {
		return nil
	}
	var files []eventFile
	for _, entry := range entries {
		match := p.eventName.FindStringSubmatch(entry.Name())
		if match != nil && entry.Type()&fs.ModeSymlink == 0 {
			files = append(files, eventFile{match[1], entry.Name()})
		}
	}
	slices.SortStableFunc(files, func(left, right eventFile) int {
		if order := compareNumbers(left.index, right.index); order != 0 {
			return order
		}
		return strings.Compare(left.name, right.name)
	})
	return files
}

// load reads a conversation's files.
func (p *Parser) load(directory string) *conversation {
	conv := &conversation{sessionID: filepath.Base(directory), modelBefore: map[string]string{},
		modelAfter: map[string]modelStep{}, eventModel: map[string]string{},
		actions: map[string][2]string{}}
	meta := loadObject(filepath.Join(directory, "meta.json"), &conv.problems)
	state := loadObject(filepath.Join(directory, "base_state.json"), &conv.problems)
	conv.projectPath = parser.Text(firstTruthy(meta.Get("workspace").Get("working_dir"),
		state.Get("workspace").Get("working_dir")))
	conv.model = parser.Text(state.Get("agent").Get("llm").Get("model"))
	files := p.eventFiles(directory)
	if len(files) > 0 {
		conv.firstEvent = files[0].name
	}
	loaded := make([]loadedEvent, len(files))
	for i, file := range files {
		loaded[i] = loadedEvent{file.name, readObject(filepath.Join(directory, "events", file.name))}
	}
	first := jsonvalue.NewNull()
	if len(loaded) > 0 {
		first = loaded[0].event
	}
	conv.trackModels(state, loaded)
	conv.settings = settingRows(state, conv.actions)
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

// floorDivide returns the floor of left over right as Python's float //
// computes it, from the remainder, so a quotient that rounds up in a plain
// division does not.
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
	model := firstNonEmpty(conv.initialModel, conv.model)
	if model != "" {
		model = "model=" + model
	}
	return join("conversation start", parser.Text(meta.Get("title")), labelled("id", meta.Get("id")),
		labelled("parent", meta.Get("parent_conversation_id")),
		labelled("forked_from", meta.Get("forked_from_conversation_id")), model,
		labelled("status", state.Get("execution_status")), strconv.Itoa(events)+" events", basis)
}

// formatOffset returns an offset in seconds as "UTC+HH:MM".
func formatOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return "UTC" + sign + pad2(seconds/3600) + ":" + pad2(seconds%3600/60)
}

// pad2 returns a number as two digits at least.
func pad2(number int) string {
	if number < 10 {
		return "0" + strconv.Itoa(number)
	}
	return strconv.Itoa(number)
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

// naive returns the time of an ISO 8601 text that has no zone, else false:
// a date, optionally followed by one separator character and an hour with
// optional minutes, seconds and a fraction.
func naive(value jsonvalue.Value) (time.Time, bool) {
	if !value.IsString() || value.StringValue() == "" {
		return time.Time{}, false
	}
	text := strings.Trim(value.StringValue(), " \t\n\r\f\v")
	if len(text) < 10 || !digitsAt(text, 0, 4) || text[4] != '-' || !digitsAt(text, 5, 2) ||
		text[7] != '-' || !digitsAt(text, 8, 2) {
		return time.Time{}, false
	}
	year, month, day := number(text[0:4]), number(text[5:7]), number(text[8:10])
	rest := text[10:]
	hour, minute, second, nanos := 0, 0, 0, 0
	if rest != "" {
		_, width := utf8.DecodeRuneInString(rest)
		var ok bool
		if hour, minute, second, nanos, ok = clock(rest[width:]); !ok {
			return time.Time{}, false
		}
	}
	moment := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if year < 1 || month < 1 || month > 12 || day < 1 || moment.Day() != day ||
		int(moment.Month()) != month {
		return time.Time{}, false
	}
	return moment.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute +
		time.Duration(second)*time.Second + time.Duration(nanos)), true
}

// digitsAt reports whether count ASCII digits start at offset.
func digitsAt(text string, offset, count int) bool {
	if len(text) < offset+count {
		return false
	}
	for _, char := range []byte(text[offset : offset+count]) {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// number returns the value of ASCII digits.
func number(digits string) int {
	value, _ := strconv.Atoi(digits)
	return value
}

// clock reads "HH[:MM[:SS[.f]]]", a fraction kept to microseconds, and
// returns its parts and whether the text is exactly that. An hour of 24 is
// the next midnight, as Python reads it.
func clock(text string) (hour, minute, second, nanos int, ok bool) {
	if !digitsAt(text, 0, 2) {
		return 0, 0, 0, 0, false
	}
	hour, text = number(text[0:2]), text[2:]
	if strings.HasPrefix(text, ":") && digitsAt(text, 1, 2) {
		minute, text = number(text[1:3]), text[3:]
		if strings.HasPrefix(text, ":") && digitsAt(text, 1, 2) {
			second, text = number(text[1:3]), text[3:]
			if text != "" && (text[0] == '.' || text[0] == ',') {
				digits := len(text) - len(strings.TrimLeft(text[1:], "0123456789")) - 1
				if digits < 1 {
					return 0, 0, 0, 0, false
				}
				nanos = number((text[1:1+min(digits, 6)] + "000000")[:6]) * 1000
				text = text[1+digits:]
			}
		}
	}
	if text != "" || minute > 59 || second > 59 || hour > 24 ||
		(hour == 24 && (minute != 0 || second != 0 || nanos != 0)) {
		return 0, 0, 0, 0, false
	}
	return hour, minute, second, nanos, true
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
// usages of base_state.json's stats; the first model of an id stands.
func responseModels(state jsonvalue.Value) map[string]string {
	byResponse := map[string]string{}
	for _, metrics := range state.Get("stats").Get("usage_to_metrics").Members() {
		for _, usage := range parser.Elements(metrics.Value.Get("token_usages")) {
			response, model := usage.Get("response_id"), usage.Get("model")
			if responseID, ok := nonEmptyText(response); ok {
				if name, ok := nonEmptyText(model); ok {
					if _, set := byResponse[responseID]; !set {
						byResponse[responseID] = name
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
func (c *conversation) trackModels(state jsonvalue.Value, loaded []loadedEvent) {
	byResponse := responseModels(state)
	initial, switched := "", false
	for _, each := range loaded {
		if next, _ := switchOf(each.event); next != "" {
			switched = true
			break
		}
		initial = byResponse[parser.Text(each.event.Get("llm_response_id"))]
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
		if isString(event.Get("kind"), "ActionEvent") && !event.Get("id").IsEmpty() {
			c.actions[event.Get("id").Text()] = [2]string{parser.Text(event.Get("tool_name")),
				parser.Text(event.Get("tool_call_id"))}
		}
		if model := byResponse[parser.Text(event.Get("llm_response_id"))]; model != "" && model != current {
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
	if !isString(event.Get("kind"), "ObservationEvent") ||
		!slices.ContainsFunc(switchKinds, func(kind string) bool {
			return isString(observation.Get("kind"), kind)
		}) {
		return "", ""
	}
	next, ok := nonEmptyText(observation.Get("active_model"))
	if !observation.Get("is_error").IsEmpty() || !ok {
		return "", ""
	}
	suffix := ""
	if profile, ok := nonEmptyText(firstTruthy(observation.Get("profile_name"), observation.Get("model"))); ok {
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
// hook-blocked action, joined to its tool call when the events name it.
func settingRows(state jsonvalue.Value, actions map[string][2]string) []setting {
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
			text += " confirm_unknown=" + parser.SettingValue(policy.Get("confirm_unknown"))
		}
		rows = append(rows, setting{text: text + tail})
	}
	analyzer := orDefault(state.Get("security_analyzer").Get("kind"), "none")
	rows = append(rows, setting{text: "permission setting: security_analyzer=" + analyzer + tail})
	for _, blocked := range state.Get("blocked_actions").Members() {
		held := actions[blocked.Key]
		rows = append(rows, setting{"permission setting: blocked_actions." + blocked.Key + "=" +
			parser.SettingValue(blocked.Value) + tail, held[0], held[1]})
	}
	return rows
}
