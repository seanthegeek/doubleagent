package geminicli

import (
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// stored is one message of a session's replay: the message as last
// written, the line that wrote it, and the session it was first written
// under.
type stored struct {
	message   jsonvalue.Value
	line      int
	sessionID string
}

// messageLog holds the replayed messages by id, in the order the CLI's
// recording service keeps them.
type messageLog struct {
	order []string
	byID  map[string]*stored
}

// put records a message: a new id goes last, an id already present keeps
// its place and session and takes the new message and line.
func (m *messageLog) put(message jsonvalue.Value, line int, sessionID string) {
	messageID := message.Get("id").StringValue()
	if entry, ok := m.byID[messageID]; ok {
		entry.message, entry.line = message, line
		return
	}
	m.order = append(m.order, messageID)
	m.byID[messageID] = &stored{message: message, line: line, sessionID: sessionID}
}

// drop removes the messages of the ids present and returns how many it
// removed.
func (m *messageLog) drop(ids []string) int {
	removed := map[string]bool{}
	for _, messageID := range ids {
		if _, ok := m.byID[messageID]; ok {
			delete(m.byID, messageID)
			removed[messageID] = true
		}
	}
	if len(removed) > 0 {
		m.order = slices.DeleteFunc(m.order, func(messageID string) bool { return removed[messageID] })
	}
	return len(removed)
}

// moveToEnd moves each id present to the end, in turn: the ids moved end
// up after the others, in the order of their last mention.
func (m *messageLog) moveToEnd(ids []string) {
	last := map[string]int{}
	for i, messageID := range ids {
		if _, ok := m.byID[messageID]; ok {
			last[messageID] = i
		}
	}
	if len(last) == 0 {
		return
	}
	moved := make([]string, 0, len(last))
	for messageID := range last {
		moved = append(moved, messageID)
	}
	slices.SortFunc(moved, func(left, right string) int { return last[left] - last[right] })
	m.order = append(slices.DeleteFunc(m.order, func(messageID string) bool {
		_, ok := last[messageID]
		return ok
	}), moved...)
}

// event is a system row the replay gives beside the messages: a session
// start, a summary, a rewind or a removal.
type event struct {
	line                       int
	timestamp, sessionID, text string
}

// replay replays the records of one session file as the CLI does: the
// messages in force, the starts and events, the session id, the latest
// time seen and the projectHash of the metadata.
type replay struct {
	messages       messageLog
	starts, events []event
	sessionID      string
	latest         string
	projectHash    jsonvalue.Value
}

// newReplay returns an empty replay.
func newReplay() *replay {
	return &replay{messages: messageLog{byID: map[string]*stored{}}}
}

// seen keeps the latest of the times seen so far.
func (r *replay) seen(timestamp string) {
	r.latest = max(r.latest, timestamp)
}

// apply replays one record, by its form: a rewind, a patch, a message, a
// metadata update or a session header.
func (r *replay) apply(line int, record jsonvalue.Value) {
	switch {
	case record.Get("$rewindTo").IsString():
		r.rewind(line, record.Get("$rewindTo").StringValue())
	case record.Get("$patch").IsObject():
		r.patch(line, record.Get("$patch"))
	case record.Get("id").IsString():
		r.seen(timeutil.FromValue(record.Get("timestamp")))
		r.messages.put(record, line, r.sessionID)
	case record.Get("$set").IsObject():
		r.set(line, record.Get("$set"))
	case record.Get("sessionId").IsString():
		r.header(line, record)
	}
}

// rewind drops the target message and every one after it, or every
// message when the target is not present.
func (r *replay) rewind(line int, target string) {
	cut := max(slices.Index(r.messages.order, target), 0)
	dropped := r.messages.drop(slices.Clone(r.messages.order[cut:]))
	r.events = append(r.events, event{line, r.latest, r.sessionID,
		"rewind to " + target + ": " + strconv.Itoa(dropped) + " message(s) dropped"})
}

// patch applies a $patch: its own message change and those of its
// updates, then the removals and the new order of the ids it names.
func (r *replay) patch(line int, patch jsonvalue.Value) {
	if patch.Get("id").IsString() {
		r.applyPatch(patch)
	}
	for _, update := range patch.Get("updates").Items() {
		if update.IsObject() && update.Get("id").IsString() {
			r.applyPatch(update)
		}
	}
	if removed := texts(patch.Get("removeIds")); len(removed) > 0 {
		dropped := r.messages.drop(removed)
		r.events = append(r.events, event{line, r.latest, r.sessionID,
			"patch removed " + strconv.Itoa(dropped) + " message(s)"})
	}
	r.messages.moveToEnd(texts(patch.Get("orderIds")))
}

// texts returns the string items of an array.
func texts(list jsonvalue.Value) []string {
	var out []string
	for _, item := range list.Items() {
		if item.IsString() {
			out = append(out, item.StringValue())
		}
	}
	return out
}

// applyPatch changes the message a patch names, when present: its content
// when the patch has one, and the result of each of its tool calls the
// patch gives a result for. The message is copied, never changed in place.
func (r *replay) applyPatch(patch jsonvalue.Value) {
	entry, ok := r.messages.byID[patch.Get("id").StringValue()]
	if !ok {
		return
	}
	message := copyObject(entry.message)
	if content := patch.Get("content"); !content.IsNull() {
		message.Set("content", content)
	}
	calls := entry.message.Get("toolCalls")
	if patch.Get("toolCalls").IsArray() && isText(message.Get("type"), "gemini") && calls.IsArray() {
		copied := make([]jsonvalue.Value, 0, calls.Len())
		for _, call := range calls.Items() {
			if call.IsObject() {
				call = copyObject(call)
			}
			copied = append(copied, call)
		}
		for _, toolPatch := range patch.Get("toolCalls").Items() {
			result, ok := toolPatch.Lookup("result")
			if !toolPatch.IsObject() || !ok {
				continue
			}
			index := slices.IndexFunc(copied, func(call jsonvalue.Value) bool {
				return call.IsObject() && equal(call.Get("id"), toolPatch.Get("id"))
			})
			if index >= 0 {
				copied[index].Set("result", result)
			}
		}
		message.Set("toolCalls", jsonvalue.NewArray(copied...))
	}
	entry.message = message
}

// copyObject returns a new object holding an object's members, so a
// change to it leaves the original as it was.
func copyObject(object jsonvalue.Value) jsonvalue.Value {
	return jsonvalue.NewObject(object.Members()...)
}

// set applies a $set: a new session id, a new message list, metadata, and
// a summary row at the update's time, else the latest time seen.
func (r *replay) set(line int, changes jsonvalue.Value) {
	if named := changes.Get("sessionId"); named.IsString() && named.StringValue() != "" {
		r.sessionID = named.StringValue()
	}
	if messages := changes.Get("messages"); messages.IsArray() {
		r.messages = messageLog{byID: map[string]*stored{}}
		r.putAll(messages, line)
	}
	r.metadata(changes)
	if summary := changes.Get("summary"); !summary.IsEmpty() {
		timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(changes.Get("lastUpdated")), r.latest)
		r.events = append(r.events, event{line, timestamp, r.sessionID, "summary: " + summary.Text()})
	}
}

