package tabby

import (
	"sort"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// spanned is an element met in a walk of the document: its position in
// document order and the position just past the elements below it, so an
// element lies below another exactly when its position falls between the
// other's two.
type spanned struct {
	element    *element
	start, end int
}

// historyCommands returns the value of every element under a "history"
// option of the plugin's component, for each such component in document
// order, then each such option under it, then each element under that,
// so an element under two nested components or options is given once for
// each. Every element is walked once, in document order, and an option or
// component is only looked at again for the commands it gives, counted in
// work.
func historyCommands(document *element, work parser.Work) []string {
	var components, options, valued []spanned
	// entry is an element's place in one of the lists, to close when its
	// children are done.
	type entry struct {
		list     *[]spanned
		position int
	}
	type frame struct {
		element *element
		next    int
		entries []entry
	}
	stack := []frame{{element: document}}
	position := 0
	opened := func(current *element) []entry {
		work.Add("elements looked at", 1)
		var lists []*[]spanned
		name, _ := current.attribute("name")
		switch {
		case current.tag == "component" && name == intellijComponent:
			lists = append(lists, &components)
		case current.tag == "option" && name == "history":
			lists = append(lists, &options)
		}
		if _, ok := current.attribute("value"); ok {
			lists = append(lists, &valued)
		}
		entries := make([]entry, len(lists))
		for i, list := range lists {
			entries[i] = entry{list: list, position: len(*list)}
			*list = append(*list, spanned{element: current, start: position})
		}
		position++
		return entries
	}
	stack[0].entries = opened(document)
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next < len(top.element.children) {
			child := top.element.children[top.next]
			top.next++
			stack = append(stack, frame{element: child, entries: opened(child)})
			continue
		}
		for _, each := range top.entries {
			(*each.list)[each.position].end = position
		}
		stack = stack[:len(stack)-1]
	}
	// below returns the positions in list of the elements from start up to
	// end in document order.
	below := func(list []spanned, start, end int) (int, int) {
		first := sort.Search(len(list), func(i int) bool { return list[i].start >= start })
		last := sort.Search(len(list), func(i int) bool { return list[i].start >= end })
		return first, last
	}
	type giving struct {
		start      int
		first, end int
	}
	var givers []giving
	for _, option := range options {
		if first, last := below(valued, option.start+1, option.end); first < last {
			givers = append(givers, giving{start: option.start, first: first, end: last})
		}
	}
	var commands []string
	for _, component := range components {
		first := sort.Search(len(givers), func(i int) bool { return givers[i].start >= component.start })
		for _, giver := range givers[first:] {
			if giver.start >= component.end {
				break
			}
			for _, each := range valued[giver.first:giver.end] {
				work.Add("commands given", 1)
				value, _ := each.element.attribute("value")
				commands = append(commands, value)
			}
		}
	}
	return commands
}
