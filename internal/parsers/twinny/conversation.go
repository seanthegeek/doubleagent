package twinny

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// conversationRows builds the rows of one conversation: each takes the
// conversation's updatedAt and id, and the project of the workspace whose
// state holds it.
type conversationRows struct {
	owner     *Parser
	artifact  *input.Artifact
	timestamp string
	sessionID string
	project   string
	emit      parserkit.Emit
}

// row emits a row of the conversation with its text compacted.
func (c conversationRows) row(turnType, text, model, toolName, toolUseID string) {
	row := c.owner.BaseRow(c.artifact)
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.ProjectPath = c.project
	row.TurnType = turnType
	row.Model = model
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	c.emit(row)
}

// text emits a message's text as a user or assistant row, or as a system
// row naming any other role; empty text gives nothing.
func (c conversationRows) text(role jsonvalue.Value, text, model string) {
	switch {
	case text == "":
	case role.IsString() && role.StringValue() == "user":
		c.row(timeline.TurnUser, text, "", "", "")
	case role.IsString() && role.StringValue() == "assistant":
		c.row(timeline.TurnAssistant, text, model, "", "")
	default:
		c.row(timeline.TurnSystem, parserkit.TextOr(role, "unknown")+": "+text, model, "", "")
	}
}

// stepBody returns the text of a tool call: its command, else its
// arguments as JSON, else its summary, else "".
func stepBody(step jsonvalue.Value) string {
	if command := step.Get("command"); !command.IsEmpty() {
		return command.Text()
	}
	if args := step.Get("args"); !args.IsEmpty() {
		return parserkit.JSONText(args)
	}
	if summary := step.Get("summary"); !summary.IsEmpty() {
		return summary.Text()
	}
	return ""
}

// toolStep emits the rows of one tool step: a message the user sent while
// the reply ran, or a tool call, its result and a permission decision.
func (c conversationRows) toolStep(step jsonvalue.Value, model string) {
	if said, ok := step.Get("said").Bool(); ok && said {
		c.row(timeline.TurnUser, step.Get("summary").Text(), "", "", "")
		return
	}
	name, toolUseID := parserkit.Text(step.Get("name")), parserkit.Text(step.Get("id"))
	c.row(timeline.TurnToolUse, stepBody(step), model, name, toolUseID)
	out := step.Get("output").Text()
	status := parserkit.Text(step.Get("status"))
	if status != "" && status != "done" {
		out = "[" + status + "] " + out
	}
	c.row(timeline.TurnToolResult, out, model, name, toolUseID)
	if status == "waiting" || status == "skipped" {
		decision := "permission decision: " + status
		if approval := step.Get("approval"); !approval.IsEmpty() {
			decision += " approval=" + parserkit.SettingValue(approval)
		}
		c.row(timeline.TurnSystem, decision, model, name, toolUseID)
	}
}

// changes emits a "file changed:" row per changed file of a reply, and
// counts in tally a changes or changes.files of the wrong kind.
func (c conversationRows) changes(changes jsonvalue.Value, model string, tally *parserkit.EntryTally) {
	if !tally.Check(changes, "changes", jsonvalue.Object) || changes.IsNull() {
		return
	}
	carried := parserkit.NewCarried(c.artifact)
	changeID := replyID(carried, changes.Get("id"))
	for _, entry := range tally.EntriesOfKind(changes.Get("files"), "changes.files", jsonvalue.Object) {
		c.row(timeline.TurnSystem, fileChangeText(changeID, entry), model, "", "")
	}
	var problems parserkit.Problems
	carried.Report(0, &problems)
	parserkit.EmitAll(c.emit, c.owner.ErrorRows(c.artifact, problems, c.sessionID, c.project))
}

