// Package musecode reads Muse Code's session logs, prompt history and
// trusted projects.
//
// It reads, relative to the home (the same layout on macOS and Windows):
//
//   - .local/share/muse/sessions/YYYY/MM/DD/<session id>/session.jsonl and
//     its subagent/<child id>/session.jsonl logs: one JSON object per line,
//     an envelope (stream.id, recorded_at in microseconds, payload_type,
//     payload), a retained_frame (children[].child_index and
//     children[].record_json, a whole envelope as a JSON string) or a
//     skipped retained_marker. A file whose first record is neither is not
//     Muse.
//   - runtime.session records: payload.kind "run" (run_id and an event) or
//     "approval"; and fact records, read from payload.record then payload:
//     the session metadata (workspace_root, model_id), route facts (cwd),
//     the permission declarations, the opened session, the workspace branch,
//     the session name, model configuration and reconfiguration, the accepted
//     user intent, user shell commands and results, and subagent control.
//     Everything else is skipped.
//   - .local/share/muse/tui-history.jsonl (secondary): a JSON string, then
//     {project, session}.
//   - .config/muse/trust.json: projects.<path>.decision.
//
// A subagent log records no workspace: its project and branch come from the
// parent log, read up to parentScanLines lines. Model and permission mode
// rows are tracked per log. See research/muse-code.md.
package musecode

import (
	"cmp"
	"errors"
	"iter"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The files outside the session logs, and how far into a parent log the
// search for its workspace reads.
const (
	historyRel      = ".local/share/muse/tui-history.jsonl"
	trustRel        = ".config/muse/trust.json"
	parentScanLines = 2000
)

// argumentKeys are the arguments of a tool call that stand for the call, in
// the order they are tried.
var argumentKeys = []string{"command", "cmd", "url", "query", "path", "file_path",
	"target_path", "target", "filename"}

// resultKeys are the fields that hold a tool result's text, in the order
// they are tried.
var resultKeys = []string{"text", "output", "result", "tool_result_model_visible_content",
	"content"}

// resultEvents are the run events that carry tool results.
var resultEvents = []string{"tool_result_batch_committed", "tool_results_committed",
	"tool_result_committed", "tool_result"}

// loggedEvents are the run events written as a row of their compact JSON.
var loggedEvents = []string{"context_compaction_installed", "run_retracted", "turn_task_died"}

// subagentControl are the payload types of subagent control records.
var subagentControl = []string{"subagent.control.spawn_accepted",
	"subagent.control.start_attested"}

// Parser reads Muse Code's files. It keeps the workspaces of the parent logs
// it has read, by the log's path on disk.
type Parser struct {
	parser.Base
	session     *regexp.Regexp
	parentCache map[string][2]string
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base: parser.NewBase("muse-code", "muse-code"),
		session: regexp.MustCompile(`\A\.local/share/muse/sessions/[0-9]{4}/[0-9]{2}/[0-9]{2}/` +
			`[^/]+/(?:subagent/[^/]+/)*session\.jsonl\z`),
		parentCache: map[string][2]string{},
	}
}

// Wants reports whether the artifact is a session log, the prompt history or
// trust.json.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return artifact.Rel == historyRel || artifact.Rel == trustRel ||
		p.session.MatchString(artifact.Rel)
}

// Secondary reports whether the artifact is the prompt history, which
// repeats the sessions' prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return artifact.Rel == historyRel
}

// Parse emits the rows of one Muse Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch artifact.Rel {
	case historyRel:
		p.parseHistory(artifact, emit)
	case trustRel:
		p.parseTrust(artifact, emit)
	default:
		p.parseSession(artifact, options, emit)
	}
	return nil
}

// missing reports whether value is null or the empty string.
func missing(value jsonvalue.Value) bool {
	return value.IsNull() || (value.IsString() && value.StringValue() == "")
}

// exitSucceeded reports whether an exit code is no failure: null, false or
// a number equal to zero; a number beyond the float64 range is not zero.
func exitSucceeded(value jsonvalue.Value) bool {
	switch {
	case value.IsNull():
		return true
	case value.IsBool():
		flag, _ := value.Bool()
		return !flag
	case value.IsNumber():
		number, ok := value.Float()
		return ok && number == 0
	}
	return false
}

// fact returns a fact record's fields: payload.record, falling back to the
// payload.
func fact(payload jsonvalue.Value) jsonvalue.Value {
	if record := payload.Get("record"); record.IsObject() {
		return record
	}
	return payload
}

