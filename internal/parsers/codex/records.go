package codex

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
func callSummary(payload jsonvalue.Value) string {
	switch {
	case parserkit.IsText(payload.Get("type"), "local_shell_call"):
		action := objectOrEmpty(payload.Get("action"))
		if command := action.Get("command"); command.IsArray() {
			return joinedCommand(command)
		}
		return cmp.Or(parserkit.Text(action.Get("command")), parserkit.JSONText(action))
	case parserkit.IsText(payload.Get("type"), "web_search_call"):
		action := objectOrEmpty(payload.Get("action"))
		return cmp.Or(parserkit.Text(action.Get("query")), parserkit.JSONText(action))
	}
	for _, key := range []string{"arguments", "input"} {
		value := payload.Get(key)
		switch {
		case !parserkit.Present(value):
		case value.IsString():
			return value.StringValue()
		default:
			return parserkit.JSONText(value)
		}
	}
	return parserkit.JSONText(without(payload, "type", "id", "call_id", "name", "status"))
}

// objectOrEmpty returns value, or an empty object when value is empty
// (jsonvalue.Value.IsEmpty).
func objectOrEmpty(value jsonvalue.Value) jsonvalue.Value {
	if !value.IsEmpty() {
		return value
	}
	return jsonvalue.NewObject()
}

