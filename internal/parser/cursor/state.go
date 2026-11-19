package cursor

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
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
	emit     parser.Emit
}

// newStamper returns the stamper of a composer.
func (p *Parser) newStamper(artifact *input.Artifact, current *composer, emit parser.Emit) *stamper {
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

// state emits the rows of the primary store: each composer's rows, the
// rows the deleted composers' blobs recover, then the settings,
// integrations, cloud agents, MCP servers and plugins of ItemTable.
func (p *Parser) state(artifact *input.Artifact, store *store, options parser.Options, emit parser.Emit) {
	found := readComposers(store)
	keyValues := store.table("cursorDiskKV")
	drafts := map[string]keyed{}
	if keyValues != nil {
		for _, row := range store.rows(prefixed(keyValues, "composerDraft:")) {
			ident := row.key[len("composerDraft:"):]
			drafts[ident] = row
			found.get(ident)
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
	if len(deleted) > 0 && keyValues != nil {
		reader.blobs(deleted, emit)
	}
	var problems parser.Problems
	if reader.strayChunks > 0 {
		problems.Add(0, strayChunksText(reader.strayChunks))
	}
	items := p.itemRows(artifact, store, &problems)
	items.standing(emit)
	items.integrations(emit)
	items.cloud(emit)
	items.mcp(emit)
	items.plugins(emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// composer emits the rows of one composer: its conversation (a draft never
// sent has none) or, when only its header is left, its deletion, then its
// unsent draft and its problems. It reports whether it was deleted.
func (r *stateReader) composer(current *composer, drafts map[string]keyed, emit parser.Emit) bool {
	stamp := r.owner.newStamper(r.artifact, current, emit)
	deleted := false
	if current.hasData {
		entries := r.entries(current)
		if len(entries) > 0 || !parser.IsText(current.data.Get("status"), "none") {
			r.conversation(stamp, entries)
		}
	} else if !current.head.Get("agentLocationHistory").IsEmpty() || current.archived() {
		deleted = true
		r.owner.deleted(stamp)
	}
	if draft, ok := drafts[current.ident]; ok {
		r.owner.draft(stamp, draft)
	}
	parser.EmitAll(emit, r.owner.ErrorRows(r.artifact, current.problems, current.ident, stamp.path))
	return deleted
}

// stateReader reads the conversations of the primary store.
type stateReader struct {
	owner     *Parser
	artifact  *input.Artifact
	store     *store
	keyValues *sqlitefile.Table
	options   parser.Options
	bubbles   map[string]keyed
	// chunks holds the composerConvHeaders rows by composer id, each
	// composer's in key order, once chunksRead is set; strayChunks counts
	// the rows whose key names no composer.
	chunks      map[string][]keyed
	chunksRead  bool
	strayChunks int
}

// loadBubbles reads every bubbleId row once, the first row of each key in
// key order, for the conversations to look up.
func (r *stateReader) loadBubbles() {
	r.bubbles = map[string]keyed{}
	if r.keyValues == nil {
		return
	}
	for _, row := range r.store.rows(prefixed(r.keyValues, "bubbleId:")) {
		if _, seen := r.bubbles[row.key]; !seen {
			r.bubbles[row.key] = row
		}
	}
}

// entries returns the conversation's header list: composerData's inline
// fullConversationHeadersOnly, else its composerConvHeaders chunks. An
// inline list or a chunk that is not a JSON array is a problem.
func (r *stateReader) entries(current *composer) []jsonvalue.Value {
	var entries []jsonvalue.Value
	inline := current.data.Get("fullConversationHeadersOnly")
	inlineEntries := parser.EntriesOfKind(inline, "fullConversationHeadersOnly", jsonvalue.Object,
		int(current.dataRowid), &current.problems)
	if inline.IsArray() && inline.Len() > 0 {
		for _, entry := range inlineEntries {
			entries = append(entries, entry)
		}
		return entries
	}
	for _, row := range r.headerChunks(current.ident) {
		for _, entry := range objectEntries(row, &current.problems) {
			entries = append(entries, entry)
		}
	}
	return entries
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
		rows := r.store.rows(prefixed(r.keyValues, chunkPrefix))
		slices.SortStableFunc(rows, func(left, right keyed) int { return strings.Compare(left.key, right.key) })
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

// strayChunksText returns the problem text of n composerConvHeaders rows
// whose key has no chunk number, so no composer reads them.
func strayChunksText(n int) string {
	if n == 1 {
		return "1 " + strings.TrimSuffix(chunkPrefix, ":") + " key has no chunk number and is not read"
	}
	return strconv.Itoa(n) + " " + strings.TrimSuffix(chunkPrefix, ":") + " keys have no chunk number and are not read"
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

// conversation emits the rows of a conversation: its name, its origin,
// each bubble's rows, and whether it is archived.
func (r *stateReader) conversation(stamp *stamper, entries []jsonvalue.Value) {
	current := stamp.composer
	if name := timeline.CompactValue(firstOrLast(current.data.Get("name"), current.head.Get("name"))); name != "" {
		stamp.add(timeline.TurnSystem, "session name: "+name, current.created(), current.dataRowid, "")
	}
	r.owner.origin(stamp)
	tracked, model := parser.Tracker{}, ""
	for _, entry := range entries {
		bubbleID := parser.Text(entry.Get("bubbleId"))
		key := "bubbleId:" + current.ident + ":" + bubbleID
		found, ok := r.bubbles[key]
		if bubbleID == "" || !ok {
			stamp.add(timeline.TurnSystem, "parser: missing bubble "+bubbleID,
				timeutil.FromValue(entry.Get("createdAt")), 0, "")
			continue
		}
		bubble := parser.ObjectOrEmpty(decodeRecord(found.rowid, key, found.value, &current.problems))
		if bubble.Len() == 0 {
			continue
		}
		model = r.bubble(stamp, bubble, entry, found.rowid, tracked, model)
	}
	if current.archived() {
		stamp.add(timeline.TurnSystem, "conversation archived", current.headerUpdated(), current.dataRowid, model)
	}
}

// bubble emits the rows of one bubble: a prompt (with a model change), a
// tool call, or a reply with its reasoning. It returns the model in use.
func (r *stateReader) bubble(stamp *stamper, bubble, entry jsonvalue.Value, rowid int64, tracked parser.Tracker,
	model string) string {
	ident := stamp.composer.ident
	switch {
	case equalsOne(firstOrLast(bubble.Get("type"), entry.Get("type"))):
		change := tracked.Change("model", ident, parser.Text(bubble.Get("modelInfo").Get("modelName")))
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

// equalsOne reports whether value is the number 1, by value (1 or 1.0).
func equalsOne(value jsonvalue.Value) bool {
	return jsonvalue.Equal(value, jsonvalue.NewInt(1))
}

// origin emits "session origin: remote <type>" for a conversation whose
// agent ran anywhere but locally.
func (p *Parser) origin(stamp *stamper) {
	kind := parser.Text(stamp.composer.head.Get("agentLocation").Get("type"))
	if kind != "" && kind != "local" {
		stamp.add(timeline.TurnSystem, "session origin: remote "+kind, stamp.composer.created(), 0, "")
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

// draft emits a composer's unsent draft.
func (p *Parser) draft(stamp *stamper, row keyed) {
	draft := parser.ObjectOrEmpty(decodeRecord(row.rowid, row.key, row.value, &stamp.composer.problems))
	if text := timeline.CompactValue(draft.Get("text")); text != "" {
		stamp.add(timeline.TurnSystem, "unsent draft: "+text, stamp.composer.created(), row.rowid, "")
	}
}

// blobs emits user and assistant rows from the JSON agentKv blobs that
// name a deleted composer; a blob that is not JSON is protobuf, not an
// error.
func (r *stateReader) blobs(deleted []*composer, emit parser.Emit) {
	idents := make([]string, len(deleted))
	for i, current := range deleted {
		idents[i] = current.ident
	}
	named := newTextSet(idents, r.store.work)
	for _, row := range r.store.rows(prefixed(r.keyValues, "agentKv:blob:")) {
		blob, err := jsonvalue.DecodeString(asText(row.value))
		if err != nil || !blob.Has("content") {
			continue
		}
		content := blob.Get("content")
		for _, position := range named.find(parser.SettingValue(content)) {
			current := deleted[position]
			stamp := r.owner.newStamper(r.artifact, current, emit)
			role := parser.Text(blob.Get("role"))
			text := timeline.Compact(parser.TextOf(content))
			if role == "user" || role == "assistant" {
				made := stamp.row(role, text, "", row.rowid, "")
				made.Recovered = "yes"
				emit(made)
				continue
			}
			label := "system prompt"
			if role != "system" {
				label = cmp.Or(role, "-") + " message"
			}
			stamp.add(timeline.TurnSystem, label+": "+text, "", row.rowid, "")
		}
	}
}

// keyedValue returns the value of the first row whose key is text, false
// when there is none.
func keyedValue(store *store, table *sqlitefile.Table, text string) (keyed, bool) {
	rows := store.rows(keyedRows(table, keyEquals(text)))
	if len(rows) == 0 {
		return keyed{}, false
	}
	return rows[0], true
}
