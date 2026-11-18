package pearai

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// standingKeys are the global state keys of Roo 3.15 that grant or limit
// what runs without asking.
func standingKeys() []string {
	return []string{"autoApprovalEnabled", "alwaysAllowReadOnly", "alwaysAllowReadOnlyOutsideWorkspace",
		"alwaysAllowWrite", "alwaysAllowWriteOutsideWorkspace", "alwaysAllowBrowser",
		"alwaysApproveResubmit", "alwaysAllowMcp", "alwaysAllowModeSwitch", "alwaysAllowSubtasks",
		"alwaysAllowExecute", "allowedCommands"}
}

// HistoryItem returns the task's taskHistory[] entry from the editor's
// global state, or null.
func (p *Parser) HistoryItem(task clinelegacy.Task) jsonvalue.Value {
	database, found := task.InRootParent("state.vscdb")
	if !found {
		return jsonvalue.NewNull()
	}
	if entry, found := p.globalState(database).history[task.TaskID]; found {
		return entry
	}
	return jsonvalue.NewNull()
}

// taskFile emits a task file's rows; beside ui_messages.json, which holds
// the transcript, the API history gives only the permission decisions it
// alone holds.
func (p *Parser) taskFile(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	task := clinelegacy.TaskOf(artifact)
	if clinelegacy.IsAPIFile(clinelegacy.TaskFileName(artifact)) &&
		parser.RegularFile(task.InDir(clinelegacy.UIFile)) {
		p.apiPermissionRows(artifact, task, emit)
		return
	}
	p.ParseTaskFile(artifact, options, emit)
}

// askIndex is the ts of each tool call ask of ui_messages.json, in time
// order, which a decision's tool_use_id names. probes counts the asks a
// lookup compared, a measure of the work that a test reads.
type askIndex struct {
	times  []jsonvalue.Value
	probes int
}

// askTimes returns the asks of the task's ui_messages.json.
func (p *Parser) askTimes(task clinelegacy.Task) *askIndex {
	var asks []jsonvalue.Value
	for _, message := range p.CachedJSON(task.InDir(clinelegacy.UIFile)).Items() {
		stamp := message.Get("ts")
		kind := parser.Text(parser.FirstNonEmpty(message.Get("ask"), message.Get("say")))
		if clinelegacy.IsNumber(stamp) && clinelegacy.IsToolUseKind(kind) {
			asks = append(asks, stamp)
		}
	}
	slices.SortStableFunc(asks, func(left, right jsonvalue.Value) int {
		return cmp.Compare(clinelegacy.OrderOf(left), clinelegacy.OrderOf(right))
	})
	return &askIndex{times: asks}
}

// apiPermissionRows emits a "permission decision:" row per result of the
// API history that opens with one of Roo's permission texts, named after
// the last tool call ask at or before its message.
func (p *Parser) apiPermissionRows(artifact *input.Artifact, task clinelegacy.Task, emit parser.Emit) {
	context := p.Context(artifact)
	asks := p.askTimes(task)
	var problems parser.Problems
	for position, message := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !parser.IsText(message.Get("role"), "user") {
			continue
		}
		timestamp := message.Get("ts")
		name := ""
		for _, block := range message.Get("content").Items() {
			text := blockText(block)
			if !text.IsString() {
				continue
			}
			if hit := resultPattern.FindStringSubmatch(strings.TrimSpace(text.StringValue())); hit != nil {
				name = hit[1]
				continue
			}
			decision := permissionDecision(text.StringValue())
			if decision == "" {
				continue
			}
			row := p.Row(artifact, context, position,
				timeutil.FirstNonEmpty(timeutil.FromValue(timestamp), context.Start), "")
			row.TurnType = timeline.TurnSystem
			row.ToolName = name
			row.ToolUseID = asks.lastBefore(timestamp)
			row.Text = "permission decision: " + decision
			emit(row)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.TaskID, context.ProjectPath))
}

// lastBefore returns the latest ask time at or before a message's time,
// as its number text, or "" when there is none or the time is not a
// number. The asks are in time order, so a binary search finds it.
func (x *askIndex) lastBefore(timestamp jsonvalue.Value) string {
	moment, ok := clinelegacy.NumberOf(timestamp)
	if !ok || !clinelegacy.IsNumber(timestamp) {
		return ""
	}
	after := sort.Search(len(x.times), func(i int) bool {
		x.probes++
		return clinelegacy.OrderOf(x.times[i]) > moment
	})
	if after == 0 {
		return ""
	}
	return x.times[after-1].Text()
}

// blockText returns a text block's text, or null for any other block.
func blockText(block jsonvalue.Value) jsonvalue.Value {
	if parser.IsText(block.Get("type"), "text") {
		return block.Get("text")
	}
	return jsonvalue.NewNull()
}

// state emits a task row per taskHistory[] entry of the editor's global
// state, with its number and creator mode, and a permission setting row
// per standing key, at the file's modification time.
func (p *Parser) state(artifact *input.Artifact, emit parser.Emit) {
	read := p.globalState(artifact.DiskPath)
	problems := read.problems
	switch {
	case read.value.IsObject():
		p.stateRows(artifact, read.value, emit)
	case !read.value.IsNull() && len(problems) == 0:
		problems = []string{extensionID + " global state is not a JSON object"}
	}
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, problem))
	}
}

// stateRows emits the task and setting rows of the extension's state.
func (p *Parser) stateRows(artifact *input.Artifact, state jsonvalue.Value, emit parser.Emit) {
	for i, item := range state.Get("taskHistory").Items() {
		if !item.IsObject() {
			continue
		}
		row := p.HistoryRow(artifact, item, i+1)
		text := row.Text
		if number := item.Get("number"); parser.Present(number) {
			text += " number=" + number.Text()
		}
		if creator := item.Get("creatorModeConfig"); creator.IsObject() && creator.Len() > 0 {
			text += " | creator mode: newProjectType=" + parser.TextOr(creator.Get("newProjectType"), "-") +
				" newProjectPath=" + parser.TextOr(creator.Get("newProjectPath"), "-")
		}
		row.Text = timeline.Compact(text)
		emit(row)
	}
	mtime := parser.FileMtime(artifact)
	for _, key := range standingKeys() {
		if value := state.Get(key); !value.IsNull() {
			row := p.SystemRow(artifact, "permission setting: "+key+"="+parser.SettingValue(value)+
				" (from state.vscdb "+extensionID+", file mtime)")
			row.TimestampUTC = mtime
			emit(row)
		}
	}
}

// permissionTexts are the texts Roo 3.15 opens a tool result with when the
// user decided on the call, each with the decision it records.
func permissionTexts() [][2]string {
	return [][2]string{
		{"The user denied this operation and provided the following feedback:", "denied with feedback"},
		{"The user approved this operation and provided the following context:", "approved with feedback"},
		{"The user denied this operation.", "denied"},
	}
}

// permissionDecision returns the decision a Roo 3.15 tool result block
// records, or "".
func permissionDecision(text string) string {
	text = strings.TrimLeftFunc(text, unicode.IsSpace)
	for _, pair := range permissionTexts() {
		if strings.HasPrefix(text, pair[0]) {
			return pair[1]
		}
	}
	return ""
}