// callName returns the tool name of a call: shell, web_search, or the
// call's own name, else its type.
func callName(payload jsonvalue.Value) string {
	switch {
	case parserkit.IsText(payload.Get("type"), "local_shell_call"):
		return "shell"
	case parserkit.IsText(payload.Get("type"), "web_search_call"):
		return "web_search"
	}
	return parserkit.Text(parserkit.FirstNonEmpty(payload.Get("name"), payload.Get("type")))
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
	body := strings.ToLower(strings.TrimSpace(text))
	for _, marker := range contextMarkers {
		if strings.HasPrefix(body, strings.ToLower(marker[1])) &&
			strings.HasSuffix(body, strings.ToLower(marker[2])) {
			return marker[0]
		}
	}
	return ""
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
func userParts(payload jsonvalue.Value, work parserkit.Work) []userPart {
	content := payload.Get("content")
	if content.IsString() {
		content = jsonvalue.NewArray(jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("input_text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	if !content.IsArray() {
		if text := parserkit.TextOf(content); text != "" {
			return []userPart{{text: text}}
		}
		return nil
	}
	kinds := payload.Get("internal_chat_message_metadata_passthrough").Get("content_item_kinds")
	useKinds := kinds.IsArray() && kinds.Len() == content.Len()
	var blocks []userPart
	for i, block := range content.Items() {
		text := parserkit.TextOf(jsonvalue.NewArray(block))
		if text == "" {
			continue
		}
		var key string
		if useKinds {
			key = contentKind(kinds.Index(i))
		} else {
			key = contextMarker(text)
		}
		blocks = append(blocks, userPart{kind: key, text: text})
	}
	return joinParts(blocks, work)
}

// joinParts merges consecutive blocks of one kind into one part, joined by
// LF. Each part's texts are joined once, so a message of many blocks costs
// work in proportion to its length; work counts the joins and their bytes.
func joinParts(blocks []userPart, work parserkit.Work) []userPart {
	var parts []userPart
	for start := 0; start < len(blocks); {
		end := start + 1
		for end < len(blocks) && blocks[end].kind == blocks[start].kind {
			end++
		}
		texts := make([]string, 0, end-start)
		for _, block := range blocks[start:end] {
			texts = append(texts, block.text)
		}
		joined := strings.Join(texts, "\n")
		work.Add("joins", 1)
		work.Add("bytes joined", len(joined))
		parts = append(parts, userPart{kind: blocks[start].kind, text: joined})
		start = end
	}
	return parts
}

// contentKind returns the run kind of a content_item_kinds entry: "" for the
// person's input, else the kind.
func contentKind(value jsonvalue.Value) string {
	kind := parserkit.Text(value)
	if strings.HasPrefix(kind, "user.") || kind == "" || kind == "unknown" {
		return ""
	}
	return kind
}

// parentOf returns the parent thread a subagent's session_meta names, or
// "": parent_thread_id, else source.subagent.thread_spawn.parent_thread_id,
// else a root session_id other than the id when the thread is not a fork.
func parentOf(metadata jsonvalue.Value) string {
	if parent := metadata.Get("parent_thread_id"); !parent.IsEmpty() {
		return parent.Text()
	}
	spawn := metadata.Get("source").Get("subagent").Get("thread_spawn")
	if parent := spawn.Get("parent_thread_id"); !parent.IsEmpty() {
		return parent.Text()
	}
	root, threadID := metadata.Get("session_id"), metadata.Get("id")
	if !root.IsEmpty() && !threadID.IsEmpty() && !sameValue(root, threadID) &&
		metadata.Get("forked_from_id").IsEmpty() {
		return root.Text()
	}
	return ""
}

// isSubagentMeta reports whether a session_meta is a subagent thread's.
func isSubagentMeta(metadata jsonvalue.Value) bool {
	return parserkit.IsText(metadata.Get("thread_source"), "subagent") ||
		metadata.Get("source").Has("subagent")
}

// isGuardian reports whether a session_meta is a guardian review thread's:
// thread_source guardian_review, or a source.subagent.other of guardian.
func isGuardian(metadata jsonvalue.Value) bool {
	return parserkit.IsText(metadata.Get("thread_source"), "guardian_review") ||
		parserkit.IsText(metadata.Get("source").Get("subagent").Get("other"), "guardian")
}

// prescan returns the ids every response item carries, as id or call_id,
// and the text of each failed exit code (exitText) of a completed
// CommandExecution item by its id, which is the call's call_id.
func prescan(records []numberedRecord) (map[string]bool, map[string]string) {
	carried := map[string]bool{}
	exitCodes := map[string]string{}
	for _, record := range records {
		payload := record.value.Get("payload")
		if !payload.IsObject() {
			continue
		}
		switch {
		case parserkit.IsText(record.value.Get("type"), "response_item"):
			for _, key := range []string{"id", "call_id"} {
				if value := payload.Get(key); !value.IsEmpty() {
					carried[value.Text()] = true
				}
			}
		case parserkit.IsText(record.value.Get("type"), "event_msg") &&
			parserkit.IsText(payload.Get("type"), "item_completed"):
			item := payload.Get("item")
			if parserkit.IsText(item.Get("type"), "CommandExecution") && !item.Get("exit_code").IsNull() {
				exitCodes[parserkit.Text(item.Get("id"))] = exitText(item.Get("exit_code"))
			}
		}
	}
	delete(carried, "")
	return carried, exitCodes
}

// toolNames returns the names of the tools a tool search found, each
// tool's name or else its JSON, joined by ", ".
func toolNames(tools jsonvalue.Value) string {
	if !tools.IsArray() {
		return parserkit.JSONText(tools)
	}
	names := make([]string, 0, tools.Len())
	for _, tool := range tools.Items() {
		if name := tool.Get("name"); !name.IsEmpty() {
			names = append(names, name.Text())
		} else {
			names = append(names, parserkit.JSONText(tool))
		}
	}
	return strings.Join(names, ", ")
}

// agentLabel returns the "agent message <author> -> <recipients>: " start
// of a message between agents.
func agentLabel(payload jsonvalue.Value) string {
	recipients := []string{parserkit.Text(payload.Get("recipient"))}
	if others := payload.Get("other_recipients"); others.IsArray() {
		for _, recipient := range others.Items() {
			recipients = append(recipients, recipient.Text())
		}
	}
	return "agent message " + parserkit.Text(payload.Get("author")) + " -> " +
		strings.Join(recipients, ", ") + ": "
}

// agentText returns the text of an agent message, marked [encrypted] when a
// block of it is encrypted.
func agentText(payload jsonvalue.Value) string {
	content := payload.Get("content")
	text := parserkit.TextOf(content)
	for _, block := range content.Items() {
		if parserkit.IsText(block.Get("type"), "encrypted_content") {
			return strings.TrimSpace(text + " [encrypted]")
		}
	}
	return text
}

// goalText returns the "thread goal:" text of a thread_goal_updated event,
// whose keys are camelCase.
func goalText(payload jsonvalue.Value) string {
	goal := objectOr(payload.Get("goal"))
	return "thread goal: status=" + cmp.Or(parserkit.Text(goal.Get("status")), "-") + " turn=" +
		cmp.Or(parserkit.Text(payload.Get("turnId")), "-") + " objective: " + parserkit.Text(goal.Get("objective"))
}

// guardianVerdict returns the verdict a guardian review's message holds: a
// JSON object, in a string, with a true outcome; false otherwise.
func guardianVerdict(artifact *input.Artifact, message string) (jsonvalue.Value, bool) {
	verdict, err := parserkit.DecodeJSON(artifact, message)
	if err != nil || verdict.Get("outcome").IsEmpty() {
		return jsonvalue.NewNull(), false
	}
	return verdict, true
}

// guardianText returns the "permission decision:" text of a guardian
// review's verdict on call, by reviewer.
func guardianText(verdict jsonvalue.Value, call, reviewer string) string {
	return "permission decision: " + verdict.Get("outcome").Text() + " (auto_review risk_level=" +
		cmp.Or(parserkit.Text(verdict.Get("risk_level")), "-") + " user_authorization=" +
		cmp.Or(parserkit.Text(verdict.Get("user_authorization")), "-") + ") exec " + call + " reviewer " +
		reviewer + ": " + parserkit.Text(verdict.Get("rationale"))
}
