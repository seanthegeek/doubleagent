package cursor

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textset"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// chunkPrefix starts the key of a chunk of a composer's header list.
const chunkPrefix = "composerConvHeaders:"

// The texts of the rows of a deleted conversation.
const (
	deletedText = "conversation deleted (header kept; time is the header's lastUpdatedAt)"
	// The delete sets the header's isArchived, so a deleted header reads
	// archived although the conversation never was.
	archivedByDelete = " (isArchived is set by the delete)"
	stoppedText      = "permission decision: stopped by the user (reviewData.status None," +
		" additionalData.status rejected)"
)

// sortByRowid sorts key-value rows by rowid.
func sortByRowid(rows []keyed) {
	slices.SortStableFunc(rows, func(left, right keyed) int {
		switch {
		case left.rowid < right.rowid:
			return -1
		case left.rowid > right.rowid:
			return 1
		}
		return strings.Compare(left.key, right.key)
	})
}

// stamper emits the rows of one composer: each takes its id, project and
// branch.
type stamper struct {
	owner    *Parser
	artifact *input.Artifact
	composer *composer
	path     string
	branch   string
	emit     parserkit.Emit
}

// newStamper returns the stamper of a composer.
func (p *Parser) newStamper(artifact *input.Artifact, current *composer, emit parserkit.Emit) *stamper {
	path, branch := current.project()
	return &stamper{owner: p, artifact: artifact, composer: current, path: path, branch: branch, emit: emit}
}

// row returns a row of the composer.
func (s *stamper) row(turnType, text, timestamp string, line int64, model string) timeline.Row {
	row := s.owner.row(s.artifact, turnType, text, timestamp, line)
	row.SessionID, row.ProjectPath, row.GitBranch, row.Model = s.composer.ident, s.path, s.branch, model
	return row
}

// add emits a row of the composer.
func (s *stamper) add(turnType, text, timestamp string, line int64, model string) {
	s.emit(s.row(turnType, text, timestamp, line, model))
}

// sent reports whether a composerData row of the composer has a status
// other than none, which only a draft never sent has.
func sent(current *composer) bool {
	for _, stored := range current.stored {
		if !parserkit.IsText(stored.value.Get("status"), "none") {
			return true
		}
	}
	return false
}

// entryCount returns the number of entries of the lists.
func entryCount(lists []entryList) int {
	count := 0
	for _, list := range lists {
		count += len(list.entries)
	}
	return count
}

// chunkComposer returns the composer id a chunk key names, the key being
// composerConvHeaders:<composerId>:<NNNNNN> with a chunk number of ASCII
// digits; false for a key of any other form.
func chunkComposer(key string) (string, bool) {
	rest, found := strings.CutPrefix(key, chunkPrefix)
	cut := strings.LastIndexByte(rest, ':')
	if !found || cut < 0 || cut == len(rest)-1 || strings.Trim(rest[cut+1:], "0123456789") != "" {
		return "", false
	}
	return rest[:cut], true
}

// entryList is the header list of one composerData row, with the rowid
// and the problems its repeated entries are counted in.
type entryList struct {
	entries  []jsonvalue.Value
	rowid    int64
	problems *parserkit.Problems
}

// headerChunks returns a composer's composerConvHeaders:<id>:<NNNNNN> rows
// in key order. Every such row is read once, on first use, sorted by key
// and filed under the composer its key names, so a composer whose id is the
// start of another's (a and a:b) never takes the other's chunks; a row whose
// key has no chunk number names no composer and is only counted.
func (r *stateReader) headerChunks(ident string) []keyed {
	if r.keyValues == nil {
		return nil
	}
	if !r.chunksRead {
		r.chunksRead = true
		rows := r.store.rows(prefixed(r.keyValues, chunkPrefix, "key"))
		r.chunks = map[string][]keyed{}
		for _, row := range rows {
			if owner, ok := chunkComposer(row.key); ok {
				r.chunks[owner] = append(r.chunks[owner], row)
			} else {
				r.strayChunks++
			}
		}
	}
	return r.chunks[ident]
}

// stateReader reads the conversations of the primary store.
type stateReader struct {
	owner     *Parser
	artifact  *input.Artifact
	store     *store
	keyValues *sqlitefile.Table
	options   parserkit.Options
	bubbles   map[string][]keyed
	// chunks holds the composerConvHeaders rows by composer id, each
	// composer's in key order, once chunksRead is set; strayChunks counts
	// the rows whose key names no composer.
	chunks      map[string][]keyed
	chunksRead  bool
	strayChunks int
}

