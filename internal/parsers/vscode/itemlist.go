package vscode

// itemList holds an array's items in order. A delete leaves an empty slot
// where the item was instead of moving every later item down one place,
// and counts, a Fenwick tree (a binary indexed tree: counts[i] holds how
// many items the slots i-(i&-i)+1 to i hold, numbering slots from 1), finds
// the slot of the item at a position in steps that grow with the logarithm
// of the slots. So neither a delete nor a lookup costs work in proportion
// to the items after it, and the empty slots are passed over once when the
// array is read whole.
type itemList struct {
	slots  []*node
	counts []int
	size   int
}

// Len returns how many items the list holds.
func (l *itemList) Len() int {
	return l.size
}

// prefix returns how many items the first n slots hold.
func (l *itemList) prefix(n int) int {
	total := 0
	for ; n > 0; n -= n & -n {
		total += l.counts[n]
	}
	return total
}

// push adds an item at the end.
func (l *itemList) push(item *node) {
	if l.counts == nil {
		l.counts = []int{0}
	}
	l.slots = append(l.slots, item)
	n := len(l.slots)
	l.counts = append(l.counts, 1+l.prefix(n-1)-l.prefix(n-(n&-n)))
	l.size++
}

// slot returns the index in slots of the item at position, which must be
// below Len.
func (l *itemList) slot(position int) int {
	index, left := 0, position+1
	step := 1
	for step*2 < len(l.counts) {
		step *= 2
	}
	for ; step > 0; step /= 2 {
		if next := index + step; next < len(l.counts) && l.counts[next] < left {
			index, left = next, left-l.counts[next]
		}
	}
	return index
}

// item returns the item at position, which must be below Len.
func (l *itemList) item(position int) *node {
	return l.slots[l.slot(position)]
}

// replace puts item at position, which must be below Len.
func (l *itemList) replace(position int, item *node) {
	l.slots[l.slot(position)] = item
}

// remove deletes the item at position, which must be below Len, leaving
// its slot empty.
func (l *itemList) remove(position int) {
	index := l.slot(position)
	l.slots[index] = nil
	for n := index + 1; n < len(l.counts); n += n & -n {
		l.counts[n]--
	}
	l.size--
}

// truncate keeps the first keep items, keep being at most Len.
func (l *itemList) truncate(keep int) {
	if keep >= l.size {
		return
	}
	cut := 0
	if keep > 0 {
		cut = l.slot(keep)
	}
	l.slots, l.counts, l.size = l.slots[:cut], l.counts[:cut+1], keep
	if cut == 0 {
		l.counts = nil
	}
}

// items returns the items in order, the empty slots passed over.
func (l *itemList) items() []*node {
	held := make([]*node, 0, l.size)
	for _, item := range l.slots {
		if item != nil {
			held = append(held, item)
		}
	}
	return held
}
