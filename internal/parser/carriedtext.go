package parser

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

// Cut returns value, a carried key, name or path called name in the
// problem ("settings key"), as written when it is CarriedLimit bytes or
// fewer, else cut to that and counted for Report.
func (c *Carried) Cut(name, value string) string {
	if len(value) <= CarriedLimit {
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
	return last.cutOf(recorderOf(c.artifact), value, true)
}

// Report adds, at line, one problem (CarriedProblem) per name cut since
// the last Report, in the order first cut, and starts the count afresh.
func (c *Carried) Report(line int, problems *Problems) {
	for _, name := range c.names {
		problems.Add(line, CarriedProblem(name, c.counts[name]))
	}
	c.names, c.counts = nil, map[string]int{}
}