// entryLists returns the conversation's header lists, one per
// composerData row in rowid order: the row's inline
// fullConversationHeadersOnly, else the composer's composerConvHeaders
// chunks, read once for the first row without an inline list. An inline
// list or a chunk that is not a JSON array is a problem.
func (r *stateReader) entryLists(current *composer) []entryList {
	var lists []entryList
	chunked := false
	for _, stored := range current.stored {
		inline := stored.value.Get("fullConversationHeadersOnly")
		inlineEntries := parserkit.EntriesOfKind(inline, "fullConversationHeadersOnly", jsonvalue.Object,
			int(stored.rowid), stored.problems)
		list := entryList{rowid: stored.rowid, problems: stored.problems}
		if inline.IsArray() && inline.Len() > 0 {
			for _, entry := range inlineEntries {
				list.entries = append(list.entries, entry)
			}
			lists = append(lists, list)
			continue
		}
		if chunked {
			continue
		}
		chunked, list.problems = true, current.problems
		for _, row := range r.headerChunks(current.ident) {
			for _, entry := range objectEntries(r.artifact, row, current.problems) {
				list.entries = append(list.entries, entry)
			}
		}
		lists = append(lists, list)
	}
	return lists
}

// equalsOne reports whether value is the number 1, by value (1 or 1.0).
func equalsOne(value jsonvalue.Value) bool {
	return jsonvalue.Equal(value, jsonvalue.NewInt(1))
}

// bubble emits the rows of one bubble: a prompt (with a model change), a
// tool call, or a reply with its reasoning. It returns the model in use.
func (r *stateReader) bubble(stamp *stamper, bubble, entry jsonvalue.Value, rowid int64, tracked parserkit.Tracker,
	model string) string {
	ident := stamp.composer.ident
	switch {
	case equalsOne(firstOrLast(bubble.Get("type"), entry.Get("type"))):
		change := tracked.Change("model", ident, parserkit.Text(bubble.Get("modelInfo").Get("modelName")))
		model = tracked[ident]
		when := timeutil.FromValue(firstOrLast(bubble.Get("createdAt"), entry.Get("createdAt")))
		if change != "" {
			stamp.add(timeline.TurnSystem, change, when, rowid, model)
		}
		if text := timeline.CompactValue(bubble.Get("text")); text != "" {
			stamp.add(timeline.TurnUser, text, when, rowid, model)
		}
	case bubble.Get("toolFormerData").IsObject():
		r.owner.tool(stamp, bubble, entry, rowid, model)
	default:
		when := timeutil.FromValue(firstOrLast(bubble.Get("createdAt"), bubble.Get("startedAtMs"),
			entry.Get("createdAt")))
		thinking := timeline.CompactValue(bubble.Get("thinking").Get("text"))
		if thinking != "" && r.options.IncludeThinking {
			stamp.add(timeline.TurnThinking, thinking, when, rowid, model)
		}
		if text := timeline.CompactValue(bubble.Get("text")); text != "" {
			stamp.add(timeline.TurnAssistant, text, when, rowid, model)
		}
	}
	return model
}

// repeatedBubble emits the rows of a bubble key cursorDiskKV holds in more
// than one row, which only damage or an edit writes, the key being
// UNIQUE: each stored row gives a parser: row saying so and its own rows,
// reading the model in force before them. Nothing takes from a stored row
// but its own rows, so when one names another model, the model after them
// is none until a later bubble names one, and the parser: rows say so.
func (r *stateReader) repeatedBubble(stamp *stamper, entry jsonvalue.Value, found []keyed, tracked parserkit.Tracker,
	model string) string {
	ident := stamp.composer.ident
	before, hasBefore := tracked[ident]
	bubbles := make([]jsonvalue.Value, len(found))
	moved := false
	for i, stored := range found {
		bubbles[i] = parserkit.ObjectOrEmpty(decodeRecord(r.artifact, stored.rowid, stored.key, stored.value, stamp.composer.problems))
		named := parserkit.Text(bubbles[i].Get("modelInfo").Get("modelName"))
		if equalsOne(firstOrLast(bubbles[i].Get("type"), entry.Get("type"))) && named != "" && named != before {
			moved = true
		}
	}
	notice := repeatedKeyText("cursorDiskKV", len(found))
	if moved {
		notice += modelNotLent
	}
	for i, stored := range found {
		when := timeutil.FromValue(firstOrLast(bubbles[i].Get("createdAt"), bubbles[i].Get("startedAtMs"),
			entry.Get("createdAt")))
		stamp.add(timeline.TurnSystem, notice, when, stored.rowid, "")
		if bubbles[i].Len() == 0 {
			continue
		}
		own := parserkit.Tracker{}
		if hasBefore {
			own[ident] = before
		}
		r.bubble(stamp, bubbles[i], entry, stored.rowid, own, model)
	}
	if !moved {
		return model
	}
	delete(tracked, ident)
	return ""
}

