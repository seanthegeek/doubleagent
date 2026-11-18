package analyze

import (
	"fmt"
	"io"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// runCatalog prints the bundled catalog as collect --list does, or with
// --agents each home agent of the catalog and whether a parser reads it.
func runCatalog(options CatalogOptions, env Environment, stdout, stderr io.Writer) int {
	if !options.Agents {
		_, _ = stdout.Write(catalog.Text())
		return ExitFound
	}
	found, err := matcher()
	if err != nil {
		fmt.Fprint(stderr, "error: "+err.Error()+"\n")
		return ExitError
	}
	parsed := parsers.ByAgent(env.Parsers().All())
	for _, agent := range found.Agents() {
		state := "detect only"
		if len(parsed[agent]) > 0 {
			state = "parser"
		}
		fmt.Fprintf(stdout, "%-20s %s\n", agent, state)
	}
	return ExitFound
}
