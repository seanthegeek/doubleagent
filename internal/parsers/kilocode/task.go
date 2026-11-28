package kilocode

import (
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/opencode"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// indexEntries returns the entries of the _index.json at indexPath whose id
// is the string taskID. The file is read and indexed once per state for the
// parser's life, since every task file below it looks it up, and the
// problems of that read are added for each lookup as a fresh read would; a
// file that does not hold a JSON object, and an entries member that is not
// an array, are among them. Its repeated member names are noted once, in
// the parse of artifact that reads it first.
func (p *Parser) indexEntries(artifact *input.Artifact, indexPath, taskID string, problems *parserkit.Problems) []jsonvalue.Value {
	info, err := osfile.Lstat(indexPath)
	if err != nil {
		return nil
	}
	key := indexKey{path: indexPath, mtime: info.ModTime().UnixNano(), size: info.Size()}
	index, ok := p.indexes[key]
	if !ok {
		index = &taskIndex{entries: map[string][]jsonvalue.Value{}}
		document, read := readJSON(artifact, indexPath, &index.problems)
		if read && !document.IsObject() {
			index.problems.Add(0, "_index.json is not a JSON object")
		}
		for _, entry := range parserkit.EntriesOfKind(document.Get("entries"), "entries", jsonvalue.Object, 0,
			&index.problems) {
			p.Work.Add("index entries read", 1)
			if entryID := entry.Get("id"); entryID.IsString() {
				index.entries[entryID.StringValue()] = append(index.entries[entryID.StringValue()], entry)
			}
		}
		p.indexes[key] = index
	}
	*problems = append(*problems, index.problems...)
	return index.entries[taskID]
}

// indexKey identifies one state of a side file read once, a tasks' _index.json
// or a memory folder's manifest.json: its path, modification time in
// nanoseconds and size.
type indexKey struct {
	path        string
	mtime, size int64
}

// taskIndex is a tasks' _index.json read once: the object entries of each
// string id in file order, and the problems reading the file added.
type taskIndex struct {
	entries  map[string][]jsonvalue.Value
	problems parserkit.Problems
}

// historyItems returns a pre-fork task's history item: history_item.json in
// its directory, else its entries in the tasks' _index.json, in file order
// (more than one only when the index repeats the id), else an empty
// object. The task directory is the artifact's own. Either file that is
// there but cannot be read, or does not hold a JSON object (text that is
// not JSON among it), adds a problem.
func (p *Parser) historyItems(artifact *input.Artifact, taskID string, problems *parserkit.Problems) []jsonvalue.Value {
	item := jsonvalue.NewNull()
	if itemPath, ok := parserkit.Beside(artifact, "history_item.json"); ok && parserkit.RegularFile(itemPath) {
		if read, ok := readJSON(artifact, itemPath, problems); ok {
			item = read
			if !item.IsObject() {
				problems.Add(0, "history_item.json is not a JSON object")
			}
		}
	}
	if item.IsObject() {
		return []jsonvalue.Value{item}
	}
	if indexPath, found := parserkit.Above(artifact, 1, "_index.json"); found && parserkit.RegularFile(indexPath) {
		if entries := p.indexEntries(artifact, indexPath, taskID, problems); len(entries) > 0 {
			return entries
		}
	}
	return []jsonvalue.Value{jsonvalue.NewObject()}
}

// task holds what every row of one pre-fork task shares.
type task struct {
	owner    *Parser
	artifact *input.Artifact
	taskID   string
	project  string
	started  string
	options  parserkit.Options
	// problems collects the task file's problems.
	problems *parserkit.Problems
	// group is the group of the task's id when the tasks' _index.json
	// repeats it (taskRows).
	group *timeline.Group
}

// row returns a row of the task at line and timestamp, its text compacted;
// an empty timestamp is the task's start. In a task whose index repeats its
// id, the row is a member of the id's group, taking the project, and the
// time when it is the task's start, from the entry the group goes with.
func (t *task) row(line int, timestamp, turn, text string) timeline.Row {
	takes := timeline.TakeProject
	if timestamp == "" {
		timestamp, takes = t.started, takes|timeline.TakeTime
	}
	row := t.owner.BaseRow(t.artifact)
	row.SourceLine, row.SessionID, row.ProjectPath = line, t.taskID, t.project
	row.TimestampUTC = timestamp
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	if t.group != nil {
		t.group.Member(&row, takes)
	}
	return row
}

// errorRows returns a row per problem of the task, each a member of the
// task's group as row makes its rows.
func (t *task) errorRows() []timeline.Row {
	rows := t.owner.ErrorRows(t.artifact, *t.problems, t.taskID, t.project)
	for i := range rows {
		if t.group != nil {
			t.group.Member(&rows[i], timeline.TakeProject)
		}
	}
	return rows
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

// taskRows returns the "task:" row of the task's history item, none for an
// empty one. The extension writes one index entry per task id, so only
// damage or an edit repeats one; then each entry gives its own row, with
// its own project and start, as a parent of the id's group
// (timeline.Group), and the task's other rows are members (row).
func (t *task) taskRows(items []jsonvalue.Value) []timeline.Row {
	if len(items) > 1 {
		t.group = timeline.NewGroup()
	}
	var rows []timeline.Row
	for _, item := range items {
		if item.Len() == 0 {
			continue
		}
		row := t.owner.BaseRow(t.artifact)
		row.SessionID, row.ProjectPath = t.taskID, parserkit.Text(item.Get("workspace"))
		row.TimestampUTC, row.TurnType = taskTime(item.Get("ts"), t.taskID), timeline.TurnSystem
		row.Text = timeline.Compact(strings.Join(slices.DeleteFunc(
			[]string{"task:", parserkit.Text(item.Get("task")), parserkit.Labelled("mode=", item.Get("mode")),
				parserkit.Labelled("status=", item.Get("status")), parserkit.Labelled("parent=", item.Get("parentTaskId"))},
			func(part string) bool { return part == "" }), " "))
		if t.group != nil {
			t.group.Parent(&row)
		}
		rows = append(rows, row)
	}
	return rows
}

// add appends a row of the message and returns it.
func (b *blocks) add(turn, text string) *timeline.Row {
	b.rows = append(b.rows, b.owner.row(b.position, b.timestamp, turn, text))
	return &b.rows[len(b.rows)-1]
}

// textOf returns a string value's text, "" for any other value.
func textOf(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return ""
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

// block adds the rows of one content block, or its text to the message's.
func (b *blocks) block(block jsonvalue.Value) {
	kind := block.Get("type")
	switch textOf(kind) {
	case "text", "image":
		text := "[image]"
		if textOf(kind) == "text" {
			text = parserkit.Text(block.Get("text"))
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
		row.ToolName, row.ToolUseID = parserkit.Text(block.Get("name")), parserkit.Text(block.Get("id"))
	case "tool_result":
		text := parserkit.TextOf(block.Get("content"))
		if !block.Get("is_error").IsEmpty() {
			text = "[error] " + text
		}
		b.add(timeline.TurnToolResult, text).ToolUseID = parserkit.Text(block.Get("tool_use_id"))
	case "thinking", "reasoning":
		text := parserkit.FirstNonEmpty(block.Get("thinking"), block.Get("text"))
		if b.owner.options.IncludeThinking && !text.IsEmpty() {
			b.add(timeline.TurnThinking, parserkit.Text(text))
		}
	default:
		label := "block"
		if !kind.IsEmpty() {
			label = kind.Text()
		}
		b.add(timeline.TurnSystem, label+": "+parserkit.JSONText(block))
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

// blockRows returns the rows of a message's content blocks, its text
// blocks joined into one row of the role's turn placed where the first of
// them was.
func (t *task) blockRows(position int, timestamp string, role jsonvalue.Value, content jsonvalue.Value) []timeline.Row {
	built := &blocks{owner: t, position: position, timestamp: timestamp, textAt: -1}
	for _, block := range parserkit.EntriesOfKind(content, "content", jsonvalue.Object, position, t.problems) {
		built.block(block)
	}
	if len(built.texts) > 0 {
		built.rows = slices.Insert(built.rows, built.textAt, t.row(position, timestamp, roleTurn(role),
			strings.Join(built.texts, "\n")))
	}
	return built.rows
}

// messageRows returns the rows of the record at position: a reasoning
// record's thinking row, a summary's system row, or the rows of a
// message's content blocks with its text blocks joined into one row placed
// where the first of them was.
func (t *task) messageRows(position int, message jsonvalue.Value) []timeline.Row {
	timestamp := timeutil.FromValue(message.Get("ts"))
	if parserkit.IsText(message.Get("type"), "reasoning") {
		if !t.options.IncludeThinking {
			return nil
		}
		text := parserkit.TextOf(message.Get("summary"))
		if text == "" {
			text = parserkit.Text(parserkit.FirstNonEmpty(message.Get("reasoning_content"), message.Get("text")))
		}
		if text == "" {
			return nil
		}
		return []timeline.Row{t.row(position, timestamp, timeline.TurnThinking, text)}
	}
	content := message.Get("content")
	if !message.Get("isSummary").IsEmpty() {
		return []timeline.Row{t.row(position, timestamp, timeline.TurnSystem,
			"summary: "+parserkit.TextOf(content))}
	}
	if content.IsString() {
		content = jsonvalue.NewArray(jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	return t.blockRows(position, timestamp, message.Get("role"), content)
}

// task emits the rows of a pre-fork task's api_conversation_history.json:
// its history item, when found, as a "task:" row at line 0, then each
// record's rows at its array position, timed by its ts, else the task's,
// then a row per problem.
func (p *Parser) task(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	parts := strings.Split(artifact.Rel, "/")
	var problems parserkit.Problems
	current := &task{owner: p, artifact: artifact, taskID: parts[len(parts)-2], options: options,
		problems: &problems}
	items := p.historyItems(artifact, current.taskID, &problems)
	current.project = parserkit.Text(items[0].Get("workspace"))
	current.started = taskTime(items[0].Get("ts"), current.taskID)
	parserkit.EmitAll(emit, current.taskRows(items))
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, current.errorRows())
		return
	}
	messages := slices.All(parserkit.ArrayElements(textlines.ValidUTF8(data), &problems))
	for index, message := range parserkit.EntriesOfKindFrom(messages, path.Base(artifact.Rel), jsonvalue.Object, 0,
		&problems) {
		parserkit.EmitAll(emit, current.messageRows(index+1, message))
	}
	parserkit.EmitAll(emit, current.errorRows())
}
