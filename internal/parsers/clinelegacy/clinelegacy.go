package clinelegacy

import (
	"path"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The file names of a task directory. The API history has two names: Roo's
// older releases wrote claude_messages.json.
const (
	UIFile          = "ui_messages.json"
	apiFile         = "api_conversation_history.json"
	olderAPIFile    = "claude_messages.json"
	metadataFile    = "task_metadata.json"
	taskFilePattern = `/tasks/([^/]+)/(ui_messages|api_conversation_history|claude_messages|task_metadata)\.json\z`
)

// IsAPIFile reports whether a lower-cased file name is one of the API
// history's names.
func IsAPIFile(name string) bool {
	return name == apiFile || name == olderAPIFile
}

// Family is what an agent's parser gives the TaskParser it embeds.
type Family interface {
	// HistoryItem returns the task's entry in the agent's task index, or
	// null when there is none, and the group of the task's id when the
	// index holds it in more than one entry (EntryGroup), else nil.
	HistoryItem(task Task) (jsonvalue.Value, *timeline.Group)
}

// APIRower is implemented by a Family whose API history writes tool calls
// in its own form, replacing ContentRows for each API history message.
type APIRower interface {
	// APIContentRows returns the rows of one API history message at a
	// 1-based position.
	APIContentRows(message jsonvalue.Value, position int, baseTurn string,
		includeThinking bool) []ContentRow
}

// Task names where the task a file belongs to keeps its files: the file's
// own directory, and the storage root some levels above it.
type Task struct {
	Artifact *input.Artifact
	// TaskID is the task directory's name.
	TaskID string
	// RootLevels is how many directories above the artifact's own the
	// storage root lies: 2 for a task file (the root is above tasks/), 1
	// for the extension's cache/sync-queue.json.
	RootLevels int
}

// TaskOf returns the task of a file directly inside a task directory.
func TaskOf(artifact *input.Artifact) Task {
	return Task{Artifact: artifact, TaskID: path.Base(path.Dir(artifact.Rel)), RootLevels: 2}
}

// InDir returns the disk path of a file beside the artifact, in the task
// directory, or "" when the kit refuses it; every reader treats "" as a
// file that is not there.
func (t Task) InDir(name string) string {
	path, _ := parserkit.Beside(t.Artifact, name)
	return path
}

// InRoot returns the disk path of a path under the storage root, or false
// when the root lies outside the artifact's home.
func (t Task) InRoot(rel string) (string, bool) {
	return parserkit.Above(t.Artifact, t.RootLevels, strings.Split(rel, "/")...)
}

// InRootParent returns the disk path of a file in the directory that holds
// the storage root, or false when that lies outside the artifact's home.
func (t Task) InRootParent(name string) (string, bool) {
	return parserkit.Above(t.Artifact, t.RootLevels+1, name)
}

// Source reads a file for the cache: its value, the problems the read met
// and the member names the value repeats. A nil Source reads the file as
// JSON and keeps neither, since the file's own parse reports them.
type Source func(path string) (jsonvalue.Value, []string, jsonvalue.Repeats)

// cachedFile is a file as its Source read it and, once a lookup asks for
// them, the entries of its lists by id, keyed by the member that holds the
// list ("" for the document itself).
type cachedFile struct {
	value    jsonvalue.Value
	problems []string
	repeats  jsonvalue.Repeats
	byID     map[string]*entryList
}

// entryList is one list of a cached file indexed by id: the first object
// of each id, how many objects hold each id, and the set of its ids.
type entryList struct {
	first  map[string]jsonvalue.Value
	counts map[string]int
	ids    parserkit.EvidenceIDs
}

// entryKey names one id of one list of one file: the file's path, the
// member that holds the list and the id.
type entryKey struct {
	path, member, entryID string
}

// TaskParser reads the task files under one storage root for the parser
// that embeds it.
type TaskParser struct {
	parserkit.Base
	taskPattern *parserkit.RelPattern
	// itemPattern and indexPattern match the index files of Roo Code's
	// layout (IndexKinds).
	itemPattern, indexPattern *parserkit.RelPattern
	projectKey                string
	family                    Family
	cache                     map[string]*cachedFile
	// groups holds the group of each id a list repeats, by its file, list
	// and id, for the parser's life (EntryGroup).
	groups map[entryKey]*timeline.Group
}

// NewTaskParser returns the reader of the task files under the storage
// root that root, a regular expression over the path relative to the home,
// matches. projectKey is the index entry's key of the project path. A nil
// family is the TaskParser itself, whose index is Roo Code's (HistoryItem).
func NewTaskParser(base parserkit.Base, root, projectKey string, family Family) *TaskParser {
	created := &TaskParser{Base: base, projectKey: projectKey, family: family,
		taskPattern:  parserkit.MustRelPattern(`^` + root + taskFilePattern),
		itemPattern:  parserkit.MustRelPattern(`^` + root + `/tasks/[^/]+/history_item\.json\z`),
		indexPattern: parserkit.MustRelPattern(`^` + root + `/tasks/_index\.json\z`),
		cache:        map[string]*cachedFile{}, groups: map[entryKey]*timeline.Group{}}
	if family == nil {
		created.family = created
	}
	return created
}

// ProjectKey returns the index entry's key of the project path.
func (p *TaskParser) ProjectKey() string {
	return p.projectKey
}

// TaskMatch reports whether the artifact is a task file under the root.
func (p *TaskParser) TaskMatch(artifact *input.Artifact) bool {
	return p.taskPattern.MatchString(artifact.Rel)
}

// TaskKinds returns the four task files under the root, one kind each,
// which together want what TaskMatch does.
func (p *TaskParser) TaskKinds() []parserkit.Kind {
	var kinds []parserkit.Kind
	for _, name := range []string{UIFile, apiFile, olderAPIFile, metadataFile} {
		stem := strings.TrimSuffix(name, ".json")
		kinds = append(kinds, parserkit.RelFuncKind("task "+name, func(rel string) bool {
			match := p.taskPattern.FindStringSubmatch(rel)
			return match != nil && textlines.EqualFoldASCII(match[2], stem)
		}))
	}
	return kinds
}

// cachedFile returns the cache entry of the file at path, reading the file
// with source when the entry is not there yet. The entry is kept by the
// path alone: a parser lives for one run, over evidence that does not change.
func (p *TaskParser) cachedFile(path string, source Source) *cachedFile {
	if cached, ok := p.cache[path]; ok {
		return cached
	}
	cached := &cachedFile{}
	if source == nil {
		var ignored parserkit.Problems
		cached.value, _ = parserkit.LoadJSON(nil, path, &ignored)
	} else {
		cached.value, cached.problems, cached.repeats = source(path)
	}
	p.cache[path] = cached
	return cached
}

// CachedJSON returns the file at path decoded as JSON, or null when it is
// absent, unreadable or not JSON, read once for the parser's life.
func (p *TaskParser) CachedJSON(path string) jsonvalue.Value {
	return p.cachedFile(path, nil).value
}

// CachedRead returns the file at path as source reads it, once for the
// parser's life as CachedJSON reads a file: its value, problems and repeated
// member names.
func (p *TaskParser) CachedRead(path string, source Source) (jsonvalue.Value, []string, jsonvalue.Repeats) {
	cached := p.cachedFile(path, source)
	return cached.value, cached.problems, cached.repeats
}

// entries returns a list of the file at path indexed by id, indexing it on
// first use.
func (p *TaskParser) entries(path, member string, source Source) *entryList {
	cached := p.cachedFile(path, source)
	if cached.byID == nil {
		cached.byID = map[string]*entryList{}
	}
	if entries, ok := cached.byID[member]; ok {
		return entries
	}
	list := cached.value
	if member != "" {
		list = list.Get(member)
	}
	entries := &entryList{first: map[string]jsonvalue.Value{}, counts: map[string]int{}}
	for _, item := range list.Items() {
		p.Work.Add("index items read", 1)
		if !item.IsObject() {
			continue
		}
		entryID := item.Get("id").Text()
		if entries.counts[entryID]++; entries.counts[entryID] > 1 {
			continue
		}
		entries.first[entryID] = item
		entries.ids.Add(entryID)
	}
	cached.byID[member] = entries
	return entries
}

// CachedEntry returns the first object of a list in the file at path whose
// id, as row text, is the list's spelling of entryID, a task directory's
// name (parserkit.EvidenceIDs), or null. The list is the document itself when
// member is "", else the document's member of that name. The file is read
// by source as CachedRead reads it, and its list indexed once, so the many
// task files that look up one index cost one pass over it.
func (p *TaskParser) CachedEntry(path, member, entryID string, source Source) jsonvalue.Value {
	entries := p.entries(path, member, source)
	if found, ok := entries.ids.Spelling(entryID); ok {
		return entries.first[found]
	}
	return jsonvalue.NewNull()
}

// EntryGroup returns the group of the id CachedEntry finds for entryID when
// the list holds the id in more than one object, else nil. The
// agents write one entry per task id, replacing it on each update, so only
// damage or an edit repeats one (timeline.Group): each entry of the id is a
// parent, through its own row, and the task's rows are members. The group
// is the parser's own for that file and id, so the index's parse and the
// task files' parses share it in whichever order they come.
func (p *TaskParser) EntryGroup(path, member, entryID string, source Source) *timeline.Group {
	entries := p.entries(path, member, source)
	found, ok := entries.ids.Spelling(entryID)
	if !ok || entries.counts[found] < 2 {
		return nil
	}
	key := entryKey{path: path, member: member, entryID: found}
	group := p.groups[key]
	if group == nil {
		group = timeline.NewGroup()
		p.groups[key] = group
	}
	return group
}

// Context returns what the task directory of the artifact knows about
// itself. The task's id is its index entry's spelling of the directory's
// name, which the index's own rows carry, else the directory's.
func (p *TaskParser) Context(artifact *input.Artifact) *TaskContext {
	task := TaskOf(artifact)
	item, group := p.family.HistoryItem(task)
	if entryID := item.Get("id"); entryID.IsString() && textlines.EqualFoldASCII(entryID.StringValue(), task.TaskID) {
		task.TaskID = entryID.StringValue()
	}
	var ignored parserkit.Problems
	// The metadata is parsed as its own artifact, whose parse reports its
	// repeated member names.
	metadata, _ := parserkit.LoadJSON(nil, task.InDir(metadataFile), &ignored)
	context := newTaskContext(task.TaskID, item, metadata, p.projectKey)
	context.work, context.group = p.Work, group
	return context
}

// TaskFileName returns the name of a task file with its ASCII letters
// lowered, the form compared with UIFile, the API history's names and
// metadataFile.
func TaskFileName(artifact *input.Artifact) string {
	return textlines.LowerASCII(path.Base(artifact.Rel))
}

// HasUI reports whether a ui_messages.json is beside a task file, whose
// rows then stand for the API history's.
func HasUI(artifact *input.Artifact) bool {
	return parserkit.RegularFile(TaskOf(artifact).InDir(UIFile))
}

// ParseTaskFile emits the rows of one task file: the API history only
// when no ui_messages.json is beside it.
func (p *TaskParser) ParseTaskFile(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	name := TaskFileName(artifact)
	if IsAPIFile(name) && HasUI(artifact) {
		return
	}
	context := p.Context(artifact)
	switch {
	case name == UIFile:
		p.ParseUI(artifact, context, options, emit)
	case IsAPIFile(name):
		p.ParseAPI(artifact, context, options, emit)
	case name == metadataFile:
		p.parseMetadata(artifact, context, emit)
	}
}

// Row returns a row of the task at a record's position, with its time and
// model.
func (p *TaskParser) Row(artifact *input.Artifact, context *TaskContext, position int,
	timestamp, model string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = position
	row.TimestampUTC = timestamp
	row.SessionID = context.TaskID
	row.ProjectPath = context.ProjectPath
	row.Model = model
	return row
}

// TaskErrorRows returns a row per problem of a task file, with the task's
// id and project, each a member of the task's group (TaskContext.Member).
func (p *TaskParser) TaskErrorRows(artifact *input.Artifact, problems parserkit.Problems,
	context *TaskContext) []timeline.Row {
	rows := p.ErrorRows(artifact, problems, context.TaskID, context.ProjectPath)
	for i := range rows {
		context.Member(&rows[i], true, true)
	}
	return rows
}

// HistoryRow returns the "task:" row of one task index entry.
func (p *TaskParser) HistoryRow(artifact *input.Artifact, item jsonvalue.Value, position int) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = position
	row.TimestampUTC = timeutil.FromValue(item.Get("ts"))
	row.SessionID = parserkit.Text(item.Get("id"))
	row.ProjectPath = parserkit.Text(item.Get(p.projectKey))
	row.Model = parserkit.Text(item.Get("modelId"))
	row.TurnType = timeline.TurnSystem
	extra := FormatCounts(item, "tokensIn", "tokensOut", "cacheWrites", "cacheReads", "totalCost",
		"size", "apiProvider", "mode", "status", "apiConfigName", "parentTaskId", "rootTaskId")
	row.Text = timeline.Compact("task: " + parserkit.Text(item.Get("task")) + " | " + extra)
	return row
}
