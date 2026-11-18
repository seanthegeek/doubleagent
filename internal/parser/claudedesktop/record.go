package claudedesktop

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// systemRow returns a system row of the artifact at line, text compacted.
func (p *Parser) systemRow(artifact *input.Artifact, message string, line int) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(message))
	row.SourceLine = line
	return row
}

// flag returns "true" only for a JSON true; absent or anything else is
// "false".
func flag(value jsonvalue.Value) string {
	if set, ok := value.Bool(); ok && set {
		return "true"
	}
	return "false"
}

// grantValue returns a grant as text: a string as it is, anything else as
// compact JSON.
func grantValue(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return parser.JSONText(value)
}

// sessionRecord is a session record being read and what its rows share.
type sessionRecord struct {
	parser   *Parser
	artifact *input.Artifact
	record   jsonvalue.Value
	code     bool
	created  string
	emit     parser.Emit
}

// row returns a row of the record at the given time, text compacted: its
// CLI session id, folder, branch (Code tab only) and model.
func (s *sessionRecord) row(turnType, message, timestamp string) timeline.Row {
	row := s.parser.BaseRow(s.artifact)
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	row.SessionID = s.record.Get("cliSessionId").Text()
	row.ProjectPath = folder(s.record)
	if row.ProjectPath == "" {
		row.ProjectPath = timeutil.FirstNonEmpty(s.record.Get("originCwd").Text(), s.record.Get("cwd").Text())
	}
	if s.code {
		row.GitBranch = s.record.Get("branch").Text()
	}
	row.Model = s.record.Get("model").Text()
	row.TurnType = turnType
	row.Text = timeline.Compact(message)
	return row
}

// record emits the rows of a session record local_<uuid>.json: the
// session, its grants, its first prompt and its error.
func (p *Parser) record(artifact *input.Artifact, emit parser.Emit) {
	match := recordPattern.FindStringSubmatch(artifact.Rel)
	var problems parser.Problems
	record, _ := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !record.IsObject() {
		emit(p.systemRow(artifact, "parser: session record is not a JSON object", 1))
		return
	}
	state := &sessionRecord{parser: p, artifact: artifact, record: record, emit: emit,
		code:    strings.HasPrefix(match[recordPattern.SubexpIndex("tree")], "claude-code-sessions/"),
		created: timeutil.FromValue(record.Get("createdAt"))}
	state.sessionRows()
	state.grants()
	if initial := record.Get("initialMessage").Text(); initial != "" {
		switch {
		case !state.code:
			// Cowork: always, with no time of its own (a secondary source).
			emit(state.row(timeline.TurnUser, initial, ""))
		case !sessionCollected(artifact, record):
			// Code tab: its transcript is claude-code's, another agent's,
			// which deduplication cannot pair with; only when it is absent.
			emit(state.row(timeline.TurnUser, initial, state.created))
		}
	}
	if !record.Get("error").IsEmpty() {
		message := record.Get("error").Text()
		if category := record.Get("errorCategory").Text(); category != "" {
			message = category + ": " + message
		}
		emit(state.row(timeline.TurnSystem, "session error: "+message,
			timeutil.FromValue(record.Get("errorAt"))))
	}
}

// sessionRows emits the record's session row, and for the Code tab the row
// of a Remote Control session live when the app quit to update.
func (s *sessionRecord) sessionRows() {
	record := s.record
	modeEffort := " permission_mode=" + cmp.Or(record.Get("permissionMode").Text(), "-") +
		" effort=" + cmp.Or(record.Get("effort").Text(), "-")
	if !s.code {
		s.emit(s.row(timeline.TurnSystem, "desktop session: "+record.Get("sessionType").Text()+
			` "`+record.Get("title").Text()+`"`+modeEffort, s.created))
		return
	}
	spawn, bridges := record.Get("remoteControlSpawn"), 0
	if ids := record.Get("bridgeSessionIds"); ids.IsArray() {
		bridges = ids.Len()
	}
	s.emit(s.row(timeline.TurnSystem, fmt.Sprintf(`desktop code session: "%s" remote_control=%s `+
		"bridges=%d spawned_remotely=%s steered_remotely=%s", record.Get("title").Text(),
		flag(record.Get("remoteControlUserEnabled")), bridges,
		flag(jsonvalue.NewBool(spawn.IsObject() && spawn.Len() > 0)),
		flag(record.Get("steeredByRemoteClient")))+modeEffort, s.created))
	if quitAt := record.Get("remoteControlLiveAtUpdateQuit").Get("at"); quitAt.IsNumber() {
		s.emit(s.row(timeline.TurnSystem, "remote control live at update quit",
			timeutil.FromValue(quitAt)))
	}
}