// entry emits the rows of the bubble a header entry names, or a missing
// bubble row, and returns the model in use.
func (r *stateReader) entry(stamp *stamper, entry jsonvalue.Value, bubbleID string, tracked parserkit.Tracker,
	model string) string {
	current := stamp.composer
	key := "bubbleId:" + current.ident + ":" + bubbleID
	found := r.bubbles[key]
	if bubbleID == "" || len(found) == 0 {
		stamp.add(timeline.TurnSystem, "parser: missing bubble "+bubbleID,
			timeutil.FromValue(entry.Get("createdAt")), 0, "")
		return model
	}
	if len(found) > 1 {
		return r.repeatedBubble(stamp, entry, found, tracked, model)
	}
	bubble := parserkit.ObjectOrEmpty(decodeRecord(r.artifact, found[0].rowid, key, found[0].value, current.problems))
	if bubble.Len() == 0 {
		return model
	}
	return r.bubble(stamp, bubble, entry, found[0].rowid, tracked, model)
}

// repeatedEntriesText returns the problem text of n header entries that
// name a bubble an earlier entry of the same list names.
func repeatedEntriesText(n int) string {
	if n == 1 {
		return "1 header entry names a bubble an earlier entry names"
	}
	return strconv.Itoa(n) + " header entries name a bubble an earlier entry names"
}

// origin emits "session origin: remote <type>" for a conversation whose
// agent ran anywhere but locally.
func (p *Parser) origin(stamp *stamper) {
	kind := parserkit.Text(stamp.composer.head.Get("agentLocation").Get("type"))
	if kind != "" && kind != "local" {
		stamp.add(timeline.TurnSystem, "session origin: remote "+kind, stamp.composer.created(), 0, "")
	}
}

// conversation emits the rows of a conversation: each view's name and
// origin, each bubble's rows, and each view's archive. Each bubble is read
// once, at the first header entry naming it; the later entries of the
// same list naming it give no row and are counted as one problem, and
// those of a later list (a repeated composerData key) give none.
func (r *stateReader) conversation(views []view, stamp *stamper, lists []entryList) {
	for _, found := range views {
		shown := found.composer
		if name := timeline.CompactValue(firstOrLast(shown.data.Get("name"), shown.head.Get("name"))); name != "" {
			found.stamp.add(timeline.TurnSystem, "session name: "+name, shown.created(), shown.dataRowid, "")
		}
		r.owner.origin(found.stamp)
	}
	tracked, model := parserkit.Tracker{}, ""
	met := map[string]int{}
	for i, list := range lists {
		repeats := 0
		for _, entry := range list.entries {
			bubbleID := parserkit.Text(entry.Get("bubbleId"))
			if seen := met[bubbleID]; seen != 0 {
				if seen == i+1 {
					repeats++
				}
				continue
			}
			met[bubbleID] = i + 1
			model = r.entry(stamp, entry, bubbleID, tracked, model)
		}
		if repeats > 0 {
			list.problems.Add(int(list.rowid), repeatedEntriesText(repeats))
		}
	}
	for _, found := range views {
		if shown := found.composer; shown.archived() {
			found.stamp.add(timeline.TurnSystem, "conversation archived", shown.headerUpdated(), shown.dataRowid,
				model)
		}
	}
}

// deleted emits the rows of a conversation deleted with its header kept.
func (p *Parser) deleted(stamp *stamper) {
	current := stamp.composer
	line := int64(0)
	if current.header != nil {
		line = current.header.Rowid
	}
	if name := timeline.CompactValue(current.head.Get("name")); name != "" {
		stamp.add(timeline.TurnSystem, "session name: "+name, current.created(), line, "")
	}
	p.origin(stamp)
	text := deletedText
	if current.archived() {
		text += archivedByDelete
	}
	stamp.add(timeline.TurnSystem, text, current.headerUpdated(), line, "")
}

