package geminicli

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// stored is one message of a session's replay: the message as last
// written, the line that wrote it, the session it was first written
// under, and its place in the replay's order.
type stored struct {
	message   jsonvalue.Value
	line      int
	sessionID string
	place     int
	// patched is set once a patch has replaced the message with its own
	// copy, which later patches change in place; calls finds that copy's
	// tool calls by the key of their id (parserkit.ValueKey).
	patched bool
	calls   map[string]jsonvalue.Value
}

// placement is an id and a place it took in the order. A placement whose
// id has since moved or gone is stale and is skipped.
type placement struct {
	messageID string
	place     int
}

// messageLog holds the replayed messages by id, in the order the CLI's
// recording service keeps them. The order is a place number per message,
// growing with each message added or moved to the end, so a record costs
// no rewrite of the order: placed lists the places in the order they were
// taken, and the messages in force are its entries that are not stale.
type messageLog struct {
	byID   map[string]*stored
	placed []placement
	next   int
	// work counts the placements read, which a test holds to linear.
	work parserkit.Work
}

// newMessageLog returns an empty log that counts its work in work.
func newMessageLog(work parserkit.Work) messageLog {
	return messageLog{byID: map[string]*stored{}, work: work}
}

// place gives a message the next place, at the end of the order.
func (m *messageLog) place(messageID string, entry *stored) {
	entry.place = m.next
	m.placed = append(m.placed, placement{messageID, m.next})
	m.next++
}

// live reports whether a placement is the current place of its message.
func (m *messageLog) live(each placement) bool {
	entry, ok := m.byID[each.messageID]
	return ok && entry.place == each.place
}

// put records a message: a new id goes last, an id already present keeps
// its place and session and takes the new message and line.
func (m *messageLog) put(message jsonvalue.Value, line int, sessionID string) {
	messageID := message.Get("id").StringValue()
	if entry, ok := m.byID[messageID]; ok {
		entry.message, entry.line, entry.patched, entry.calls = message, line, false, nil
		return
	}
	entry := &stored{message: message, line: line, sessionID: sessionID}
	m.byID[messageID] = entry
	m.place(messageID, entry)
}

// drop removes the messages of the ids present and returns how many it
// removed.
func (m *messageLog) drop(ids []string) int {
	removed := 0
	for _, messageID := range ids {
		if _, ok := m.byID[messageID]; ok {
			delete(m.byID, messageID)
			removed++
		}
	}
	return removed
}

// dropFrom removes the target message and every one after it, or every
// message when the target is not present, and returns how many it removed.
// The placements from the target's on are read once and let go.
func (m *messageLog) dropFrom(target string) int {
	cut := 0
	if entry, ok := m.byID[target]; ok {
		cut, _ = slices.BinarySearchFunc(m.placed, entry.place, func(each placement, place int) int {
			return cmp.Compare(each.place, place)
		})
	}
	removed := 0
	for _, each := range m.placed[cut:] {
		m.work.Add("placements read", 1)
		if m.live(each) {
			delete(m.byID, each.messageID)
			removed++
		}
	}
	m.placed = m.placed[:cut]
	return removed
}

// moveToEnd moves each id present to the end, in turn: the ids moved end
// up after the others, in the order of their last mention.
func (m *messageLog) moveToEnd(ids []string) {
	for _, messageID := range ids {
		if entry, ok := m.byID[messageID]; ok {
			m.place(messageID, entry)
		}
	}
}

// order returns the ids of the messages in force, in replay order.
func (m *messageLog) order() []string {
	ids := make([]string, 0, len(m.byID))
	for _, each := range m.placed {
		m.work.Add("placements read", 1)
		if m.live(each) {
			ids = append(ids, each.messageID)
		}
	}
	return ids
}

// event is a system row the replay gives beside the messages: a session
// start, a summary, a rewind or a removal.
type event struct {
	line                       int
	timestamp, sessionID, text string
}

// replay replays the records of one session file as the CLI does: the
// messages in force, the starts and events, the session id, the latest
// time seen, the projectHash of the metadata and the problems of the
// members of the wrong kind its records hold.
type replay struct {
	messages       messageLog
	starts, events []event
	sessionID      string
	latest         string
	projectHash    jsonvalue.Value
	problems       parserkit.Problems
	// work counts the tool call lookups of patches and the placements read,
	// which a test holds to the size of the patches and to linear.
	work parserkit.Work
}

// newReplay returns an empty replay that counts its work in work.
func newReplay(work parserkit.Work) *replay {
	return &replay{messages: newMessageLog(work), work: work}
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
	dropped := r.messages.dropFrom(target)
	r.events = append(r.events, event{line, r.latest, r.sessionID,
		"rewind to " + target + ": " + strconv.Itoa(dropped) + " message(s) dropped"})
}

// patch applies a $patch: its own message change and those of its
// updates, then the removals and the new order of the ids it names. An
// updates, removeIds or orderIds that is not an array adds a problem, and
// so does a toolCalls of the wrong kind, once however many updates hold it
// so.
func (r *replay) patch(line int, patch jsonvalue.Value) {
	var tally parserkit.EntryTally
	if patch.Get("id").IsString() {
		r.applyPatch(patch, "$patch.toolCalls", &tally)
	}
	for _, update := range parserkit.EntriesOfKind(patch.Get("updates"), "$patch.updates", jsonvalue.Object, line,
		&r.problems) {
		if update.Get("id").IsString() {
			r.applyPatch(update, "$patch.updates[].toolCalls", &tally)
		}
	}
	tally.Report(line, &r.problems)
	removeIDs := parserkit.ListOfKind(patch.Get("removeIds"), "$patch.removeIds", jsonvalue.String, line, &r.problems)
	orderIDs := parserkit.ListOfKind(patch.Get("orderIds"), "$patch.orderIds", jsonvalue.String, line, &r.problems)
	if removed := texts(removeIDs); len(removed) > 0 {
		dropped := r.messages.drop(removed)
		r.events = append(r.events, event{line, r.latest, r.sessionID,
			"patch removed " + strconv.Itoa(dropped) + " message(s)"})
	}
	r.messages.moveToEnd(texts(orderIDs))
}

