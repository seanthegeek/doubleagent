// Package pearai reads PearAI's chat sessions (a fork of Continue) and
// agent tasks (a fork of Roo Code 3.15).
//
// Both forks are older snapshots of their upstreams, so the shapes differ
// from packages continuedev and roocode. It reads:
//
//   - .pearai/sessions/<sessionId>.json: history[] and perplexityHistory[]
//     items' message, contextItems[], citations[] and
//     promptLogs[].completionOptions.model; every row takes the index's
//     dateCreated, and source_line is the item's position across both lists;
//   - .pearai/sessions/sessions.json, the index, read as Continue's; it
//     yields rows only for what it loses;
//   - User/globalStorage/pearai.pearai-roo-cline/tasks/<taskId>/ task files,
//     read through package clinelegacy; tool calls are XML tags in the
//     assistant text and have no id, so tool_use_id is the message's
//     position, and results that open with one of Roo's permission texts
//     give "permission decision:" rows;
//   - User/globalStorage/state.vscdb, row pearai.pearai-roo-cline:
//     taskHistory[] (one row per task) and the standing permission keys;
//   - the PearAI output channel log and .pearai/dev_data/chat.jsonl,
//     secondary sources repeating the sessions' prompts and replies, with no
//     time, and .pearai/dev_data/tokens_generated.jsonl, one "tokens
//     generated:" row per request.
//
// See research/pearai.md.
package pearai

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/parsers/continuedev"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// root matches the agent extension's global storage in the editor.
const root = `(?:.*/)?User/globalStorage/pearai\.pearai-roo-cline`

// extensionID is the agent extension's id, its state.vscdb row's key.
const extensionID = "pearai.pearai-roo-cline"

// The files read, by path relative to the home. The log is the editor's
// output channel file of the chat extension's PearAI channel (pearai-app
// src/vs/workbench/api/common/extHostOutput.ts:121, :160-170).
var (
	sessionPattern = regexp.MustCompile(`^(?:.*/)?\.pearai/sessions/([^/]+)\.json\z`)
	logPattern     = regexp.MustCompile(
		`^(?:.*/)?logs/[^/]+/window[0-9]+/exthost/output_logging_[^/]+/[0-9]+-PearAI\.log\z`)
	devDataPattern = regexp.MustCompile(`^(?:.*/)?\.pearai/dev_data/(chat|tokens_generated)\.jsonl\z`)
	statePattern   = regexp.MustCompile(`^(?:.*/)?User/globalStorage/state\.vscdb\z`)
)

// stateKey identifies one state of the editor's database: its path,
// modification time in nanoseconds and size.
type stateKey struct {
	path  string
	mtime int64
	size  int64
}

// globalState is the extension's global state read from state.vscdb, the
// problems met, its taskHistory[] entries by task id, the first entry of an
// id standing, and how many entries hold each id.
type globalState struct {
	value    jsonvalue.Value
	problems []string
	// repeats are the member names the value repeats, which the parse of
	// the database itself reports, not a task file's lookup of it.
	repeats jsonvalue.Repeats
	history map[string]jsonvalue.Value
	counts  map[string]int
}

// Parser reads PearAI's chat sessions, agent tasks, editor state and chat
// logs.
type Parser struct {
	*clinelegacy.TaskParser
	indexes *continuedev.Indexes
	states  map[stateKey]globalState
	// groups holds the group of each task id a taskHistory repeats, by the
	// state database's path and the id (historyGroup).
	groups map[[2]string]*timeline.Group
}

// New returns the parser.
func New() *Parser {
	created := &Parser{indexes: continuedev.NewIndexes(), states: map[stateKey]globalState{},
		groups: map[[2]string]*timeline.Group{}}
	created.TaskParser = clinelegacy.NewTaskParser(parserkit.NewBase("pearai", "pearai"), root,
		"workspace", created)
	return created
}

