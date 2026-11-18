package codex

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The keys of .codex-global-state.json, the desktop app's own state
// (research/codex-cli.md, Other stores): the key holding the app's
// persisted UI atoms, the keys read inside it, and those at the top level.
const (
	atomKey           = "electron-persisted-atom-state"
	promptHistoryKey  = "prompt-history"
	descriptionsKey   = "thread-descriptions-v1"
	activityKey       = "thread-user-activity-times-v1"
	permissionsKey    = "heartbeat-thread-permissions-by-id"
	deletedPrefix     = "codex-writing-block-deleted-thread-v1:"
	selectionPrefix   = "permission-selection-by-host-id:"
	rootHintsKey      = "thread-workspace-root-hints"
	outputDirsKey     = "thread-projectless-output-directories"
	mobileDeviceKey   = "codex-mobile-has-connected-device"
	remoteEnvironment = "electron-local-remote-control-environment-id"
)

// hostSettingKeys are the atoms holding a setting per host.
var hostSettingKeys = []string{"agent-mode-by-host-id",
	"preferred-non-full-access-agent-mode-by-host-id"}

// asObject returns value when it is a JSON object, or the object a string
// holds; else an empty object.
func asObject(value jsonvalue.Value) jsonvalue.Value {
	if value.IsString() {
		decoded, err := jsonvalue.DecodeString(value.StringValue())
		if err != nil {
			return jsonvalue.NewObject()
		}
		value = decoded
	}
	return objectOr(value)
}

// objectIn returns asObject(value) for a member of the global state that
// the app writes as an object: a member present as anything else, or as a
// string that does not hold an object, adds a problem naming it.
func objectIn(value jsonvalue.Value, member string, problems *parser.Problems) jsonvalue.Value {
	held := value
	if value.IsString() {
		if decoded, err := jsonvalue.DecodeString(value.StringValue()); err == nil {
			held = decoded
		}
	}
	if !held.IsObject() && !held.IsNull() {
		problems.Add(0, parser.WrongKind(member, jsonvalue.Object))
	}
	return objectOr(held)
}

// activityByThread returns the last activity time per thread id, from the
// keys `["local","<id>"]` of the activity atom; an atom that is not an
// object adds a problem.
func activityByThread(atom jsonvalue.Value, problems *parser.Problems) jsonvalue.Value {
	out := jsonvalue.NewObject()
	for _, member := range objectIn(atom.Get(activityKey), activityKey, problems).Members() {
		parts, err := jsonvalue.DecodeString(member.Key)
		if err == nil && parts.IsArray() && parts.Len() == 2 && parts.Index(1).IsString() {
			out.Set(parts.Index(1).StringValue(), member.Value)
		}
	}
	return out
}

// promptEntry is one prompt-history entry: its thread, 1-based position in
// its list, and text.
type promptEntry struct {
	threadID string
	index    int
	text     string
}

// promptEntries returns the prompt-history entries with text, list by
// list. An entry of the "global" list, the composer's history across
// threads, has no thread and is left out when a thread's list in the same
// file has its text, since two rows of one file never pair in
// deduplication. A prompt-history that is not an object, or a list of it
// that is not an array, adds a problem.
func promptEntries(atom jsonvalue.Value, problems *parser.Problems) []promptEntry {
	var listed []promptEntry
	threaded := map[string]bool{}
	for _, member := range objectIn(atom.Get(promptHistoryKey), promptHistoryKey, problems).Members() {
		list := parser.ArrayMember(member.Value, promptHistoryKey+"."+member.Key, 0, problems)
		for i, entry := range list {
			markdown := entry.Get("markdown")
			if !markdown.IsString() || strings.TrimSpace(markdown.StringValue()) == "" {
				continue
			}
			listed = append(listed, promptEntry{member.Key, i + 1, markdown.StringValue()})
			if member.Key != "global" {
				threaded[markdown.StringValue()] = true
			}
		}
	}
	var entries []promptEntry
	for _, entry := range listed {
		switch {
		case entry.threadID != "global":
			entries = append(entries, entry)
		case !threaded[entry.text]:
			entries = append(entries, promptEntry{"", entry.index, entry.text})
		}
	}
	return entries
}

// standingText returns the "permission mode (standing):" text of a
// thread's standing permission entry.
func standingText(entry jsonvalue.Value, suffix string) string {
	profile := entry.Get("activePermissionProfile")
	if profile.IsObject() {
		profile = profile.Get("id")
	}
	pairs := []struct {
		label string
		value jsonvalue.Value
	}{
		{"approval", entry.Get("approvalPolicy")},
		{"sandbox", entry.Get("sandboxPolicy")},
		{"reviewer", entry.Get("approvalsReviewer")},
		{"profile_id", profile},
	}
	parts := make([]string, len(pairs))
	for i, pair := range pairs {
		shown := "-"
		if parser.Present(pair.value) {
			shown = parser.SettingValue(pair.value)
		}
		parts[i] = pair.label + "=" + shown
	}
	return "permission mode (standing): " + strings.Join(parts, " ") + " " + suffix
}

// stateText is the session and text of one file-time row of the global
// state.
type stateText struct {
	sessionID, text string
}