// putAll records each message of a list that is an object with a string
// id.
func (r *replay) putAll(messages jsonvalue.Value, line int) {
	for _, message := range messages.Items() {
		if message.IsObject() && message.Get("id").IsString() {
			r.seen(timeutil.FromValue(message.Get("timestamp")))
			r.messages.put(message, line, r.sessionID)
		}
	}
}

// metadata takes the projectHash of a $set or a session header, when it
// names one.
func (r *replay) metadata(record jsonvalue.Value) {
	if hash, ok := record.Lookup("projectHash"); ok {
		r.projectHash = hash
	}
}

// header applies a session header (or a legacy whole-session record): the
// session id, a "session start:" row and, in the legacy form, the
// messages it holds.
func (r *replay) header(line int, record jsonvalue.Value) {
	r.metadata(record)
	if named := record.Get("sessionId").StringValue(); named != "" {
		r.sessionID = named
	}
	timestamp := timeutil.FromValue(record.Get("startTime"))
	r.seen(timestamp)
	kind := parser.Str(record.Get("kind"))
	if kind == "" {
		kind = "main"
	}
	text := "session start: kind=" + kind + " projectHash=" + parser.Str(record.Get("projectHash"))
	if directories := record.Get("directories"); !directories.IsEmpty() {
		text += " directories=" + parser.CompactJSON(directories)
	}
	r.starts = append(r.starts, event{line, timestamp, r.sessionID, text})
	r.putAll(record.Get("messages"), line)
}
