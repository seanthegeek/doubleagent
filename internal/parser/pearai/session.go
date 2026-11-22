package pearai

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/continuedev"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// session emits the rows of a chat session file: its start row, then each
// item of history[] and perplexityHistory[], then the problems of a member
// that is not an array and of a damaged file.
func (p *Parser) session(artifact *input.Artifact, fileID string, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.ReadText(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, fileID, ""))
		return
	}
	document, problem := continuedev.LoadSession(artifact, raw)
	base := p.BaseRow(artifact)
	base.SessionID = cmp.Or(parser.Text(document.Get("sessionId")), fileID)
	var entry jsonvalue.Value
	var repeat *continuedev.Repeat
	if index, ok := parser.Beside(artifact, continuedev.IndexName); ok {
		entry, repeat = p.indexes.Index(index)[base.SessionID], p.indexes.Repeated(index, base.SessionID)
	}
	base.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
	base.ProjectPath = parser.Text(parser.FirstNonEmpty(document.Get("workspaceDirectory"),
		entry.Get("workspaceDirectory")))
	if repeat != nil {
		repeat.Group.Member(&base, continuedev.MemberTakes(document))
	}
	row := func(position int, turnType, text, model string) {
		each := base
		each.SourceLine, each.TurnType, each.Model = position, turnType, model
		each.Text = timeline.Compact(text)
		emit(each)
	}
	history := parser.ArrayMember(document.Get("history"), "history", 0, &problems)
	search := parser.ArrayMember(document.Get("perplexityHistory"), "perplexityHistory", 0, &problems)
	if repeat == nil {
		carried := parser.NewCarried(artifact)
		row(0, timeline.TurnSystem, startText(document, entry, len(history), len(search),
			base.TimestampUTC != "", carried), "")
		carried.Report(0, &problems)
	} else {
		parser.EmitAll(emit, p.repeatedStarts(artifact, base, document, repeat, len(history), len(search)))
	}
	models := parser.Tracker{}
	for i, item := range append(append([]jsonvalue.Value{}, history...), search...) {
		if message := item.Get("message"); message.IsObject() {
			sessionItem(i+1, item, message, base.SessionID, models, &problems, row)
		}
	}
	if problem != nil {
		problems.Add(0, problem.Error())
	}
	for _, found := range problems {
		row(found.Line, timeline.TurnSystem, parser.ProblemPrefix+found.Message, "")
	}
}

// repeatedStarts returns a start row for each entry of a session id the
// index repeats, with the entry's time and, when the file has none,
// project, and the problems of its text, as rows of the entry's parent
// shown only when the session's rows go with it.
func (p *Parser) repeatedStarts(artifact *input.Artifact, base timeline.Row, document jsonvalue.Value,
	repeat *continuedev.Repeat, history, search int) []timeline.Row {
	var rows []timeline.Row
	for place, entry := range repeat.Entries {
		start := base
		start.Group = nil
		start.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
		if document.Get("workspaceDirectory").IsEmpty() {
			start.ProjectPath = parser.Text(entry.Get("workspaceDirectory"))
		}
		start.TurnType = timeline.TurnSystem
		carried := parser.NewCarried(artifact)
		start.Text = timeline.Compact(startText(document, entry, history, search, start.TimestampUTC != "", carried))
		var problems parser.Problems
		carried.Report(0, &problems)
		each := []timeline.Row{start}
		for _, found := range problems {
			problem := start
			problem.SourceLine, problem.Text = found.Line, parser.ProblemPrefix+found.Message
			each = append(each, problem)
		}
		for i := range each {
			repeat.Parents[place].Row(&each[i], timeline.WhenChosen)
		}
		rows = append(rows, each...)
	}
	return rows
}

// startText returns the text of a session's start row: its title, the
// index's integration, the item counts and whether the time is known. What
// comes from the index entry, which any number of session files can name,
// is cut by carried; the file's own title is not.
func startText(document, entry jsonvalue.Value, history, search int, dated bool, carried *parser.Carried) string {
	bits := []string{"session start: " + continuedev.Title(document, entry, carried)}
	if integration := entry.Get("integrationType"); !integration.IsEmpty() {
		bits = append(bits, "integration="+carried.Cut("integration type", integration.Text()))
	}
	bits = append(bits, "history="+strconv.Itoa(history)+" perplexityHistory="+strconv.Itoa(search))
	if !dated {
		bits = append(bits, "dateCreated unknown")
	}
	return strings.Join(bits, " | ")
}

// sessionItem emits the rows of one session item at a 1-based position:
// a model row on each change, then its message, a user message with its
// context items and a reply with its citations. A promptLogs that is not
// an array adds a problem.
func sessionItem(position int, item, message jsonvalue.Value, sessionID string, models parser.Tracker,
	problems *parser.Problems, row func(int, string, string, string)) {
	role := message.Get("role")
	text := continuedev.MessageText(message.Get("content"))
	model := ""
	for _, log := range parser.ArrayMember(item.Get("promptLogs"), "promptLogs", position, problems) {
		if options := log.Get("completionOptions"); !options.Get("model").IsEmpty() {
			model = options.Get("model").Text()
			if change := models.Change("model", sessionID, model); change != "" {
				row(position, timeline.TurnSystem, change, model)
			}
		}
	}
	switch {
	case parser.IsText(role, "user"):
		if labels := itemLabels(item.Get("contextItems"), "description", "name"); labels != "" {
			text += "\n[context: " + labels + "]"
		}
		if text != "" {
			row(position, timeline.TurnUser, text, "")
		}
	case parser.IsText(role, "assistant"):
		if labels := itemLabels(item.Get("citations"), "url", "title"); labels != "" {
			text += "\n[citations: " + labels + "]"
		}
		if text != "" {
			row(position, timeline.TurnAssistant, text, model)
		}
	case text != "":
		if !parser.IsText(role, "system") {
			text = cmp.Or(parser.Text(role), "unknown") + ": " + text
		}
		row(position, timeline.TurnSystem, text, model)
	}
}

// itemLabels returns the labels of a list of objects, each its first key
// or else its second, the empty ones left out, joined by ", ".
func itemLabels(list jsonvalue.Value, first, second string) string {
	var labels []string
	for _, each := range list.Items() {
		if !each.IsObject() {
			continue
		}
		if label := parser.Text(parser.FirstNonEmpty(each.Get(first), each.Get(second))); label != "" {
			labels = append(labels, label)
		}
	}
	return strings.Join(labels, ", ")
}