// Wants reports whether the artifact is a session (the index included),
// task, log, dev_data or state file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the sessions, the task files, the logs, the two dev_data
// logs and the state database.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := append([]parserkit.Kind{parserkit.PatternKind("session", sessionPattern)}, p.TaskKinds()...)
	kinds = append(kinds, parserkit.PatternKind("log", logPattern))
	for _, name := range []string{"chat", "tokens_generated"} {
		kinds = append(kinds, parserkit.RelFuncKind("dev_data "+name+".jsonl", func(rel string) bool {
			match := devDataPattern.FindStringSubmatch(rel)
			return match != nil && match[1] == name
		}))
	}
	return append(kinds, parserkit.PatternKind("state.vscdb", statePattern))
}

// Secondary reports whether the artifact is the output channel log or
// dev_data/chat.jsonl.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	match := devDataPattern.FindStringSubmatch(artifact.Rel)
	return logPattern.MatchString(artifact.Rel) || (match != nil && match[1] == "chat")
}

// Parse emits the rows of one PearAI file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	session := sessionPattern.FindStringSubmatch(rel)
	devData := devDataPattern.FindStringSubmatch(rel)
	switch {
	case session != nil && session[1]+".json" == continuedev.IndexName:
		parserkit.EmitAll(emit, p.ErrorRows(artifact, p.indexes.Errors(artifact.DiskPath), "", ""))
		if index, ok := parserkit.Beside(artifact, continuedev.IndexName); ok {
			parserkit.EmitAll(emit, p.indexes.Notices(index, p.BaseRow(artifact)))
		}
	case session != nil:
		p.session(artifact, session[1], emit)
	case p.TaskMatch(artifact):
		p.taskFile(artifact, options, emit)
	case logPattern.MatchString(rel):
		p.outputLog(artifact, emit)
	case devData != nil && devData[1] == "chat":
		p.devChat(artifact, emit)
	case devData != nil:
		p.tokens(artifact, emit)
	case statePattern.MatchString(rel):
		p.state(artifact, emit)
	}
	return nil
}

// globalState returns the extension's global state from the state.vscdb
// at path and its problems, read once per state of the file; nothing for a
// path that cannot be looked at.
func (p *Parser) globalState(path string) globalState {
	info, err := osfile.Lstat(path)
	if err != nil {
		return globalState{value: jsonvalue.NewNull()}
	}
	key := stateKey{path: path, mtime: info.ModTime().UnixNano(), size: info.Size()}
	if found, ok := p.states[key]; ok {
		return found
	}
	value, problems, repeats := vscodestate.ReadItem(path, extensionID)
	history, counts := p.historyIndex(value)
	p.states[key] = globalState{value: value, problems: problems, repeats: repeats, history: history,
		counts: counts}
	return p.states[key]
}

// historyIndex returns the taskHistory[] objects of a global state by
// task id, the first entry of an id standing, and how many hold each id.
func (p *Parser) historyIndex(state jsonvalue.Value) (map[string]jsonvalue.Value, map[string]int) {
	index, counts := map[string]jsonvalue.Value{}, map[string]int{}
	for _, entry := range state.Get("taskHistory").Items() {
		p.Work.Add("history entries read", 1)
		if !entry.IsObject() {
			continue
		}
		taskID := entry.Get("id").Text()
		if counts[taskID]++; counts[taskID] == 1 {
			index[taskID] = entry
		}
	}
	return index, counts
}

// historyGroup returns the group of a task id the taskHistory of the
// state.vscdb at path holds in more than one entry, else nil. The extension
// writes one entry per task id, replacing it on each update, so only damage
// or an edit repeats one (timeline.Group): each entry's task row is a
// parent and the task's rows are members. The group is the parser's own
// for that database and id, so the database's parse and the task files'
// parses share it in whichever order they come.
func (p *Parser) historyGroup(path, taskID string) *timeline.Group {
	if p.globalState(path).counts[taskID] < 2 {
		return nil
	}
	key := [2]string{path, taskID}
	group := p.groups[key]
	if group == nil {
		group = timeline.NewGroup()
		p.groups[key] = group
	}
	return group
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ clinelegacy.APIRower      = (*Parser)(nil)
	_ clinelegacy.Family        = (*Parser)(nil)
)
