package parser

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// CarriedLimit is the most bytes a carried value is written with: a value
// one place in the evidence gives that a parse copies onto many rows (a
// field of a row other than its text, such as a project path, model or
// session id, or a key or name written into the text of each row of a
// list). It is the bound the input puts on a file's path
// (input.MaxPathLength, Linux's PATH_MAX), far above any identifier, path
// or key an agent writes, and it keeps the output a fixed size per row
// however long the value: without it a value of n bytes on m rows costs n
// times m.
const CarriedLimit = input.MaxPathLength

// markStart and markEnd are the text around the length in a cut's mark.
const (
	markStart = " [cut from "
	markEnd   = " bytes]"
)

// cutMark returns the text a cut value ends with, which gives the whole
// value's length in bytes: " [cut from <length> bytes]". The length keeps
// apart two cut values that start alike but differ in length, though not
// a value within the limit that is written like a cut: it is kept as it
// is, so it equals the cut of the longer value it copies. The mark starts
// with white space and holds no URL, as secrets.CutPoint asks.
func cutMark(length int) string {
	return markStart + strconv.Itoa(length) + markEnd
}

// cutValue returns value, longer than limit, cut to at most limit bytes
// (CarriedLimit, or less to leave room for what follows it): its first
// bytes up to secrets.CutPoint, which keeps whole or leaves out each URL
// credential, each earlier cut it holds (earlierCuts) and each span of
// whole, then cutMark. The cut is recorded in recorder
// (secrets.Recorder.Cut, inText when it stands in a row's text), so a
// secret the cut runs through is withheld whole, and the same value always
// gives the same text.
func cutValue(recorder *secrets.Recorder, value string, limit int, inText bool, whole ...[2]int) string {
	mark := cutMark(len(value))
	kept := secrets.CutPoint(value, limit-len(mark), append(earlierCuts(value), whole...))
	written := value[:kept] + mark
	recorder.Cut(value, written, kept, inText)
	return written
}

// earlierCuts returns the spans of value that may hold the text written
// for a value cut before (a Carried.Cut result a parser put into a field):
// for each text shaped like a mark, any length in it, the CarriedLimit
// bytes up to its end, from the start of a character. A cut keeps each
// whole or leaves it out, so the output still finds the end of the earlier
// cut that it withholds from a secret's start. Only the first twice
// CarriedLimit bytes are read: a later span starts after any cut point.
func earlierCuts(value string) [][2]int {
	window := value[:min(len(value), 2*CarriedLimit)]
	var spans [][2]int
	for from := 0; ; {
		found := strings.Index(window[from:], markStart)
		if found < 0 {
			return spans
		}
		digits := from + found + len(markStart)
		end := digits
		for end < len(window) && '0' <= window[end] && window[end] <= '9' {
			end++
		}
		if end > digits && strings.HasPrefix(window[end:], markEnd) {
			end += len(markEnd)
			start := max(0, end-CarriedLimit)
			for start > 0 && !utf8.RuneStart(value[start]) {
				start--
			}
			spans = append(spans, [2]int{start, end})
		}
		from = digits
	}
}

// CarriedProblem returns the problem text of a carried value cut n times:
// "<name> longer than 4096 bytes, cut to 4096 bytes (<n> times)", or
// "(1 time)" for one.
func CarriedProblem(name string, n int) string {
	times := " times)"
	if n == 1 {
		times = " time)"
	}
	return name + " longer than " + strconv.Itoa(CarriedLimit) + " bytes, cut to " +
		strconv.Itoa(CarriedLimit) + " bytes (" + strconv.Itoa(n) + times
}

// lastCut is the value, as the parser gave it, that a cut was last made of,
// the limit it was cut to and the text written for it, so a value carried
// onto many rows is cut once; cut is false for a project path that its one
// spelling brought within the limit.
type lastCut struct {
	value, written string
	limit          int
	cut            bool
}

// cutOf returns the cut of value to limit, from the last when it is the
// same value and limit.
func (l *lastCut) cutOf(recorder *secrets.Recorder, value string, limit int, inText bool) string {
	if l.written == "" || l.limit != limit || l.value != value {
		*l = lastCut{value: value, written: cutValue(recorder, value, limit, inText), limit: limit, cut: true}
	}
	return l.written
}

// carriedFields cuts the fields of a parse's rows (every string but the
// text) that are longer than CarriedLimit, counting the cuts of each field,
// and keeps the first row cut, whose host, user, agent, session, project and
// source file the problem rows carry.
type carriedFields struct {
	recorder *secrets.Recorder
	names    []string
	counts   map[string]int
	last     map[string]*lastCut
	first    *timeline.Row
}

// cut cuts the long fields of row.
func (c *carriedFields) cut(row *timeline.Row) {
	cutAny := false
	for _, field := range row.FieldsButText() {
		if len(*field.Value) <= CarriedLimit {
			continue
		}
		written, cut := c.written(field.Name, *field.Value)
		*field.Value = written
		if !cut {
			continue
		}
		if c.counts[field.Name] == 0 {
			c.names = append(c.names, field.Name)
		}
		c.counts[field.Name]++
		cutAny = true
	}
	if cutAny && c.first == nil {
		first := *row
		c.first = &first
	}
}

// bound cuts the long fields of row as cut does, counting none of them: a
// group's copy of a parent's attributes, whose own rows report the cuts.
func (c *carriedFields) bound(row *timeline.Row) {
	for _, field := range row.FieldsButText() {
		if len(*field.Value) > CarriedLimit {
			*field.Value, _ = c.written(field.Name, *field.Value)
		}
	}
}

// written returns what the kit writes for value, a field longer than
// CarriedLimit, and whether that is a cut, from the field's last value when
// value is the same as the parser gave it. Telling that reads both values
// when they are two copies of one text, as long as the value is, so a
// parser makes a value it stamps on many rows once and gives each row that
// string, which compares in one step. A project path is first given its
// one spelling (timeline.CanonicalProjectPath), which the run gives every
// project path, so each spelling of one long path is cut alike; the part
// of it the spelling keeps as written, from a URL with a credential on, is
// left out whole, so the cut text holds none of it and is its own spelling.
func (c *carriedFields) written(name, value string) (string, bool) {
	if c.counts == nil {
		c.counts, c.last = map[string]int{}, map[string]*lastCut{}
	}
	last := c.last[name]
	if last == nil {
		last = &lastCut{}
		c.last[name] = last
	}
	if last.written == "" || last.value != value {
		written, cut := value, false
		var whole [][2]int
		if name == "project_path" {
			written = timeline.CanonicalProjectPath(value)
			whole = [][2]int{{timeline.KeptAsWrittenFrom(written), len(written)}}
		}
		if len(written) > CarriedLimit {
			written, cut = cutValue(c.recorder, written, CarriedLimit, false, whole...), true
		}
		*last = lastCut{value: value, written: written, cut: cut}
	}
	return last.written, last.cut
}

// problemRows returns one "parser:" system row per field cut, in the order
// first cut, saying how many times; none when nothing was cut.
func (c *carriedFields) problemRows() []timeline.Row {
	rows := make([]timeline.Row, 0, len(c.names))
	for _, name := range c.names {
		rows = append(rows, timeline.Row{Host: c.first.Host, User: c.first.User, Agent: c.first.Agent,
			SessionID: c.first.SessionID, ProjectPath: c.first.ProjectPath, TurnType: timeline.TurnSystem,
			Text: ProblemPrefix + CarriedProblem(name, c.counts[name]), SourceFile: c.first.SourceFile})
	}
	return rows
}
