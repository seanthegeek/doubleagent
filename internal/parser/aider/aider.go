// Package aider reads Aider's chat, input and LLM history files.
//
// It reads .aider.chat.history.md (the primary store) and
// .aider.input.history and .aider.llm.history (secondary sources), selected
// by file name wherever they sit: Aider writes them into the git root or
// working directory, so they usually arrive through the project catalog with
// the project as the home. Rows are labelled aider, and the project path is
// the directory holding the file.
//
// Formats read: the chat history's "# aider chat started at <local time>"
// headers, "#### " user lines, "> " tool and system output lines and
// assistant markdown, where a "> " run's announcement lines (Model:, Main
// model:, Editor model:, Weak model:) give the model rows and its
// confirm_ask lines the permission rows; the input history's prompt_toolkit
// FileHistory entries ("# <local time>" then "+<line>" lines); and the LLM
// history's "TO LLM <time>" and "LLM RESPONSE <time>" blocks of
// role-prefixed lines.
//
// Every timestamp is host local time with no zone and is emitted as if it
// were UTC. The session id is "<path of .aider.chat.history.md>#<header
// timestamp>"; input and LLM rows take the last header at or before their
// own time, and a chat user row takes the time of the matching input-history
// entry. No git branch or tool call id is recorded. See research/aider.md.
package aider

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The three file names.
const (
	chatName  = ".aider.chat.history.md"
	inputName = ".aider.input.history"
	llmName   = ".aider.llm.history"
)

// notSpace matches one character that is not whitespace by unicode.IsSpace,
// so a model name in an announcement line starts at its first visible
// character whatever spaces Aider's console padded it with.
const notSpace = `[^` + parser.Space + `]`

// Parser reads Aider's history files.
type Parser struct {
	parser.Base
	file, mainModel, editorModel, weakModel, confirm *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base: parser.NewBase("aider", "aider"),
		file: regexp.MustCompile(`\A(?:.*/)?(\.aider\.(?:chat\.history\.md|input\.history|llm\.history))\z`),
		// Announcement lines (get_announcements): the main model with its edit
		// format and options, the editor model in architect mode, the weak
		// model when it is not the main model.
		mainModel: regexp.MustCompile(`\A(?:Main model|Model): (` + notSpace +
			`.*?) with (` + notSpace + `+) edit format(?:, .*)?\z`),
		editorModel: regexp.MustCompile(`\AEditor model: (` + notSpace + `.*?) with (` + notSpace +
			`+) edit format\z`),
		weakModel: regexp.MustCompile(`\AWeak model: (` + notSpace + `.*)\z`),
		// A confirm_ask line: the question, its options, the default in
		// brackets, and the first letter of the answer, lower case.
		confirm: regexp.MustCompile(`\A(.+?) \(Y\)es/\(N\)o(?:/\(A\)ll)?(?:/\(S\)kip all)?` +
			`(?:/\(D\)on't ask again)? \[([^\]]*)\]: ([ynasd])\z`),
	}
}

// Wants reports whether the artifact is one of the three history files.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return p.file.MatchString(artifact.Rel)
}

// Secondary reports whether the artifact is the input or LLM history.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	match := p.file.FindStringSubmatch(artifact.Rel)
	return match != nil && match[1] != chatName
}

// baseRow returns a row labelled aider whose project is the file's
// directory.
func (p *Parser) baseRow(artifact *input.Artifact) timeline.Row {
	row := p.BaseRow(artifact)
	row.ProjectPath = projectDir(artifact.Original)
	return row
}

// Parse emits the rows of one history file.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	switch p.file.FindStringSubmatch(artifact.Rel)[1] {
	case chatName:
		p.parseChat(artifact, emit)
	case inputName:
		p.parseInput(artifact, emit)
	default:
		p.parseLLM(artifact, emit)
	}
	return nil
}

