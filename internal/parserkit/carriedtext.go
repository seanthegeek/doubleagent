package parserkit

import "github.com/seanthegeek/doubleagent/internal/input"

// Carried cuts the values a parser copies from one place in the evidence
// into the text of more than one row, such as a settings key written
// before each of its leaves or a group's name written into each of its
// members' rows, and counts them for one problem per kind each time the
// parser calls Report: a parser makes one per parse, or one per message or
// event and reports it there. A value is cut as the kit cuts a row's
// fields (CarriedLimit, with the whole length after it) and only when
// longer; a parser never cuts one itself, and never cuts what an element
// itself says. Nothing checks that a parser passes every such value here.
type Carried struct {
	artifact *input.Artifact
	names    []string
	counts   map[string]int
	// last holds the cut last made under each name, so values carried
	// under names that take turns are each cut once.
	last map[string]*lastCut
}

// NewCarried returns the Carried of one parse of artifact.
func NewCarried(artifact *input.Artifact) *Carried {
	return &Carried{artifact: artifact, counts: map[string]int{}, last: map[string]*lastCut{}}
}

// cut returns value as written when it is limit bytes or fewer, else cut
// to limit and counted for Report under name; inText says whether it
// stands in a row's text or in a field.
func (c *Carried) cut(name, value string, limit int, inText bool) string {
	if len(value) <= limit {
		return value
	}
	if c.counts[name] == 0 {
		c.names = append(c.names, name)
	}
	c.counts[name]++
	last := c.last[name]
	if last == nil {
		last = &lastCut{}
		c.last[name] = last
	}
	return last.cutOf(recorderOf(c.artifact), value, limit, inText)
}

// Cut returns value, a carried key, name or path called name in the
// problem ("settings key"), as written when it is CarriedLimit bytes or
// fewer, else cut to that and counted for Report.
func (c *Carried) Cut(name, value string) string {
	return c.cut(name, value, CarriedLimit, true)
}

// CutAhead returns value, which a field of each row starts with ahead of a
// suffix of at most room bytes (a long chat id before ":<call id>" in each
// call's tool_use_id), as written when the two fit in CarriedLimit, else
// cut so that they do and counted for Report under name. The value is then
// cut and given to the secrets recorder once however many suffixes follow
// it, and distinct suffixes keep the fields distinct. The cut is recorded
// as a field's, not as row text. A suffix longer than room makes the field
// longer than CarriedLimit, and the kit cuts that field as it cuts any.
func (c *Carried) CutAhead(name, value string, room int) string {
	return c.cut(name, value, CarriedLimit-room, false)
}

// Report adds, at line, one problem (CarriedProblem) per name cut since
// the last Report, in the order first cut, and starts the count afresh.
func (c *Carried) Report(line int, problems *Problems) {
	for _, name := range c.names {
		problems.Add(line, CarriedProblem(name, c.counts[name]))
	}
	c.names, c.counts = nil, map[string]int{}
}
