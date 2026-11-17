package cursor

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

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
		if len(entries) > 0 || !isText(current.data.Get("status"), "none") {
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

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// stateReader reads the conversations of the primary store.
type stateReader struct {
	owner     *Parser
	artifact  *input.Artifact
	store     *store
	keyValues *sqlitefile.Table
	options   parser.Options
	bubbles   map[string]keyed
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
// fullConversationHeadersOnly, else its composerConvHeaders chunks.
func (r *stateReader) entries(current *composer) []jsonvalue.Value {
	var entries []jsonvalue.Value
	if inline := current.data.Get("fullConversationHeadersOnly"); inline.IsArray() && inline.Len() > 0 {
		for _, entry := range inline.Items() {
			if entry.IsObject() {
				entries = append(entries, entry)
			}
		}
		return entries
	}
	if r.keyValues == nil {
		return nil
	}
	for _, row := range r.store.rows(prefixed(r.keyValues, "composerConvHeaders:"+current.ident+":")) {
		chunk := decodeRecord(row.rowid, row.key, row.value, &current.problems)
		for _, entry := range chunk.Items() {
			if entry.IsObject() {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// conversation emits the rows of a conversation: its name, its origin,
// each bubble's rows, and whether it is archived.
func (r *stateReader) conversation(stamp *stamper, entries []jsonvalue.Value) {
	current := stamp.composer
	if name := timeline.CompactValue(firstTruthy(current.data.Get("name"), current.head.Get("name"))); name != "" {
		stamp.add(timeline.TurnSystem, "session name: "+name, current.created(), current.dataRowid, "")
	}
	r.owner.origin(stamp)
	tracked, model := parser.Tracker{}, ""
	for _, entry := range entries {
		bubbleID := parser.Str(entry.Get("bubbleId"))
		key := "bubbleId:" + current.ident + ":" + bubbleID
		found, ok := r.bubbles[key]
		if bubbleID == "" || !ok {
			stamp.add(timeline.TurnSystem, "parser: missing bubble "+bubbleID,
				timeutil.FromValue(entry.Get("createdAt")), 0, "")
			continue
		}
		bubble := objectOf(decodeRecord(found.rowid, key, found.value, &current.problems))
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
	case equalsOne(firstTruthy(bubble.Get("type"), entry.Get("type"))):
		change := tracked.Change("model", ident, parser.Str(bubble.Get("modelInfo").Get("modelName")))
		model = tracked[ident]
		when := timeutil.FromValue(firstTruthy(bubble.Get("createdAt"), entry.Get("createdAt")))
		if change != "" {
			stamp.add(timeline.TurnSystem, change, when, rowid, model)
		}
		if text := timeline.CompactValue(bubble.Get("text")); text != "" {
			stamp.add(timeline.TurnUser, text, when, rowid, model)
		}
	case bubble.Get("toolFormerData").IsObject():
		r.owner.tool(stamp, bubble, entry, rowid, model)
	default:
		when := timeutil.FromValue(firstTruthy(bubble.Get("createdAt"), bubble.Get("startedAtMs"),
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

// equalsOne reports whether value == 1 in Python: 1, 1.0 or true.
func equalsOne(value jsonvalue.Value) bool {
	if flag, ok := value.Bool(); ok {
		return flag
	}
	number, ok := value.Float()
	return ok && number == 1
}

// origin emits "session origin: remote <type>" for a conversation whose
// agent ran anywhere but locally.
func (p *Parser) origin(stamp *stamper) {
	kind := parser.Str(stamp.composer.head.Get("agentLocation").Get("type"))
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
	draft := objectOf(decodeRecord(row.rowid, row.key, row.value, &stamp.composer.problems))
	if text := timeline.CompactValue(draft.Get("text")); text != "" {
		stamp.add(timeline.TurnSystem, "unsent draft: "+text, stamp.composer.created(), row.rowid, "")
	}
}

// blobs emits user and assistant rows from the JSON agentKv blobs that
// name a deleted composer; a blob that is not JSON is protobuf, not an
// error.
func (r *stateReader) blobs(deleted []*composer, emit parser.Emit) {
	for _, row := range r.store.rows(prefixed(r.keyValues, "agentKv:blob:")) {
		blob, err := jsonvalue.DecodeString(asText(row.value))
		if err != nil || !blob.Has("content") {
			continue
		}
		content := blob.Get("content")
		haystack := textOrJSON(content)
		for _, current := range deleted {
			if !strings.Contains(haystack, current.ident) {
				continue
			}
			stamp := r.owner.newStamper(r.artifact, current, emit)
			role := parser.Str(blob.Get("role"))
			text := timeline.Compact(parser.TextOf(content))
			if role == "user" || role == "assistant" {
				made := stamp.row(role, text, "", row.rowid, "")
				made.Recovered = "yes"
				emit(made)
				continue
			}
			label := "system prompt"
			if role != "system" {
				label = orDash(role) + " message"
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
