package twinny

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// conversationRows builds the rows of one conversation: each takes the
// conversation's updatedAt and id.
type conversationRows struct {
	owner     *Parser
	artifact  *input.Artifact
	timestamp string
	sessionID string
	emit      parser.Emit
}

// row emits a row of the conversation with its text compacted.
func (c conversationRows) row(turnType, text, model, toolName, toolUseID string) {
	row := c.owner.BaseRow(c.artifact)
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.TurnType = turnType
	row.Model = model
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	c.emit(row)
}

// conversation emits the rows of one conversation: a heading system row,
// then each message's rows.
func (p *Parser) conversation(artifact *input.Artifact, conversation pyjson.Value, options parser.Options,
	emit parser.Emit) {
	rows := conversationRows{owner: p, artifact: artifact, emit: emit,
		timestamp: timeutil.FromValue(conversation.Get("updatedAt")),
		sessionID: parser.Str(conversation.Get("id"))}
	var messages []pyjson.Value
	for _, message := range conversation.Get("messages").Items() {
		if message.IsObject() {
			messages = append(messages, message)
		}
	}
	heading := "conversation: " + parser.Str(conversation.Get("title")) + " | messages=" +
		strconv.Itoa(len(messages))
	if rows.timestamp == "" {
		heading += " | updatedAt unknown"
	}
	rows.row(timeline.TurnSystem, heading, "", "", "")
	models := parser.Tracker{} // per conversation, so the key needs no session id
	for _, message := range messages {
		rows.message(message, models, options)
	}
}

// message emits the rows of one message: a model change, its reasoning,
// its text, its tool steps, its file changes and what the secret shield
// withheld.
func (c conversationRows) message(message pyjson.Value, models parser.Tracker, options parser.Options) {
	role := message.Get("role")
	meta := message.Get("meta")
	model := parser.Str(meta.Get("model"))
	isAssistant := role.IsString() && role.Text() == "assistant"
	if isAssistant {
		if change := models.Change("model", "model", model); change != "" {
			c.row(timeline.TurnSystem, change, model, "", "")
		}
	}
	content := message.Get("content")
	text := content.Text()
	if !content.IsString() {
		text = parser.TextOf(content)
	}
	text, thoughts := splitThink(text)
	if options.IncludeThinking {
		for _, thought := range thoughts {
			c.row(timeline.TurnThinking, thought, model, "", "")
		}
	}
	if images := message.Get("images"); images.IsArray() && images.Len() > 0 {
		text = pytext.Strip(text + "\n[" + strconv.Itoa(images.Len()) + " image(s)]")
	}
	if text != "" {
		switch {
		case role.IsString() && role.Text() == "user":
			c.row(timeline.TurnUser, text, "", "", "")
		case isAssistant:
			c.row(timeline.TurnAssistant, text, model, "", "")
		default:
			c.row(timeline.TurnSystem, roleName(role)+": "+text, model, "", "")
		}
	}
	for _, step := range parser.Elements(message.Get("toolSteps")) {
		if step.IsObject() {
			c.toolStep(step, model)
		}
	}
	c.changes(message.Get("changes"), model)
	if withheld := meta.Get("withheld"); withheld.Truthy() {
		c.row(timeline.TurnSystem, "secret shield withheld: "+parser.CompactJSON(withheld), model, "", "")
	}
}

// roleName returns a role as a system row names it, "unknown" when it is
// missing or empty.
func roleName(role pyjson.Value) string {
	if name := parser.Str(role); name != "" {
		return name
	}
	return "unknown"
}

// toolStep emits the rows of one tool step: a message the user sent while
// the reply ran, or a tool call, its result and a permission decision.
func (c conversationRows) toolStep(step pyjson.Value, model string) {
	if said, ok := step.Get("said").Bool(); ok && said {
		summary := step.Get("summary")
		text := summary.Text()
		if !summary.IsString() && !summary.IsNull() {
			text = parser.CompactJSON(summary) // a summary that is not a string is kept whole
		}
		c.row(timeline.TurnUser, text, "", "", "")
		return
	}
	name, toolUseID := parser.Str(step.Get("name")), parser.Str(step.Get("id"))
	c.row(timeline.TurnToolUse, stepBody(step), model, name, toolUseID)
	output := step.Get("output")
	out := output.Text()
	if !output.IsString() && !output.IsNull() {
		out = parser.CompactJSON(output)
	}
	status := parser.Str(step.Get("status"))
	if status != "" && status != "done" {
		out = "[" + status + "] " + out
	}
	c.row(timeline.TurnToolResult, out, model, name, toolUseID)
	if status == "waiting" || status == "skipped" {
		decision := "permission decision: " + status
		if approval := step.Get("approval"); approval.Truthy() {
			decision += " approval=" + parser.SettingValue(approval)
		}
		c.row(timeline.TurnSystem, decision, model, name, toolUseID)
	}
}

// stepBody returns the text of a tool call: its command, else its
// arguments as JSON, else its summary, else "".
func stepBody(step pyjson.Value) string {
	if command := step.Get("command"); command.Truthy() {
		return command.Str()
	}
	if args := step.Get("args"); args.Truthy() {
		return parser.CompactJSON(args)
	}
	if summary := step.Get("summary"); summary.Truthy() {
		return summary.Str()
	}
	return ""
}

// changes emits a "file changed:" row per changed file of a reply.
func (c conversationRows) changes(changes pyjson.Value, model string) {
	if !changes.IsObject() || !changes.Get("files").IsArray() {
		return
	}
	for _, entry := range changes.Get("files").Items() {
		if entry.IsObject() {
			c.row(timeline.TurnSystem, fileChangeText(changes.Get("id"), entry), model, "", "")
		}
	}
}

// splitThink returns a message's text without its reasoning blocks, and the
// reasoning texts that are not blank. It stands for Python's
// re.sub(r"<(think|thinking)>(.*?)(?:</\1>|$)", ..., flags=re.S), whose
// backreference Go's engine has no form for: a block opens at <think> or
// <thinking> and ends at the first matching close tag, else just before a
// final LF, else at the end.
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
		if strings.HasSuffix(body, "\n") {
			end, next = len(body)-1, len(body)-1
		}
		if found := strings.Index(body, closing); found >= 0 && found < end {
			end, next = found, found+len(closing)
		}
		if thought := pytext.Strip(body[:end]); thought != "" {
			thoughts = append(thoughts, thought)
		}
		rest = body[next:]
	}
	return pytext.Strip(out.String()), thoughts
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
