package roocode

import (
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// approvedText is the decision of a tool ask marked isAnswered: approved by
// a click or automatically.
const approvedText = "permission decision: approved isAnswered=true (user or auto-approval)"

// systemLike returns a system row with text and every other field of like.
func systemLike(like timeline.Row, text string) timeline.Row {
	like.TurnType = timeline.TurnSystem
	like.Text = timeline.Compact(text)
	return like
}

// taskFile emits a task file's rows with the permission decisions beside
// them.
func (p *Parser) taskFile(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	name := clinelegacy.TaskFileName(artifact)
	switch {
	case name == clinelegacy.UIFile:
		p.uiWithDecisions(artifact, options, emit)
	case clinelegacy.IsAPIFile(name):
		p.apiWithDecisions(artifact, options, emit)
	default:
		p.ParseTaskFile(artifact, options, emit)
	}
}

// uiWithDecisions emits the ui_messages.json rows with each approval after
// its tool call. The pre-scan's problems are dropped: the reader reports
// them.
func (p *Parser) uiWithDecisions(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	answered := map[int]bool{}
	var ignored parser.Problems
	for position, message := range parser.JSONArray(artifact.DiskPath, &ignored) {
		if parser.IsText(message.Get("type"), "ask") && parser.IsText(message.Get("ask"), "tool") {
			if flag, ok := message.Get("isAnswered").Bool(); ok && flag {
				answered[position] = true
			}
		}
	}
	p.ParseUI(artifact, p.Context(artifact), options, func(row timeline.Row) {
		emit(row)
		if row.TurnType == timeline.TurnToolUse && answered[row.SourceLine] {
			delete(answered, row.SourceLine)
			emit(systemLike(row, approvedText))
		}
	})
}

// apiWithDecisions emits the API history's permission decisions. Beside
// ui_messages.json only the decisions and the file's problems are emitted,
// since the transcript rows come from that file; alone, the API history's
// rows come with each decision after its tool_result row.
func (p *Parser) apiWithDecisions(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	context := p.Context(artifact)
	if parser.RegularFile(clinelegacy.TaskOf(artifact).InDir(clinelegacy.UIFile)) {
		var problems parser.Problems
		for _, decision := range apiDecisions(artifact, artifact.DiskPath, &problems) {
			ownTime := timeutil.FromValue(decision.timestamp)
			row := p.Row(artifact, context, decision.position, timeutil.FirstNonEmpty(ownTime, context.Start), "")
			context.Member(&row, ownTime != "", true)
			row.ToolUseID, row.ToolName = decision.toolUseID, decision.toolName
			emit(systemLike(row, decision.text))
		}
		parser.EmitAll(emit, p.TaskErrorRows(artifact, problems, context))
		return
	}
	var ignored parser.Problems
	pending := map[string][]decision{}
	for _, decision := range apiDecisions(artifact, artifact.DiskPath, &ignored) {
		pending[decision.toolUseID] = append(pending[decision.toolUseID], decision)
	}
	p.ParseAPI(artifact, context, options, func(row timeline.Row) {
		emit(row)
		waiting := pending[row.ToolUseID]
		if row.TurnType != timeline.TurnToolResult || len(waiting) == 0 {
			return
		}
		pending[row.ToolUseID] = waiting[1:]
		result := systemLike(row, waiting[0].text)
		if result.ToolName == "" {
			result.ToolName = waiting[0].toolName
		}
		emit(result)
	})
}

// decision is one permission decision a tool_result of the API history
// records: the record's position and ts, the call's id and tool name, and
// the row text.
type decision struct {
	position                  int
	timestamp                 jsonvalue.Value
	toolUseID, toolName, text string
}

// apiDecisions returns the permission decision of each tool_result whose
// text opens with the JSON Roo writes for a denial or an approval with
// feedback: {"status":"denied","message":...}, {"status":"denied",
// "feedback":...} or {"status":"approved","feedback":...}, the last
// followed by the tool output.
func apiDecisions(artifact *input.Artifact, path string, problems *parser.Problems) []decision {
	var decisions []decision
	names := map[string]string{}
	for position, message := range parser.JSONArray(path, problems) {
		for _, block := range message.Get("content").Items() {
			if parser.IsText(block.Get("type"), "tool_use") {
				names[parser.Text(block.Get("id"))] = parser.Text(block.Get("name"))
				continue
			}
			if !parser.IsText(block.Get("type"), "tool_result") {
				continue
			}
			text, ok := decisionText(artifact, block.Get("content"))
			if !ok {
				continue
			}
			toolUseID := parser.Text(block.Get("tool_use_id"))
			decisions = append(decisions, decision{position: position, timestamp: message.Get("ts"),
				toolUseID: toolUseID, toolName: names[toolUseID], text: text})
		}
	}
	return decisions
}

// decisionText returns the "permission decision:" text of a tool result's
// content, or false when its text does not open with a decision object.
func decisionText(artifact *input.Artifact, content jsonvalue.Value) (string, bool) {
	body := strings.TrimLeftFunc(parser.TextOf(content), unicode.IsSpace)
	if !strings.HasPrefix(body, "{") {
		return "", false
	}
	document := jsonvalue.NewDocument(body)
	status, _, err := document.DecodePrefix(0)
	parser.NoteRepeats(artifact, document.TakeRepeats())
	if err != nil {
		return "", false
	}
	state := status.Get("status")
	if !parser.IsText(state, "denied") && !parser.IsText(state, "approved") {
		return "", false
	}
	text := "permission decision: " + state.StringValue()
	if feedback := status.Get("feedback"); !feedback.IsNull() {
		text += " feedback=" + parser.SettingValue(feedback)
	}
	return text, true
}