// draft emits the unsent draft of one composerDraft row.
func (p *Parser) draft(stamp *stamper, row keyed) {
	draft := parserkit.ObjectOrEmpty(decodeRecord(stamp.artifact, row.rowid, row.key, row.value, stamp.composer.problems))
	if text := timeline.CompactValue(draft.Get("text")); text != "" {
		stamp.add(timeline.TurnSystem, "unsent draft: "+text, stamp.composer.created(), row.rowid, "")
	}
}

// drafts emits a composer's unsent draft. A composerDraft key cursorDiskKV
// holds in more than one row, which only damage or an edit writes, the
// key being UNIQUE, gives each stored row's draft after a parser: row
// saying so; nothing takes from a draft.
func (p *Parser) drafts(stamp *stamper, stored []keyed) {
	if len(stored) == 1 {
		p.draft(stamp, stored[0])
		return
	}
	for _, row := range stored {
		stamp.add(timeline.TurnSystem, repeatedKeyText("cursorDiskKV", len(stored)), stamp.composer.created(),
			row.rowid, "")
		p.draft(stamp, row)
	}
}

// composer emits the rows of one composer: its conversation (a draft never
// sent has none) or, when only its header is left, its deletion, then its
// unsent draft and its problems. It reports whether it was deleted. The
// rows of each stored row of a repeated id are its view's (repeat.go).
func (r *stateReader) composer(current *composer, drafts map[string][]keyed, emit parserkit.Emit) bool {
	views := r.views(current, emit)
	stamp := r.owner.memberStamp(r.artifact, current, emit, timeline.TakeProject|timeline.TakeBranch)
	deleted := false
	if current.hasData {
		lists := r.entryLists(current)
		if entryCount(lists) > 0 || sent(current) {
			r.conversation(views, stamp, lists)
		}
	} else {
		for _, found := range views {
			if shown := found.composer; !shown.head.Get("agentLocationHistory").IsEmpty() || shown.archived() {
				deleted = true
				r.owner.deleted(found.stamp)
			}
		}
	}
	if stored := drafts[current.ident]; len(stored) > 0 {
		r.owner.drafts(r.owner.memberStamp(r.artifact, current, emit,
			timeline.TakeTime|timeline.TakeProject|timeline.TakeBranch), stored)
	}
	for _, found := range views {
		r.ownProblems(found)
	}
	problems := r.owner.memberStamp(r.artifact, current, emit, timeline.TakeProject)
	parserkit.EmitAll(problems.emit, r.owner.ErrorRows(r.artifact, *current.problems, current.ident, stamp.path))
	return deleted
}

// loadBubbles reads every bubbleId row once, the rows of each key in
// rowid order, for the conversations to look up.
func (r *stateReader) loadBubbles() {
	r.bubbles = map[string][]keyed{}
	if r.keyValues == nil {
		return
	}
	for _, row := range r.store.rows(prefixed(r.keyValues, "bubbleId:")) {
		r.bubbles[row.key] = append(r.bubbles[row.key], row)
	}
	for _, rows := range r.bubbles {
		if len(rows) > 1 {
			sortByRowid(rows)
		}
	}
}

// strayChunksText returns the problem text of n composerConvHeaders rows
// whose key has no chunk number, so no composer reads them.
func strayChunksText(n int) string {
	if n == 1 {
		return "1 " + strings.TrimSuffix(chunkPrefix, ":") + " key has no chunk number and is not read"
	}
	return strconv.Itoa(n) + " " + strings.TrimSuffix(chunkPrefix, ":") + " keys have no chunk number and are not read"
}

// blobLabel returns the turn type and the label of the rows of a blob of
// role: a user or assistant message, else a system row labelled by the
// role, which is cut (carried) since every composer's row repeats it.
func blobLabel(role string, carried *parserkit.Carried) (string, string) {
	switch role {
	case "user", "assistant":
		return role, ""
	case "system":
		return timeline.TurnSystem, "system prompt: "
	}
	return timeline.TurnSystem, cmp.Or(carried.Cut("agentKv blob role", role), "-") + " message: "
}

