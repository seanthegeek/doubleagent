package dedup

import (
	"cmp"
	"slices"
)

// skips is a disjoint-set forest (a union-find structure) over the nodes of
// a candidate list, each node pointing toward the next that may still be
// free, so a taken candidate is passed once and later lookups jump over it.
// A list keeps one forest per direction; going ahead a node is a position,
// going back it is the position plus one, and node 0 or the last node is
// the end.
type skips struct {
	parent []int
}

// newSkips returns a forest over an empty list: only the end node.
func newSkips() skips {
	return skips{parent: []int{0}}
}

// grow adds a node for a candidate added at the end of the list.
func (s *skips) grow() {
	s.parent = append(s.parent, len(s.parent))
}

// find returns the first node from node on that is not known to be passed
// over, halving the path it follows.
func (s *skips) find(node int, steps *int) int {
	for s.parent[node] != node {
		*steps++
		s.parent[node] = s.parent[s.parent[node]]
		node = s.parent[node]
	}
	return node
}

// jump returns the first node from start that neither the forest nor class
// passes over. Class holds, for one source file, the nodes of candidates
// from that file: every class node passed is then pointed at the result, so
// the next lookup for the same file passes the whole stretch at once.
func jump(forest *skips, class map[int]int, start int, steps *int) int {
	root := start
	for {
		root = forest.find(root, steps)
		next, ok := class[root]
		if !ok {
			break
		}
		*steps++
		root = next
	}
	for node := start; class != nil; {
		node = forest.find(node, steps)
		next, ok := class[node]
		if !ok {
			break
		}
		class[node] = root
		node = next
	}
	return root
}

// candidateList is a list of candidates in order (dated ones by time then
// row index, undated ones by when they were filed) with what lookups have
// learned about which of them may not be taken.
type candidateList struct {
	stamps        []stamp
	ahead, behind skips
	// aheadOf and behindOf hold, by source file, the nodes of candidates
	// from that file, which a row of the same file may not take.
	aheadOf, behindOf map[string]map[int]int
}

// newCandidateList returns an empty list.
func newCandidateList() *candidateList {
	return &candidateList{ahead: newSkips(), behind: newSkips()}
}

// push adds a candidate at the end of the list.
func (l *candidateList) push(entry stamp) {
	l.stamps = append(l.stamps, entry)
	l.ahead.grow()
	l.behind.grow()
}

// next returns the first position at or after start not yet known to be
// taken or from the refused file; len(stamps) when there is none.
func (l *candidateList) next(start int, refused *refusal, steps *int) int {
	return jump(&l.ahead, refused.classOf(l.aheadOf), start, steps)
}

// previous returns the last position at or before start that next would
// not pass over, or -1.
func (l *candidateList) previous(start int, refused *refusal, steps *int) int {
	return jump(&l.behind, refused.classOf(l.behindOf), start+1, steps) - 1
}

// markTaken records that the candidate at position is taken.
func (l *candidateList) markTaken(position int) {
	l.ahead.parent[position] = position + 1
	l.behind.parent[position+1] = position
}

// markFile records that the candidate at position is from file.
func (l *candidateList) markFile(file string, position int) {
	if l.aheadOf == nil {
		l.aheadOf, l.behindOf = map[string]map[int]int{}, map[string]map[int]int{}
	}
	if l.aheadOf[file] == nil {
		l.aheadOf[file], l.behindOf[file] = map[int]int{}, map[int]int{}
	}
	l.aheadOf[file][position] = position + 1
	l.behindOf[file][position+1] = position
}

// firstFrom returns the position of the first stamp at or after millis.
func firstFrom(stamps []stamp, millis int64) int {
	position, _ := slices.BinarySearchFunc(stamps, millis,
		func(entry stamp, target int64) int { return cmp.Compare(entry.millis, target) })
	return position
}

// firstAfter returns the position of the first stamp after millis.
func firstAfter(stamps []stamp, millis int64) int {
	position, _ := slices.BinarySearchFunc(stamps, millis, func(entry stamp, target int64) int {
		if entry.millis <= target {
			return -1
		}
		return 1
	})
	return position
}

// refusal is the file a lookup refuses: a row never pairs with a candidate
// from its own file among secondary copies.
type refusal struct {
	file string
}

// classOf returns the nodes known to be from the refused file, or nil when
// nothing is refused.
func (r *refusal) classOf(byFile map[string]map[int]int) map[int]int {
	if r == nil {
		return nil
	}
	return byFile[r.file]
}

// search is one secondary row's lookup in one pass: the candidates it
// refuses for their file, and for a widened row the bound past which it may
// not take a candidate. A bound is given only against the primary store,
// where no file is refused.
type search struct {
	state   *pass
	refused *refusal
	bound   *forwardBound
}

// accept says whether the candidate at position may be taken, recording a
// taken candidate, or one from the refused file, for every later lookup.
func (q *search) accept(list *candidateList, position int) bool {
	q.state.steps++
	entry := list.stamps[position]
	switch {
	case q.state.used[entry.index]:
		list.markTaken(position)
		return false
	case q.refused != nil && q.state.rows[entry.index].SourceFile == q.refused.file:
		list.markFile(q.refused.file, position)
		return false
	}
	return true
}