// microsToUTC returns recorded_at, in microseconds, as UTC; a plain epoch
// would read numbers of this size as milliseconds. A number beyond the
// float64 range is no time.
func microsToUTC(value jsonvalue.Value) string {
	switch {
	case value.IsNumber():
		number, ok := value.Float()
		if !ok {
			return ""
		}
		return timeutil.FromEpoch(number / 1_000_000)
	case value.IsString() && isASCIIDigits(value.StringValue()):
		number, err := strconv.ParseFloat(value.StringValue(), 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return ""
		}
		return timeutil.FromEpoch(number / 1_000_000)
	}
	return ""
}

// isASCIIDigits reports whether text is one or more ASCII digits.
func isASCIIDigits(text string) bool {
	return text != "" && strings.Trim(text, "0123456789") == ""
}

// argumentsSummary returns the first argumentKeys argument of a tool call,
// else its arguments as compact JSON; arguments given as text are decoded
// first, and text that is not JSON stands for itself.
func argumentsSummary(arguments jsonvalue.Value) string {
	if arguments.IsString() {
		decoded, err := jsonvalue.DecodeString(arguments.StringValue())
		if err != nil {
			return arguments.StringValue()
		}
		arguments = decoded
	}
	if !arguments.IsObject() {
		return arguments.Text()
	}
	for _, key := range argumentKeys {
		if value := arguments.Get(key); !missing(value) {
			return value.Text()
		}
	}
	return parser.JSONText(arguments)
}

// resultText returns a tool result's text; a bash result is a JSON object
// with output and exit_code.
func resultText(result jsonvalue.Value) string {
	if !result.IsString() {
		return result.Text()
	}
	raw := result.StringValue()
	if !strings.HasPrefix(raw, "{") {
		return raw
	}
	decoded, err := jsonvalue.DecodeString(raw)
	if err != nil || !decoded.IsObject() || !decoded.Has("output") {
		return raw
	}
	output := decoded.Get("output").Text()
	if code := decoded.Get("exit_code"); !exitSucceeded(code) {
		output = "[exit " + code.Text() + "] " + output
	}
	return output
}

// envelope is one record of a session log, with the number of its line.
type envelope struct {
	line   int
	record jsonvalue.Value
}

// notMuse is the problem of a session log whose first record is not Muse.
const notMuse = "not a Muse session log (the first record is neither an envelope nor a retained frame)"

// records yields every envelope of the log at path with its line number,
// the children of each frame unpacked in child order and the markers
// dropped. It stops at once when the first record is not Muse, which adds
// that problem.
func records(path string, problems *parser.Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		first := true
		for number, record := range parser.JSONLines(path, problems) {
			if first {
				first = false
				if !record.Has("retained_frame") && !(record.Has("stream") && record.Has("payload_type")) {
					problems.Add(number, notMuse)
					return
				}
			}
			switch {
			case record.Has("retained_marker"):
			case record.Has("retained_frame"):
				for _, inner := range frameChildren(number, record, problems) {
					if !yield(number, inner) {
						return
					}
				}
			default:
				if !yield(number, record) {
					return
				}
			}
		}
	}
}

// frameChildren returns the envelopes held in a retained frame's children,
// in child_index order. A child that does not decode adds a problem.
func frameChildren(number int, frame jsonvalue.Value, problems *parser.Problems) []jsonvalue.Value {
	var children []jsonvalue.Value
	for _, item := range parser.Elements(frame.Get("children")) {
		if item.IsObject() {
			children = append(children, item)
		}
	}
	slices.SortStableFunc(children, func(left, right jsonvalue.Value) int {
		return compareIndex(left.Get("child_index"), right.Get("child_index"))
	})
	var inners []jsonvalue.Value
	for _, child := range children {
		encoded := child.Get("record_json")
		inner, err := decodeChild(encoded)
		if err != nil {
			raw := ""
			if encoded.IsString() {
				raw = encoded.StringValue()
			}
			problems.AddRaw(number, "unparseable line (retained_frame child: "+err.Error()+")", raw)
		} else if inner.IsObject() {
			inners = append(inners, inner)
		}
	}
	return inners
}

// errChildNotText is the reason a frame child's record_json is not text.
var errChildNotText = errors.New("the JSON record is not text")

