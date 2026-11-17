package continuedev

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// reviewWorktree matches a cn review worktree's basename,
// cn-review-<epoch ms>-<n> (extensions/cli/src/commands/review/worktree.ts:15-20).
var reviewWorktree = regexp.MustCompile(`(?:^|[/\\])cn-review-[0-9]+-[0-9]+[/\\]?\z`)

// policyViolation is the output item the extension gives a call its policy
// disables (gui/src/redux/thunks/evaluateToolPolicies.ts:93-117).
const policyViolation = "Security Policy Violation"

// sessionFile is the state of one session file's parse: the rows' common
// fields, the session's model, the model changes seen, the tool names by
// call id and the calls a tool message answers.
type sessionFile struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	emit     parser.Emit
	base     timeline.Row
	model    string
	models   parser.Tracker
	names    map[string]string
	answered map[string]bool
}

// row returns a row of the session at a history position, with a model
// and turn type.
func (s *sessionFile) row(position int, model, turnType string) timeline.Row {
	row := s.base
	row.SourceLine, row.Model, row.TurnType = position, model, turnType
	return row
}

// text emits a row with its text compacted.
func (s *sessionFile) text(row timeline.Row, text string) {
	row.Text = timeline.Compact(text)
	s.emit(row)
}

// session emits the rows of one session file: its start, model, mode and
// review worktree rows, then each history item's rows, then the problem a
// damaged file has.
func (p *Parser) session(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	fileID := sessionPattern.FindStringSubmatch(artifact.Rel)[1]
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, fileID, ""))
		return
	}
	document, problem := LoadSession(pytext.DecodeUTF8(content))
	sessionID := clinelegacy.FirstTruthy(document.Get("sessionId"), pyjson.NewString(fileID)).Str()
	entry := p.indexes.Index(parser.Beside(artifact))[sessionID]
	state := &sessionFile{parser: p, artifact: artifact, options: options, emit: emit,
		base: p.BaseRow(artifact), models: parser.Tracker{}, names: map[string]string{},
		answered: map[string]bool{}}
	state.base.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
	state.base.SessionID = sessionID
	state.base.ProjectPath = parser.Str(clinelegacy.FirstTruthy(document.Get("workspaceDirectory"),
		entry.Get("workspaceDirectory")))
	state.model = parser.Str(document.Get("chatModelTitle"))
	history := document.Get("history").Items()
	state.header(document, entry, len(history))
	for _, item := range history {
		message := item.Get("message")
		if isText(message.Get("role"), "tool") && message.Get("toolCallId").Truthy() {
			state.answered[message.Get("toolCallId").Str()] = true
		}
	}
	for i, item := range history {
		if message := item.Get("message"); message.IsObject() {
			state.item(i+1, item, message)
		}
	}
	if problem != "" {
		row := state.row(0, "", timeline.TurnSystem)
		row.Text = parser.ProblemPrefix + problem
		emit(row)
	}
}

// header emits a session's start row, its first model, its mode, and the
// scratch directory row of a cn review worktree.
func (s *sessionFile) header(document, entry pyjson.Value, items int) {
	model := s.model
	bits := []string{"session start: " + parser.Str(clinelegacy.FirstTruthy(document.Get("title"),
		entry.Get("title")))}
	if mode := document.Get("mode"); mode.Truthy() {
		bits = append(bits, "mode="+mode.Str())
	}
	bits = append(bits, "items="+strconv.Itoa(items))
	for _, field := range []string{"promptTokens", "completionTokens", "totalCost"} {
		if value := document.Get("usage").Get(field); !value.IsNull() {
			bits = append(bits, field+"="+value.Str())
		}
	}
	if s.base.TimestampUTC == "" {
		bits = append(bits, "dateCreated unknown")
	}
	s.text(s.row(0, model, timeline.TurnSystem), strings.Join(bits, " "))
	var texts []string
	if change := s.models.Change("model", s.base.SessionID, model); change != "" {
		texts = append(texts, change)
	}
	if mode := document.Get("mode"); mode.IsString() && mode.Text() != "" {
		texts = append(texts, "permission mode: "+mode.Text())
	}
	if reviewWorktree.MatchString(s.base.ProjectPath) {
		texts = append(texts, "scratch directory: "+s.base.ProjectPath+" (cn review worktree)")
	}
	for _, text := range texts {
		row := s.row(0, model, timeline.TurnSystem)
		row.Text = text
		s.emit(row)
	}
}

// item emits the rows of one history item at a 1-based position.
func (s *sessionFile) item(position int, item, message pyjson.Value) {
	model := s.promptModels(position, item)
	if reasoning := item.Get("reasoning"); s.options.IncludeThinking && reasoning.IsObject() &&
		reasoning.Get("text").Truthy() {
		row := s.row(position, model, timeline.TurnThinking)
		row.TimestampUTC = timeutil.FirstNonEmpty(timeutil.FromValue(reasoning.Get("startAt")),
			s.base.TimestampUTC)
		row.Text = timeline.CompactValue(reasoning.Get("text"))
		s.emit(row)
	}
	role := message.Get("role")
	text := MessageText(message.Get("content"))
	switch {
	case isText(role, "user"):
		if text != "" {
			s.text(s.row(position, "", timeline.TurnUser), text)
		}
	case isText(role, "assistant"):
		if text != "" {
			s.text(s.row(position, model, timeline.TurnAssistant), text)
		}
		s.toolCalls(position, model, item, message)
	case isText(role, "thinking"):
		if s.options.IncludeThinking && text != "" {
			s.text(s.row(position, model, timeline.TurnThinking), text)
		}
	case isText(role, "tool"):
		row := s.row(position, "", timeline.TurnToolResult)
		row.ToolUseID = parser.Str(message.Get("toolCallId"))
		row.ToolName = s.names[row.ToolUseID]
		s.text(row, text)
	case text != "":
		if !isText(role, "system") {
			text = clinelegacy.FirstTruthy(role, pyjson.NewString("unknown")).Str() + ": " + text
		}
		s.text(s.row(position, "", timeline.TurnSystem), text)
	}
}
