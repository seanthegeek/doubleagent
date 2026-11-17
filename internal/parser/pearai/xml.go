package pearai

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// space is one character Python's \s matches.
const space = `[` + parser.Space + `]`

// The texts of a Roo 3.15 user message. A tool result opens with
// "[<tool> ...] Result:" (Python's \b after the tool name is a character
// that is not a letter, digit or underscore in any script), a skipped
// call's notice with "Skipping tool [<tool>" or "Tool [<tool>", and the
// first message wraps the task in <task> tags.
var (
	resultPattern = regexp.MustCompile(`(?s)^\[([a-z_]+)(?:[^\p{L}\p{N}_].*)?\] Result:\z`)
	noticePattern = regexp.MustCompile(`^(?:Skipping tool|Tool) \[([a-z_]+)`)
	taskPattern   = regexp.MustCompile(`(?s)^` + space + `*<task>` + space + `*(.*?)` + space +
		`*</task>` + space + `*\z`)
	roleBlock = regexp.MustCompile(`(?m)^<(system|user|assistant)>\n`)
)

// toolNames are the tools of Roo 3.15 whose XML tags an assistant message
// holds, after the thinking tag.
func toolNames() []string {
	return []string{"thinking", "execute_command", "read_file", "write_to_file", "apply_diff",
		"insert_content", "search_and_replace", "search_files", "list_files",
		"list_code_definition_names", "browser_action", "use_mcp_tool", "access_mcp_resource",
		"ask_followup_question", "attempt_completion", "switch_mode", "new_task", "fetch_instructions"}
}

// tagAt returns the tool or thinking tag whose opening "<name>" starts at
// the offset of text, or "".
func tagAt(text string, offset int) string {
	for _, name := range toolNames() {
		if strings.HasPrefix(text[offset:], "<"+name+">") {
			return name
		}
	}
	return ""
}

// element is one XML element of a text: its tag, its body, and the offsets
// where it starts and ends.
type element struct {
	tag, body  string
	start, end int
}

// nextElement returns the first element at or after offset whose tag
// tagOf names, its body running to its closing tag or to the end of the
// text. It stands for the Python pattern `<(<tags>)>(.*?)(?:</\1>|$)`,
// whose backreference Go's engine has no form for; where Python's `$`
// stops before a final LF, the body here keeps it, which every caller
// strips.
func nextElement(text string, offset int, tagOf func(string, int) string) (element, bool) {
	for {
		found := strings.IndexByte(text[offset:], '<')
		if found < 0 {
			return element{}, false
		}
		start := offset + found
		tag := tagOf(text, start)
		if tag == "" {
			offset = start + 1
			continue
		}
		bodyStart := start + len(tag) + 2
		closing := "</" + tag + ">"
		if end := strings.Index(text[bodyStart:], closing); end >= 0 {
			return element{tag: tag, body: text[bodyStart : bodyStart+end], start: start,
				end: bodyStart + end + len(closing)}, true
		}
		return element{tag: tag, body: text[bodyStart:], start: start, end: len(text)}, true
	}
}

// paramAt returns the lower-case name of a parameter tag "<name>" at the
// offset of text, or "".
func paramAt(text string, offset int) string {
	end := offset + 1
	for end < len(text) && (text[end] == '_' || ('a' <= text[end] && text[end] <= 'z')) {
		end++
	}
	if end == offset+1 || end >= len(text) || text[end] != '>' {
		return ""
	}
	return text[offset+1 : end]
}

// xmlBlocks returns the rows of Roo assistant text with XML tool tags: the
// text between tags as assistant rows, a thinking tag as a thinking row
// when kept, and a tool tag as a tool_use row of its parameters' values
// joined by " | ", or of its body when it has none.
func xmlBlocks(text string, includeThinking bool) []clinelegacy.ContentRow {
	var rows []clinelegacy.ContentRow
	position := 0
	for {
		block, ok := nextElement(text, position, tagAt)
		if !ok {
			break
		}
		if before := strings.TrimSpace(text[position:block.start]); before != "" {
			rows = append(rows, clinelegacy.ContentRow{TurnType: timeline.TurnAssistant, Text: before})
		}
		position = block.end
		if block.tag == "thinking" {
			if body := strings.TrimSpace(block.body); includeThinking && body != "" {
				rows = append(rows, clinelegacy.ContentRow{TurnType: timeline.TurnThinking, Text: body})
			}
			continue
		}
		rows = append(rows, clinelegacy.ContentRow{TurnType: timeline.TurnToolUse,
			Text: toolText(block.body), ToolName: block.tag})
	}
	if rest := strings.TrimSpace(text[position:]); rest != "" {
		rows = append(rows, clinelegacy.ContentRow{TurnType: timeline.TurnAssistant, Text: rest})
	}
	return rows
}

// toolText returns the text of a tool tag's body: its parameters' values
// joined by " | ", or the body itself when it has none.
func toolText(body string) string {
	var values []string
	for offset := 0; ; {
		param, ok := nextElement(body, offset, paramAt)
		if !ok {
			break
		}
		offset = param.end
		if value := strings.TrimSpace(param.body); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return strings.TrimSpace(body)
	}
	return strings.Join(values, " | ")
}

