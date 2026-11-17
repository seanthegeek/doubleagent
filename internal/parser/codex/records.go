package codex

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// callTypes and outputTypes are the response items of a tool call and of
// its result.
var (
	callTypes   = []string{"function_call", "custom_tool_call", "local_shell_call", "web_search_call"}
	outputTypes = []string{"function_call_output", "custom_tool_call_output",
		"local_shell_call_output"}
)

// callSummary returns the text of a tool call: its command, its query, its
// arguments or input, or else the call's other fields as JSON.
func callSummary(payload pyjson.Value) string {
	switch {
	case isText(payload.Get("type"), "local_shell_call"):
		action := objectOrEmpty(payload.Get("action"))
		if command := action.Get("command"); command.IsArray() {
			return joinedCommand(command)
		}
		return strOr(action.Get("command"), parser.CompactJSON(action))
	case isText(payload.Get("type"), "web_search_call"):
		action := objectOrEmpty(payload.Get("action"))
		return strOr(action.Get("query"), parser.CompactJSON(action))
	}
	for _, key := range []string{"arguments", "input"} {
		value := payload.Get(key)
		switch {
		case isNoneOrEmpty(value):
		case value.IsString():
			return value.Text()
		default:
			return parser.CompactJSON(value)
		}
	}
	return parser.CompactJSON(without(payload, "type", "id", "call_id", "name", "status"))
}

// objectOrEmpty returns value, or an empty object when Python counts it as
// false: the form `value or {}`.
func objectOrEmpty(value pyjson.Value) pyjson.Value {
	if value.Truthy() {
		return value
	}
	return pyjson.NewObject()
}

// callName returns the tool name of a call: shell, web_search, or the
// call's own name, else its type.
func callName(payload pyjson.Value) string {
	switch {
	case isText(payload.Get("type"), "local_shell_call"):
		return "shell"
	case isText(payload.Get("type"), "web_search_call"):
		return "web_search"
	}
	return parser.Str(firstTruthy(payload.Get("name"), payload.Get("type")))
}

// contextMarkers are the wrappers the harness puts around the user-role
// context it injects, as name, start and end; Codex recognises the same
// blocks (research/codex-cli.md, Parser plan). They classify the blocks of
// rollouts that carry no content kinds.
var contextMarkers = [][3]string{
	{"agents_md_instructions", "# AGENTS.md instructions", "</INSTRUCTIONS>"},
	{"environment_context", "<environment_context>", "</environment_context>"},
	{"agent_message_board_notification", "<agent_message_board_notification>",
		"</agent_message_board_notification>"},
	{"skill", "<skill>", "</skill>"},
	{"user_shell_command", "<user_shell_command>", "</user_shell_command>"},
	{"turn_aborted", "<turn_aborted>", "</turn_aborted>"},
	{"subagent_notification", "<subagent_notification>", "</subagent_notification>"},
	{"recommended_plugins", "<recommended_plugins>", "</recommended_plugins>"},
}

// contextMarker returns the name of the harness wrapper text is, or "": the
// text, stripped, starts with the open marker and ends with the close
// marker, ignoring case.
func contextMarker(text string) string {
	body := lowerForMarkers(pytext.Strip(text))
	for _, marker := range contextMarkers {
		if strings.HasPrefix(body, strings.ToLower(marker[1])) &&
			strings.HasSuffix(body, strings.ToLower(marker[2])) {
			return marker[0]
		}
	}
	return ""
}

// lowerForMarkers lowers text as far as a comparison with the ASCII markers
// can tell from Python's str.lower(): ASCII letters, the Kelvin sign, the one
// other character that lowers to an ASCII letter, and the capital I with dot,
// which lowers to two characters. Every other character stays, and matches
// no marker either way.
func lowerForMarkers(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for _, character := range text {
		switch {
		case character >= 'A' && character <= 'Z':
			out.WriteRune(character + 'a' - 'A')
		case character == 'K':
			out.WriteByte('k')
		case character == 'İ':
			out.WriteString("i̇")
		default:
			out.WriteRune(character)
		}
	}
	return out.String()
}

// userPart is one run of a user-role message: kind is "" for input the
// person supplied, else the context kind or marker name.
type userPart struct {
	kind string
	text string
}

// userParts returns the runs of a user-role message, consecutive blocks of
// one kind joined by LF. The kind comes from the content_item_kinds list
// when it is as long as the content (user.* and unknown are the person's),
// else from the block's wrapper (contextMarker).
func userParts(payload pyjson.Value) []userPart {
	content := payload.Get("content")
	if content.IsString() {
		content = pyjson.NewArray(pyjson.NewObject(
			pyjson.Member{Key: "type", Value: pyjson.NewString("input_text")},
			pyjson.Member{Key: "text", Value: content}))
	}
	if !content.IsArray() {
		if text := parser.TextOf(content); text != "" {
			return []userPart{{text: text}}
		}
		return nil
	}
	kinds := payload.Get("internal_chat_message_metadata_passthrough").Get("content_item_kinds")
	useKinds := kinds.IsArray() && kinds.Len() == content.Len()
	var parts []userPart
	for i, block := range content.Items() {
		text := parser.TextOf(pyjson.NewArray(block))
		if text == "" {
			continue
		}
		var key string
		if useKinds {
			key = contentKind(kinds.Index(i))
		} else {
			key = contextMarker(text)
		}
		if last := len(parts) - 1; last >= 0 && parts[last].kind == key {
			parts[last].text += "\n" + text
		} else {
			parts = append(parts, userPart{kind: key, text: text})
		}
	}
	return parts
}