// grants emits one "permission grant:" row per grant entry or key of the
// record. The record keeps no time per grant, so the rows take
// lastActivityAt (the grant held by then), else createdAt, and say which.
func (s *sessionRecord) grants() {
	timestamp, field := timeutil.FromValue(s.record.Get("lastActivityAt")), "lastActivityAt"
	if timestamp == "" {
		timestamp, field = s.created, "createdAt"
	}
	grant := func(message string) {
		s.emit(s.row(timeline.TurnSystem, "permission grant: "+message+" (as of "+field+")", timestamp))
	}
	for _, key := range []string{"alwaysAllowedReasons", "sessionPermissionUpdates"} {
		for _, entry := range s.record.Get(key).Items() {
			grant(key + " " + grantValue(entry))
		}
	}
	for _, key := range []string{"chromePermissionMode", "chromeAllowedDomains", "cuAllowedApps",
		"cuGrantFlags", "cuFlagsGrantedAt"} {
		value := s.record.Get(key)
		if !parser.Present(value) || ((value.IsArray() || value.IsObject()) && value.Len() == 0) {
			continue
		}
		grant(key + "=" + grantValue(value))
	}
	// Cowork's tool, network and folder grants: which key is a list and
	// which an object is not determined, so either is read.
	for _, key := range []string{"approvedToolNames", "webFetchAllowedUrls", "egressAllowedDomains",
		"userApprovedFileAccessPaths", "enabledMcpTools", "fileDeleteApprovedMounts"} {
		value := s.record.Get(key)
		switch {
		case value.IsArray():
			for _, entry := range value.Items() {
				grant(key + " " + grantValue(entry))
			}
		case value.IsObject():
			for _, member := range value.Members() {
				grant(key + "." + member.Key + "=" + grantValue(member.Value))
			}
		case parser.Present(value):
			grant(key + "=" + grantValue(value))
		}
	}
}

// readObject returns a state file as a JSON object, or false after
// emitting why not: the unreadable rows, or a row "parser: unreadable
// <name>" at line 1 for a file that is not a JSON object.
func (p *Parser) readObject(artifact *input.Artifact, name string, emit parser.Emit) (jsonvalue.Value, bool) {
	var problems parser.Problems
	document, _ := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return document, false
	}
	if !document.IsObject() {
		emit(p.systemRow(artifact, "parser: unreadable "+name, 1))
		return document, false
	}
	return document, true
}

// scheduled emits one row per task of scheduled-tasks.json, at the task's
// position in the list. A fresh install's file has no task list yet.
func (p *Parser) scheduled(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.readObject(artifact, "scheduled-tasks.json", emit)
	if !ok {
		return
	}
	for i, task := range document.Get("scheduledTasks").Items() {
		if !task.IsObject() {
			continue
		}
		when := task.Get("cronExpression").Text()
		if task.Get("cronExpression").IsEmpty() {
			when = timeutil.FromValue(task.Get("fireAt"))
		}
		row := p.systemRow(artifact, "scheduled task: "+task.Get("id").Text()+" "+when+" enabled="+
			task.Get("enabled").Text()+" permission_mode="+cmp.Or(task.Get("permissionMode").Text(), "-")+
			" model="+cmp.Or(task.Get("model").Text(), "-"), i+1)
		row.TimestampUTC = timeutil.FromValue(task.Get("createdAt"))
		row.Model = task.Get("model").Text()
		row.ProjectPath = timeutil.FirstNonEmpty(task.Get("cwd").Text(), folder(task))
		emit(row)
	}
}

// worktrees emits one row per worktree of git-worktrees.json, at its
// position. A fresh install's file has no worktrees key until the Code tab
// creates one.
func (p *Parser) worktrees(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.readObject(artifact, "git-worktrees.json", emit)
	if !ok {
		return
	}
	for i, member := range document.Get("worktrees").Members() {
		worktree := member.Value
		if !worktree.IsObject() {
			continue
		}
		message := "worktree created: " + worktree.Get("path").Text() + " branch=" +
			worktree.Get("branch").Text() + " from=" + worktree.Get("sourceBranch").Text()
		if leased := worktree.Get("leasedBy"); !leased.IsEmpty() {
			message += " leased by " + leased.Text()
		}
		row := p.systemRow(artifact, message, i+1)
		row.TimestampUTC = timeutil.FromValue(worktree.Get("createdAt"))
		row.ProjectPath = worktree.Get("baseRepo").Text()
		row.GitBranch = worktree.Get("branch").Text()
		emit(row)
	}
}

// spaces emits one "permission setting:" row per folder granted to a remote
// Cowork session in remote-session-spaces.json, keyed by the entry's
// sessionId, the server's id, which the text keeps; the rows have no
// session id.
func (p *Parser) spaces(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := p.code.JSONObject(artifact, "remote-session-spaces.json")
	if !ok {
		emit(bad)
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, entry := range document.Get("entries").Items() {
		folders := entry.Get("folders")
		if !entry.IsObject() || !folders.IsArray() {
			continue
		}
		key := "entries.folders"
		if sessionID := entry.Get("sessionId").Text(); sessionID != "" {
			key = "entries." + sessionID + ".folders"
		}
		for _, granted := range folders.Items() {
			if !parser.Present(granted) {
				continue
			}
			emit(p.code.StandingRow(artifact, timestamp, granted.Text(), key, granted,
				claudecode.PermissionSetting))
		}
	}
}
