package continuedev

import (
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// byteOrderMark is U+FEFF in UTF-8, which a JSON file may start with.
const byteOrderMark = "\ufeff"

// index is one read of a sessions.json index: its entries by session id,
// the first of an id standing, the session ids it repeats, the entries of
// those ids in file order, and the problems met.
type index struct {
	entries  map[string]jsonvalue.Value
	repeats  map[string]*Repeat
	notices  []notice
	problems parserkit.Problems
}

// Repeat is a session id the index holds in more than one entry. The
// extension updates a session's entry in place and adds one only for an id
// it does not hold (core/util/history.ts), so only damage or an edit
// repeats one (timeline.Group): each entry is a parent of the group,
// registered when the index is read so that every file's rows find it in
// whichever order they are parsed, and the rows that take an entry's
// dateCreated or workspaceDirectory are members.
type Repeat struct {
	Entries []jsonvalue.Value
	Parents []*timeline.Parent
	Group   *timeline.Group
}

// notice is one entry of a repeated id: its position in the file, its id
// and its place among the id's entries.
type notice struct {
	position  int
	sessionID string
	place     int
}

// Indexes reads sessions.json indexes and keeps each for the parser's
// life, so the many session files that share an index read it once and its
// problems are reported once, on the index's own artifact.
type Indexes struct {
	read map[string]*index
}

// NewIndexes returns an empty cache of indexes.
func NewIndexes() *Indexes {
	return &Indexes{read: map[string]*index{}}
}

// repeat adds an entry of a repeated id as a new parent of the id's group:
// the parent holds the entry's dateCreated, id and workspaceDirectory,
// which a member takes.
func (read *index) repeat(element indexElement) {
	found := read.repeats[element.sessionID]
	if found == nil {
		found = &Repeat{Group: timeline.NewGroup()}
		read.repeats[element.sessionID] = found
	}
	stored := timeline.Row{TimestampUTC: timeutil.FromValue(element.entry.Get("dateCreated")),
		SessionID: element.sessionID, ProjectPath: parserkit.Text(element.entry.Get("workspaceDirectory"))}
	read.notices = append(read.notices, notice{position: element.position, sessionID: element.sessionID,
		place: len(found.Entries)})
	found.Entries = append(found.Entries, element.entry)
	found.Parents = append(found.Parents, found.Group.Parent(&stored))
}

// readIndex returns the entries of the sessions.json index at path in file
// order, and its problems: an unreadable file, a file that is not a JSON
// array, an element that does not decode (with the text from it to the end
// of the file) and an element that is not an object with a sessionId.
// Entries keyed session_id, a pre-1.0 format the extension filters out, are
// skipped without a problem. An absent file or a symbolic link is an empty
// index with no problem.
func readIndex(path string) ([]indexElement, parserkit.Problems) {
	var elements []indexElement
	var problems parserkit.Problems
	text, ok := parserkit.ReadText(path, &problems)
	if !ok {
		return elements, problems
	}
	for position, element := range parserkit.ArrayText(text, &problems) {
		switch {
		case element.IsObject() && !element.Get("sessionId").IsEmpty():
			elements = append(elements, indexElement{position: position,
				sessionID: element.Get("sessionId").Text(), entry: element})
		case element.IsObject() && !element.Get("session_id").IsEmpty():
		default:
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+
				" (not an object with a sessionId)", parserkit.JSONText(element))
		}
	}
	return elements, problems
}

// get returns the index at path, reading it on first use. Every path is
// one the parser kit or the input built, which joins its parts with one
// separator and no dot element, so one file has one spelling and one key.
func (x *Indexes) get(path string) *index {
	if found, ok := x.read[path]; ok {
		return found
	}
	elements, problems := readIndex(path)
	read := &index{entries: map[string]jsonvalue.Value{}, repeats: map[string]*Repeat{}, problems: problems}
	counts := map[string]int{}
	for _, element := range elements {
		counts[element.sessionID]++
	}
	for _, element := range elements {
		if _, held := read.entries[element.sessionID]; !held {
			read.entries[element.sessionID] = element.entry
		}
		if counts[element.sessionID] > 1 {
			read.repeat(element)
		}
	}
	x.read[path] = read
	return read
}

// Index returns the entries, by session id, of the sessions.json index at
// path: none when it is absent or unreadable, which is reported once, by
// Errors on the index's own artifact.
func (x *Indexes) Index(path string) map[string]jsonvalue.Value {
	return x.get(path).entries
}

// Repeated returns the repeat of a session id the index at path holds in
// more than one entry, else nil.
func (x *Indexes) Repeated(path, sessionID string) *Repeat {
	return x.get(path).repeats[sessionID]
}

// Member makes row a member of the group of its session id when the index
// at path repeats the id, taking what takes names from the entry the group
// goes with.
func (x *Indexes) Member(path, sessionID string, row *timeline.Row, takes timeline.Takes) {
	if found := x.Repeated(path, sessionID); found != nil {
		found.Group.Member(row, takes)
	}
}

// Errors returns the problems of the index at path, a new list each call.
func (x *Indexes) Errors(path string) parserkit.Problems {
	return slices.Clone(x.get(path).problems)
}

// Notices returns a `parser:` row per entry of a session id the index at
// path repeats, at the entry's position, with its dateCreated, id and
// workspaceDirectory, each the row of its parent (timeline.Parent.Texts),
// so the filters can keep an entry by it; base gives the rows' file.
func (x *Indexes) Notices(path string, base timeline.Row) []timeline.Row {
	read := x.get(path)
	rows := make([]timeline.Row, 0, len(read.notices))
	for _, each := range read.notices {
		found := read.repeats[each.sessionID]
		entry := found.Entries[each.place]
		row := base
		row.SourceLine, row.TurnType, row.SessionID = each.position, timeline.TurnSystem, each.sessionID
		row.TimestampUTC = timeutil.FromValue(entry.Get("dateCreated"))
		row.ProjectPath = parserkit.Text(entry.Get("workspaceDirectory"))
		prefix := parserkit.ProblemPrefix + IndexName + " holds " + strconv.Itoa(len(found.Entries)) +
			" entries of this entry's sessionId; its session's rows are shown once, with "
		found.Parents[each.place].Texts(&row, timeline.RowText{Text: prefix + "this entry"},
			timeline.RowText{Text: prefix + "another of them"})
		rows = append(rows, row)
	}
	return rows
}

// Title returns a session's title: the file's own, else its index entry's.
// Any number of session files can name one entry, each writing its title
// into a row, so the entry's is cut by carried; the file's own is not.
func Title(document, entry jsonvalue.Value, carried *parserkit.Carried) string {
	if !document.Get("title").IsEmpty() {
		return parserkit.Text(document.Get("title"))
	}
	return carried.Cut("index title", parserkit.Text(entry.Get("title")))
}

// indexElement is one entry of a sessions.json index: its position, its
// sessionId as text and the entry.
type indexElement struct {
	position  int
	sessionID string
	entry     jsonvalue.Value
}