// separator returns the offset of the last "/" or backslash of a source
// path, -1 when there is none: the source host's separator is kept.
func separator(original string) int {
	return max(strings.LastIndex(original, "/"), strings.LastIndex(original, `\`))
}

// sibling returns the path of name beside original, in original's
// separator.
func sibling(original, name string) string {
	i := separator(original)
	if i < 0 {
		return name
	}
	return original[:i+1] + name
}

// projectDir returns the directory of a source path, in either separator.
func projectDir(original string) string {
	if i := separator(original); i > 0 {
		return original[:i]
	} else if i == 0 {
		return original[:1]
	}
	return ""
}

// InputEntry is one prompt_toolkit FileHistory entry: the line of its
// timestamp (of its first "+" line when it has no timestamp), the local
// timestamp, and the prompt text.
type InputEntry struct {
	Line      int
	Timestamp string
	Text      string
}

// ReadInputHistory returns one entry per FileHistory entry of the file at
// path, in file order. A non-blank line that is neither a timestamp nor a
// "+" entry line adds a problem with the line as raw text, and a file that
// cannot be read adds its unreadable problem. Other parsers that read an
// Aider history call it too.
func ReadInputHistory(path string, problems *parser.Problems) []InputEntry {
	var entries []InputEntry
	var texts [][]string
	for number, raw := range parser.Lines(path, problems) {
		line := parser.LineText(raw)
		switch {
		case isInputStamp(line):
			entries = append(entries, InputEntry{Line: number, Timestamp: inputStamp(line)})
			texts = append(texts, nil)
		case strings.HasPrefix(line, "+"):
			if len(entries) == 0 {
				entries = append(entries, InputEntry{Line: number})
				texts = append(texts, nil)
			}
			texts[len(texts)-1] = append(texts[len(texts)-1], line[1:])
		case strings.Trim(line, textlines.ASCIISpace) != "":
			problems.AddRaw(number, "unparseable line (not a FileHistory line)", line)
		}
	}
	for i := range entries {
		entries[i].Text = strings.Join(texts[i], "\n")
	}
	return entries
}

// isDigits reports whether text is only ASCII digits.
func isDigits(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}

// isStamp reports whether text starts with "YYYY-MM-DD<sep>HH:MM:SS".
func isStamp(text string, sep byte) bool {
	return len(text) >= 19 && isDigits(text[0:4]) && text[4] == '-' && isDigits(text[5:7]) &&
		text[7] == '-' && isDigits(text[8:10]) && text[10] == sep && isDigits(text[11:13]) &&
		text[13] == ':' && isDigits(text[14:16]) && text[16] == ':' && isDigits(text[17:19])
}

// stampEnd returns the offset after a stamp at the start of text, with an
// optional ".<digits>" fraction when fraction is set, and then only spaces
// and tabs to the end of text; -1 when text is not exactly that.
func stampEnd(text string, sep byte, fraction bool) int {
	if !isStamp(text, sep) {
		return -1
	}
	end := 19
	if fraction && len(text) > end && text[end] == '.' {
		digits := end + 1
		for digits < len(text) && text[digits] >= '0' && text[digits] <= '9' {
			digits++
		}
		if digits > end+1 {
			end = digits
		}
	}
	if strings.Trim(text[end:], " \t") != "" {
		return -1
	}
	return end
}

// isInputStamp reports whether a line is "# <local time>" of an input
// history.
func isInputStamp(line string) bool {
	return strings.HasPrefix(line, "# ") && stampEnd(line[2:], ' ', true) >= 0
}

// inputStamp returns the time of an input history stamp line.
func inputStamp(line string) string {
	rest := line[2:]
	return rest[:stampEnd(rest, ' ', true)]
}

// chatHeader returns the local time of an "# aider chat started at" line.
func chatHeader(line string) (string, bool) {
	const prefix = "# aider chat started at "
	if !strings.HasPrefix(line, prefix) || stampEnd(line[len(prefix):], ' ', false) < 0 {
		return "", false
	}
	return line[len(prefix) : len(prefix)+19], true
}

// readHeaders returns the session header times of the chat history at
// path, in file order. A file that is not a regular file or cannot be read
// gives what was read.
func readHeaders(path string) []string {
	if !parser.RegularFile(path) {
		return nil
	}
	var headers []string
	var ignored parser.Problems
	for _, raw := range parser.Lines(path, &ignored) {
		if header, ok := chatHeader(parser.LineText(raw)); ok {
			headers = append(headers, header)
		}
	}
	return headers
}

// sessionFor returns a function mapping a local time to the session of the
// chat history beside the artifact: the last header at or before the time.
// The chat history is a pre-scan: the parser reports its errors when it
// parses that file in full.
func sessionFor(artifact *input.Artifact) func(timestamp string) string {
	var headers []string
	if chatPath, ok := parser.Beside(artifact, chatName); ok {
		headers = readHeaders(chatPath)
	}
	slices.Sort(headers)
	chat := sibling(artifact.Original, chatName)
	return func(timestamp string) string {
		if timestamp == "" || len(headers) == 0 {
			return ""
		}
		key := strings.ReplaceAll(timestamp, "T", " ")
		key = key[:min(len(key), 19)]
		after := sort.Search(len(headers), func(i int) bool { return headers[i] > key })
		if after == 0 {
			return ""
		}
		return chat + "#" + headers[after-1]
	}
}

// parseInput emits one user row per input history entry.
func (p *Parser) parseInput(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	session := sessionFor(artifact)
	for _, entry := range ReadInputHistory(artifact.DiskPath, &problems) {
		row := p.baseRow(artifact)
		row.SourceLine = entry.Line
		row.TimestampUTC = timeutil.FromText(entry.Timestamp)
		row.SessionID = session(entry.Timestamp)
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(entry.Text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", projectDir(artifact.Original)))
}

// chatReader is the state of one walk over a chat history.
type chatReader struct {
	parser      *Parser
	artifact    *input.Artifact
	emit        parser.Emit
	headers     []string
	inputs      []InputEntry
	nextInput   int
	models      map[string]string
	sessionID   string
	header      string
	nextHeader  string
	headerCount int
	timestamp   string
	kind        string
	buffer      []string
	lineNumbers []int
	start       int
}

// parseChat emits the rows of a chat history: a system row per session
// header, a row per run of user, tool or assistant lines, and the model and
// permission rows of each tool run.
func (p *Parser) parseChat(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	// Pre-scans of this file and of the input history beside it: their errors
	// are reported when each file is parsed in full.
	reader := &chatReader{parser: p, artifact: artifact, emit: emit,
		headers: readHeaders(artifact.DiskPath), models: map[string]string{}}
	if inputPath, ok := parser.Beside(artifact, inputName); ok && parser.RegularFile(inputPath) {
		var ignored parser.Problems
		for _, entry := range ReadInputHistory(inputPath, &ignored) {
			if entry.Timestamp != "" {
				reader.inputs = append(reader.inputs, entry)
			}
		}
	}
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		reader.line(number, parser.LineText(raw))
	}
	reader.flush()
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", projectDir(artifact.Original)))
}

// line takes one line of the chat history.
func (c *chatReader) line(number int, line string) {
	if header, ok := chatHeader(line); ok {
		c.startSession(number, header)
		return
	}
	var kind, content string
	switch {
	case strings.HasPrefix(line, "#### "):
		kind, content = "user", line[5:]
	case strings.TrimRightFunc(line, unicode.IsSpace) == "####":
		kind = "user"
	case strings.HasPrefix(line, "> "):
		kind, content = "tool", line[2:]
	case line == ">":
		kind = "tool"
	default:
		kind, content = "assistant", line
	}
	if kind != c.kind {
		c.flush()
		c.kind, c.buffer, c.lineNumbers, c.start = kind, nil, nil, number
	}
	if !slices.ContainsFunc(c.buffer, func(held string) bool { return strings.TrimSpace(held) != "" }) {
		c.start = number
	}
	if kind != "assistant" {
		content = strings.TrimRightFunc(content, unicode.IsSpace)
	}
	c.buffer = append(c.buffer, content)
	c.lineNumbers = append(c.lineNumbers, number)
}

// startSession ends the run in progress and emits the session's header row.
func (c *chatReader) startSession(number int, header string) {
	c.flush()
	c.kind, c.buffer, c.lineNumbers = "", nil, nil
	c.models = map[string]string{}
	c.header = header
	c.headerCount++
	c.nextHeader = ""
	if c.headerCount < len(c.headers) {
		c.nextHeader = c.headers[c.headerCount]
	}
	c.timestamp = timeutil.FromText(header)
	c.sessionID = c.artifact.Original + "#" + header
	row := c.parser.baseRow(c.artifact)
	row.SourceLine = number
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.TurnType = timeline.TurnSystem
	row.Text = "aider chat started at " + header + " (local time)"
	c.emit(row)
}

// flush emits the run of lines held, with its model and permission rows.
func (c *chatReader) flush() {
	text := strings.TrimSpace(strings.Join(c.buffer, "\n"))
	if text == "" || c.kind == "" {
		return
	}
	if c.kind == "user" && c.header != "" {
		c.matchInput(text)
	}
	row := c.parser.baseRow(c.artifact)
	row.SourceLine = c.start
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.TurnType = map[string]string{"user": timeline.TurnUser, "tool": timeline.TurnToolResult,
		"assistant": timeline.TurnAssistant}[c.kind]
	row.Text = timeline.Compact(text)
	c.emit(row)
	if c.kind == "tool" {
		c.toolRows()
	}
}

// normalise returns text with each run of whitespace as one space.
func normalise(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// matchInput gives the run the time of the input history entry that holds
// the same prompt, searching the entries of the session from the one after
// the last match.
func (c *chatReader) matchInput(text string) {
	for j := c.nextInput; j < len(c.inputs); j++ {
		stamp := c.inputs[j].Timestamp[:19]
		if stamp < c.header {
			continue
		}
		if c.nextHeader != "" && stamp >= c.nextHeader {
			break
		}
		if normalise(c.inputs[j].Text) == normalise(text) {
			c.timestamp = timeutil.FromText(c.inputs[j].Timestamp)
			c.nextInput = j + 1
			break
		}
	}
}

// modelChange is one change of the session's models a line announces.
type modelChange struct {
	role, name, label string
	first             bool
}

// toolRows emits the "model:" and "permission decision:" rows of the
// blockquote run held, which follow the run's tool_result row.
func (c *chatReader) toolRows() {
	patterns := c.parser
	announced := slices.ContainsFunc(c.buffer, patterns.mainModel.MatchString)
	for i, line := range c.buffer {
		var changes []modelChange
		if announced {
			changes = patterns.announcement(line)
		}
		for _, change := range changes {
			old := c.models[change.role]
			c.models[change.role] = change.name
			if old != change.name && (old != "" || change.first) {
				text := "model: " + change.name
				if old != "" {
					text = "model: " + old + " -> " + change.name
				}
				row := c.systemRow(c.lineNumbers[i], text+change.label)
				row.Model = change.name
				c.emit(row)
			}
		}
		if len(changes) > 0 {
			continue
		}
		if match := patterns.confirm.FindStringSubmatch(line); match != nil {
			c.emit(c.systemRow(c.lineNumbers[i], decision(match)))
		}
	}
}

// announcement returns the model changes an announcement line makes.
func (p *Parser) announcement(line string) []modelChange {
	if match := p.mainModel.FindStringSubmatch(line); match != nil {
		changes := []modelChange{{"main", match[1], "", true}}
		if strings.HasPrefix(line, "Model: ") {
			// With no separate weak model the weak model is the main model, and
			// it is a row only when it replaces a separate one.
			changes = append(changes, modelChange{"weak", match[1], " (weak model)", false})
		}
		return changes
	}
	if match := p.editorModel.FindStringSubmatch(line); match != nil {
		return []modelChange{{"editor", match[1], " (editor model)", true}}
	}
	if match := p.weakModel.FindStringSubmatch(line); match != nil {
		return []modelChange{{"weak", match[1], " (weak model)", true}}
	}
	return nil
}

// decisions are the words of the answers to a confirmation.
var decisions = map[string]string{"y": "yes", "n": "no", "a": "all", "s": "skip all",
	"d": "no, don't ask again"}

// decision returns the text of a permission row from the groups of a
// confirmation line: question, default and answer.
func decision(match []string) string {
	answer := match[3]
	text := "permission decision: " + decisions[answer] + " (" + match[1] + ") answer=" +
		answer + " default=" + match[2]
	if answer == "y" || answer == "n" {
		text += "; may be automatic (--yes-always)"
	}
	return text
}

// systemRow returns a system row of the session in progress.
func (c *chatReader) systemRow(number int, text string) timeline.Row {
	row := c.parser.baseRow(c.artifact)
	row.SourceLine = number
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.TurnType = timeline.TurnSystem
	row.Text = text
	return row
}

// llmBlock is one "TO LLM" or "LLM RESPONSE" block of the LLM history.
type llmBlock struct {
	kind, stamp string
	line        int
	lines       []string
}

// llmMarker returns the kind and time of a block marker line.
func llmMarker(line string) (kind, stamp string, ok bool) {
	for _, kind := range []string{"TO LLM ", "LLM RESPONSE "} {
		rest, found := strings.CutPrefix(line, kind)
		if found && stampEnd(rest, 'T', false) >= 0 {
			return strings.TrimSuffix(kind, " "), rest[:19], true
		}
	}
	return "", "", false
}

// parseLLM emits one row per block of the LLM history.
func (p *Parser) parseLLM(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	session := sessionFor(artifact)
	var block *llmBlock
	finish := func() {
		if block != nil {
			p.llmRow(artifact, block, session, emit)
		}
	}
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		line := parser.LineText(raw)
		if kind, stamp, ok := llmMarker(line); ok {
			finish()
			block = &llmBlock{kind: kind, stamp: stamp, line: number}
		} else if block != nil {
			block.lines = append(block.lines, line)
		} else if strings.Trim(line, textlines.ASCIISpace) != "" {
			problems.AddRaw(number,
				"unparseable line (text before the first TO LLM / LLM RESPONSE marker)", line)
		}
	}
	finish()
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", projectDir(artifact.Original)))
}

// llmRow emits the row of one block: the response text, or a summary of the
// messages sent.
func (p *Parser) llmRow(artifact *input.Artifact, block *llmBlock,
	session func(string) string, emit parser.Emit) {
	row := p.baseRow(artifact)
	row.SourceLine = block.line
	row.TimestampUTC = timeutil.FromText(block.stamp)
	row.SessionID = session(block.stamp)
	if block.kind == "LLM RESPONSE" {
		stripped := make([]string, len(block.lines))
		for i, line := range block.lines {
			stripped[i] = stripRole(line, "ASSISTANT")
		}
		text := strings.TrimSpace(strings.Join(stripped, "\n"))
		if text == "" {
			return
		}
		row.TurnType = timeline.TurnAssistant
		row.Text = timeline.Compact(text)
		emit(row)
		return
	}
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(summarise(block.lines))
	emit(row)
}

// message is one role's message of a "TO LLM" block.
type message struct {
	role string
	body []string
}

// summarise returns "TO LLM: <n> message(s) (<role> <count>, ...); last user
// message: <text>" for the lines of a request block, whose messages are
// separated by lines of seven dashes and start with the role's name.
func summarise(lines []string) string {
	var messages []*message
	for _, line := range lines {
		if line == "-------" {
			messages = append(messages, &message{})
		} else if len(messages) > 0 {
			current := messages[len(messages)-1]
			if current.role == "" {
				current.role, _, _ = strings.Cut(line, " ")
			}
			current.body = append(current.body, stripRole(line, current.role))
		}
	}
	var roles []string
	counts := map[string]int{}
	lastUser := ""
	for _, each := range messages {
		if counts[each.role] == 0 {
			roles = append(roles, each.role)
		}
		counts[each.role]++
		if each.role == "USER" {
			lastUser = strings.Join(each.body, "\n")
		}
	}
	var listed []string
	for _, role := range roles {
		listed = append(listed, role+" "+strconv.Itoa(counts[role]))
	}
	return "TO LLM: " + strconv.Itoa(len(messages)) + " message(s) (" + strings.Join(listed, ", ") +
		"); last user message: " + lastUser
}

// stripRole returns line without the role name that starts it.
func stripRole(line, role string) string {
	if line == role {
		return ""
	}
	if role != "" && strings.HasPrefix(line, role+" ") {
		return line[len(role)+1:]
	}
	return line
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
