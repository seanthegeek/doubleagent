package textset

import "slices"

// Counter counts units of work by name, so a test can hold a search's cost
// to a bound by a count rather than by time; parserkit.Work is one.
type Counter interface {
	Add(name string, n int)
}

// Set is a list of texts to search for.
type Set struct {
	// edges maps a node and a byte, as node<<8 | byte, to the child node.
	edges map[uint64]int32
	// children lists each node's children with the byte that leads there.
	children [][]step
	// fallback is a node's longest proper suffix in the tree, and matched
	// the nearest node on its fallback chain that ends a listed text (-1
	// when none).
	fallback, matched []int32
	// depth is the length of the prefix a node stands for.
	depth []int
	// ends holds the positions in the list of the texts a node ends.
	ends [][]int
	// mark holds, per node, the search that last reported it, so a node
	// and the chain below it are reported once per search.
	mark     []int
	search   int
	backward bool
	work     Counter
}

// step is one edge of the tree: the byte read and the node it leads to.
type step struct {
	value byte
	node  int32
}

// byteAt returns the i-th byte the set reads of text: from its start, or
// from its end for a backward set.
func (s *Set) byteAt(text string, i int) byte {
	if s.backward {
		return text[len(text)-1-i]
	}
	return text[i]
}

// count adds n units of the named work, when the set counts.
func (s *Set) count(name string, n int) {
	if s.work != nil {
		s.work.Add(name, n)
	}
}

// edgeKey returns the key in edges of the edge from node by value.
func edgeKey(node int32, value byte) uint64 {
	return uint64(node)<<8 | uint64(value)
}

// child returns the node one edge from node by value, false when the tree
// has none.
func (s *Set) child(node int32, value byte) (int32, bool) {
	child, ok := s.edges[edgeKey(node, value)]
	return child, ok
}

// node adds an empty node standing for a prefix of depth bytes and returns
// its number.
func (s *Set) node(depth int) int32 {
	s.children = append(s.children, nil)
	s.fallback = append(s.fallback, 0)
	s.matched = append(s.matched, -1)
	s.depth = append(s.depth, depth)
	s.ends = append(s.ends, nil)
	s.mark = append(s.mark, 0)
	return int32(len(s.fallback) - 1)
}

// next returns the node a search reaches from node on reading value: the
// child by value of node, or of the first node on its fallback chain that
// has one, else the root.
func (s *Set) next(node int32, value byte) int32 {
	for {
		if child, ok := s.child(node, value); ok {
			return child
		}
		if node == 0 {
			return 0
		}
		s.count("fallbacks taken", 1)
		node = s.fallback[node]
	}
}

// link sets each node's fallback and matched links breadth first, so a
// node's fallback, which is shallower, is linked before it.
func (s *Set) link() {
	queue := []int32{0}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range s.children[current] {
			if current != 0 {
				s.fallback[edge.node] = s.next(s.fallback[current], edge.value)
			}
			back := s.fallback[edge.node]
			s.matched[edge.node] = s.matched[back]
			if len(s.ends[back]) > 0 {
				s.matched[edge.node] = back
			}
			queue = append(queue, edge.node)
		}
	}
}

// build returns the set of texts, each read from its end when backward.
func build(texts []string, work Counter, backward bool) *Set {
	set := &Set{edges: map[uint64]int32{}, backward: backward, work: work}
	set.node(0)
	for position, text := range texts {
		current := int32(0)
		for i := range len(text) {
			value := set.byteAt(text, i)
			child, ok := set.child(current, value)
			if !ok {
				child = set.node(set.depth[current] + 1)
				set.edges[edgeKey(current, value)] = child
				set.children[current] = append(set.children[current], step{value, child})
			}
			current = child
		}
		set.ends[current] = append(set.ends[current], position)
		set.count("bytes indexed", len(text))
	}
	set.link()
	return set
}

// New returns the set of texts, counting in work (nil counts nothing) the
// bytes of the texts it is built from, the bytes each search reads and the
// fallbacks it and the building take.
func New(texts []string, work Counter) *Set {
	return build(texts, work, false)
}

// NewBackward returns the set of texts read from their ends, for Longest
// to report the texts that start at each position.
func NewBackward(texts []string, work Counter) *Set {
	return build(texts, work, true)
}

// report adds to found the texts that end at node and at the nodes of its
// matched chain, stopping at a node this search has reported, whose chain
// is then reported already.
func (s *Set) report(node int32, found []int) []int {
	for node >= 0 && s.mark[node] != s.search {
		s.mark[node] = s.search
		found = append(found, s.ends[node]...)
		node = s.matched[node]
	}
	return found
}

// Find returns the positions in the list of the texts that occur in text,
// ascending; an empty listed text occurs in every text.
func (s *Set) Find(text string) []int {
	s.search++
	s.count("bytes searched", len(text))
	found := s.report(0, nil)
	current := int32(0)
	for i := range len(text) {
		current = s.next(current, s.byteAt(text, i))
		found = s.report(current, found)
	}
	slices.Sort(found)
	return found
}

// Longest raises longest[i], for each byte i of text, to the length of the
// longest listed text that starts at byte i (a backward set) or ends with
// byte i (a forward set); longest has text's length. The longest text found
// at a position is the node the search is at when it ends a listed text,
// else the nearest such node on its fallback chain, so each position costs
// one step and the fallbacks that step takes.
func (s *Set) Longest(text string, longest []int) {
	s.count("bytes searched", len(text))
	current := int32(0)
	for i := range len(text) {
		current = s.next(current, s.byteAt(text, i))
		found := current
		if len(s.ends[found]) == 0 {
			found = s.matched[found]
		}
		position := i
		if s.backward {
			position = len(text) - 1 - i
		}
		if found > 0 && s.depth[found] > longest[position] {
			longest[position] = s.depth[found]
		}
	}
}