// openTag returns the offset and name of the first <think> or <thinking>
// tag in text, or -1.
func openTag(text string) (int, string) {
	for i := strings.IndexByte(text, '<'); i >= 0; {
		switch {
		case strings.HasPrefix(text[i:], "<think>"):
			return i, "think"
		case strings.HasPrefix(text[i:], "<thinking>"):
			return i, "thinking"
		}
		next := strings.IndexByte(text[i+1:], '<')
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return -1, ""
}

// splitThink returns a message's text without its reasoning blocks, and the
// reasoning texts that are not blank. A block opens at <think> or <thinking>
// and ends at the first close tag of the same name, else at the end of the
// text; a close tag of the other name is part of the block. Matching the close tag to the open one needs a
// backreference, which Go's regexp has no form for, so the scan is by hand.
func splitThink(content string) (string, []string) {
	var out strings.Builder
	var thoughts []string
	rest := content
	for {
		start, tag := openTag(rest)
		if start < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:start])
		body := rest[start+len(tag)+2:]
		closing := "</" + tag + ">"
		end, next := len(body), len(body)
		if found := strings.Index(body, closing); found >= 0 {
			end, next = found, found+len(closing)
		}
		if thought := strings.TrimSpace(body[:end]); thought != "" {
			thoughts = append(thoughts, thought)
		}
		rest = body[next:]
	}
	return strings.TrimSpace(out.String()), thoughts
}

// message emits the rows of one message: a model change, its reasoning,
// text, tool steps and file changes, and what the secret shield withheld;
// a member of the wrong kind is counted in tally.
func (c conversationRows) message(message jsonvalue.Value, models parserkit.Tracker, options parserkit.Options,
	tally *parserkit.EntryTally) {
	role := message.Get("role")
	meta := message.Get("meta")
	model := parserkit.Text(meta.Get("model"))
	isAssistant := role.IsString() && role.StringValue() == "assistant"
	if isAssistant {
		if change := models.Change("model", "model", model); change != "" {
			c.row(timeline.TurnSystem, change, model, "", "")
		}
	}
	text, thoughts := splitThink(parserkit.TextOf(message.Get("content")))
	if options.IncludeThinking {
		for _, thought := range thoughts {
			c.row(timeline.TurnThinking, thought, model, "", "")
		}
	}
	if images := message.Get("images"); images.IsArray() && images.Len() > 0 {
		text = strings.TrimSpace(text + "\n[" + strconv.Itoa(images.Len()) + " image(s)]")
	}
	c.text(role, text, model)
	for _, step := range tally.EntriesOfKind(message.Get("toolSteps"), "toolSteps", jsonvalue.Object) {
		c.toolStep(step, model)
	}
	c.changes(message.Get("changes"), model, tally)
	if withheld := meta.Get("withheld"); !withheld.IsEmpty() {
		c.row(timeline.TurnSystem, "secret shield withheld: "+parserkit.JSONText(withheld), model, "", "")
	}
}

// conversation emits the rows of one conversation, each with project as its
// project path: a heading system row, then each message's rows, a member of
// the wrong kind one problem however many messages hold it.
func (p *Parser) conversation(artifact *input.Artifact, conversation jsonvalue.Value, project string,
	options parserkit.Options, emit parserkit.Emit) {
	rows := conversationRows{owner: p, artifact: artifact, emit: emit, project: project,
		timestamp: timeutil.FromValue(conversation.Get("updatedAt")),
		sessionID: parserkit.Text(conversation.Get("id"))}
	var problems parserkit.Problems
	var messages []jsonvalue.Value
	for _, message := range parserkit.EntriesOfKind(conversation.Get("messages"), "messages", jsonvalue.Object, 0,
		&problems) {
		messages = append(messages, message)
	}
	heading := "conversation: " + parserkit.Text(conversation.Get("title")) + " | messages=" +
		strconv.Itoa(len(messages))
	if rows.timestamp == "" {
		heading += " | updatedAt unknown"
	}
	rows.row(timeline.TurnSystem, heading, "", "", "")
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, rows.sessionID, project))
	models := parserkit.Tracker{} // per conversation, so the key needs no session id
	var tally parserkit.EntryTally
	for _, message := range messages {
		rows.message(message, models, options, &tally)
	}
	var wrong parserkit.Problems
	tally.Report(0, &wrong)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, wrong, rows.sessionID, project))
}
