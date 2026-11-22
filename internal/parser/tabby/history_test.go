package tabby

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

// commandsByWalk returns the history commands by walking down from the
// root with the path to each element, giving an element with a value once
// when its nearest enclosing component is the plugin's and a history
// option lies between that component and the element, which
// historyCommands must agree with.
func commandsByWalk(document *element) []string {
	var commands []string
	var walk func(current *element, path []*element)
	walk = func(current *element, path []*element) {
		if value, ok := current.attribute("value"); ok && givenByPath(path) {
			commands = append(commands, value)
		}
		for _, child := range current.children {
			walk(child, append(slices.Clone(path), current))
		}
	}
	walk(document, nil)
	return commands
}

// givenByPath reports whether the elements above an element, root first,
// make it a history command.
func givenByPath(path []*element) bool {
	history := false
	for i := len(path) - 1; i >= 0; i-- {
		name, _ := path[i].attribute("name")
		switch {
		case path[i].tag == "component":
			return history && name == intellijComponent
		case path[i].tag == "option" && name == "history":
			history = true
		}
	}
	return false
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

// nestedHistory returns a history file of the plugin's component inside
// another, each holding a history option inside another, around n values.
func nestedHistory(n int) string {
	var document strings.Builder
	document.WriteString("<application>\n")
	for range 2 {
		document.WriteString(`<component name="` + intellijComponent + `">` + "\n")
	}
	for range 2 {
		document.WriteString(`<option name="history"><list>` + "\n")
	}
	for i := range n {
		document.WriteString(`<option value="command ` + strconv.Itoa(i) + `" />` + "\n")
	}
	for range 2 {
		document.WriteString("</list></option>\n")
	}
	for range 2 {
		document.WriteString("</component>\n")
	}
	document.WriteString("</application>\n")
	return document.String()
}

func TestEachHistoryCommandIsGivenOnceHoweverDeepItsComponentAndOptionNest(t *testing.T) {
	for _, n := range []int{1, 10, 100} {
		t.Run(strconv.Itoa(n)+" commands", func(t *testing.T) {
			rows := parseFile(t, "Library/Application Support/JetBrains/IC2024/options/"+
				"intellij-tabby-command-history.xml", nestedHistory(n))
			if len(rows) != n+1 {
				t.Fatalf("got %d rows, want the history row and %d commands", len(rows), n)
			}
			want := "inline edit command history: " + strconv.Itoa(n) + " commands"
			if !strings.HasPrefix(rows[0], want) {
				t.Errorf("got %q, want it to start %q", rows[0], want)
			}
		})
	}
}

func TestACommandUnderAnotherComponentInsideThePluginsIsNotGiven(t *testing.T) {
	document := `<application><component name="` + intellijComponent + `">` +
		`<option name="history"><list><option value="mine" /></list></option>` +
		`<component name="other"><option name="history"><list><option value="theirs" />` +
		`</list></option></component></component></application>`
	root, err := parseXML([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if got := historyCommands(root, parser.Work{}); !slices.Equal(got, []string{"mine"}) {
		t.Errorf("got %q, want only the plugin's own command", got)
	}
}
