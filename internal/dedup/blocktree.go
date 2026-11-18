package dedup

import (
	"math"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// blockTree finds the candidates of one list that a widened row of one text
// may take, never looking at one it may not. Position i holds the latest
// time of a secondary row of the text that could take candidate i, at or
// before that candidate's time (latestUpTo); a widened row at millis may
// take it when that time is at most millis. As this does not depend on the
// widened row, it is worked out once per list and text, and a tree of
// minimums finds the first allowed position in steps that grow with the
// logarithm of the list. A tree is built per text, so a list looked up by
// tool_use_id costs its length once for each text of the widened rows that
// look there (no agent widens a tool row). A taken candidate is closed.
type blockTree struct {
	width int
	least []int64
}

// treeKey names the blockTree of one candidate list for one text.
type treeKey struct {
	list *candidateList
	text identity
}

// newBlockTree returns the blockTree of a list for a bound's text, rows
// being the candidates' rows and used the ones already taken.
func newBlockTree(list *candidateList, bound *forwardBound, rows []timeline.Row, used []bool,
	steps *int) *blockTree {
	width := 1
	for width < len(list.stamps) {
		width *= 2
	}
	tree := &blockTree{width: width, least: make([]int64, 2*width)}
	for position := range tree.least[width:] {
		tree.least[width+position] = math.MaxInt64
	}
	for position, entry := range list.stamps {
		*steps++
		row := rows[entry.index]
		latest, found := bound.limits.latestUpTo(bound.text, entry.millis, row.SessionID,
			row.ProjectPath)
		switch {
		case used[entry.index]:
		case found:
			tree.least[width+position] = latest
		default:
			tree.least[width+position] = math.MinInt64
		}
	}
	for node := width - 1; node > 0; node-- {
		tree.least[node] = min(tree.least[2*node], tree.least[2*node+1])
	}
	return tree
}

// close records that the candidate at position is taken.
func (t *blockTree) close(position int) {
	node := t.width + position
	t.least[node] = math.MaxInt64
	for node /= 2; node > 0; node /= 2 {
		t.least[node] = min(t.least[2*node], t.least[2*node+1])
	}
}

// first returns the first position from start to before end whose time is
// at most millis, or -1 when there is none.
func (t *blockTree) first(start, end int, millis int64, steps *int) int {
	return t.firstBelow(1, 0, t.width, start, end, millis, steps)
}

// firstBelow is first within the positions low to before high that node
// covers.
func (t *blockTree) firstBelow(node, low, high, start, end int, millis int64,
	steps *int) int {
	*steps++
	if high <= start || low >= end || t.least[node] > millis {
		return -1
	}
	if high-low == 1 {
		return low
	}
	middle := (low + high) / 2
	if found := t.firstBelow(2*node, low, middle, start, end, millis, steps); found >= 0 {
		return found
	}
	return t.firstBelow(2*node+1, middle, high, start, end, millis, steps)
}
