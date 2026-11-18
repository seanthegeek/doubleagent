package kilocode

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/opencode"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// historyItem returns a pre-fork task's history item: history_item.json in
// its directory, else its entry in the tasks' _index.json, else an empty
// object. The task directory is the artifact's own. Either file that is
// there but cannot be read adds a problem.
func (p *Parser) historyItem(artifact *input.Artifact, taskID string, problems *parser.Problems) jsonvalue.Value {
	item := jsonvalue.NewNull()
	if itemPath, ok := parser.Beside(artifact, "history_item.json"); ok && parser.RegularFile(itemPath) {
		item, _ = readJSON(itemPath, problems)
	}
	if item.IsObject() {
		return item
	}
	if indexPath, found := parser.Above(artifact, 1, "_index.json"); found && parser.RegularFile(indexPath) {
		if entry, ok := p.indexEntry(indexPath, taskID, problems); ok {
			return entry
		}
	}
	return jsonvalue.NewObject()
}

// indexKey identifies one state of a tasks' _index.json: its path,
// modification time in nanoseconds and size.
type indexKey struct {
	path        string
	mtime, size int64
}

// taskIndex is a tasks' _index.json read once: the first object entry of
// each string id, and the problems reading the file added.
type taskIndex struct {
	entries  map[string]jsonvalue.Value
	problems parser.Problems
}

// indexEntry returns the entry of the _index.json at indexPath whose id is
// the string taskID. The file is read and indexed once per state for the
// parser's life, since every task file below it looks it up, and the
// problems of that read are added for each lookup as a fresh read would.
func (p *Parser) indexEntry(indexPath, taskID string, problems *parser.Problems) (jsonvalue.Value, bool) {
	info, err := osfile.Lstat(indexPath)
	if err != nil {
		return jsonvalue.NewNull(), false
	}
	key := indexKey{path: indexPath, mtime: info.ModTime().UnixNano(), size: info.Size()}
	index, ok := p.indexes[key]
	if !ok {
		index = &taskIndex{entries: map[string]jsonvalue.Value{}}
		document, _ := readJSON(indexPath, &index.problems)
		for _, entry := range document.Get("entries").Items() {
			p.indexed++
			if _, seen := index.entries[entry.Get("id").StringValue()]; entry.IsObject() &&
				entry.Get("id").IsString() && !seen {
				index.entries[entry.Get("id").StringValue()] = entry
			}
		}
		p.indexes[key] = index
	}
	*problems = append(*problems, index.problems...)
	entry, found := index.entries[taskID]
	return entry, found
}

// task holds what every row of one pre-fork task shares.
type task struct {
	owner    *Parser
	artifact *input.Artifact
	taskID   string
	project  string
	started  string
	options  parser.Options
}

// row returns a row of the task at line and timestamp, its text compacted.
func (t *task) row(line int, timestamp, turn, text string) timeline.Row {
	row := t.owner.BaseRow(t.artifact)
	row.SourceLine, row.SessionID, row.ProjectPath = line, t.taskID, t.project
	row.TimestampUTC = timestamp
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	return row
}

// task emits the rows of a pre-fork task's api_conversation_history.json:
// its history item, when found, as a "task:" row at line 0, then each
// record's rows at its array position, timed by its ts, else the task's,
// then a row per problem.
func (p *Parser) task(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	parts := strings.Split(artifact.Rel, "/")
	var problems parser.Problems
	current := &task{owner: p, artifact: artifact, taskID: parts[len(parts)-2], options: options}
	item := p.historyItem(artifact, current.taskID, &problems)
	current.project = parser.Text(item.Get("workspace"))
	current.started = taskTime(item.Get("ts"), current.taskID)
	if item.Len() > 0 {
		emit(current.row(0, current.started, timeline.TurnSystem, strings.Join(slices.DeleteFunc(
			[]string{"task:", parser.Text(item.Get("task")), labelled("mode=", item.Get("mode")),
				labelled("status=", item.Get("status")), labelled("parent=", item.Get("parentTaskId"))},
			func(part string) bool { return part == "" }), " ")))
	}
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, current.taskID, current.project))
		return
	}
	for index, message := range parser.ArrayElements(textlines.ValidUTF8(data), &problems) {
		if message.IsObject() {
			parser.EmitAll(emit, current.messageRows(index+1, message))
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, current.taskID, current.project))
}

// taskTime returns a task's start: its ts, else its id read as an epoch
// number when the id is all ASCII digits.
func taskTime(stamp jsonvalue.Value, taskID string) string {
	if started := timeutil.FromValue(stamp); started != "" {
		return started
	}
	if taskID == "" || strings.Trim(taskID, "0123456789") != "" {
		return ""
	}
	epoch, err := strconv.ParseInt(taskID, 10, 64)
	if err != nil {
		return ""
	}
	return timeutil.FromInt(epoch)
}

