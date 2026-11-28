package continuedev

import (
	"cmp"
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
// call id, the calls a tool message answers and the problems met.
type sessionFile struct {
	parser   *Parser
	artifact *input.Artifact
	options  parserkit.Options
	emit     parserkit.Emit
	base     timeline.Row
	model    string
	models   parserkit.Tracker
	names    map[string]string
	answered map[string]bool
	problems parserkit.Problems
	// repeat is the session id's repeat when the index holds the id in more
	// than one entry, and takes what a row takes from the entry the id's
	// group goes with.
	repeat *Repeat
	takes  timeline.Takes
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

// MemberTakes returns what a row of a session file whose id the index
// repeats takes from the entry the id's group goes with: the entry's
// dateCreated, and its workspaceDirectory when the file has none.
func MemberTakes(document jsonvalue.Value) timeline.Takes {
	if document.Get("workspaceDirectory").IsEmpty() {
		return timeline.TakeTime | timeline.TakeProject
	}
	return timeline.TakeTime
}

// member returns emit with each row not yet in the repeat's group made a
// member of it, taking s.takes.
func (s *sessionFile) member(repeat *Repeat, emit parserkit.Emit) parserkit.Emit {
	return func(row timeline.Row) {
		if row.Group == nil {
			repeat.Group.Member(&row, s.takes)
		}
		emit(row)
	}
}

// header emits a session's start row, its first model, its mode, and the
// scratch directory row of a cn review worktree; an index title it cuts
// adds a problem.
func (s *sessionFile) header(document, entry jsonvalue.Value, items int) {
	model := s.model
	carried := parserkit.NewCarried(s.artifact)
	bits := []string{"session start: " + Title(document, entry, carried)}
	carried.Report(0, &s.problems)
	if mode := document.Get("mode"); !mode.IsEmpty() {
		bits = append(bits, "mode="+mode.Text())
	}
	bits = append(bits, "items="+strconv.Itoa(items))
	for _, field := range []string{"promptTokens", "completionTokens", "totalCost"} {
		if value := document.Get("usage").Get(field); !value.IsNull() {
			bits = append(bits, field+"="+value.Text())
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
	if mode := document.Get("mode"); mode.IsString() && mode.StringValue() != "" {
		texts = append(texts, "permission mode: "+mode.StringValue())
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

// repeatedHeaders emits the header rows once for each entry of a session
// id the index repeats, with the entry's time and, when the file has none,
// project, and the problems the header adds, as rows of the entry's parent
// shown only when the session's rows go with it.
func (s *sessionFile) repeatedHeaders(document jsonvalue.Value, repeat *Repeat, items int) {
	base, emit, problems := s.base, s.emit, s.problems
	for place, entry := range repeat.Entries {
		s.base.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
		if s.takes&timeline.TakeProject != 0 {
			s.base.ProjectPath = parserkit.Text(entry.Get("workspaceDirectory"))
		}
		s.models, s.problems = parserkit.Tracker{}, nil
		parent := repeat.Parents[place]
		s.emit = func(row timeline.Row) {
			parent.Row(&row, timeline.WhenChosen)
			emit(row)
		}
		s.header(document, entry, items)
		parserkit.EmitAll(s.emit, s.parser.ErrorRows(s.artifact, s.problems, s.base.SessionID, s.base.ProjectPath))
	}
	s.base, s.emit, s.problems = base, emit, problems
}

// ownTime makes row, a row with a time of its own when own is set, a
// member of the session's group that keeps that time.
func (s *sessionFile) ownTime(row *timeline.Row, own string) {
	if own != "" && s.repeat != nil {
		s.repeat.Group.Member(row, s.takes&^timeline.TakeTime)
	}
}

// item emits the rows of one history item at a 1-based position.
func (s *sessionFile) item(position int, item, message jsonvalue.Value) {
	model := s.promptModels(position, item)
	if reasoning := item.Get("reasoning"); s.options.IncludeThinking && reasoning.IsObject() &&
		!reasoning.Get("text").IsEmpty() {
		row := s.row(position, model, timeline.TurnThinking)
		own := timeutil.FromValue(reasoning.Get("startAt"))
		row.TimestampUTC = cmp.Or(own, s.base.TimestampUTC)
		row.Text = timeline.CompactValue(reasoning.Get("text"))
		s.ownTime(&row, own)
		s.emit(row)
	}
	role := message.Get("role")
	text := MessageText(message.Get("content"))
	switch {
	case parserkit.IsText(role, "user"):
		if text != "" {
			s.text(s.row(position, "", timeline.TurnUser), text)
		}
	case parserkit.IsText(role, "assistant"):
		if text != "" {
			s.text(s.row(position, model, timeline.TurnAssistant), text)
		}
		s.toolCalls(position, model, item, message)
	case parserkit.IsText(role, "thinking"):
		if s.options.IncludeThinking && text != "" {
			s.text(s.row(position, model, timeline.TurnThinking), text)
		}
	case parserkit.IsText(role, "tool"):
		row := s.row(position, "", timeline.TurnToolResult)
		row.ToolUseID = parserkit.Text(message.Get("toolCallId"))
		row.ToolName = s.names[row.ToolUseID]
		s.text(row, text)
	case text != "":
		if !parserkit.IsText(role, "system") {
			text = cmp.Or(parserkit.Text(role), "unknown") + ": " + text
		}
		s.text(s.row(position, "", timeline.TurnSystem), text)
	}
}

// session emits the rows of one session file: its start, model, mode and
// review worktree rows, then each history item's rows, then the problems
// of a member that is not an array and of a damaged file.
func (p *Parser) session(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	fileID := sessionPattern.FindStringSubmatch(artifact.Rel)[1]
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, fileID, ""))
		return
	}
	document, problem := LoadSession(artifact, textlines.ValidUTF8(content))
	sessionID := parserkit.Text(document.Get("sessionId"))
	named := sessionID == ""
	if named {
		sessionID = fileID
	}
	var entry jsonvalue.Value
	var repeat *Repeat
	if index, ok := parserkit.Beside(artifact, IndexName); ok {
		if named {
			sessionID = p.indexes.Resolve(index, sessionID)
		}
		entry, repeat = p.indexes.Index(index)[sessionID], p.indexes.Repeated(index, sessionID)
	}
	state := &sessionFile{parser: p, artifact: artifact, options: options, emit: emit,
		base: p.BaseRow(artifact), models: parserkit.Tracker{}, names: map[string]string{},
		answered: map[string]bool{}}
	state.base.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
	state.base.SessionID = sessionID
	state.base.ProjectPath = parserkit.Text(parserkit.FirstNonEmpty(document.Get("workspaceDirectory"),
		entry.Get("workspaceDirectory")))
	state.model = parserkit.Text(document.Get("chatModelTitle"))
	history := parserkit.ArrayMember(document.Get("history"), "history", 0, &state.problems)
	if repeat == nil {
		state.header(document, entry, len(history))
	} else {
		state.repeat, state.takes = repeat, MemberTakes(document)
		state.emit = state.member(repeat, emit)
		state.repeatedHeaders(document, repeat, len(history))
	}
	for _, item := range history {
		message := item.Get("message")
		if parserkit.IsText(message.Get("role"), "tool") && !message.Get("toolCallId").IsEmpty() {
			state.answered[message.Get("toolCallId").Text()] = true
		}
	}
	for i, item := range history {
		if message := item.Get("message"); message.IsObject() {
			state.item(i+1, item, message)
		}
	}
	if problem != nil {
		state.problems.Add(0, problem.Error())
	}
	parserkit.EmitAll(state.emit, p.ErrorRows(artifact, state.problems, state.base.SessionID, state.base.ProjectPath))
}
