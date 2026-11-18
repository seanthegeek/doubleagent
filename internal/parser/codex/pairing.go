package codex

import (
	"cmp"
	"math"
	"slices"
	"sort"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// pairGap is how many seconds apart an exec row and a request row may be
// logged and still be one decision.
const pairGap = 2

// intsWithin reports whether two integers are at most gap apart, without
// the overflow a subtraction of extreme values would give.
func intsWithin(left, right, gap int64) bool {
	low, high := min(left, right), max(left, right)
	return uint64(high)-uint64(low) <= uint64(gap)
}

// minTree holds a value per slot and answers the least value of a range
// of slots, so the first unpaired exec row of a range of times is found
// without reading the range.
type minTree struct {
	size  int
	cells []int
}

// unset is the value of a slot that holds nothing.
const unset = math.MaxInt

// newMinTree returns a tree holding values.
func newMinTree(values []int) minTree {
	tree := minTree{size: len(values), cells: make([]int, 2*len(values))}
	copy(tree.cells[tree.size:], values)
	for i := tree.size - 1; i > 0; i-- {
		tree.cells[i] = min(tree.cells[2*i], tree.cells[2*i+1])
	}
	return tree
}

// clear empties a slot.
func (m *minTree) clear(slot int) {
	i := slot + m.size
	m.cells[i] = unset
	for i > 1 {
		i /= 2
		m.cells[i] = min(m.cells[2*i], m.cells[2*i+1])
	}
}

// least returns the least value of the slots from low up to high, high
// excluded, and how many cells it read.
func (m *minTree) least(low, high int) (int, int) {
	least, read := unset, 0
	for low, high = low+m.size, high+m.size; low < high; low, high = low/2, high/2 {
		if low%2 == 1 {
			least, read = min(least, m.cells[low]), read+1
			low++
		}
		if high%2 == 1 {
			high--
			least, read = min(least, m.cells[high]), read+1
		}
	}
	return least, read
}

// stampGroup is the exec rows of one storage class of ts, integer or
// float, sorted by ts, with a tree of their positions in rowid order.
type stampGroup struct {
	stamps    []sqlitefile.Value
	positions []int
	tree      minTree
}

// newStampGroup returns the group of the exec rows at positions, sorted by
// their ts with order.
func newStampGroup(execs []logRecord, positions []int, order func(left, right sqlitefile.Value) int) stampGroup {
	slices.SortStableFunc(positions, func(left, right int) int {
		return order(execs[left].timestamp, execs[right].timestamp)
	})
	group := stampGroup{positions: positions, stamps: make([]sqlitefile.Value, len(positions))}
	for slot, position := range positions {
		group.stamps[slot] = execs[position].timestamp
	}
	group.tree = newMinTree(slices.Clone(positions))
	return group
}

// execPairs finds, for each request row in turn, the first exec row in
// rowid order that no earlier request took and whose ts is near the
// request's: integer ts values at most pairGap apart, else the two as
// floats at most pairGap apart. A ts that is not a finite number is near
// nothing. Each lookup is two binary searches and a tree query per group.
type execPairs struct {
	ints, floats stampGroup
	slots        map[int]int
	// steps counts the comparisons and tree cells read, the work a test
	// holds to a logarithm per request.
	steps int
}

// newExecPairs returns the pairing over the exec rows.
func newExecPairs(execs []logRecord) *execPairs {
	var ints, floats []int
	for position, exec := range execs {
		switch stamp := exec.timestamp; {
		case stamp.Type == sqlitefile.Integer:
			ints = append(ints, position)
		case stamp.Type == sqlitefile.Float && !math.IsInf(stamp.Float, 0) && !math.IsNaN(stamp.Float):
			floats = append(floats, position)
		}
	}
	pairs := &execPairs{slots: map[int]int{},
		ints: newStampGroup(execs, ints, func(left, right sqlitefile.Value) int {
			return cmp.Compare(left.Int, right.Int)
		}),
		floats: newStampGroup(execs, floats, func(left, right sqlitefile.Value) int {
			return cmp.Compare(left.Float, right.Float)
		})}
	for slot, position := range pairs.ints.positions {
		pairs.slots[position] = slot
	}
	for slot, position := range pairs.floats.positions {
		pairs.slots[position] = slot
	}
	return pairs
}

// take returns the position of the exec row paired with a request logged
// at stamp, marking it taken, or -1 when none is near.
func (p *execPairs) take(stamp sqlitefile.Value) int {
	request, ok := numberOf(stamp)
	if !ok || math.IsInf(request, 0) || math.IsNaN(request) {
		return -1
	}
	best, group := unset, (*stampGroup)(nil)
	for _, each := range []*stampGroup{&p.ints, &p.floats} {
		low, high := p.window(each, stamp, request)
		found, read := each.tree.least(low, high)
		p.steps += read
		if found < best {
			best, group = found, each
		}
	}
	if group == nil {
		return -1
	}
	group.tree.clear(p.slots[best])
	return best
}

// window returns the slots of a group whose ts is near a request's: an
// integer request against integer rows exactly, any other pair as floats.
// Both bounds are monotone in the sorted ts, so each is a binary search.
func (p *execPairs) window(group *stampGroup, stamp sqlitefile.Value, request float64) (int, int) {
	asFloat := func(value sqlitefile.Value) float64 {
		number, _ := numberOf(value)
		return number
	}
	search := func(after func(sqlitefile.Value) bool) int {
		return sort.Search(len(group.stamps), func(slot int) bool {
			p.steps++
			return after(group.stamps[slot])
		})
	}
	if group == &p.ints && stamp.Type == sqlitefile.Integer {
		first := search(func(exec sqlitefile.Value) bool {
			return exec.Int >= stamp.Int || intsWithin(exec.Int, stamp.Int, pairGap)
		})
		past := search(func(exec sqlitefile.Value) bool {
			return exec.Int > stamp.Int && !intsWithin(exec.Int, stamp.Int, pairGap)
		})
		return first, past
	}
	first := search(func(exec sqlitefile.Value) bool { return request-asFloat(exec) <= pairGap })
	past := search(func(exec sqlitefile.Value) bool { return asFloat(exec)-request > pairGap })
	return first, past
}
