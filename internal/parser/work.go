package parser

// Work counts the units of work a parser does, by name (the queries run, the
// entries read, the comparisons made), so a test can hold a cost to a bound
// by a count rather than by time. The zero Work counts nothing: a parser
// carries one in Base, and its helpers a copy, at the cost of one nil check
// per Add; a test sets a counting Work before the work is done.
type Work struct {
	counts map[string]int
}

// CountingWork returns a Work that counts. Copies of it share the counts.
func CountingWork() Work {
	return Work{counts: map[string]int{}}
}

// Add counts n units of the named work, when w counts.
func (w Work) Add(name string, n int) {
	if w.counts != nil {
		w.counts[name] += n
	}
}

// Count returns the units of the named work counted so far, 0 when w does
// not count.
func (w Work) Count(name string) int {
	return w.counts[name]
}