// before reports whether one candidate comes before another for a row at
// millis: nearer in time, then earlier, then the lower row index.
func before(first, second stamp, millis int64) bool {
	firstDistance, secondDistance := first.millis-millis, second.millis-millis
	if firstDistance < 0 {
		firstDistance = -firstDistance
	}
	if secondDistance < 0 {
		secondDistance = -secondDistance
	}
	if firstDistance != secondDistance {
		return firstDistance < secondDistance
	}
	return compareStamps(first, second) < 0
}

// nearestIn returns the candidate of a list a row at millis takes from low
// to high: the first in the order of before.
func (q *search) nearestIn(list *candidateList, millis, low, high int64) (stamp, bool) {
	left, leftFound := q.behindRow(list, millis, low, high)
	right, rightFound := q.aheadOfRow(list, millis, low, high)
	switch {
	case !rightFound:
		return left, leftFound
	case !leftFound || before(right, left, millis):
		return right, true
	}
	return left, true
}

// behindRow returns the latest candidate before millis, from low to high,
// with the lowest row index of those at its time.
func (q *search) behindRow(list *candidateList, millis, low, high int64) (stamp, bool) {
	steps := &q.state.steps
	lowest := firstFrom(list.stamps, low)
	end := min(firstFrom(list.stamps, millis), firstAfter(list.stamps, high))
	position := list.previous(end-1, q.refused, steps)
	for ; position >= lowest; position = list.previous(position-1, q.refused, steps) {
		if !q.accept(list, position) {
			continue
		}
		earliest := list.next(firstFrom(list.stamps, list.stamps[position].millis), q.refused, steps)
		for !q.accept(list, earliest) {
			earliest = list.next(earliest+1, q.refused, steps)
		}
		return list.stamps[earliest], true
	}
	return stamp{}, false
}

// aheadOfRow returns the first candidate at or after millis, from low to
// high, that the row may take.
func (q *search) aheadOfRow(list *candidateList, millis, low, high int64) (stamp, bool) {
	steps := &q.state.steps
	end := firstAfter(list.stamps, high)
	start := max(firstFrom(list.stamps, millis), firstFrom(list.stamps, low))
	if q.bound != nil {
		return q.boundedAhead(list, start, end)
	}
	position := list.next(start, q.refused, steps)
	for ; position < end; position = list.next(position+1, q.refused, steps) {
		if q.accept(list, position) {
			return list.stamps[position], true
		}
	}
	return stamp{}, false
}

// boundedAhead returns the first candidate from position start to before
// end that a widened row may take under its bound, through the list's
// blockTree for the row's text.
func (q *search) boundedAhead(list *candidateList, start, end int) (stamp, bool) {
	key := treeKey{list, q.bound.text}
	tree := q.state.trees[key]
	if tree == nil {
		tree = newBlockTree(list, q.bound, q.state.rows, q.state.used, &q.state.steps)
		q.state.trees[key] = tree
	}
	for {
		position := tree.first(start, end, q.bound.millis, &q.state.steps)
		if position < 0 {
			return stamp{}, false
		}
		if !q.state.used[list.stamps[position].index] {
			return list.stamps[position], true
		}
		tree.close(position)
		list.markTaken(position)
		start = position + 1
	}
}

// firstIn returns the first candidate of a list, in its order, that the row
// may take.
func (q *search) firstIn(list *candidateList) (stamp, bool) {
	steps := &q.state.steps
	position := list.next(0, q.refused, steps)
	for ; position < len(list.stamps); position = list.next(position+1, q.refused, steps) {
		if q.accept(list, position) {
			return list.stamps[position], true
		}
	}
	return stamp{}, false
}

// nearest returns the row index of the candidate of the parts a row at
// millis takes from low to high.
func (q *search) nearest(parts []*bucketPart, millis, low, high int64) (int, bool) {
	best, found := stamp{}, false
	for _, part := range parts {
		for _, list := range []*candidateList{part.settled, part.arriving} {
			candidate, ok := q.nearestIn(list, millis, low, high)
			if ok && (!found || before(candidate, best, millis)) {
				best, found = candidate, true
			}
		}
	}
	return best.index, found
}

// earliest returns the row index of the earliest candidate of the parts an
// undated row takes, the undated candidates last, in filing order.
func (q *search) earliest(parts []*bucketPart) (int, bool) {
	for _, dated := range []bool{true, false} {
		best, found := stamp{}, false
		for _, part := range parts {
			lists := []*candidateList{part.undated}
			if dated {
				lists = []*candidateList{part.settled, part.arriving}
			}
			for _, list := range lists {
				candidate, ok := q.firstIn(list)
				if ok && (!found || compareStamps(candidate, best) < 0) {
					best, found = candidate, true
				}
			}
		}
		if found {
			return best.index, true
		}
	}
	return -1, false
}
