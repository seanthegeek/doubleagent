package jsonvalue

import (
	"strconv"
	"strings"
)

// RepeatedNamesKept is how many repeated member names a Repeats keeps by
// name, the first met: enough for a report to show what to look for, while
// the count carries the rest. Each later repeat is compared with at most
// this many names and adds only to the count, so a document of a million
// repeats costs a counter and these names, never a record per repeat.
const RepeatedNamesKept = 3

// RepeatedName is a member name that repeats within an object and how many
// times a later member of an object gave it again.
type RepeatedName struct {
	Name  string
	Times int
}

// Repeats is what a Document noted of the member names that repeat within
// one object of the values it decoded: how many members gave a name an
// earlier member of the same object had, which take the place of that
// member's value (the last value of a name is the one kept), and the
// first RepeatedNamesKept such names. A name is the decoded key itself,
// kept without copying, so a report bounds how much of it to show.
type Repeats struct {
	count int
	names []RepeatedName
}

// Count returns how many members repeated a name of their object.
func (r Repeats) Count() int { return r.count }

// Names returns the first names that repeated, at most RepeatedNamesKept,
// in the order of their first repeat, with the repeats of each. The slice
// is the record's own: a caller does not modify it.
func (r Repeats) Names() []RepeatedName { return r.names }

// Merge adds other's repeats to r, keeping r's names first.
func (r *Repeats) Merge(other Repeats) {
	r.count += other.count - sumTimes(other.names)
	for _, name := range other.names {
		r.note(name.Name, name.Times)
	}
}

// sumTimes returns the repeats the names count.
func sumTimes(names []RepeatedName) int {
	total := 0
	for _, name := range names {
		total += name.Times
	}
	return total
}

// note counts times repeats of name, adding to a name kept or keeping it
// while fewer than RepeatedNamesKept are.
func (r *Repeats) note(name string, times int) {
	r.count += times
	for i := range r.names {
		if r.names[i].Name == name {
			r.names[i].Times += times
			return
		}
	}
	if len(r.names) < RepeatedNamesKept {
		r.names = append(r.names, RepeatedName{Name: name, Times: times})
	}
}

// Decode reads the document's whole text as one JSON document, as
// DecodeString reads it, noting the member names that repeat for
// TakeRepeats.
func (d *Document) Decode() (Value, error) {
	value, end, found, err := d.decodeAt(0)
	if err != nil {
		return Value{}, err
	}
	if rest := skipSpace(d.text, end); rest < len(d.text) {
		return Value{}, d.errorAt(rest, errExtraData)
	}
	d.repeats.Merge(found)
	return value, nil
}

// TakeRepeats returns the repeats noted in the values the document has
// returned since the last TakeRepeats, and starts the record afresh; a
// value that does not decode notes none.
func (d *Document) TakeRepeats() Repeats {
	taken := d.repeats
	d.repeats = Repeats{}
	return taken
}

// skipSpace returns the first offset of text from offset that is not JSON
// whitespace.
func skipSpace(text string, offset int) int {
	for offset < len(text) && (text[offset] == ' ' || text[offset] == '\t' || text[offset] == '\r' ||
		text[offset] == '\n') {
		offset++
	}
	return offset
}

// Describe returns the report of the repeats: "1 member name repeats within
// its JSON object (<name>); the last value is used", or "<n> member names
// repeat within their JSON objects (<name> <k> times, ...); the last value
// of each is used", naming the names kept, with "..." when others
// repeated. A name is written as a JSON string, or as "a name of <length>
// bytes" when longer than longest bytes, so the text has a bound and holds
// no part of a long name. The value a repeat replaced is never written.
func (r Repeats) Describe(longest int) string {
	if r.count == 1 && len(r.names) == 1 {
		return "1 member name repeats within its JSON object (" + describeName(r.names[0].Name, longest) +
			"); the last value is used"
	}
	shown := make([]string, 0, len(r.names)+1)
	for _, name := range r.names {
		times := " times"
		if name.Times == 1 {
			times = " time"
		}
		shown = append(shown, describeName(name.Name, longest)+" "+strconv.Itoa(name.Times)+times)
	}
	if sumTimes(r.names) < r.count {
		shown = append(shown, "...")
	}
	return strconv.Itoa(r.count) + " member names repeat within their JSON objects (" +
		strings.Join(shown, ", ") + "); the last value of each is used"
}

// describeName returns a repeated name as Describe writes it.
func describeName(name string, longest int) string {
	if len(name) > longest {
		return "a name of " + strconv.Itoa(len(name)) + " bytes"
	}
	return NewString(name).JSON()
}
