package crush

import (
	"html"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The text of a tool result the user denied, the hook messages an error
// result starts with when the hook metadata is missing, and the finish
// reasons of a turn that ended normally.
const denied = "User denied permission"

var (
	hookPrefixes = [...][2]string{{"Tool call blocked by hook.", "deny"},
		{"Turn halted by hook.", "halt"}}
	normalFinish = [...]string{"", "end_turn", "tool_use", "stop"}
)

// modelState is a session's last model: the provider, the requested model
// and the model that served.
type modelState struct {
	provider, requested, served string
}

// messageReader turns message rows into timeline rows, tracking each
// session's model and the relays already reported.
type messageReader struct {
	store   *store
	options parser.Options
	models  map[string]modelState
	relayed map[[2]string]bool
}

// modelText returns the "model:" row text for one message, or "" when the
// session's model is unchanged. The served model is prism when set, else
// requested; a message without prism that repeats the last provider and
// request keeps the last served model rather than counting as a change.
func modelText(seen map[string]modelState, sessionID, provider, requested, prism string) string {
	old, known := seen[sessionID]
	served := prism
	if served == "" {
		if known && old.provider == provider && old.requested == requested {
			return ""
		}
		served = requested
	}
	if served == "" {
		return ""
	}
	current := modelState{provider: provider, requested: requested, served: served}
	if known && old == current {
		return ""
	}
	seen[sessionID] = current
	text := "model: " + served
	if known {
		text = "model: " + old.served + " -> " + served
	}
	if provider != "" {
		text += " provider=" + provider
	}
	if requested != "" && requested != served {
		text += " requested=" + requested
	}
	return text
}

// columnText returns a column's text when it is text, else "".
func columnText(value sqlitefile.Value) string {
	if value.Type == sqlitefile.Text {
		return value.String()
	}
	return ""
}

// message is one message being read: the row and what its parts share.
type message struct {
	record             sqlitefile.Row
	role, sessionID    string
	model, created     string
	finished, textType string
}

// base returns a row of the message at timestamp, else at its created_at.
func (r *messageReader) base(current *message, timestamp string) timeline.Row {
	row := r.store.parser.BaseRow(r.store.artifact)
	row.SourceLine = int(current.record.Rowid)
	row.SessionID = current.sessionID
	row.ProjectPath = r.store.projectPath
	row.Model = current.model
	row.TimestampUTC = timeutil.FirstNonEmpty(timestamp, current.created)
	return row
}

// message emits the rows of one message: its model change, then a row per
// part, each followed by the rows that part adds.
func (r *messageReader) message(record sqlitefile.Row) {
	provider := columnText(record.Get("provider"))
	requested, prism := columnText(record.Get("model")), columnText(record.Get("prism_model_id"))
	modelID := timeutil.FirstNonEmpty(prism, requested)
	current := &message{record: record, role: orText(record.Get("role")),
		sessionID: orText(record.Get("session_id")), model: modelID,
		created: parser.ColumnTime(record.Get("created_at"))}
	if provider != "" && modelID != "" {
		current.model = provider + "/" + modelID
	}
	if text := modelText(r.models, current.sessionID, provider, requested, prism); text != "" {
		row := r.base(current, "")
		row.TurnType, row.Text = timeline.TurnSystem, text
		r.store.emit(row)
	}
	parts, ok := r.parts(current)
	if !ok {
		return
	}
	if finished := record.Get("finished_at"); !finished.IsEmpty() {
		current.finished = parser.ColumnTime(finished)
	}
	for _, part := range parts {
		if part.IsObject() && isText(part.Get("type"), "finish") {
			stamp := part.Get("data").Get("time")
			if !stamp.IsEmpty() && current.finished == "" {
				current.finished = timeutil.FromValue(stamp)
			}
		}
	}
	current.textType = timeline.TurnSystem
	if current.role == timeline.TurnUser || current.role == timeline.TurnAssistant {
		current.textType = current.role
	}
	for _, part := range parts {
		if part.IsObject() {
			r.part(current, part)
		}
	}
}

// parts returns the message's parts, or false after emitting the bad-record
// row of a parts column that is not a JSON array.
func (r *messageReader) parts(current *message) ([]jsonvalue.Value, bool) {
	value := current.record.Get("parts")
	raw := ""
	switch value.Type {
	case sqlitefile.Blob:
		raw = textlines.ValidUTF8(value.Bytes)
	case sqlitefile.Null:
	default:
		raw = value.String()
	}
	source := raw
	if source == "" {
		source = "[]"
	}
	parts, err := jsonvalue.DecodeString(source)
	reason := ""
	switch {
	case err != nil:
		reason = err.Error()
	case !parts.IsArray():
		reason = "not a JSON array"
	default:
		return parts.Items(), true
	}
	rowid := current.record.Rowid
	var problems parser.Problems
	problems.AddRaw(int(rowid), "unparseable record "+strconv.FormatInt(rowid, 10)+" (message "+
		current.record.Get("id").String()+" parts: "+reason+")", raw)
	parser.EmitAll(r.store.emit, r.store.parser.ErrorRows(r.store.artifact, problems,
		current.sessionID, r.store.projectPath))
	return nil, false
}

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// part emits the row of one part and the rows it adds after it.
func (r *messageReader) part(current *message, part jsonvalue.Value) {
	kind, data := part.Get("type"), part.Get("data")
	if !data.IsObject() {
		data = jsonvalue.NewObject()
	}
	row := r.base(current, "")
	var after []timeline.Row
	text, keep := "", true
	switch {
	case !kind.IsString():
		row.TurnType, text = timeline.TurnSystem, kind.Text()+": "+parser.JSONText(data)
	case kind.StringValue() == "text":
		text, after = r.textPart(current, data, &row)
	case kind.StringValue() == "reasoning":
		if keep = r.options.IncludeThinking && !data.Get("thinking").IsEmpty(); keep {
			row.TurnType = timeline.TurnThinking
			row.TimestampUTC = timeutil.FirstNonEmpty(timeutil.FromValue(data.Get("started_at")),
				current.created)
			text = data.Get("thinking").Text()
		}
	case kind.StringValue() == "tool_call":
		row.TurnType = timeline.TurnToolUse
		row.TimestampUTC = timeutil.FirstNonEmpty(current.finished, current.created)
		row.ToolName, row.ToolUseID = parser.Text(data.Get("name")), parser.Text(data.Get("id"))
		text = parser.JSONText(data.Get("input"))
		if input := data.Get("input"); input.IsString() {
			text = input.StringValue()
		}
	case kind.StringValue() == "tool_result":
		text, after = r.resultPart(current, data, &row)
	default:
		text, keep = r.otherPart(current, kind.StringValue(), data, &row)
	}
	if !keep {
		return
	}
	row.Text = timeline.Compact(text)
	r.store.emit(row)
	parser.EmitAll(r.store.emit, after)
}

// textPart fills the row of a text part and returns its text, with the
// relay row of a user message an MCP channel pushed, once per session and
// relay.
func (r *messageReader) textPart(current *message, data jsonvalue.Value, row *timeline.Row) (string,
	[]timeline.Row) {
	row.TurnType = current.textType
	text := parser.Text(data.Get("text"))
	if current.textType == timeline.TurnSystem {
		text = current.role + ": " + text
	}
	if !data.Get("hidden").IsEmpty() {
		text = "[hidden] " + text
	}
	relay, key := r.relayText(text, orText(current.record.Get("id")))
	if relay == "" || current.role != timeline.TurnUser || r.relayed[[2]string{current.sessionID, key}] {
		return text, nil
	}
	r.relayed[[2]string{current.sessionID, key}] = true
	extra := r.base(current, "")
	extra.TurnType, extra.Text = timeline.TurnSystem, timeline.Compact(relay)
	return text, []timeline.Row{extra}
}

// relayText returns the "session relayed:" text of a message an MCP
// channel pushed, which starts `<channel source="<server>" <meta>>` with
// each attribute value HTML-escaped (research/crush.md), and
// its relay key: the part of the text naming the source, sender and group.
// Any other message gives two empty strings.
func (r *messageReader) relayText(text, messageID string) (string, string) {
	match := r.store.parser.channel.FindStringSubmatch(text)
	if match == nil {
		return "", ""
	}
	attributes := map[string]string{}
	for _, attribute := range r.store.parser.attribute.FindAllStringSubmatch(match[1], -1) {
		attributes[attribute[1]] = html.UnescapeString(attribute[2])
	}
	key := "session relayed: channel " + attributes["source"]
	for _, name := range []string{"sender", "group"} {
		if attributes[name] != "" {
			key += " " + name + " " + attributes[name]
		}
	}
	return key + " (from crush.db message " + messageID + ")", key
}

// resultPart fills the row of a tool result part and returns its text,
// with the permission decision row it records.
func (r *messageReader) resultPart(current *message, data jsonvalue.Value, row *timeline.Row) (string,
	[]timeline.Row) {
	row.TurnType = timeline.TurnToolResult
	row.TimestampUTC = timeutil.FirstNonEmpty(current.finished, current.created)
	row.ToolName, row.ToolUseID = parser.Text(data.Get("name")), parser.Text(data.Get("tool_call_id"))
	text := parser.Text(data.Get("content"))
	if mime := data.Get("mime_type"); text == "" && !mime.IsEmpty() {
		text = "[" + mime.Text() + "]"
	}
	if !data.Get("is_error").IsEmpty() {
		text = "[error] " + text
	}
	decision := permissionText(data)
	if decision == "" {
		return text, nil
	}
	extra := r.base(current, row.TimestampUTC)
	extra.TurnType = timeline.TurnSystem
	extra.ToolName, extra.ToolUseID = row.ToolName, row.ToolUseID
	extra.Text = timeline.Compact(decision)
	return text, []timeline.Row{extra}
}

// otherPart fills the row of a finish, shell command, image, binary or
// unknown part and returns its text, or false for a finish part that ended
// the turn normally without a message, which gives no row.
func (r *messageReader) otherPart(current *message, kind string, data jsonvalue.Value,
	row *timeline.Row) (string, bool) {
	row.TurnType = timeline.TurnSystem
	switch kind {
	case "finish":
		reason := parser.Text(data.Get("reason"))
		if slices.Contains(normalFinish[:], reason) && data.Get("message").IsEmpty() {
			return "", false
		}
		if stamp := data.Get("time"); !stamp.IsEmpty() {
			row.TimestampUTC = timeutil.FromValue(stamp)
		}
		return parser.JoinNonEmpty(" | ", "finish: "+reason, parser.Text(data.Get("message")),
			parser.Text(data.Get("details"))), true
	case "shell_command":
		exit := ""
		if code, ok := data.Lookup("exit_code"); ok {
			exit = "exit=" + code.Text()
		}
		return parser.JoinNonEmpty(" | ", "shell: "+parser.Text(data.Get("command")), exit,
			parser.Text(data.Get("output"))), true
	case "image_url":
		row.TurnType = current.textType
		url := parser.Text(data.Get("url"))
		if strings.HasPrefix(url, "data:") {
			url, _, _ = strings.Cut(url, ",")
		}
		return "[image] " + url, true
	case "binary":
		row.TurnType = current.textType
		return "[binary] " + parser.JoinNonEmpty(" | ",
			parser.Text(parser.FirstNonEmpty(data.Get("Path"), data.Get("path"))),
			parser.Text(parser.FirstNonEmpty(data.Get("MIMEType"), data.Get("mime_type")))), true
	}
	return kind + ": " + parser.JSONText(data), true
}

// metadata returns a tool result's metadata, stored as a JSON string, as
// an object; anything else is an empty object.
func metadata(value jsonvalue.Value) jsonvalue.Value {
	if value.IsString() && value.StringValue() != "" {
		decoded, err := jsonvalue.DecodeString(value.StringValue())
		if err != nil {
			return jsonvalue.NewObject()
		}
		value = decoded
	}
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}

// permissionText returns the "permission decision:" text of a tool result
// part's data, or "". A user denial is the error content "User denied
// permission"; a hook's decision is metadata.hook, and an error result that
// starts with a hook's block or halt message stands for it when the
// metadata is missing. A hook decision of none without a halt is not one.
func permissionText(data jsonvalue.Value) string {
	content := textOnly(data.Get("content"))
	isError := !data.Get("is_error").IsEmpty()
	if isError && content == denied {
		return "permission decision: denied by user"
	}
	if hook := metadata(data.Get("metadata")).Get("hook"); hook.IsObject() {
		return hookDecision(hook)
	}
	if isError {
		for _, prefix := range hookPrefixes {
			if strings.HasPrefix(content, prefix[0]) {
				return "permission decision: " + prefix[1] + " by hook"
			}
		}
	}
	return ""
}

// hookDecision returns the "permission decision:" text of a hook's
// decision, naming the hooks that made it (all of them when none is
// marked), or "" for a decision that is neither allow, deny nor halt.
func hookDecision(hook jsonvalue.Value) string {
	decision := textOnly(hook.Get("decision"))
	if !hook.Get("halt").IsEmpty() {
		decision = "halt"
	}
	if decision != "allow" && decision != "deny" && decision != "halt" {
		return ""
	}
	var hooks, chosen []jsonvalue.Value
	for _, entry := range parser.Elements(hook.Get("hooks")) {
		if !entry.IsObject() {
			continue
		}
		hooks = append(hooks, entry)
		if decision == "halt" && !entry.Get("halt").IsEmpty() ||
			decision != "halt" && isText(entry.Get("decision"), decision) {
			chosen = append(chosen, entry)
		}
	}
	if len(chosen) == 0 {
		chosen = hooks
	}
	var names []string
	for _, entry := range chosen {
		if name := textOnly(entry.Get("name")); name != "" {
			names = append(names, name)
		}
	}
	text := "permission decision: " + decision + " by hook"
	if len(names) > 0 {
		text += " " + strings.Join(names, ",")
	}
	if reason := textOnly(hook.Get("reason")); reason != "" {
		text += " reason=" + reason
	}
	return text
}