// decodeChild decodes a frame child's record_json; a missing or empty one is
// an error, as is one that is not text.
func decodeChild(encoded jsonvalue.Value) (jsonvalue.Value, error) {
	switch {
	case encoded.IsEmpty():
		return jsonvalue.DecodeString("")
	case !encoded.IsString():
		return jsonvalue.NewNull(), errChildNotText
	}
	return jsonvalue.DecodeString(encoded.StringValue())
}

// compareIndex orders two child_index values: a missing or false one is 0,
// numbers compare as numbers and texts as texts.
func compareIndex(left, right jsonvalue.Value) int {
	if left.IsString() && right.IsString() && !left.IsEmpty() && !right.IsEmpty() {
		return strings.Compare(left.StringValue(), right.StringValue())
	}
	leftNumber, rightNumber := indexNumber(left), indexNumber(right)
	switch {
	case leftNumber < rightNumber:
		return -1
	case leftNumber > rightNumber:
		return 1
	}
	return 0
}

// indexNumber returns a child_index as a number for ordering: true is 1,
// and the rest as parser.NumberOrder orders them.
func indexNumber(value jsonvalue.Value) float64 {
	if flag, ok := value.Bool(); ok && flag {
		return 1
	}
	return parser.NumberOrder(value)
}

// workspace returns the project path and branch from a session log's
// records: the last metadata workspace_root (else the route facts' cwd) and
// the last branch name. limit caps the lines read, 0 reads them all.
func workspace(log iter.Seq2[int, jsonvalue.Value], limit int) (string, string) {
	var project, cwd, branch string
	for number, record := range log {
		if limit > 0 && number > limit {
			break
		}
		payload := record.Get("payload")
		facts := fact(payload)
		switch payloadType(record) {
		case "runtime.session.metadata":
			if root := parser.FirstNonEmpty(facts.Get("workspace_root"), payload.Get("workspace_root")); !root.IsEmpty() {
				project = root.Text()
			}
		case "runtime.session.route_facts":
			if !facts.Get("cwd").IsEmpty() {
				cwd = facts.Get("cwd").Text()
			}
		case "session.workspace_branch.observed":
			if reference := facts.Get("reference"); isString(reference.Get("kind"), "branch") &&
				!reference.Get("name").IsEmpty() {
				branch = reference.Get("name").Text()
			}
		}
	}
	if project == "" {
		project = cwd
	}
	return project, branch
}

// payloadType returns an envelope's payload_type when it is text, else "".
func payloadType(record jsonvalue.Value) string {
	if kind := record.Get("payload_type"); kind.IsString() {
		return kind.StringValue()
	}
	return ""
}

// isString reports whether value is the string want.
func isString(value jsonvalue.Value, want string) bool {
	return value.IsString() && value.StringValue() == want
}

// parentWorkspace returns the project and branch of the nearest ancestor log
// that has them. The walk goes up through subagent/<id>/ levels; symbolic
// links are not read.
func (p *Parser) parentWorkspace(artifact *input.Artifact) (string, string) {
	for levels := 0; ; levels += 2 {
		if above, ok := parser.Above(artifact, levels+1); !ok || filepath.Base(above) != "subagent" {
			break
		}
		log, ok := parser.Above(artifact, levels+2, "session.jsonl")
		if !ok {
			break
		}
		if !parser.RegularFile(log) {
			continue
		}
		found, cached := p.parentCache[log]
		if !cached {
			var ignored parser.Problems
			project, branch := workspace(records(log, &ignored), parentScanLines)
			found = [2]string{project, branch}
			p.parentCache[log] = found
		}
		if found[0] != "" {
			return found[0], found[1]
		}
	}
	return "", ""
}

// walk is the state of one pass over a session log.
type walk struct {
	parser     *Parser
	artifact   *input.Artifact
	options    parser.Options
	emit       parser.Emit
	entries    []envelope
	isSubagent bool

	// What the pre-pass finds, for rows that need it before the record that
	// sets it.
	runModel, runCompletedModel map[string]string
	startedRuns, committed      map[string]bool
	lastDelta                   map[string]int
	sessionModel                string

	// What the pass itself tracks.
	sessionID, project, branch string
	deltas                     map[string][]string
	calls                      map[string]string
	approvals                  map[string][2]string
	lastRequest                [2]string
	shellIDs                   []string
	models, modes              parser.Tracker
	declaredFormat             string
}

// recordStep is one record of the pass: what its rows are built from.
type recordStep struct {
	index                      int
	line                       int
	record, payload, facts     jsonvalue.Value
	runID, model               string
	sessionID, project, branch string
	payloadType                string
}