// texts returns the string items of a list.
func texts(items []jsonvalue.Value) []string {
	var out []string
	for _, item := range items {
		if item.IsString() {
			out = append(out, item.StringValue())
		}
	}
	return out
}

// applyPatch changes the message a patch names, when present: its content
// when the patch has one, and the result of each of its tool calls the
// patch gives a result for. The message as recorded is never changed: the
// first patch replaces it with a copy, tool calls included, which later
// patches change in place, so a patch costs work in proportion to its own
// size, not the message's. A toolCalls of the patch of the wrong kind is
// counted in tally, naming it as member, whether or not the message is
// present.
func (r *replay) applyPatch(patch jsonvalue.Value, member string, tally *parserkit.EntryTally) {
	toolPatches := tally.ListOfKind(patch.Get("toolCalls"), member, jsonvalue.Object)
	entry, ok := r.messages.byID[patch.Get("id").StringValue()]
	if !ok {
		return
	}
	if !entry.patched {
		entry.message, entry.calls = patchableCopy(entry.message)
		entry.patched = true
	}
	message := entry.message
	if content := patch.Get("content"); !content.IsNull() {
		message.Set("content", content)
	}
	calls := message.Get("toolCalls")
	if len(toolPatches) == 0 || !parserkit.IsText(message.Get("type"), "gemini") || !calls.IsArray() {
		return
	}
	for _, toolPatch := range toolPatches {
		result, ok := toolPatch.Lookup("result")
		if !ok {
			continue
		}
		if call, found := r.patchedCall(entry, toolPatch.Get("id")); found {
			call.Set("result", result)
		}
	}
}

// patchableCopy returns a copy of a message whose tool calls are copies
// too, and the first of those calls of each id, by the id's key: ids are
// compared by value, so 1 and 1.0 name one call, and an absent id is null.
func patchableCopy(message jsonvalue.Value) (jsonvalue.Value, map[string]jsonvalue.Value) {
	copied := parserkit.CopyObject(message)
	calls := message.Get("toolCalls")
	if !calls.IsArray() {
		return copied, nil
	}
	byID := map[string]jsonvalue.Value{}
	items := make([]jsonvalue.Value, 0, calls.Len())
	for _, call := range calls.Items() {
		if call.IsObject() {
			call = parserkit.CopyObject(call)
			key := parserkit.ValueKey(call.Get("id"))
			if _, seen := byID[key]; !seen {
				byID[key] = call
			}
		}
		items = append(items, call)
	}
	copied.Set("toolCalls", jsonvalue.NewArray(items...))
	return copied, byID
}

// patchedCall returns the first tool call of a patched message whose id
// equals callID, found by the key of the id.
func (r *replay) patchedCall(entry *stored, callID jsonvalue.Value) (jsonvalue.Value, bool) {
	r.work.Add("tool call lookups", 1)
	call, ok := entry.calls[parserkit.ValueKey(callID)]
	return call, ok
}

// set applies a $set: a new session id, a new message list, metadata, and
// a summary row at the update's time, else the latest time seen. A
// messages that is not an array adds a problem and leaves the list as it
// was.
func (r *replay) set(line int, changes jsonvalue.Value) {
	if named := changes.Get("sessionId"); named.IsString() && named.StringValue() != "" {
		r.sessionID = named.StringValue()
	}
	messages := changes.Get("messages")
	list := parserkit.ListOfKind(messages, "$set.messages", jsonvalue.Object, line, &r.problems)
	if messages.IsArray() {
		r.messages = newMessageLog(r.work)
		r.putAll(list, line)
	}
	r.metadata(changes)
	if summary := changes.Get("summary"); !summary.IsEmpty() {
		timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(changes.Get("lastUpdated")), r.latest)
		r.events = append(r.events, event{line, timestamp, r.sessionID, "summary: " + summary.Text()})
	}
}

// putAll records each message of a list of objects that has a string id.
func (r *replay) putAll(messages []jsonvalue.Value, line int) {
	for _, message := range messages {
		if message.Get("id").IsString() {
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
// messages it holds; a messages that is not an array adds a problem.
func (r *replay) header(line int, record jsonvalue.Value) {
	r.metadata(record)
	if named := record.Get("sessionId").StringValue(); named != "" {
		r.sessionID = named
	}
	timestamp := timeutil.FromValue(record.Get("startTime"))
	r.seen(timestamp)
	kind := parserkit.Text(record.Get("kind"))
	if kind == "" {
		kind = "main"
	}
	text := "session start: kind=" + kind + " projectHash=" + parserkit.Text(record.Get("projectHash"))
	if directories := record.Get("directories"); !directories.IsEmpty() {
		text += " directories=" + parserkit.JSONText(directories)
	}
	r.starts = append(r.starts, event{line, timestamp, r.sessionID, text})
	r.putAll(parserkit.ListOfKind(record.Get("messages"), "messages", jsonvalue.Object, line, &r.problems), line)
}