// promptTurn returns the turn type and text of a logged prompt: for a
// prompt of <system>, <user> and <assistant> blocks, its last user block;
// any other prompt is a system row "model request: <prompt>".
func promptTurn(prompt string) (string, string) {
	blocks := roleBlock.FindAllStringSubmatchIndex(prompt, -1)
	last := -1
	for i, block := range blocks {
		if prompt[block[2]:block[3]] == "user" {
			last = i
		}
	}
	if len(blocks) == 0 || blocks[0][0] != 0 || last < 0 {
		return timeline.TurnSystem, "model request: " + prompt
	}
	end := len(prompt)
	if last+1 < len(blocks) {
		end = blocks[last+1][0]
	}
	return timeline.TurnUser, prompt[blocks[last][1]:end]
}

// APIContentRows returns the rows of one API history message: assistant
// text split at its XML tool tags, each call's id its message's position
// (with ".<n>" after the first); and in a user message each "[<tool> ...]
// Result:" block as a tool_result row followed by the permission decision
// its text records.
func (p *Parser) APIContentRows(message jsonvalue.Value, position int, baseTurn string,
	includeThinking bool) []clinelegacy.ContentRow {
	content := message.Get("content")
	if baseTurn == timeline.TurnAssistant {
		return assistantRows(content, position, includeThinking)
	}
	if baseTurn != timeline.TurnUser || !content.IsArray() {
		return clinelegacy.ContentRows(content, baseTurn, includeThinking)
	}
	results := &resultRows{callID: strconv.Itoa(position - 1)}
	for _, block := range content.Items() {
		results.block(block, includeThinking)
	}
	results.flush()
	return results.rows
}

// assistantRows returns an assistant message's rows with its text split at
// the XML tool tags.
func assistantRows(content jsonvalue.Value, position int, includeThinking bool) []clinelegacy.ContentRow {
	var rows []clinelegacy.ContentRow
	tools := 0
	for _, row := range clinelegacy.ContentRows(content, timeline.TurnAssistant, includeThinking) {
		if row.TurnType != timeline.TurnAssistant {
			rows = append(rows, row)
			continue
		}
		for _, block := range xmlBlocks(row.Text, includeThinking) {
			if block.TurnType == timeline.TurnToolUse {
				tools++
				block.ToolUseID = strconv.Itoa(position)
				if tools > 1 {
					block.ToolUseID += "." + strconv.Itoa(tools)
				}
			}
			rows = append(rows, block)
		}
	}
	return rows
}

// resultRows gathers the rows of a Roo 3.15 user message, whose text
// blocks after a "[<tool> ...] Result:" block (or a skipped call's notice)
// are that call's result until the next one or the environment details.
type resultRows struct {
	rows []clinelegacy.ContentRow
	// callID is the id of the calls the results answer: the position of
	// the assistant message before.
	callID string
	// inResult is set while blocks belong to a result, whose parts, tool
	// name and decision follow.
	inResult bool
	parts    []string
	name     string
	decision string
}

// flush adds the result being gathered, then the permission decision its
// text records.
func (r *resultRows) flush() {
	if !r.inResult {
		return
	}
	var kept []string
	for _, part := range r.parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	r.rows = append(r.rows, clinelegacy.ContentRow{TurnType: timeline.TurnToolResult,
		Text: strings.Join(kept, "\n"), ToolName: r.name, ToolUseID: r.callID})
	if r.decision != "" {
		r.rows = append(r.rows, clinelegacy.ContentRow{TurnType: timeline.TurnSystem,
			Text: "permission decision: " + r.decision, ToolName: r.name, ToolUseID: r.callID})
	}
}

// block reads one content block of the message.
func (r *resultRows) block(block jsonvalue.Value, includeThinking bool) {
	text := blockText(block)
	if !text.IsString() {
		rows := clinelegacy.ContentRows(jsonvalue.NewArray(block), timeline.TurnUser, includeThinking)
		if !r.inResult {
			r.rows = append(r.rows, rows...)
			return
		}
		for _, row := range rows {
			r.parts = append(r.parts, row.Text)
		}
		return
	}
	stripped := strings.TrimSpace(text.StringValue())
	result := resultPattern.FindStringSubmatch(stripped)
	notice := noticePattern.FindStringSubmatch(stripped)
	switch {
	case result != nil || notice != nil:
		r.flush()
		r.inResult, r.parts, r.decision = true, nil, ""
		if result != nil {
			r.name = result[1]
		} else {
			r.name, r.parts = notice[1], []string{stripped}
		}
	case strings.HasPrefix(strings.TrimLeftFunc(text.StringValue(), unicode.IsSpace), "<environment_details>"):
		r.flush()
		r.inResult = false
		r.rows = append(r.rows, clinelegacy.ContentRow{TurnType: timeline.TurnSystem, Text: stripped})
	case r.inResult:
		if len(r.parts) == 0 && r.decision == "" {
			r.decision = permissionDecision(text.StringValue())
		}
		r.parts = append(r.parts, text.StringValue())
	default:
		r.userText(text.StringValue())
	}
}

// userText adds a text block outside any result as a user row, the task
// out of its <task> tags.
func (r *resultRows) userText(text string) {
	if task := taskPattern.FindStringSubmatch(text); task != nil {
		text = task[1]
	}
	if strings.TrimSpace(text) != "" {
		r.rows = append(r.rows, clinelegacy.ContentRow{TurnType: timeline.TurnUser, Text: text})
	}
}