// makeRow returns a row of the record, as built from the values the pass
// held when the record began.
func (w *walk) makeRow(step *recordStep, turn, text, toolName, toolUseID string) timeline.Row {
	row := w.parser.BaseRow(w.artifact)
	row.SourceLine = step.line
	row.TimestampUTC = microsToUTC(step.record.Get("recorded_at"))
	row.SessionID = step.sessionID
	row.ProjectPath = step.project
	row.GitBranch = step.branch
	row.Model = step.model
	row.TurnType = turn
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	return row
}

// seqOf returns the entries as a sequence of (line, record).
func seqOf(entries []envelope) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		for _, entry := range entries {
			if !yield(entry.line, entry.record) {
				return
			}
		}
	}
}

// parseSession emits the rows of one session log.
func (p *Parser) parseSession(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	var entries []envelope
	for number, record := range records(artifact.DiskPath, &problems) {
		entries = append(entries, envelope{number, record})
	}
	if len(entries) == 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	pass := &walk{parser: p, artifact: artifact, options: options, emit: emit, entries: entries,
		isSubagent: strings.Contains(artifact.Rel, "/subagent/"),
		runModel:   map[string]string{}, runCompletedModel: map[string]string{},
		startedRuns: map[string]bool{}, committed: map[string]bool{},
		lastDelta: map[string]int{}, deltas: map[string][]string{}, calls: map[string]string{},
		approvals: map[string][2]string{}, models: parser.Tracker{}, modes: parser.Tracker{}}
	pass.prepass()
	pass.project, pass.branch = workspace(seqOf(entries), 0)
	if pass.isSubagent && pass.project == "" {
		pass.project, pass.branch = p.parentWorkspace(artifact)
	}
	for i, entry := range entries {
		pass.record(i, entry)
	}
	for _, row := range p.ErrorRows(artifact, problems, pass.sessionID, pass.project) {
		row.GitBranch = pass.branch
		emit(row)
	}
}

// prepass collects the values a row may need before the record that sets
// them: the model of each run, the runs started, the reasoning summaries
// committed and the last delta of each.
func (w *walk) prepass() {
	for i, entry := range w.entries {
		payload := entry.record.Get("payload")
		facts := fact(payload)
		switch kind := payloadType(entry.record); {
		case kind == "run.model.configured":
			runKey := facts.Get("run_stream").Get("id")
			if !runKey.IsEmpty() && !facts.Get("model_id").IsEmpty() {
				w.runModel[runKey.Text()] = facts.Get("model_id").Text()
			}
		case kind == "runtime.session.metadata" && !facts.Get("model_id").IsEmpty() &&
			w.sessionModel == "":
			w.sessionModel = facts.Get("model_id").Text()
		case kind == "runtime.session" && isString(payload.Get("kind"), "run"):
			w.noteRunEvent(i, parser.Text(payload.Get("run_id")), payload.Get("event"))
		}
	}
}

// noteRunEvent records what the pre-pass takes from one run event.
func (w *walk) noteRunEvent(index int, runKey string, event jsonvalue.Value) {
	messageID := parser.Text(event.Get("message_id"))
	switch {
	case isString(event.Get("kind"), "started"):
		w.startedRuns[runKey] = true
	case isString(event.Get("kind"), "model_completed") && !event.Get("model").IsEmpty():
		if _, set := w.runCompletedModel[runKey]; !set {
			w.runCompletedModel[runKey] = event.Get("model").Text()
		}
	case isString(event.Get("kind"), "reasoning_summary_committed"):
		w.committed[messageID] = true
	case isString(event.Get("kind"), "reasoning_summary_delta"):
		w.lastDelta[messageID] = index
	}
}

