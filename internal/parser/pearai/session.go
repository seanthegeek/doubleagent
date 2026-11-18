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
// item of history[] and perplexityHistory[], then the problem of a damaged
// file.
func (p *Parser) session(artifact *input.Artifact, fileID string, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.ReadText(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, fileID, ""))
		return
	}
	document, problem := continuedev.LoadSession(raw)
	base := p.BaseRow(artifact)
	base.SessionID = cmp.Or(parser.Text(document.Get("sessionId")), fileID)
	var entry jsonvalue.Value
	if directory, ok := parser.Beside(artifact); ok {
		entry = p.indexes.Index(directory)[base.SessionID]
	}
	base.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
	base.ProjectPath = parser.Text(parser.FirstNonEmpty(document.Get("workspaceDirectory"),
		entry.Get("workspaceDirectory")))
	row := func(position int, turnType, text, model string) {
		each := base
		each.SourceLine, each.TurnType, each.Model = position, turnType, model
		each.Text = timeline.Compact(text)
		emit(each)
	}
	history := document.Get("history").Items()
	search := document.Get("perplexityHistory").Items()
	row(0, timeline.TurnSystem, startText(document, entry, len(history), len(search),
		base.TimestampUTC != ""), "")
	models := parser.Tracker{}
	for i, item := range append(append([]jsonvalue.Value{}, history...), search...) {
		if message := item.Get("message"); message.IsObject() {
			sessionItem(i+1, item, message, base.SessionID, models, row)
		}
	}
	if problem != nil {
		row(0, timeline.TurnSystem, parser.ProblemPrefix+problem.Error(), "")
	}
}

// startText returns the text of a session's start row: its title, the
// index's integration, the item counts and whether the time is known.
func startText(document, entry jsonvalue.Value, history, search int, dated bool) string {
	bits := []string{"session start: " + parser.Text(parser.FirstNonEmpty(document.Get("title"),
		entry.Get("title")))}
	if integration := entry.Get("integrationType"); !integration.IsEmpty() {
		bits = append(bits, "integration="+integration.Text())
	}
	bits = append(bits, "history="+strconv.Itoa(history)+" perplexityHistory="+strconv.Itoa(search))
	if !dated {
		bits = append(bits, "dateCreated unknown")
	}
	return strings.Join(bits, " | ")
}

// sessionItem emits the rows of one session item at a 1-based position:
// a model row on each change, then its message, a user message with its
// context items and a reply with its citations.
func sessionItem(position int, item, message jsonvalue.Value, sessionID string, models parser.Tracker,
	row func(int, string, string, string)) {
	role := message.Get("role")
	text := continuedev.MessageText(message.Get("content"))
	model := ""
	for _, log := range item.Get("promptLogs").Items() {
		if options := log.Get("completionOptions"); !options.Get("model").IsEmpty() {
			model = options.Get("model").Text()
			if change := models.Change("model", sessionID, model); change != "" {
				row(position, timeline.TurnSystem, change, model)
			}
		}
	}
	switch {
	case isText(role, "user"):
		if labels := itemLabels(item.Get("contextItems"), "description", "name"); labels != "" {
			text += "\n[context: " + labels + "]"
		}
		if text != "" {
			row(position, timeline.TurnUser, text, "")
		}
	case isText(role, "assistant"):
		if labels := itemLabels(item.Get("citations"), "url", "title"); labels != "" {
			text += "\n[citations: " + labels + "]"
		}
		if text != "" {
			row(position, timeline.TurnAssistant, text, model)
		}
	case text != "":
		if !isText(role, "system") {
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