// blobPointer is the pointer of a later row of an agentKv blob: the row of
// the same source line under another session the blob names holds its
// text.
func blobPointer(holder *timeline.Row, _ bool) string {
	return "[agentKv blob text shown in full above, under another session at source line " +
		strconv.Itoa(holder.SourceLine) + "]"
}

// blob emits the row of one JSON agentKv blob, whose text is shared, in a
// composer: a user or assistant message recovered, or a system row with
// the label of any other role.
func (r *stateReader) blob(stamp *stamper, turnType, label string, shared *timeline.SharedText, rowid int64) {
	made := stamp.row(turnType, label, "", rowid, "")
	if shared.Text() != "" {
		var out timeline.TextBuilder
		out.Write(label)
		out.Share(shared, "[agentKv blob text]", blobPointer)
		out.Set(&made)
	}
	if turnType != timeline.TurnSystem {
		made.Recovered = "yes"
	}
	stamp.emit(made)
}

// blobs emits user and assistant rows from the JSON agentKv blobs that
// name a deleted composer; a blob that is not JSON is protobuf, not an
// error. Each deleted composer a blob names gets its row, and the rows
// share the blob's text: the output writes it in full in the first of them
// it keeps and a pointer to that row in each later one, so the text is
// never repeated. The role in each row's label is cut once per blob
// (parserkit.Carried), the cuts counted in problems.
func (r *stateReader) blobs(deleted []*composer, problems *parserkit.Problems, emit parserkit.Emit) {
	carried := parserkit.NewCarried(r.artifact)
	defer carried.Report(0, problems)
	idents := make([]string, len(deleted))
	for i, current := range deleted {
		idents[i] = current.ident
	}
	named := textset.New(idents, r.store.work)
	for _, row := range r.store.rows(prefixed(r.keyValues, "agentKv:blob:", "key")) {
		blob, err := parserkit.DecodeJSON(r.artifact, row.value.UTF8Text())
		if err != nil || !blob.Has("content") {
			continue
		}
		content := blob.Get("content")
		text := timeline.NewSharedText(timeline.Compact(parserkit.TextOf(content)))
		turnType, label := blobLabel(parserkit.Text(blob.Get("role")), carried)
		for _, position := range named.Find(parserkit.SettingValue(content)) {
			stamp := r.owner.memberStamp(r.artifact, deleted[position], emit, timeline.TakeProject|timeline.TakeBranch)
			r.blob(stamp, turnType, label, text, row.rowid)
		}
	}
}

// state emits the rows of the primary store: each composer's rows, the
// rows the deleted composers' blobs recover, then the settings,
// integrations, cloud agents, MCP servers and plugins of ItemTable.
func (p *Parser) state(artifact *input.Artifact, store *store, options parserkit.Options, emit parserkit.Emit) {
	found := readComposers(store)
	p.assignGroups(sharedState(artifact), found)
	keyValues := store.table("cursorDiskKV")
	drafts := map[string][]keyed{}
	if keyValues != nil {
		for _, row := range store.rows(prefixed(keyValues, "composerDraft:", "key")) {
			ident := row.key[len("composerDraft:"):]
			drafts[ident] = append(drafts[ident], row)
			found.get(ident)
		}
		for _, stored := range drafts {
			sortByRowid(stored)
		}
	}
	reader := &stateReader{owner: p, artifact: artifact, store: store, keyValues: keyValues, options: options}
	reader.loadBubbles()
	var deleted []*composer
	for _, current := range found.list {
		if reader.composer(current, drafts, emit) {
			deleted = append(deleted, current)
		}
	}
	var problems parserkit.Problems
	if len(deleted) > 0 && keyValues != nil {
		reader.blobs(deleted, &problems, emit)
	}
	if reader.strayChunks > 0 {
		problems.Add(0, strayChunksText(reader.strayChunks))
	}
	items := p.itemRows(artifact, store, &problems)
	items.standing(emit)
	items.integrations(emit)
	items.cloud(emit)
	items.mcp(emit)
	items.plugins(emit)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// keyedValues returns the rows whose key is text, in rowid order; more
// than one only damage or an edit writes, the key being UNIQUE.
func keyedValues(store *store, table *sqlitefile.Table, text string) []keyed {
	rows := store.rows(keyedRows(table, keyEquals(text)))
	sortByRowid(rows)
	return rows
}
