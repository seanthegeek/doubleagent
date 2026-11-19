package tabby

import "github.com/seanthegeek/doubleagent/internal/parser"

// placement is where an element stands for the history: whether the
// nearest component above or at it is the plugin's, and whether it is or
// lies below a "history" option of that component.
type placement struct {
	plugin, history bool
}

// placed returns the placement of an element whose parent has the
// placement above. The IDE writes each component's state under its own
// component element, so the nearest component is the one whose state
// holds an element, and a component starts a placement afresh.
func placed(current *element, above placement) placement {
	name, _ := current.attribute("name")
	switch {
	case current.tag == "component":
		return placement{plugin: name == intellijComponent}
	case current.tag == "option" && name == "history" && above.plugin:
		return placement{plugin: true, history: true}
	}
	return above
}

// historyCommands returns, in document order, the value of every element
// below a "history" option of the plugin's component, the nearest
// component above it: an element under two nested history options or
// components is given once, and one under another component nested in the
// plugin's is not given. Every element is looked at once, counted in work.
func historyCommands(document *element, work parser.Work) []string {
	// below is whether the element lies below a history option.
	type frame struct {
		element *element
		placed  placement
		below   bool
	}
	var commands []string
	pending := []frame{{element: document, placed: placed(document, placement{})}}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		work.Add("elements looked at", 1)
		if value, ok := current.element.attribute("value"); ok && current.below {
			work.Add("commands given", 1)
			commands = append(commands, value)
		}
		for i := len(current.element.children) - 1; i >= 0; i-- {
			child := current.element.children[i]
			pending = append(pending, frame{element: child, placed: placed(child, current.placed),
				below: current.placed.history})
		}
	}
	return commands
}
