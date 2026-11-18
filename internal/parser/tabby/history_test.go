package tabby

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// commandsByWalk returns the history commands by walking the subtree of
// each component and option again, which historyCommands must agree with.
func commandsByWalk(document *element) []string {
	var commands []string
	for _, component := range document.iter("component") {
		if name, _ := component.attribute("name"); name != intellijComponent {
			continue
		}
		for _, option := range component.iter("option") {
			if name, _ := option.attribute("name"); name != "history" {
				continue
			}
			for _, each := range option.iter("")[1:] {
				if value, ok := each.attribute("value"); ok {
					commands = append(commands, value)
				}
			}
		}
	}
	return commands
}

// randomTree returns an element with up to depth levels of random
// children: components and options with and without the names the
// history needs, and elements with and without a value.
func randomTree(source *rand.Rand, depth int, counter *int) *element {
	*counter++
	created := &element{attributes: map[string]string{}}
	switch source.IntN(4) {
	case 0:
		created.tag = "component"
		created.attributes["name"] = []string{intellijComponent, "other"}[source.IntN(2)]
	case 1:
		created.tag = "option"
		created.attributes["name"] = []string{"history", "other"}[source.IntN(2)]
	default:
		created.tag = "item"
	}
	if source.IntN(2) == 0 {
		created.attributes["value"] = "v" + strconv.Itoa(*counter)
	}
	if depth > 0 {
		for range source.IntN(4) {
			created.children = append(created.children, randomTree(source, depth-1, counter))
		}
	}
	return created
}

func TestHistoryCommandsAreTheOnesAWalkOfEachSubtreeFinds(t *testing.T) {
	for seed := range uint64(400) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			counter := 0
			document := randomTree(source, 5, &counter)
			if got, want := historyCommands(document, parser.Work{}), commandsByWalk(document); !slices.Equal(got,
				want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// nestedComponents returns n components of the plugin, each inside the
// last, holding no history.
func nestedComponents(n int) *element {
	root := &element{tag: "application"}
	current := root
	for range n {
		child := &element{tag: "component", attributes: map[string]string{"name": intellijComponent}}
		current.children = []*element{child}
		current = child
	}
	return root
}

func TestNestedComponentsAreWalkedOnce(t *testing.T) {
	for _, n := range []int{100, 1000} {
		work := parser.CountingWork()
		if commands := historyCommands(nestedComponents(n), work); len(commands) != 0 {
			t.Fatalf("%d nested components gave commands %q", n, commands)
		}
		if got := work.Count("elements looked at") + work.Count("commands given"); got != n+1 {
			t.Errorf("%d nested components: looked at %d elements, want %d", n, got, n+1)
		}
	}
}
