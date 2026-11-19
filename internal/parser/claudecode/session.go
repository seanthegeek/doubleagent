package claudecode

import (
	"cmp"
	"sort"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// teleportSynced is the sync row of a teleported cloud session, beside its
// origin row: the vendor holds the conversation the remoteSourced records
// replay.
const teleportSynced = timeline.SyncedPrefix +
	" cloud session (teleported: remoteSourced records replayed from the vendor's copy)"

// seenKey is a session id and a value already reported for it.
type seenKey struct{ session, value string }

// bridgeKey is the bridge id and sequence number last reported for a
// session, each as compact JSON.
type bridgeKey struct{ bridge, sequence string }

// session is the state of one transcript's parse: what the records so far
// have set, and the record being read.
type session struct {
	reader   *Reader
	artifact *input.Artifact
	options  parser.Options
	emit     parser.Emit
	problems parser.Problems
	scan     *prescan

	fileAgentID         string
	headerDone          bool
	taskDone            bool
	sessionID           string
	lastTimestamp       string
	lastCwd, lastBranch string
	relocated, surface  string
	seen                map[string]map[seenKey]bool
	modes, models       parser.Tracker
	bridges             map[string]bridgeKey

	line      int
	timestamp string
}

// Session emits the rows of one Claude Code transcript, a session's file
// or a subagent's, then a row per problem met.
func (r *Reader) Session(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	state := &session{reader: r, artifact: artifact, options: options, emit: emit,
		scan: prescanFile(artifact), seen: map[string]map[seenKey]bool{},
		modes: parser.Tracker{}, models: parser.Tracker{}, bridges: map[string]bridgeKey{}}
	state.lastCwd, state.lastBranch = state.scan.cwd, state.scan.branch
	state.headerDone, state.taskDone = true, true
	if match := subagentPattern.FindStringSubmatch(artifact.Rel); match != nil {
		state.fileAgentID = match[1]
		state.headerDone, state.taskDone = false, state.scan.fork
	}
	for line, record := range parser.JSONLines(artifact.DiskPath, &state.problems) {
		state.record(line, record)
	}
	parser.EmitAll(emit, r.ErrorRows(artifact, state.problems, state.sessionID, ""))
}

// firstSeen reports whether value is new for the current session in the
// named set, and adds it.
func (s *session) firstSeen(set, value string) bool {
	key := seenKey{session: s.sessionID, value: value}
	if s.seen[set] == nil {
		s.seen[set] = map[seenKey]bool{}
	}
	if s.seen[set][key] {
		return false
	}
	s.seen[set][key] = true
	return true
}

// base returns a row of the current record at the given time, with the
// session, project and branch the records so far set.
func (s *session) base(timestamp string) timeline.Row {
	row := s.reader.BaseRow(s.artifact)
	row.SourceLine = s.line
	row.TimestampUTC = timestamp
	row.SessionID = s.sessionID
	row.ProjectPath = s.relocated
	if row.ProjectPath == "" {
		row.ProjectPath = s.lastCwd
	}
	row.GitBranch = s.lastBranch
	return row
}

// row returns a row of the current record at its own time, with the turn
// type and text given (text compacted).
func (s *session) row(turnType, text string) timeline.Row {
	row := s.base(s.timestamp)
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// stateRow returns a system row of the current record at its time, or the
// last time seen when it has none.
func (s *session) stateRow(text string) timeline.Row {
	row := s.base(timeutil.FirstNonEmpty(s.timestamp, s.lastTimestamp))
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(text)
	return row
}

// isTranscriptType reports whether a record type is one of the transcript's
// own: a turn, a system event or an attachment.
func isTranscriptType(recordType string) bool {
	switch recordType {
	case "user", "assistant", "system", "attachment":
		return true
	}
	return false
}

// typeOf returns a record's type, "" when it is not a string.
func typeOf(record jsonvalue.Value) string {
	return record.Get("type").StringValue()
}

// record reads one transcript record.
func (s *session) record(line int, record jsonvalue.Value) {
	s.line = line
	if sessionID := record.Get("sessionId"); !sessionID.IsEmpty() {
		s.sessionID = sessionID.Text()
	} else if sessionID := record.Get("session_id"); !sessionID.IsEmpty() {
		s.sessionID = sessionID.Text()
	}
	s.timestamp = timeutil.FromValue(record.Get("timestamp"))
	if s.timestamp != "" {
		s.lastTimestamp = s.timestamp
	}
	if cwd := record.Get("cwd"); !cwd.IsEmpty() {
		s.lastCwd = cwd.Text()
	}
	if branch := record.Get("gitBranch"); !branch.IsEmpty() {
		s.lastBranch = branch.Text()
	}
	recordType := typeOf(record)
	if !s.headerDone && isTranscriptType(recordType) {
		s.headerDone = true
		agentID := s.fileAgentID
		if recorded := record.Get("agentId"); !recorded.IsEmpty() {
			agentID = recorded.Text()
		}
		header := s.base(s.timestamp)
		header.TurnType = timeline.TurnSystem
		header.Text = "subagent " + agentID + " of " + s.sessionID
		s.emit(header)
	}
	s.stateRows(record, recordType)
	s.content(record, recordType)
}

// stateRows emits the rows of what a record sets for its session: the
// surface it was written from, its origin, the permission mode and the
// model.
func (s *session) stateRows(record jsonvalue.Value, recordType string) {
	if entrypoint := record.Get("entrypoint"); entrypoint.IsString() && entrypoint.StringValue() != "" &&
		entrypoint.StringValue() != s.surface {
		text := "session surface: " + entrypoint.StringValue()
		if s.surface != "" {
			text = "session surface: " + s.surface + " -> " + entrypoint.StringValue()
		}
		s.emit(s.stateRow(text))
		s.surface = entrypoint.StringValue()
	}
	if isTranscriptType(recordType) {
		if origin := originText(record, s.scan); origin != "" && s.firstSeen("origin", origin) {
			s.emit(s.stateRow(origin))
			if strings.HasSuffix(origin, "(teleport, remoteSourced)") {
				// The conversation is held on the vendor's servers and
				// replayed here, so the session is synced as well.
				s.emit(s.stateRow(teleportSynced))
			}
		}
	}
	if recordType == "user" || recordType == "permission-mode" {
		if mode := record.Get("permissionMode"); mode.IsString() {
			if text := s.modes.Change("permission mode", s.sessionID, mode.StringValue()); text != "" {
				s.emit(s.stateRow(text))
			}
		}
	}
	if recordType == "assistant" {
		s.modelRow(record)
	}
}

// originText returns the "session origin:" text a record marks, or "": an
// sdk-cli record in a Remote Control spawn's file, a remoteSourced record
// (a teleported cloud turn), or a record of the VS Code extension.
func originText(record jsonvalue.Value, scan *prescan) string {
	entrypoint := record.Get("entrypoint")
	if parser.IsText(entrypoint, "sdk-cli") && scan.sdkSpawn {
		return timeline.OriginPrefix + " (Remote Control, entrypoint sdk-cli)"
	}
	if flag, ok := record.Get("remoteSourced").Bool(); ok && flag {
		return timeline.OriginPrefix + " (teleport, remoteSourced)"
	}
	if parser.IsText(entrypoint, "claude-vscode") {
		return "session origin: local (VS Code extension)"
	}
	return ""
}

// modelRow emits the "model:" row of a session's first or changed
// message.model, with the model asked for when it differs.
func (s *session) modelRow(record jsonvalue.Value) {
	model := parser.Text(record.Get("message").Get("model"))
	if model == "" || model == "<synthetic>" || !record.Get("isApiErrorMessage").IsEmpty() {
		return
	}
	text := s.models.Change("model", s.sessionID, model)
	if text == "" {
		return
	}
	if requested := record.Get("requestedModel"); parser.Present(requested) && requested.Text() != model {
		text += " requested=" + requested.Text()
	}
	row := s.row(timeline.TurnSystem, text)
	row.Model = model
	s.emit(row)
}

// content emits the rows of a record's own content, by its type.
func (s *session) content(record jsonvalue.Value, recordType string) {
	switch recordType {
	case "user":
		s.user(record, !s.taskDone)
		s.taskDone = true
	case "assistant":
		s.assistant(record)
	case "system":
		s.system(record)
	case "attachment":
		s.attachment(record)
	case "continued-in":
		s.emit(s.stateRow("session continued in: " + cmp.Or(parser.Text(record.Get("continuedInSessionId")), "-")))
	case "bridge-session":
		s.bridge(record)
	case "ai-title", "custom-title", "agent-name":
		title := parser.Text(record.Get(titleField(recordType)))
		if title != "" && s.firstSeen("title", title) {
			s.emit(s.stateRow("session title: " + title))
		}
	case "relocated":
		if cwd := parser.Text(record.Get("relocatedCwd")); cwd != "" {
			s.relocated = cwd
			s.emit(s.stateRow("cwd changed: " + cwd))
		}
	case "pr-link":
		url := parser.Text(record.Get("prUrl"))
		if url == "" {
			url = parser.Text(record.Get("prNumber"))
		}
		if s.firstSeen("pr", url) {
			s.emit(s.stateRow("pr-link: " + url))
		}
	case "queue-operation":
		s.queue(record)
	}
}

// titleField returns the field that holds the title of a title record.
func titleField(recordType string) string {
	switch recordType {
	case "ai-title":
		return "aiTitle"
	case "custom-title":
		return "customTitle"
	}
	return "agentName"
}

// system emits the row of a system record: its subtype and its content,
// else the record's own fields as JSON.
func (s *session) system(record jsonvalue.Value) {
	subtype := parser.Text(record.Get("subtype"))
	if subtype == "" {
		subtype = "system"
	}
	detail := parser.TextOf(record.Get("content"))
	switch {
	case subtype == "turn_duration":
		detail = record.Get("durationMs").Text() + " ms, " + record.Get("messageCount").Text() + " messages"
	case subtype == "scheduled_task_fire":
		s.emit(s.row(timeline.TurnSystem, "scheduled task fired: "+parser.SettingOrDash(record.Get("taskId"))+
			" cron="+parser.SettingOrDash(record.Get("cron"))))
		return
	case detail == "":
		if rest := withoutKeys(record, isSystemEnvelope); rest.Len() > 0 {
			detail = parser.JSONText(rest)
		}
	}
	text := subtype
	if detail != "" {
		text = subtype + ": " + detail
	}
	s.emit(s.row(timeline.TurnSystem, text))
}

// isSystemEnvelope reports whether a key of a system record is part of its
// envelope, left out of the record's detail JSON.
func isSystemEnvelope(key string) bool {
	switch key {
	case "type", "subtype", "uuid", "parentUuid", "logicalParentUuid", "timestamp", "sessionId",
		"session_id", "cwd", "gitBranch", "version", "isSidechain", "userType", "entrypoint",
		"slug", "agentId", "isMeta", "content", "sessionKind":
		return true
	}
	return false
}

// withoutKeys returns a new object holding the members of record whose key
// drop does not report, in order.
func withoutKeys(record jsonvalue.Value, drop func(string) bool) jsonvalue.Value {
	var kept []jsonvalue.Member
	for _, member := range record.Members() {
		if !drop(member.Key) {
			kept = append(kept, member)
		}
	}
	return jsonvalue.NewObject(kept...)
}

// queue emits the row of a queue-operation record: where an enqueued
// prompt was delivered, the first delivery after the record else the first
// of all, or the operation and its content. The deliveries are in line
// order, so the first after the record is found by a binary search.
func (s *session) queue(record jsonvalue.Value) {
	content := parser.TextOf(record.Get("content"))
	if parser.IsText(record.Get("operation"), "enqueue") {
		if lines := s.scan.delivered[content]; len(lines) > 0 {
			deliveredAt := lines[0]
			if after := sort.Search(len(lines), func(i int) bool {
				s.reader.Work.Add("deliveries looked at", 1)
				return lines[i] > s.line
			}); after < len(lines) {
				deliveredAt = lines[after]
			}
			s.emit(s.stateRow("prompt queued: delivered at line " + strconv.Itoa(deliveredAt)))
			return
		}
	}
	s.emit(s.stateRow("queue " + record.Get("operation").Text() + ": " + content))
}
