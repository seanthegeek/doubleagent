package continuedev

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// byteOrderMark is U+FEFF in UTF-8, which a JSON file may start with.
const byteOrderMark = "\ufeff"

// index is one read of a sessions.json index: its entries by session id
// and the problems met.
type index struct {
	entries  map[string]jsonvalue.Value
	problems parser.Problems
}

// Indexes reads sessions.json indexes and keeps each for the parser's
// life, so the many session files that share an index read it once and its
// problems are reported once, on the index's own artifact.
type Indexes struct {
	read map[string]index
}

// NewIndexes returns an empty cache of indexes.
func NewIndexes() *Indexes {
	return &Indexes{read: map[string]index{}}
}

// get returns the index at path, reading it on first use. A path is one
// key however it was spelled.
func (x *Indexes) get(path string) index {
	key := filepath.Clean(path)
	if found, ok := x.read[key]; ok {
		return found
	}
	entries, problems := ReadIndex(path)
	x.read[key] = index{entries: entries, problems: problems}
	return x.read[key]
}

// Index returns the entries, by session id, of the sessions.json in the
// directory at path: none when it is absent or unreadable, which is
// reported once, by Errors on the index's own artifact.
func (x *Indexes) Index(directory string) map[string]jsonvalue.Value {
	return x.get(filepath.Join(directory, IndexName)).entries
}

// Errors returns the problems of the index at path, a new list each call.
func (x *Indexes) Errors(path string) parser.Problems {
	return slices.Clone(x.get(path).problems)
}

// ReadIndex returns the entries, by sessionId, of the sessions.json index
// at path, and its problems: an unreadable file, a file that is not a JSON
// array, an element that does not decode (with the text from it to the end
// of the file) and an element that is not an object with a sessionId.
// Entries keyed session_id, a pre-1.0 format the extension filters out,
// are skipped without a problem. An absent file or a symbolic link is an
// empty index with no problem.
func ReadIndex(path string) (map[string]jsonvalue.Value, parser.Problems) {
	entries := map[string]jsonvalue.Value{}
	var problems parser.Problems
	if !parser.RegularFile(path) {
		return entries, problems
	}
	content, ok := parser.FileBytes(path, &problems)
	if !ok {
		return entries, problems
	}
	text := strings.TrimPrefix(textlines.ValidUTF8(content), byteOrderMark)
	ArrayRecords(text, &problems, func(position int, element jsonvalue.Value) {
		switch {
		case element.IsObject() && !element.Get("sessionId").IsEmpty():
			entries[element.Get("sessionId").Text()] = element
		case element.IsObject() && !element.Get("session_id").IsEmpty():
		default:
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+
				" (not an object with a sessionId)", parser.JSONText(element))
		}
	})
	return entries, problems
}

// ArrayRecords passes each element of the JSON array text holds to visit,
// salvaging a damaged array as parser.ArrayAt does, and adds a problem for
// data after the array.
func ArrayRecords(text string, problems *parser.Problems, visit func(int, jsonvalue.Value)) {
	end, count := -1, 0
	for position, element := range parser.ArrayAt(text, 0, problems, &end) {
		count = position
		visit(position, element)
	}
	if end >= 0 && strings.Trim(text[end:], textlines.JSONSpace) != "" {
		problems.Add(count+1, "unparseable data after the JSON array")
	}
}