// globalStateTexts returns the file-time system rows of the global state:
// deleted threads, thread descriptions, standing permissions, host
// settings, the remote control state and the projectless directories,
// from the document, its atom and its output directories. A member of the
// atom read here that is not an object adds a problem.
func globalStateTexts(document, atom, outputs jsonvalue.Value, rel string,
	problems *parser.Problems) []stateText {
	source := func(key string) string { return "(from " + rel + " " + key + ", file mtime)" }
	setting := func(text string) stateText {
		return stateText{"", "permission setting: " + text + " (from " + rel + ", file mtime)"}
	}
	var texts []stateText
	for _, member := range atom.Members() {
		if flag, ok := member.Value.Bool(); ok && flag && strings.HasPrefix(member.Key, deletedPrefix) {
			sessionID := strings.TrimPrefix(member.Key, deletedPrefix)
			texts = append(texts, stateText{sessionID, "session deleted: " + sessionID + " " +
				source(strings.TrimSuffix(deletedPrefix, ":"))})
		}
	}
	texts = append(texts, descriptionTexts(atom, source(descriptionsKey), problems)...)
	for _, member := range objectIn(atom.Get(permissionsKey), permissionsKey, problems).Members() {
		if member.Value.IsObject() {
			texts = append(texts, stateText{member.Key,
				standingText(member.Value, source(permissionsKey))})
		}
	}
	for _, key := range hostSettingKeys {
		for _, member := range objectIn(atom.Get(key), key, problems).Members() {
			texts = append(texts, setting(key+"."+member.Key+"="+parser.SettingValue(member.Value)))
		}
	}
	for _, member := range atom.Members() {
		if strings.HasPrefix(member.Key, selectionPrefix) {
			texts = append(texts, setting(member.Key+"="+parser.SettingValue(member.Value)))
		}
	}
	return append(texts, remoteTexts(document, outputs, source)...)
}

// descriptionTexts returns a "thread description:" text per described
// thread, then per thread with only an activity time. Either atom that is
// not an object adds a problem.
func descriptionTexts(atom jsonvalue.Value, suffix string, problems *parser.Problems) []stateText {
	descriptions := objectIn(atom.Get(descriptionsKey), descriptionsKey, problems)
	activity := activityByThread(atom, problems)
	sessions := descriptions.Keys()
	for _, threadID := range activity.Keys() {
		if !descriptions.Has(threadID) {
			sessions = append(sessions, threadID)
		}
	}
	texts := make([]stateText, 0, len(sessions))
	for _, sessionID := range sessions {
		description := descriptions.Get(sessionID)
		shown := "-"
		if description.IsString() && description.StringValue() != "" {
			shown = description.StringValue()
		}
		texts = append(texts, stateText{sessionID, "thread description: " + shown +
			" last_activity=" + cmp.Or(timeutil.FromValue(activity.Get(sessionID)), "-") + " " + suffix})
	}
	return texts
}

// remoteTexts returns the remote control row of the two top-level keys,
// when either is present, and a scratch directory row per projectless
// thread's output directory of outputs.
func remoteTexts(document, outputs jsonvalue.Value, source func(string) string) []stateText {
	var texts []stateText
	mobile, hasMobile := document.Lookup(mobileDeviceKey)
	if hasMobile || document.Has(remoteEnvironment) {
		connected := "-"
		if hasMobile {
			connected = parser.SettingValue(mobile)
		}
		texts = append(texts, stateText{"", "remote control: mobile device connected=" + connected +
			" environment=" + cmp.Or(parser.Text(document.Get(remoteEnvironment)), "-") + " " +
			source(mobileDeviceKey)})
	}
	for _, member := range outputs.Members() {
		if member.Value.IsString() && member.Value.StringValue() != "" {
			texts = append(texts, stateText{member.Key, timeline.ScratchPrefix + " " +
				member.Value.StringValue() + " (projectless thread " + member.Key + ") " +
				source(outputDirsKey)})
		}
	}
	return texts
}

// readGlobalState returns the global state's JSON object, or false after
// emitting the rows that say why it cannot be read.
func (p *Parser) readGlobalState(artifact *input.Artifact, emit parser.Emit) (jsonvalue.Value, bool) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return jsonvalue.NewNull(), false
	}
	document, err := jsonvalue.Decode(content)
	if err != nil {
		emit(p.ProblemRow(artifact, "unreadable global state: "+err.Error()))
		return jsonvalue.NewNull(), false
	}
	if !document.IsObject() {
		emit(p.ProblemRow(artifact, "global state is not a JSON object"))
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// parseGlobalState emits the rows of .codex-global-state.json: the app's
// prompt history first, as user rows with no time, then the system rows at
// the file's modification time, then a problem row per member of the wrong
// kind.
func (p *Parser) parseGlobalState(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.readGlobalState(artifact, emit)
	if !ok {
		return
	}
	var problems parser.Problems
	atom := objectIn(document.Get(atomKey), atomKey, &problems)
	roots := asObject(document.Get(rootHintsKey))
	outputs := objectIn(document.Get(outputDirsKey), outputDirsKey, &problems)
	stamp := func(sessionID string) timeline.Row {
		row := p.stamp(artifact, nil, sessionID)
		if sessionID != "" {
			row.ProjectPath = parser.Text(parser.FirstNonEmpty(roots.Get(sessionID), outputs.Get(sessionID)))
		}
		return row
	}
	for _, entry := range promptEntries(atom, &problems) {
		row := stamp(entry.threadID)
		row.TurnType = timeline.TurnUser
		row.SourceLine = entry.index
		row.Text = timeline.Compact(entry.text)
		emit(row)
	}
	mtime := parser.FileMtime(artifact)
	for _, found := range globalStateTexts(document, atom, outputs, artifact.Rel, &problems) {
		row := stamp(found.sessionID)
		row.TurnType = timeline.TurnSystem
		row.SourceLine = 1
		row.TimestampUTC = mtime
		row.Text = timeline.Compact(found.text)
		if strings.HasPrefix(found.text, timeline.ScratchPrefix) {
			// The row names the projectless directory itself, whatever root
			// hint the thread has.
			row.ProjectPath = outputs.Get(found.sessionID).StringValue()
		}
		emit(row)
	}
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, problem.Message))
	}
}
