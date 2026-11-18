package openclaw

import (
	"bytes"
	"errors"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/zed"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// errEventNotText is the reason a cold archive event whose event_json is
// neither text nor empty is unreadable.
var errEventNotText = errors.New("event_json is not text")

// zstdMagic starts every zstd frame.
const zstdMagic = "\x28\xb5\x2f\xfd"

// maybeZstd returns a blob decompressed when it is a zstd frame, and
// anything else as it is; it fails only for a frame that does not decode.
func maybeZstd(blob []byte) ([]byte, error) {
	if !bytes.HasPrefix(blob, []byte(zstdMagic)) {
		return blob, nil
	}
	return zed.Decompress(blob)
}

// numbered is one JSON object line of a blob and its line number.
type numbered struct {
	line   int
	record jsonvalue.Value
}

// blobLines returns the JSON object lines of data, numbered from 1. A line
// that does not decode, or holds another JSON value, adds an "unparseable
// line" problem with the line as raw text.
func blobLines(data []byte, problems *parser.Problems) []numbered {
	var records []numbered
	for line, record := range parser.JSONRecords(parser.SplitLines(data), problems) {
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
		if isText(kind, "header") && current.sessionID == "" {
			current.sessionID = parser.Text(record.Get("sessionId"))
		}
		inner := record.Get("row")
		if !isText(kind, "event") || !inner.IsObject() {
			continue
		}
		createdAt := timeutil.FromValue(inner.Get("created_at"))
		entry, err := eventJSON(inner.Get("event_json"))
		if err != nil {
			w.bad(current, line, "cold archive event seq "+inner.Get("seq").Text()+
				" unreadable: "+err.Error(), createdAt)
			continue
		}
		if entry.IsObject() {
			w.entry(current, line, entry, createdAt)
		}
	}
}

// eventJSON decodes the event_json text of a cold archive's event record;
// a value that is not text is an error.
func eventJSON(value jsonvalue.Value) (jsonvalue.Value, error) {
	text := ""
	switch {
	case value.IsString():
		text = value.StringValue()
	case !value.IsEmpty():
		return jsonvalue.Value{}, errEventNotText
	}
	return jsonvalue.DecodeString(text)
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
	if ok {
		current.key, keyError = legacyKey(artifact, current.sessionID)
	}
	var problems parser.Problems
	raw, read := parser.FileBytes(artifact.DiskPath, &problems)
	if read {
		data, err := maybeZstd(raw)
		if err != nil {
			out.bad(current, 1, "could not decompress ("+err.Error()+")", "")
		} else {
			out.records(current, blobLines(data, &problems), 0)
		}
	}
	if keyError != nil {
		problems.Add(0, keyError.Error())
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, current.sessionID, current.cwd))
}

// legacyKey returns the session key of sessionID from the sibling legacy
// sessions.json, or an error whose text is the message of the `parser:`
// row of a sessions.json that cannot be read, does not decode or is not a
// JSON object. A missing file is no error.
func legacyKey(artifact *input.Artifact, sessionID string) (string, error) {
	store := parser.Beside(artifact, "sessions.json")
	if !parser.RegularFile(store) {
		return "", nil
	}
	var problems parser.Problems
	content, ok := parser.FileBytes(store, &problems)
	if !ok {
		return "", errors.New(problems[0].Message)
	}
	data, err := jsonvalue.Decode(content)
	switch {
	case err != nil:
		return "", errors.New("sessions.json unparseable (" + err.Error() + ")")
	case !data.IsObject():
		return "", errors.New("sessions.json is not a JSON object")
	}
	for _, member := range data.Members() {
		if entry := member.Value; entry.IsObject() && isText(entry.Get("sessionId"), sessionID) {
			return member.Key, nil
		}
	}
	return "", nil
}
