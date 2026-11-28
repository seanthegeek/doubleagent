package pearai

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
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
// global state, or null, and the group of its id when the taskHistory
// repeats it.
func (p *Parser) HistoryItem(task clinelegacy.Task) (jsonvalue.Value, *timeline.Group) {
	database, found := task.InRootParent("state.vscdb")
	if !found {
		return jsonvalue.NewNull(), nil
	}
	if entry, found := p.globalState(task.Artifact, database).history[task.TaskID]; found {
		return entry, p.historyGroup(task.Artifact, database, task.TaskID)
	}
	return jsonvalue.NewNull(), nil
}

// askTimes returns the asks of the task's ui_messages.json.
func (p *Parser) askTimes(task clinelegacy.Task) *askIndex {
	var asks []jsonvalue.Value
	for _, message := range p.CachedJSON(task.InDir(clinelegacy.UIFile)).Items() {
		stamp := message.Get("ts")
		kind := parserkit.Text(parserkit.FirstNonEmpty(message.Get("ask"), message.Get("say")))
		if clinelegacy.IsNumber(stamp) && clinelegacy.IsToolUseKind(kind) {
			asks = append(asks, stamp)
		}
	}
	slices.SortStableFunc(asks, func(left, right jsonvalue.Value) int {
		return cmp.Compare(clinelegacy.OrderOf(left), clinelegacy.OrderOf(right))
	})
	return &askIndex{times: asks, work: p.Work}
}

// askIndex is the ts of each tool call ask of ui_messages.json, in time
// order, which a decision's tool_use_id names. work counts the asks a
// lookup compared, which a test reads.
type askIndex struct {
	times []jsonvalue.Value
	work  parserkit.Work
}

// lastBefore returns the latest ask time at or before a message's time
// (clinelegacy.TimeOf), as its number text, or "" when there is none or
// the time is not a number. The asks are in time order, so a binary search finds it.
func (x *askIndex) lastBefore(timestamp jsonvalue.Value) string {
	moment, ok := clinelegacy.TimeOf(timestamp)
	if !ok {
		return ""
	}
	after := sort.Search(len(x.times), func(i int) bool {
		x.work.Add("asks compared", 1)
		return clinelegacy.OrderOf(x.times[i]) > moment
	})
	if after == 0 {
		return ""
	}
	return x.times[after-1].Text()
}

// blockText returns a text block's text, or null for any other block.
func blockText(block jsonvalue.Value) jsonvalue.Value {
	if parserkit.IsText(block.Get("type"), "text") {
		return block.Get("text")
	}
	return jsonvalue.NewNull()
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

// apiPermissionRows emits a "permission decision:" row per result of the
// API history that opens with one of Roo's permission texts, named after
// the last tool call ask at or before its message.
func (p *Parser) apiPermissionRows(artifact *input.Artifact, task clinelegacy.Task, emit parserkit.Emit) {
	context := p.Context(artifact)
	asks := p.askTimes(task)
	var problems parserkit.Problems
	for position, message := range parserkit.JSONArray(artifact.DiskPath, &problems) {
		if !parserkit.IsText(message.Get("role"), "user") {
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
			ownTime := timeutil.FromValue(timestamp)
			row := p.Row(artifact, context, position, cmp.Or(ownTime, context.Start), "")
			context.Member(&row, ownTime != "", true)
			row.TurnType = timeline.TurnSystem
			row.ToolName = name
			row.ToolUseID = asks.lastBefore(timestamp)
			row.Text = "permission decision: " + decision
			emit(row)
		}
	}
	parserkit.EmitAll(emit, p.TaskErrorRows(artifact, problems, context))
}

// taskFile emits a task file's rows; beside ui_messages.json, which holds
// the transcript, the API history gives only the permission decisions it
// alone holds.
func (p *Parser) taskFile(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	if clinelegacy.IsAPIFile(clinelegacy.TaskFileName(artifact)) && clinelegacy.HasUI(artifact) {
		p.apiPermissionRows(artifact, clinelegacy.TaskOf(artifact), emit)
		return
	}
	p.ParseTaskFile(artifact, options, emit)
}

// stateRows emits the task and setting rows of the extension's state, and
// the problem of a taskHistory that is not an array. The task row of each
// entry of an id the taskHistory repeats is a parent of the id's group
// (historyGroup).
func (p *Parser) stateRows(artifact *input.Artifact, state jsonvalue.Value, emit parserkit.Emit) {
	var problems parserkit.Problems
	database, shared := parserkit.Beside(artifact, "state.vscdb")
	history := parserkit.ArrayMember(state.Get("taskHistory"), "taskHistory", 0, &problems)
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, problem.Message))
	}
	for i, item := range history {
		if !item.IsObject() {
			continue
		}
		row := p.HistoryRow(artifact, item, i+1)
		text := row.Text
		if number := item.Get("number"); parserkit.Present(number) {
			text += " number=" + number.Text()
		}
		if creator := item.Get("creatorModeConfig"); creator.IsObject() && creator.Len() > 0 {
			text += " | creator mode: newProjectType=" + parserkit.TextOr(creator.Get("newProjectType"), "-") +
				" newProjectPath=" + parserkit.TextOr(creator.Get("newProjectPath"), "-")
		}
		row.Text = timeline.Compact(text)
		if shared {
			if group := p.historyGroup(artifact, database, item.Get("id").Text()); group != nil {
				group.Parent(&row)
			}
		}
		emit(row)
	}
	mtime := parserkit.FileMtime(artifact)
	for _, key := range standingKeys() {
		if value := state.Get(key); !value.IsNull() {
			row := p.SystemRow(artifact, "permission setting: "+key+"="+parserkit.SettingValue(value)+
				" (from state.vscdb "+extensionID+", file mtime)")
			row.TimestampUTC = mtime
			emit(row)
		}
	}
}

// state emits a task row per taskHistory[] entry of the editor's global
// state, with its number and creator mode, and a permission setting row
// per standing key, at the file's modification time.
func (p *Parser) state(artifact *input.Artifact, emit parserkit.Emit) {
	read := p.globalState(artifact, artifact.DiskPath)
	parserkit.NoteRepeats(artifact, read.repeats)
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
