package analyze

import (
	"fmt"
	"io"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/pytext"
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
		fmt.Fprintf(stderr, "error: %v\n", err)
		return ExitError
	}
	parsed := parsers.ByAgent(env.Parsers().All())
	for _, agent := range found.Agents() {
		state := "detect only"
		if len(parsed[agent]) > 0 {
			state = "parser"
		}
		fmt.Fprintf(stdout, "%s %s\n", pytext.PadRight(agent, 20), state)
	}
	return ExitFound
}