// contentKind returns the run kind of a content_item_kinds entry: "" for the
// person's input, else the kind.
func contentKind(value pyjson.Value) string {
	kind := parser.Str(value)
	if strings.HasPrefix(kind, "user.") || kind == "" || kind == "unknown" {
		return ""
	}
	return kind
}

// parentOf returns the parent thread a subagent's session_meta names, or
// "": parent_thread_id, else source.subagent.thread_spawn.parent_thread_id,
// else a root session_id other than the id when the thread is not a fork.
func parentOf(metadata pyjson.Value) string {
	if parent := metadata.Get("parent_thread_id"); parent.Truthy() {
		return parent.Str()
	}
	spawn := metadata.Get("source").Get("subagent").Get("thread_spawn")
	if parent := spawn.Get("parent_thread_id"); parent.Truthy() {
		return parent.Str()
	}
	root, threadID := metadata.Get("session_id"), metadata.Get("id")
	if root.Truthy() && threadID.Truthy() && !sameValue(root, threadID) &&
		!metadata.Get("forked_from_id").Truthy() {
		return root.Str()
	}
	return ""
}

// isSubagentMeta reports whether a session_meta is a subagent thread's.
func isSubagentMeta(metadata pyjson.Value) bool {
	return isText(metadata.Get("thread_source"), "subagent") ||
		metadata.Get("source").Has("subagent")
}

// isGuardian reports whether a session_meta is a guardian review thread's:
// thread_source guardian_review, or a source.subagent.other of guardian.
func isGuardian(metadata pyjson.Value) bool {
	return isText(metadata.Get("thread_source"), "guardian_review") ||
		isText(metadata.Get("source").Get("subagent").Get("other"), "guardian")
}

// prescan returns the ids every response item carries, as id or call_id,
// and the exit code of each completed CommandExecution item by its id,
// which is the call's call_id.
func prescan(records []numberedRecord) (map[string]bool, map[string]pyjson.Value) {
	carried := map[string]bool{}
	exitCodes := map[string]pyjson.Value{}
	for _, record := range records {
		payload := record.value.Get("payload")
		if !payload.IsObject() {
			continue
		}
		switch {
		case isText(record.value.Get("type"), "response_item"):
			for _, key := range []string{"id", "call_id"} {
				if value := payload.Get(key); value.Truthy() {
					carried[value.Str()] = true
				}
			}
		case isText(record.value.Get("type"), "event_msg") &&
			isText(payload.Get("type"), "item_completed"):
			item := payload.Get("item")
			if isText(item.Get("type"), "CommandExecution") && !item.Get("exit_code").IsNull() {
				exitCodes[parser.Str(item.Get("id"))] = item.Get("exit_code")
			}
		}
	}
	delete(carried, "")
	return carried, exitCodes
}

// toolNames returns the names of the tools a tool search found, each
// tool's name or else its JSON, joined by ", ".
func toolNames(tools pyjson.Value) string {
	if !tools.IsArray() {
		return parser.CompactJSON(tools)
	}
	names := make([]string, 0, tools.Len())
	for _, tool := range tools.Items() {
		if name := tool.Get("name"); name.Truthy() {
			names = append(names, name.Str())
		} else {
			names = append(names, parser.CompactJSON(tool))
		}
	}
	return strings.Join(names, ", ")
}

// agentLabel returns the "agent message <author> -> <recipients>: " start
// of a message between agents.
func agentLabel(payload pyjson.Value) string {
	recipients := []string{parser.Str(payload.Get("recipient"))}
	if others := payload.Get("other_recipients"); others.IsArray() {
		for _, recipient := range others.Items() {
			recipients = append(recipients, recipient.Str())
		}
	}
	return "agent message " + parser.Str(payload.Get("author")) + " -> " +
		strings.Join(recipients, ", ") + ": "
}

// agentText returns the text of an agent message, marked [encrypted] when a
// block of it is encrypted.
func agentText(payload pyjson.Value) string {
	content := payload.Get("content")
	text := parser.TextOf(content)
	for _, block := range content.Items() {
		if isText(block.Get("type"), "encrypted_content") {
			return pytext.Strip(text + " [encrypted]")
		}
	}
	return text
}

// goalText returns the "thread goal:" text of a thread_goal_updated event,
// whose keys are camelCase.
func goalText(payload pyjson.Value) string {
	goal := objectOr(payload.Get("goal"))
	return "thread goal: status=" + strOr(goal.Get("status"), "-") + " turn=" +
		strOr(payload.Get("turnId"), "-") + " objective: " + parser.Str(goal.Get("objective"))
}

// guardianVerdict returns the verdict a guardian review's message holds: a
// JSON object, in a string, with a true outcome; false otherwise.
func guardianVerdict(message string) (pyjson.Value, bool) {
	verdict, err := pyjson.DecodeString(message)
	if err != nil || !verdict.Get("outcome").Truthy() {
		return pyjson.NewNull(), false
	}
	return verdict, true
}

// guardianText returns the "permission decision:" text of a guardian
// review's verdict on call, by reviewer.
func guardianText(verdict pyjson.Value, call, reviewer string) string {
	return "permission decision: " + verdict.Get("outcome").Str() + " (auto_review risk_level=" +
		strOr(verdict.Get("risk_level"), "-") + " user_authorization=" +
		strOr(verdict.Get("user_authorization"), "-") + ") exec " + call + " reviewer " +
		reviewer + ": " + parser.Str(verdict.Get("rationale"))
}
