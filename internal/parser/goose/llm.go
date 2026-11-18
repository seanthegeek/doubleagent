package goose

import (
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// llmLog is the state of one request log being read: the last request's
// row, the rows after it, the model of the last request, and the message
// being assembled from streamed chunks (its first chunk's row, its id, its
// role and its content blocks so far).
type llmLog struct {
	parser   *Parser
	artifact *input.Artifact
	reader   *blockReader
	request  *timeline.Row
	pending  []timeline.Row
	model    string
	current  *assembly
}

// assembly is a message being assembled from streamed chunks.
type assembly struct {
	base      timeline.Row
	messageID string
	role      string
	blocks    []jsonvalue.Value
}

// flush adds the rows of the message assembled so far.
func (l *llmLog) flush() {
	if l.current == nil {
		return
	}
	l.reader.blocks(l.current.base, l.current.role, jsonvalue.NewArray(l.current.blocks...), true, nil,
		func(row timeline.Row) { l.pending = append(l.pending, row) })
	l.current = nil
}

// parseLLMLog emits the rows of a request log: the last request's row
// first, then the messages and errors in order, then the bad lines.
func (p *Parser) parseLLMLog(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	log := &llmLog{parser: p, artifact: artifact, reader: &blockReader{parser: p, artifact: artifact,
		options: options, toolNames: map[string]string{}}}
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		log.record(number, record)
	}
	log.flush()
	if log.request != nil {
		emit(*log.request)
	}
	parser.EmitAll(emit, log.pending)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// record reads one line of the log.
func (l *llmLog) record(number int, record jsonvalue.Value) {
	if record.Has("input") || record.Has("model_config") {
		l.flush()
		l.requestRow(number, record)
		return
	}
	data := record.Get("data")
	switch {
	case data.IsObject() && data.Has("content"):
		l.chunk(number, data)
	case !record.Get("error").IsEmpty():
		l.flush()
		failure := record.Get("error")
		text := parser.JSONText(failure)
		if failure.IsString() {
			text = failure.StringValue()
		}
		row := l.parser.row(l.artifact, number, "", "", l.model, "")
		row.TurnType, row.Text = timeline.TurnSystem, timeline.Compact("llm error: "+text)
		l.pending = append(l.pending, row)
	}
}

// requestRow replaces the request row with the one of a request line,
// whose model the rows after it take.
func (l *llmLog) requestRow(number int, record jsonvalue.Value) {
	config := orObject(record.Get("model_config"))
	request := record.Get("input")
	if !request.IsObject() {
		request = jsonvalue.NewObject()
	}
	l.model = parser.Text(config.Get("model_name"))
	if l.model == "" {
		l.model = parser.Text(request.Get("model"))
	}
	messages := parser.Elements(request.Get("messages"))
	lastUser := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if message := messages[i]; message.IsObject() && isText(message.Get("role"), "user") {
			lastUser = parser.TextOf(message.Get("content"))
			break
		}
	}
	if lastUser != "" {
		lastUser = "last user: " + lastUser
	}
	row := l.parser.row(l.artifact, number, "", "", l.model, "")
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(parser.JoinNonEmpty(" | ", "llm request", strconv.Itoa(len(messages))+" messages", lastUser))
	l.request = &row
}

// chunk reads one streamed chunk: a chunk of the message being assembled
// is merged into it, any other starts a new message. A chunk without
// content (usage or metadata) adds nothing.
func (l *llmLog) chunk(number int, data jsonvalue.Value) {
	content := textBlocks(data.Get("content"))
	if !content.IsArray() || content.Len() == 0 {
		return
	}
	messageID := parser.Text(data.Get("id"))
	if l.current != nil && messageID != "" && l.current.messageID == messageID {
		l.current.blocks = mergeDelta(l.current.blocks, content.Items())
		return
	}
	l.flush()
	base := l.parser.row(l.artifact, number, "", "", l.model, epochJSON(data.Get("created")))
	if l.request != nil && l.request.TimestampUTC == "" {
		l.request.TimestampUTC = base.TimestampUTC
	}
	role := parser.Text(data.Get("role"))
	if role == "" {
		role = timeline.TurnAssistant
	}
	l.current = &assembly{base: base, messageID: messageID, role: role,
		blocks: append([]jsonvalue.Value(nil), content.Items()...)}
}

// mergeDelta appends one streamed chunk's blocks to the message assembled
// so far, as Goose's conversation does for chunks with the same message
// id: a one-block text chunk extends a last text block of the same
// audience, a one-block thinking chunk extends a last thinking block that
// is unsigned or has its signature, and anything else is appended.
func mergeDelta(blocks, added []jsonvalue.Value) []jsonvalue.Value {
	if len(added) != 1 || len(blocks) == 0 || !blocks[len(blocks)-1].IsObject() || !added[0].IsObject() {
		return append(blocks, added...)
	}
	last, block := blocks[len(blocks)-1], added[0]
	kind := last.Get("type")
	switch {
	case isText(kind, "text") && isText(block.Get("type"), "text") &&
		jsonvalue.Equal(audience(last), audience(block)):
		blocks[len(blocks)-1] = withMember(last, "text",
			jsonvalue.NewString(parser.Text(last.Get("text"))+parser.Text(block.Get("text"))))
	case isText(kind, "thinking") && isText(block.Get("type"), "thinking") &&
		(last.Get("signature").IsEmpty() || jsonvalue.Equal(block.Get("signature"), last.Get("signature"))):
		merged := withMember(last, "thinking",
			jsonvalue.NewString(parser.Text(last.Get("thinking"))+parser.Text(block.Get("thinking"))))
		if signature := block.Get("signature"); !signature.IsEmpty() {
			merged = withMember(merged, "signature", signature)
		}
		blocks[len(blocks)-1] = merged
	default:
		return append(blocks, added...)
	}
	return blocks
}

// audience returns a block's annotations.audience, JSON null when there is
// none.
func audience(block jsonvalue.Value) jsonvalue.Value {
	if annotations := block.Get("annotations"); annotations.IsObject() {
		return annotations.Get("audience")
	}
	return jsonvalue.NewNull()
}

// withMember returns a copy of an object with key set to value, in the
// key's place or at the end.
func withMember(object jsonvalue.Value, key string, value jsonvalue.Value) jsonvalue.Value {
	members := slices.Clone(object.Members())
	for i := range members {
		if members[i].Key == key {
			members[i].Value = value
			return jsonvalue.NewObject(members...)
		}
	}
	return jsonvalue.NewObject(append(members, jsonvalue.Member{Key: key, Value: value})...)
}