// record emits the rows of one record of the log.
func (w *walk) record(index int, entry envelope) {
	payload := entry.record.Get("payload")
	step := &recordStep{index: index, line: entry.line, record: entry.record, payload: payload,
		facts: fact(payload), payloadType: payloadType(entry.record)}
	if streamID := entry.record.Get("stream").Get("id"); !streamID.IsEmpty() && w.sessionID == "" {
		w.sessionID = streamID.Text()
	}
	step.runID = parser.Text(payload.Get("run_id"))
	step.model = cmp.Or(w.runModel[step.runID], w.runCompletedModel[step.runID], w.sessionModel)
	step.sessionID, step.project, step.branch = w.sessionID, w.project, w.branch
	switch {
	case step.payloadType == "runtime.session.metadata":
		w.metadata(step)
	case step.payloadType == "run.model.configured" && !step.facts.Get("model_id").IsEmpty():
		w.modelRow(step, step.facts.Get("model_id").Text(), step.facts)
	case step.payloadType == "runtime.session.permission_format_declared":
		w.declaredFormat = step.facts.Get("format").Text()
	case step.payloadType == "runtime.session.permission_profile_committed":
		w.permissionProfile(step)
	case step.payloadType == "session.opened.observed" && !missing(step.facts.Get("security_mode")):
		w.securityMode(step)
	case step.payloadType == "session.workspace_branch.observed":
		w.branch = ""
		if reference := step.facts.Get("reference"); isString(reference.Get("kind"), "branch") {
			w.branch = parser.Text(reference.Get("name"))
		}
	case step.payloadType == "session.name.changed":
		w.emit(w.makeRow(step, timeline.TurnSystem, "session name: "+step.facts.Get("new_name").Text(), "", ""))
	case step.payloadType == "runtime.model_reconfigure.completed":
		w.reconfigure(step)
	case step.payloadType == "runtime.user_intent.accepted":
		w.userIntent(step)
	case step.payloadType == "user_shell.command" || step.payloadType == "user_shell.result":
		w.userShell(step)
	case slices.Contains(subagentControl, step.payloadType):
		w.emit(w.makeRow(step, timeline.TurnSystem, step.payloadType+": "+parser.JSONText(step.facts), "", ""))
	case step.payloadType == "runtime.session":
		w.runtimeSession(step)
	}
}

// metadata handles the session metadata record: the model, then the
// workspace root.
func (w *walk) metadata(step *recordStep) {
	if model := step.facts.Get("model_id"); !model.IsEmpty() {
		w.sessionModel = model.Text()
		w.modelRow(step, w.sessionModel, step.facts)
	}
	if root := parser.FirstNonEmpty(step.facts.Get("workspace_root"), step.payload.Get("workspace_root")); !root.IsEmpty() {
		w.project = root.Text()
	}
}

// modelRow emits "model: <id>" at the log's first model and "model: <old> ->
// <new>" on a change, with " provider=<id>" when the record names one.
func (w *walk) modelRow(step *recordStep, model string, facts jsonvalue.Value) {
	line := w.models.Change("model", w.sessionID, model)
	if line == "" {
		return
	}
	if provider := facts.Get("provider_id"); !missing(provider) {
		line += " provider=" + parser.SettingOrDash(provider)
	}
	row := w.makeRow(step, timeline.TurnSystem, line, "", "")
	row.Model = model
	w.emit(row)
}

// permissionProfile emits the permission mode row of a committed profile
// when the mode changed.
func (w *walk) permissionProfile(step *recordStep) {
	snapshot := step.facts.Get("resolved_snapshot")
	value := "approval=" + parser.SettingOrDash(snapshot.Get("approval")) + " reviewer=" +
		parser.SettingOrDash(snapshot.Get("reviewer")) + " profile=" + parser.SettingOrDash(step.facts.Get("source").Get("id"))
	line := w.modes.Change("permission mode", w.sessionID, value)
	if line == "" {
		return
	}
	if cause := step.facts.Get("cause"); !missing(cause) {
		line += " cause=" + parser.SettingOrDash(cause)
	}
	if w.declaredFormat != "" {
		line += " format=" + w.declaredFormat
	}
	w.emit(w.makeRow(step, timeline.TurnSystem, line, "", ""))
}

// securityMode emits the permission mode row of an opened session when its
// security mode changed.
func (w *walk) securityMode(step *recordStep) {
	line := w.modes.Change("permission mode", w.sessionID+"\x00security_mode",
		"security_mode="+parser.SettingOrDash(step.facts.Get("security_mode")))
	if line != "" {
		w.emit(w.makeRow(step, timeline.TurnSystem, line, "", ""))
	}
}

