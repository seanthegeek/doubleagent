package cursor

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// textSet finds which of a fixed list of texts occur in a text, in one pass
// over that text whatever the number of texts. It is an Aho-Corasick
// automaton: a tree of the texts' prefixes where each node also links to
// the longest proper suffix of its prefix that is a node too, so a byte
// that leaves the tree falls back along those links instead of starting
// over. Searching with strings.Contains would cost the length of the text
// once per listed text.
type textSet struct {
	// edges maps a node and a byte, as node<<8 | byte, to the child node.
	edges map[uint64]int32
	// children lists each node's children with the byte that leads there.
	children [][]step
	// fallback is a node's longest proper suffix in the tree, and matched
	// the nearest node on its fallback chain that ends a listed text (-1
	// when none).
	fallback, matched []int32
	// ends holds the positions in the list of the texts a node ends.
	ends [][]int
	// mark holds, per node, the search that last reported it, so a node
	// and the chain below it are reported once per search.
	mark   []int
	search int
	work   parser.Work
}

// step is one edge of the tree: the byte read and the node it leads to.
type step struct {
	value byte
	node  int32
}

// newTextSet returns the set of texts, counting in work the bytes each
// search reads and the fallbacks it takes.
func newTextSet(texts []string, work parser.Work) *textSet {
	set := &textSet{edges: map[uint64]int32{}, work: work}
	set.node()
	for position, text := range texts {
		current := int32(0)
		for i := range len(text) {
			child, ok := set.child(current, text[i])
			if !ok {
				child = set.node()
				set.edges[edgeKey(current, text[i])] = child
				set.children[current] = append(set.children[current], step{text[i], child})
			}
			current = child
		}
		set.ends[current] = append(set.ends[current], position)
	}
	set.link()
	return set
}

// edgeKey returns the key in edges of the edge from node by value.
func edgeKey(node int32, value byte) uint64 {
	return uint64(node)<<8 | uint64(value)
}

// child returns the node one edge from node by value, false when the tree
// has none.
func (s *textSet) child(node int32, value byte) (int32, bool) {
	child, ok := s.edges[edgeKey(node, value)]
	return child, ok
}

// next returns the node a search reaches from node on reading value: the
// child by value of node, or of the first node on its fallback chain that
// has one, else the root.
func (s *textSet) next(node int32, value byte) int32 {
	for {
		if child, ok := s.child(node, value); ok {
			return child
		}
		if node == 0 {
			return 0
		}
		s.work.Add("fallbacks taken", 1)
		node = s.fallback[node]
	}
}

// node adds an empty node and returns its number.
func (s *textSet) node() int32 {
	s.children = append(s.children, nil)
	s.fallback = append(s.fallback, 0)
	s.matched = append(s.matched, -1)
	s.ends = append(s.ends, nil)
	s.mark = append(s.mark, 0)
	return int32(len(s.fallback) - 1)
}

// link sets each node's fallback and matched links breadth first, so a
// node's fallback, which is shallower, is linked before it.
func (s *textSet) link() {
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

// find returns the positions in the list of the texts that occur in text,
// ascending; an empty listed text occurs in every text.
func (s *textSet) find(text string) []int {
	s.search++
	s.work.Add("bytes searched", len(text))
	found := s.report(0, nil)
	current := int32(0)
	for i := range len(text) {
		current = s.next(current, text[i])
		found = s.report(current, found)
	}
	slices.Sort(found)
	return found
}

// report adds to found the texts that end at node and at the nodes of its
// matched chain, stopping at a node this search has reported, whose chain
// is then reported already.
func (s *textSet) report(node int32, found []int) []int {
	for node >= 0 && s.mark[node] != s.search {
		s.mark[node] = s.search
		found = append(found, s.ends[node]...)
		node = s.matched[node]
	}
	return found
}
