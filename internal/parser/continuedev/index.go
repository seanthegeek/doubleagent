package continuedev

import (
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

// get returns the index at path, reading it on first use. Every path is
// one the parser kit or the input built, which joins its parts with one
// separator and no dot element, so one file has one spelling and one key.
func (x *Indexes) get(path string) index {
	if found, ok := x.read[path]; ok {
		return found
	}
	entries, problems := ReadIndex(path)
	x.read[path] = index{entries: entries, problems: problems}
	return x.read[path]
}

// Index returns the entries, by session id, of the sessions.json index at
// path: none when it is absent or unreadable, which is reported once, by
// Errors on the index's own artifact.
func (x *Indexes) Index(path string) map[string]jsonvalue.Value {
	return x.get(path).entries
}

// Errors returns the problems of the index at path, a new list each call.
func (x *Indexes) Errors(path string) parser.Problems {
	return slices.Clone(x.get(path).problems)
}

// Title returns a session's title: the file's own, else its index entry's.
// Any number of session files can name one entry, each writing its title
// into a row, so the entry's is cut by carried; the file's own is not.
func Title(document, entry jsonvalue.Value, carried *parser.Carried) string {
	if !document.Get("title").IsEmpty() {
		return parser.Text(document.Get("title"))
	}
	return carried.Cut("index title", parser.Text(entry.Get("title")))
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
	for position, element := range parser.ArrayText(text, &problems) {
		switch {
		case element.IsObject() && !element.Get("sessionId").IsEmpty():
			entries[element.Get("sessionId").Text()] = element
		case element.IsObject() && !element.Get("session_id").IsEmpty():
		default:
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+
				" (not an object with a sessionId)", parser.JSONText(element))
		}
	}
	return entries, problems
}