// reconfigure emits the rows of a model reconfiguration: the previous model
// when the log has none yet, and the change, which a /model the user typed
// keeps even when the model did not change.
func (w *walk) reconfigure(step *recordStep) {
	effective := step.facts.Get("effective").Get("model_id").Text()
	previous := step.facts.Get("previous").Get("model_id").Text()
	if effective == "" {
		w.emit(w.makeRow(step, timeline.TurnSystem,
			step.payloadType+": "+parser.JSONText(step.facts), "", ""))
		return
	}
	w.sessionModel = effective
	if previous != "" && !w.hasModel() {
		row := w.makeRow(step, timeline.TurnSystem,
			w.models.Change("model", w.sessionID, previous), "", "")
		row.Model = previous
		w.emit(row)
	}
	line := w.models.Change("model", w.sessionID, effective)
	if line == "" {
		line = "model: " + effective + " -> " + effective
	}
	if outcome := step.facts.Get("apply_outcome"); !missing(outcome) {
		line += " apply_outcome=" + parser.SettingOrDash(outcome)
	}
	row := w.makeRow(step, timeline.TurnSystem, line, "", "")
	row.Model = effective
	w.emit(row)
}

// hasModel reports whether the log's model tracking holds the session.
func (w *walk) hasModel() bool {
	_, held := w.models[w.sessionID]
	return held
}

// userIntent emits the prompt of an accepted user intent that no run
// started.
func (w *walk) userIntent(step *recordStep) {
	intent := parser.Text(step.payload.Get("intent_id"))
	if intent == "" || w.startedRuns[intent] {
		return
	}
	var parts []string
	for _, message := range parser.Elements(step.payload.Get("model_messages")) {
		for _, block := range parser.Elements(message.Get("content")) {
			if body := block.Get("text"); body.IsString() {
				parts = append(parts, body.StringValue())
			}
		}
	}
	if len(parts) == 0 {
		return
	}
	turn := timeline.TurnUser
	if w.isSubagent {
		turn = timeline.TurnSystem
	}
	row := w.makeRow(step, turn, strings.Join(parts, "\n"), "", "")
	row.Model = cmp.Or(w.runModel[intent], step.model)
	w.emit(row)
}

// userShell emits the call or the result of a command the user ran in the
// shell.
func (w *walk) userShell(step *recordStep) {
	commandID := step.facts.Get("command_id").Text()
	if step.payloadType == "user_shell.command" {
		w.shellIDs = append(w.shellIDs, commandID)
		w.emit(w.makeRow(step, timeline.TurnToolUse, step.facts.Get("command_text").Text(),
			"user_shell", commandID))
		return
	}
	if commandID == "" && len(w.shellIDs) > 0 {
		commandID = w.shellIDs[len(w.shellIDs)-1]
	}
	output := step.facts.Get("visible_output").Text()
	var notes []string
	if code := step.facts.Get("exit_code"); !exitSucceeded(code) {
		notes = append(notes, "exit "+code.Text())
	}
	if state := step.facts.Get("terminal_state"); !state.IsEmpty() && !isString(state, "completed") {
		notes = append(notes, state.Text())
	}
	if len(notes) > 0 {
		output = "[" + strings.Join(notes, ", ") + "] " + output
	}
	w.emit(w.makeRow(step, timeline.TurnToolResult, output, "user_shell", commandID))
}

// runtimeSession emits the rows of a runtime.session record: a run event or
// an approval.
func (w *walk) runtimeSession(step *recordStep) {
	event := step.payload.Get("event")
	switch {
	case isString(step.payload.Get("kind"), "run"):
		if isString(event.Get("kind"), "model_completed") && !event.Get("model").IsEmpty() {
			if _, configured := w.runModel[step.runID]; !configured {
				w.modelRow(step, event.Get("model").Text(), jsonvalue.NewObject())
			}
		}
		w.runEvent(step, event)
	case isString(step.payload.Get("kind"), "approval"):
		w.approval(step, event)
	}
}

