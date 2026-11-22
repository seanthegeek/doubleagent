package openclaw

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/zed"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// errEventNotText is the reason a cold archive event whose event_json is
// neither text nor empty is unreadable.
var errEventNotText = errors.New("event_json is not text")

// zstdMagic starts every zstd frame.
const zstdMagic = "\x28\xb5\x2f\xfd"

// maybeZstd returns a blob decompressed within budget when it is a zstd
// frame, and anything else as it is; it fails only for a frame that does
// not decode or would pass the budget.
func maybeZstd(blob []byte, budget *zed.Budget) ([]byte, error) {
	if !bytes.HasPrefix(blob, []byte(zstdMagic)) {
		return blob, nil
	}
	return budget.Decompress(blob)
}

// numbered is one JSON object line of a blob and its line number.
type numbered struct {
	line   int
	record jsonvalue.Value
}

// blobLines returns the JSON object lines of data, numbered from 1. A line
// that does not decode, or holds another JSON value, adds an "unparseable
// line" problem with the line as raw text. Repeated member names are noted
// in the parse of artifact, whose file may hold many blobs.
func blobLines(artifact *input.Artifact, data []byte, problems *parser.Problems) []numbered {
	var records []numbered
	for line, record := range parser.JSONRecordsIn(artifact, parser.SplitLines(data), problems) {
		records = append(records, numbered{line, record})
	}
	return records
}

// records emits the rows of the lines of a JSONL transcript or of a cold
// archive, whose lines wrap each entry in an "event" record. A non-zero
// fixedLine is the source line of every row, the rowid of the database row
// that held the blob.
func (w *writer) records(current *session, records []numbered, fixedLine int) {
	for _, each := range records {
		line := each.line
		if fixedLine != 0 {
			line = fixedLine
		}
		record := each.record
		if !record.Has("kind") || record.Has("type") {
			w.entry(current, line, record, "")
			continue
		}
		kind := record.Get("kind")
		if parser.IsText(kind, "header") && current.sessionID == "" {
			current.sessionID = parser.Text(record.Get("sessionId"))
		}
		inner := record.Get("row")
		if !parser.IsText(kind, "event") {
			continue
		}
		if !inner.IsObject() {
			if !inner.IsNull() {
				w.bad(current, line, parser.WrongKind("row", jsonvalue.Object), "")
			}
			continue
		}
		createdAt := timeutil.FromValue(inner.Get("created_at"))
		entry, err := eventJSON(w.artifact, inner.Get("event_json"))
		if err != nil {
			w.bad(current, line, "cold archive event seq "+inner.Get("seq").Text()+
				" unreadable: "+err.Error(), createdAt)
			continue
		}
		switch {
		case entry.IsObject():
			w.entry(current, line, entry, createdAt)
		case !entry.IsNull():
			w.bad(current, line, parser.WrongKind("event_json", jsonvalue.Object), createdAt)
		}
	}
}

// eventJSON decodes the event_json text of a cold archive's event record;
// a value that is not text is an error. Repeated member names are noted in
// the parse of artifact.
func eventJSON(artifact *input.Artifact, value jsonvalue.Value) (jsonvalue.Value, error) {
	text := ""
	switch {
	case value.IsString():
		text = value.StringValue()
	case !value.IsEmpty():
		return jsonvalue.Value{}, errEventNotText
	}
	return parser.DecodeJSON(artifact, text)
}

// parseFile emits the rows of a JSONL transcript, an archived or copied
// one, or a cold archive, then the problems met in it and in the sibling
// sessions.json that gives its session key.
func (p *Parser) parseFile(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	out := &writer{Parser: p, artifact: artifact, options: options, emit: emit}
	name, ok := p.matchTranscript(artifact.Rel)
	current := newSession(name.name)
	current.headerID = true
	switch {
	case name.archive != "":
		current.extra = "archive=" + strings.TrimPrefix(name.archive, ".jsonl.")
	case name.copy != "":
		current.extra = "copy=" + strings.TrimLeft(name.copy, ".")
	}
	var keyError error
	// The session key is the one sessions.json gives every copy and
	// archive of the session.
	carried := parser.NewCarried(artifact)
	if ok {
		var key string
		key, keyError = p.legacyKey(artifact, current.sessionID)
		current.key = carried.Cut("session key", key)
	}
	var problems parser.Problems
	raw, read := parser.FileBytes(artifact.DiskPath, &problems)
	if read {
		data, err := maybeZstd(raw, p.decoder.Budget(int64(len(raw)), p.budgetFloor))
		if err != nil {
			out.bad(current, 1, "could not decompress ("+err.Error()+")", "")
		} else {
			out.records(current, blobLines(artifact, data, &problems), 0)
		}
	}
	if keyError != nil {
		problems.Add(0, keyError.Error())
	}
	carried.Report(0, &problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, current.sessionID, current.cwd))
}

// legacyKey returns the session key of sessionID from the sibling legacy
// sessions.json, or an error whose text is the message of the `parser:`
// row of a sessions.json that cannot be read, does not decode or is not a
// JSON object. A missing file is no error. The file is read once per state
// of it for the parser's life, since every transcript beside it asks.
func (p *Parser) legacyKey(artifact *input.Artifact, sessionID string) (string, error) {
	store, found := parser.Beside(artifact, "sessions.json")
	if !found || !parser.RegularFile(store) {
		return "", nil
	}
	info, err := osfile.Lstat(store)
	if err != nil {
		return "", nil
	}
	key := fileState{path: store, mtime: info.ModTime().UnixNano(), size: info.Size()}
	index, ok := p.legacyIndexes[key]
	if !ok {
		index = p.readLegacyIndex(artifact, store)
		if p.legacyIndexes == nil {
			p.legacyIndexes = map[fileState]*legacyIndex{}
		}
		p.legacyIndexes[key] = index
	}
	return index.keys[sessionID], index.err
}

// fileState names a file as read: its path, modification time in
// nanoseconds and size, the key of what is read from it once.
type fileState struct {
	path        string
	mtime, size int64
}

// legacyIndex is a legacy sessions.json read once: the first member key of
// each sessionId, and the error reading or decoding it met.
type legacyIndex struct {
	keys map[string]string
	err  error
}

// readLegacyIndex reads and indexes the sessions.json at path, once for
// the parser's life, noting its repeated member names in the parse of
// artifact, the first to read it.
func (p *Parser) readLegacyIndex(artifact *input.Artifact, path string) *legacyIndex {
	p.Work.Add("sessions.json reads", 1)
	var problems parser.Problems
	content, ok := parser.FileBytes(path, &problems)
	if !ok {
		return &legacyIndex{err: errors.New(problems[0].Message)}
	}
	data, err := parser.DecodeJSONBytes(artifact, content)
	switch {
	case err != nil:
		return &legacyIndex{err: fmt.Errorf("sessions.json unparseable (%w)", err)}
	case !data.IsObject():
		return &legacyIndex{err: errors.New("sessions.json is not a JSON object")}
	}
	index := &legacyIndex{keys: map[string]string{}}
	for _, member := range data.Members() {
		sessionID := member.Value.Get("sessionId")
		if _, held := index.keys[sessionID.StringValue()]; member.Value.IsObject() && sessionID.IsString() && !held {
			index.keys[sessionID.StringValue()] = member.Key
		}
	}
	return index
}