// labelled returns label followed by the value's text when the value counts
// as true, else "".
func labelled(label string, value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.Text()
}

// messageRows returns the rows of the record at position: a reasoning
// record's thinking row, a summary's system row, or the rows of a
// message's content blocks with its text blocks joined into one row placed
// where the first of them was.
func (t *task) messageRows(position int, message jsonvalue.Value) []timeline.Row {
	timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(message.Get("ts")), t.started)
	if parser.IsText(message.Get("type"), "reasoning") {
		if !t.options.IncludeThinking {
			return nil
		}
		text := parser.TextOf(message.Get("summary"))
		if text == "" {
			text = parser.Text(parser.FirstNonEmpty(message.Get("reasoning_content"), message.Get("text")))
		}
		if text == "" {
			return nil
		}
		return []timeline.Row{t.row(position, timestamp, timeline.TurnThinking, text)}
	}
	content := message.Get("content")
	if !message.Get("isSummary").IsEmpty() {
		return []timeline.Row{t.row(position, timestamp, timeline.TurnSystem,
			"summary: "+parser.TextOf(content))}
	}
	if content.IsString() {
		content = jsonvalue.NewArray(jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	return t.blockRows(position, timestamp, message.Get("role"), content.Items())
}

// blocks holds the rows of one message's content blocks as they are built,
// with its text blocks' texts and where the first of them was.
type blocks struct {
	owner     *task
	position  int
	timestamp string
	rows      []timeline.Row
	texts     []string
	textAt    int
}

// add appends a row of the message and returns it.
func (b *blocks) add(turn, text string) *timeline.Row {
	b.rows = append(b.rows, b.owner.row(b.position, b.timestamp, turn, text))
	return &b.rows[len(b.rows)-1]
}

// blockRows returns the rows of a message's content blocks, its text
// blocks joined into one row of the role's turn placed where the first of
// them was.
func (t *task) blockRows(position int, timestamp string, role jsonvalue.Value, content []jsonvalue.Value) []timeline.Row {
	built := &blocks{owner: t, position: position, timestamp: timestamp, textAt: -1}
	for _, block := range content {
		if block.IsObject() {
			built.block(block)
		}
	}
	if len(built.texts) > 0 {
		built.rows = slices.Insert(built.rows, built.textAt, t.row(position, timestamp, roleTurn(role),
			strings.Join(built.texts, "\n")))
	}
	return built.rows
}

// block adds the rows of one content block, or its text to the message's.
func (b *blocks) block(block jsonvalue.Value) {
	kind := block.Get("type")
	switch textOf(kind) {
	case "text", "image":
		text := "[image]"
		if textOf(kind) == "text" {
			text = parser.Text(block.Get("text"))
		}
		switch {
		case strings.HasPrefix(strings.TrimLeftFunc(text, unicode.IsSpace), "<environment_details>"):
			b.add(timeline.TurnSystem, text)
		case text != "":
			if b.textAt < 0 {
				b.textAt = len(b.rows)
			}
			b.texts = append(b.texts, text)
		}
	case "tool_use":
		row := b.add(timeline.TurnToolUse, opencode.ToolSummary(block.Get("input")))
		row.ToolName, row.ToolUseID = parser.Text(block.Get("name")), parser.Text(block.Get("id"))
	case "tool_result":
		text := parser.TextOf(block.Get("content"))
		if !block.Get("is_error").IsEmpty() {
			text = "[error] " + text
		}
		b.add(timeline.TurnToolResult, text).ToolUseID = parser.Text(block.Get("tool_use_id"))
	case "thinking", "reasoning":
		text := parser.FirstNonEmpty(block.Get("thinking"), block.Get("text"))
		if b.owner.options.IncludeThinking && !text.IsEmpty() {
			b.add(timeline.TurnThinking, parser.Text(text))
		}
	default:
		label := "block"
		if !kind.IsEmpty() {
			label = kind.Text()
		}
		b.add(timeline.TurnSystem, label+": "+parser.JSONText(block))
	}
}

// roleTurn returns the turn type of a message's joined text: user,
// assistant, else system.
func roleTurn(role jsonvalue.Value) string {
	switch textOf(role) {
	case "user":
		return timeline.TurnUser
	case "assistant":
		return timeline.TurnAssistant
	}
	return timeline.TurnSystem
}

// textOf returns a string value's text, "" for any other value.
func textOf(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return ""
}