// approval emits the rows of an approval request, its automated review and
// the decision applied.
func (w *walk) approval(step *recordStep, event jsonvalue.Value) {
	switch kind := parser.Text(event.Get("kind")); kind {
	case "requested":
		toolName, toolID := event.Get("tool_name").Text(), event.Get("tool_call_id").Text()
		w.approvals[event.Get("pending_action_id").Text()] = [2]string{toolName, toolID}
		w.lastRequest = [2]string{toolName, toolID}
		w.emit(w.makeRow(step, timeline.TurnSystem,
			"approval requested: "+event.Get("raw_args").Text(), toolName, toolID))
	case "automated_review_completed":
		assessment := event.Get("assessment")
		line := "permission decision: " + parser.SettingOrDash(assessment.Get("outcome")) +
			" kind=automated_review_completed risk_level=" + parser.SettingOrDash(assessment.Get("risk_level")) +
			" user_authorization=" + parser.SettingOrDash(assessment.Get("user_authorization")) + " status=" +
			parser.SettingOrDash(event.Get("status"))
		if rationale := assessment.Get("rationale"); !missing(rationale) {
			line += " " + rationale.Text()
		}
		w.emit(w.makeRow(step, timeline.TurnSystem, line, w.lastRequest[0], w.lastRequest[1]))
	case "decision_applied":
		held := w.approvals[event.Get("pending_action_id").Text()]
		w.emit(w.makeRow(step, timeline.TurnSystem, "permission decision: "+parser.SettingOrDash(event.Get("decision"))+
			" source="+parser.SettingOrDash(event.Get("decision_source").Get("kind"))+" policy_result="+
			parser.SettingOrDash(event.Get("policy_result"))+" amendment="+parser.SettingOrDash(event.Get("amendment")),
			held[0], held[1]))
	}
}

// runEvent emits the rows of one run event of a session log: "started" is
// the prompt (a "subagent task:" row in a subagent log), "inbox_item_queued"
// a steer or an inbox row, committed messages, reasoning and tool calls and
// results their rows, reasoning deltas only when no committed summary
// exists, "terminal" a row unless "completed", and the logged events rows of
// compact JSON.
func (w *walk) runEvent(step *recordStep, event jsonvalue.Value) {
	kind := ""
	if event.Get("kind").IsString() {
		kind = event.Get("kind").StringValue()
	}
	switch {
	case kind == "started":
		prompt := event.Get("prompt").Text()
		if w.isSubagent {
			w.emit(w.makeRow(step, timeline.TurnSystem, "subagent task: "+prompt, "", ""))
		} else {
			w.emit(w.makeRow(step, timeline.TurnUser, prompt, "", ""))
		}
	case kind == "inbox_item_queued":
		w.inbox(step, event)
	case kind == "assistant_message_committed":
		if body := event.Get("text").Text(); body != "" {
			w.emit(w.makeRow(step, timeline.TurnAssistant, body, "", ""))
		}
	case kind == "reasoning_summary_committed" || kind == "reasoning_committed":
		// reasoning_committed text is usually empty: its content is the opaque
		// encrypted_content.
		if w.options.IncludeThinking && !event.Get("text").IsEmpty() {
			w.emit(w.makeRow(step, timeline.TurnThinking, event.Get("text").Text(), "", ""))
		}
	case kind == "reasoning_summary_delta":
		w.reasoningDelta(step, event)
	case kind == "assistant_tool_calls_committed":
		w.toolCalls(step, event)
	case slices.Contains(resultEvents, kind):
		w.toolResults(step, event)
	case kind == "provider_tool_call_completed":
		w.providerTool(step, event)
	case kind == "terminal":
		if term := event.Get("terminal").Text(); term != "" && term != "completed" {
			w.emit(w.makeRow(step, timeline.TurnSystem,
				"run "+term+": "+event.Get("reason").Text(), "", ""))
		}
	case slices.Contains(loggedEvents, kind):
		w.emit(w.makeRow(step, timeline.TurnSystem, kind+": "+parser.JSONText(event), "", ""))
	}
}

// inbox emits the row of a queued inbox item: the user's steer, or a system
// row naming the source.
func (w *walk) inbox(step *recordStep, event jsonvalue.Value) {
	source := event.Get("source").Get("source").Text()
	body := event.Get("payload").Get("prompt").Text()
	if body == "" {
		body = event.Get("body").Text()
	}
	if source == "user_steer" {
		w.emit(w.makeRow(step, timeline.TurnUser, body, "", ""))
		return
	}
	if body == "" {
		body = event.Get("summary").Text()
	}
	w.emit(w.makeRow(step, timeline.TurnSystem, "inbox "+source+": "+body, "", ""))
}

// reasoningDelta gathers the deltas of a reasoning summary that no commit
// replaces, and emits them as one thinking row at the last delta.
func (w *walk) reasoningDelta(step *recordStep, event jsonvalue.Value) {
	messageID := event.Get("message_id").Text()
	if w.committed[messageID] {
		return
	}
	w.deltas[messageID] = append(w.deltas[messageID], event.Get("text").Text())
	if last, ok := w.lastDelta[messageID]; ok && last == step.index && w.options.IncludeThinking {
		var parts []string
		for _, delta := range w.deltas[messageID] {
			if delta != "" {
				parts = append(parts, delta)
			}
		}
		delete(w.deltas, messageID)
		w.emit(w.makeRow(step, timeline.TurnThinking, strings.Join(parts, "\n\n"), "", ""))
	}
}

