package goose

import (
	"slices"
	"strconv"
	"strings"

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
	blocks    *streamed
}

// flush adds the rows of the message assembled so far.
func (l *llmLog) flush() {
	if l.current == nil {
		return
	}
	l.reader.blocks(l.current.base, l.current.role, jsonvalue.NewArray(l.current.blocks.values()...), true, nil,
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
		if message := messages[i]; message.IsObject() && parser.IsText(message.Get("role"), "user") {
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
		l.current.blocks.merge(content.Items())
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
		blocks: &streamed{blocks: append([]jsonvalue.Value(nil), content.Items()...), work: l.parser.Work}}
}

// streamed is the content blocks of a message assembled from streamed
// chunks. The text that chunks add to the last block is gathered in text
// and set on the block once, when another block follows or the blocks are
// read, so a message of many chunks is copied once, not once per chunk.
type streamed struct {
	blocks []jsonvalue.Value
	// field is the member of the last block that text extends, "" when
	// no chunk has extended it; text starts with the member's own text.
	field string
	text  strings.Builder
	// signature is the signature a thinking chunk gave the last block,
	// when signed is set.
	signature jsonvalue.Value
	signed    bool
	work      parser.Work
}

// merge appends one streamed chunk's blocks to the message assembled so
// far, as Goose's conversation does for chunks with the same message id: a
// one-block text chunk extends a last text block of the same audience, a
// one-block thinking chunk extends a last thinking block that is unsigned
// or has its signature, and anything else is appended.
func (s *streamed) merge(added []jsonvalue.Value) {
	if len(added) != 1 || len(s.blocks) == 0 || !s.blocks[len(s.blocks)-1].IsObject() || !added[0].IsObject() {
		s.close()
		s.blocks = append(s.blocks, added...)
		return
	}
	last, block := s.blocks[len(s.blocks)-1], added[0]
	kind := last.Get("type")
	signature := last.Get("signature")
	if s.signed {
		signature = s.signature
	}
	switch {
	case parser.IsText(kind, "text") && parser.IsText(block.Get("type"), "text") &&
		jsonvalue.Equal(audience(last), audience(block)):
		s.extend(last, "text", parser.Text(block.Get("text")))
	case parser.IsText(kind, "thinking") && parser.IsText(block.Get("type"), "thinking") &&
		(signature.IsEmpty() || jsonvalue.Equal(block.Get("signature"), signature)):
		s.extend(last, "thinking", parser.Text(block.Get("thinking")))
		if added := block.Get("signature"); !added.IsEmpty() {
			s.signature, s.signed = added, true
		}
	default:
		s.close()
		s.blocks = append(s.blocks, added...)
	}
}

// extend adds text to the named member of the last block.
func (s *streamed) extend(last jsonvalue.Value, field, text string) {
	if s.field == "" {
		s.field = field
		s.text.WriteString(parser.Text(last.Get(field)))
	}
	s.text.WriteString(text)
}

// close sets the text and signature gathered for the last block on it.
func (s *streamed) close() {
	if s.field == "" {
		return
	}
	s.work.Add("bytes copied", s.text.Len())
	last := withMember(s.blocks[len(s.blocks)-1], s.field, jsonvalue.NewString(s.text.String()))
	if s.signed {
		last = withMember(last, "signature", s.signature)
	}
	s.blocks[len(s.blocks)-1] = last
	s.field, s.signed = "", false
	s.text.Reset()
}

// values returns the blocks assembled so far.
func (s *streamed) values() []jsonvalue.Value {
	s.close()
	return s.blocks
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
