// Package clinelegacy reads the per-task layout Cline introduced and Roo
// Code and PearAI's agent inherited, for the parsers of those agents. It
// registers no parser of its own.
//
// Under a storage root (an editor's User/globalStorage/<extension id>,
// Cline's ~/.cline/data, Roo's ~/.vscode-mock/global-storage) each task is
// tasks/<taskId>/ holding:
//
//   - ui_messages.json, the preferred transcript: a JSON array of messages
//     {ts, type, ask, say, text, reasoning, images, partial, modelInfo},
//     whose text is itself JSON for several kinds;
//   - api_conversation_history.json (Roo's older claude_messages.json), read
//     only when no ui_messages.json sits beside it: model API messages
//     {role, content} with text, tool_use, tool_result, thinking, image and
//     file blocks, plus Roo's ts, isSummary and reasoning records. A record
//     without ts takes the task's start: a millisecond task id, else the
//     earliest task_metadata.json time, else the task index entry's ts;
//   - task_metadata.json: files_in_context[], model_usage[] and
//     environment_history[], one system row per dated event and model: and
//     permission mode: rows on a change.
//
// Each agent's parser embeds a TaskParser built with the storage root's
// pattern and the index key of the project path, and gives it a Family that
// finds a task's index entry. A record that does not decode stops the array
// walk and becomes one `parser: unparseable record <N> (<reason>): <raw>`
// row holding the text from that record to the end of the file. See
// research/cline.md and research/roo-code.md.
package clinelegacy