// toolCalls emits one tool_use row per call of a committed list.
func (w *walk) toolCalls(step *recordStep, event jsonvalue.Value) {
	for _, call := range parser.Elements(event.Get("tool_calls")) {
		name, callID := call.Get("name").Text(), call.Get("call_id").Text()
		w.calls[callID] = name
		w.emit(w.makeRow(step, timeline.TurnToolUse, argumentsSummary(call.Get("args")), name, callID))
	}
}

// toolResults emits one tool_result row per result of a results event, or
// of the event itself when it holds no list.
func (w *walk) toolResults(step *recordStep, event jsonvalue.Value) {
	items := []jsonvalue.Value{event}
	if results := event.Get("results"); results.IsArray() {
		items = results.Items()
	}
	for _, result := range items {
		callID := cmp.Or(parser.Text(result.Get("tool_call_id")), result.Get("call_id").Text())
		raw := jsonvalue.NewString("")
		for _, key := range resultKeys {
			if value := result.Get(key); !value.IsNull() {
				raw = value
				break
			}
		}
		w.emit(w.makeRow(step, timeline.TurnToolResult, resultText(raw), w.calls[callID], callID))
	}
}

// providerTool emits the call and the result of a tool the provider ran.
func (w *walk) providerTool(step *recordStep, event jsonvalue.Value) {
	callID := event.Get("call_id").Text()
	action := event.Get("action")
	name := cmp.Or(action.Get("type").Text(), "provider_tool")
	w.calls[callID] = name
	summary := ""
	if action.IsObject() && action.Len() > 0 {
		summary = argumentsSummary(action)
	}
	w.emit(w.makeRow(step, timeline.TurnToolUse, summary, name, callID))
	body := ""
	if results := event.Get("results"); !results.IsNull() {
		body = parser.JSONText(results)
	}
	if status := event.Get("status").Text(); status != "" && status != "completed" {
		body = "[" + status + "] " + body
	}
	w.emit(w.makeRow(step, timeline.TurnToolResult, body, name, callID))
}

// parseTrust emits one "permission setting:" row per trusted project's
// decision.
func (p *Parser) parseTrust(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp := parser.FileMtime(artifact)
	row := p.SystemRow(artifact, "")
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	document, err := jsonvalue.DecodeString(textlines.ValidUTF8(content))
	if err != nil || !document.IsObject() {
		row.Text = "parser: trust.json is not a JSON object"
		emit(row)
		return
	}
	for _, member := range document.Get("projects").Members() {
		if !member.Value.IsObject() || !member.Value.Has("decision") {
			continue
		}
		row.ProjectPath = member.Key
		row.Text = timeline.Compact("permission setting: projects." + member.Key + ".decision=" +
			parser.SettingValue(member.Value.Get("decision")) + " (from " + artifact.Rel +
			", file mtime)")
		emit(row)
	}
}

// parseHistory emits one user row per prompt of the TUI history, with its
// session and project: each prompt is a JSON string, followed by an object
// naming them when the history has one.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	type prompt struct {
		line int
		text string
	}
	var pending *prompt
	send := func(held *prompt, meta jsonvalue.Value) {
		row := p.BaseRow(artifact)
		row.SourceLine = held.line
		row.TurnType = timeline.TurnUser
		row.SessionID = meta.Get("session").Text()
		row.ProjectPath = meta.Get("project").Text()
		row.Text = timeline.Compact(held.text)
		emit(row)
	}
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		stripped := strings.Trim(string(raw), textlines.ASCIIWhitespace)
		if stripped == "" {
			continue
		}
		record, err := jsonvalue.Decode([]byte(stripped))
		if err != nil {
			problems.AddRaw(number, parser.UnparseableLine(err), parser.LineText(raw))
			continue
		}
		switch {
		case record.IsString():
			if pending != nil {
				send(pending, jsonvalue.NewNull())
			}
			pending = &prompt{number, record.StringValue()}
		case record.IsObject() && pending != nil:
			send(pending, record)
			pending = nil
		case !record.IsObject():
			problems.AddRaw(number, "unparseable line (not a JSON string or object)", parser.LineText(raw))
		}
	}
	if pending != nil {
		send(pending, jsonvalue.NewNull())
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