import (
	"path"
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The file names of a task directory. The API history has two names: Roo's
// older releases wrote claude_messages.json.
const (
	UIFile          = "ui_messages.json"
	APIFile         = "api_conversation_history.json"
	OlderAPIFile    = "claude_messages.json"
	MetadataFile    = "task_metadata.json"
	taskFilePattern = `/tasks/([^/]+)/(ui_messages|api_conversation_history|claude_messages|task_metadata)\.json\z`
)

// IsAPIFile reports whether a lower-cased file name is one of the API
// history's names.
func IsAPIFile(name string) bool {
	return name == APIFile || name == OlderAPIFile
}

// Family is what an agent's parser gives the TaskParser it embeds.
type Family interface {
	// HistoryItem returns the task's entry in the agent's task index, or
	// null when there is none.
	HistoryItem(task Task) jsonvalue.Value
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
// directory.
func (t Task) InDir(name string) string {
	return parser.Beside(t.Artifact, name)
}

// InRoot returns the disk path of a path under the storage root, or false
// when the root lies outside the artifact's home.
func (t Task) InRoot(rel string) (string, bool) {
	return parser.Above(t.Artifact, t.RootLevels, rel)
}

// InRootParent returns the disk path of a file in the directory that holds
// the storage root, or false when that lies outside the artifact's home.
func (t Task) InRootParent(name string) (string, bool) {
	return parser.Above(t.Artifact, t.RootLevels+1, name)
}

// cacheKey identifies one state of a file: its path, modification time in
// nanoseconds and size.
type cacheKey struct {
	path  string
	mtime int64
	size  int64
}

// cachedFile is a file decoded as JSON and, once a lookup asks for them,
// the entries of its lists by id, keyed by the member that holds the list
// ("" for the document itself).
type cachedFile struct {
	value jsonvalue.Value
	byID  map[string]map[string]jsonvalue.Value
}

// TaskParser reads the task files under one storage root for the parser
// that embeds it.
type TaskParser struct {
	parser.Base
	taskPattern *regexp.Regexp
	projectKey  string
	family      Family
	cache       map[cacheKey]*cachedFile
	// indexed counts the list items CachedEntry has read, the work a test
	// holds to one pass per file.
	indexed int
}

// NewTaskParser returns the reader of the task files under the storage
// root that root, a regular expression over the path relative to the home,
// matches. projectKey is the index entry's key of the project path.
func NewTaskParser(base parser.Base, root, projectKey string, family Family) *TaskParser {
	return &TaskParser{Base: base, projectKey: projectKey, family: family,
		taskPattern: regexp.MustCompile(`(?i)^` + root + taskFilePattern),
		cache:       map[cacheKey]*cachedFile{}}
}

// ProjectKey returns the index entry's key of the project path.
func (p *TaskParser) ProjectKey() string {
	return p.projectKey
}

// TaskMatch reports whether the artifact is a task file under the root.
func (p *TaskParser) TaskMatch(artifact *input.Artifact) bool {
	return p.taskPattern.MatchString(artifact.Rel)
}

// CachedJSON returns the file at path decoded as JSON, or null when it is
// absent, unreadable or not JSON. The result is kept for the parser's
// life, keyed by the path, modification time and size, so the many task
// files that look up one index read it once.
func (p *TaskParser) CachedJSON(path string) jsonvalue.Value {
	return p.cachedFile(path).value
}

// cachedFile returns the cache entry of the file at path, decoding the
// file when the entry is not there yet.
func (p *TaskParser) cachedFile(path string) *cachedFile {
	info, err := osfile.Lstat(path)
	if err != nil {
		return &cachedFile{value: jsonvalue.NewNull()}
	}
	key := cacheKey{path: path, mtime: info.ModTime().UnixNano(), size: info.Size()}
	if cached, ok := p.cache[key]; ok {
		return cached
	}
	var ignored parser.Problems
	value, _ := parser.LoadJSON(path, &ignored)
	p.cache[key] = &cachedFile{value: value}
	return p.cache[key]
}

// CachedEntry returns the first object of a list in the file at path whose
// id, as row text, is entryID, or null. The list is the document itself when
// member is "", else the document's member of that name. The file is read
// as CachedJSON reads it, and its list is indexed by id once, so the many
// task files that look up one index cost one pass over it.
func (p *TaskParser) CachedEntry(path, member, entryID string) jsonvalue.Value {
	cached := p.cachedFile(path)
	if cached.byID == nil {
		cached.byID = map[string]map[string]jsonvalue.Value{}
	}
	entries, ok := cached.byID[member]
	if !ok {
		list := cached.value
		if member != "" {
			list = list.Get(member)
		}
		entries = map[string]jsonvalue.Value{}
		for _, item := range list.Items() {
			p.indexed++
			if _, seen := entries[item.Get("id").Text()]; item.IsObject() && !seen {
				entries[item.Get("id").Text()] = item
			}
		}
		cached.byID[member] = entries
	}
	if entry, ok := entries[entryID]; ok {
		return entry
	}
	return jsonvalue.NewNull()
}

// Context returns what the task directory of the artifact knows about
// itself.
func (p *TaskParser) Context(artifact *input.Artifact) *TaskContext {
	task := TaskOf(artifact)
	item := p.family.HistoryItem(task)
	var ignored parser.Problems
	metadata, _ := parser.LoadJSON(task.InDir(MetadataFile), &ignored)
	return newTaskContext(task.TaskID, item, metadata, p.projectKey)
}

// ParseTaskFile emits the rows of one task file: the API history only
// when no ui_messages.json is beside it.
func (p *TaskParser) ParseTaskFile(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	name := TaskFileName(artifact)
	context := p.Context(artifact)
	switch {
	case name == UIFile:
		p.ParseUI(artifact, context, options, emit)
	case IsAPIFile(name):
		if parser.RegularFile(TaskOf(artifact).InDir(UIFile)) {
			return
		}
		p.ParseAPI(artifact, context, options, emit)
	case name == MetadataFile:
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

// HistoryRow returns the "task:" row of one task index entry.
func (p *TaskParser) HistoryRow(artifact *input.Artifact, item jsonvalue.Value, position int) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = position
	row.TimestampUTC = timeutil.FromValue(item.Get("ts"))
	row.SessionID = parser.Text(item.Get("id"))
	row.ProjectPath = parser.Text(item.Get(p.projectKey))
	row.Model = parser.Text(item.Get("modelId"))
	row.TurnType = timeline.TurnSystem
	extra := FormatCounts(item, "tokensIn", "tokensOut", "cacheWrites", "cacheReads", "totalCost",
		"size", "apiProvider", "mode", "status", "apiConfigName", "parentTaskId", "rootTaskId")
	row.Text = timeline.Compact("task: " + parser.Text(item.Get("task")) + " | " + extra)
	return row
}

// TaskFileName returns the name of a task file with its ASCII letters
// lowered, the form compared with UIFile, the API history's names and
// MetadataFile.
func TaskFileName(artifact *input.Artifact) string {
	return lowerASCII(path.Base(artifact.Rel))
}

// lowerASCII returns a file name with its ASCII letters lowered and every
// other byte kept: the names it is compared with are ASCII, and no other
// letter lowers to an ASCII one in them.
func lowerASCII(name string) string {
	lowered := []byte(name)
	for i, octet := range lowered {
		if 'A' <= octet && octet <= 'Z' {
			lowered[i] = octet + 'a' - 'A'
		}
	}
	return string(lowered)
}
