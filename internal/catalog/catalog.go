// Package catalog holds the seven tables that say what the collector gathers:
// the home and project artifacts, the size exclusions, the credential files,
// the Docker volumes, the relocation variables and the startup files. The
// tables are one embedded text file in the format `collect --list` prints, so
// no path or glob lives in code.
package catalog

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed catalog.txt
var catalogText string

// Entry is one `agent|glob` line.
type Entry struct {
	Agent string
	Glob  string
}

// RelocationVar is one `agent|VARIABLE|replaces` line: an environment variable
// that moves an agent's state. Replaces is the home-relative path the value
// stands in for, empty when the variable adds a store with no default.
type RelocationVar struct {
	Agent    string
	Variable string
	Replaces string
}

// StartupFile is one `scope|glob` line: a file that may set an environment
// variable, in the scope home, system or project.
type StartupFile struct {
	Scope string
	Glob  string
}

// Tables holds the seven tables in file order.
type Tables struct {
	Catalog        []Entry
	ProjectCatalog []Entry
	Excludes       []string
	SecretGlobs    []string
	DockerVolumes  []Entry
	RelocationVars []RelocationVar
	StartupFiles   []StartupFile
}

// Text returns the embedded catalog file, byte for byte, as `--list` prints it.
func Text() []byte {
	return []byte(catalogText)
}

// Load parses the embedded catalog file.
func Load() (*Tables, error) {
	return Parse(Text())
}

// sectionName returns the table a comment LINE opens, and whether it opens
// one. Only a comment equal to one of the seven headers switches the table;
// any other comment, a vendor note inside a table included, is skipped.
func sectionName(line string) (string, bool) {
	switch line {
	case "# home artifacts (agent|glob relative to home)":
		return "home", true
	case "# project artifacts (relative to each discovered project)":
		return "project", true
	case "# excluded unless --full":
		return "exclude", true
	case "# credential files (flagged secret, skipped with --no-secrets)":
		return "secret", true
	case "# docker volumes (agent|volume name glob)":
		return "docker", true
	case "# relocation variables (agent|VARIABLE|home-relative path it replaces)":
		return "relocation", true
	case "# startup files searched for them (scope|glob; scope home, system or project)":
		return "startup", true
	}
	return "", false
}

// Parse reads the text of a catalog file into its tables. A line that does not
// have the fields its table needs is an error naming the line number.
func Parse(text []byte) (*Tables, error) {
	tables := &Tables{}
	section := "home"
	for number, line := range strings.Split(string(text), "\n") {
		if isBlank(line) {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if name, ok := sectionName(line); ok {
				section = name
			}
			continue
		}
		if err := tables.add(section, line); err != nil {
			return nil, fmt.Errorf("catalog line %d: %w", number+1, err)
		}
	}
	return tables, nil
}

// isBlank reports whether LINE holds only ASCII white space.
func isBlank(line string) bool {
	return strings.Trim(line, " \t\r\v\f") == ""
}

// add stores one non-comment LINE in the table SECTION names.
func (tables *Tables) add(section, line string) error {
	switch section {
	case "exclude":
		tables.Excludes = append(tables.Excludes, line)
	case "secret":
		tables.SecretGlobs = append(tables.SecretGlobs, line)
	case "relocation":
		return tables.addRelocation(line)
	case "startup":
		return tables.addStartup(line)
	default:
		entry, err := parseEntry(line)
		if err != nil {
			return err
		}
		tables.addEntry(section, entry)
	}
	return nil
}

// addEntry appends ENTRY to the `agent|glob` table SECTION names.
func (tables *Tables) addEntry(section string, entry Entry) {
	switch section {
	case "home":
		tables.Catalog = append(tables.Catalog, entry)
	case "project":
		tables.ProjectCatalog = append(tables.ProjectCatalog, entry)
	case "docker":
		tables.DockerVolumes = append(tables.DockerVolumes, entry)
	}
}

// parseEntry splits an `agent|glob` line at its first bar; the glob may hold
// further bars.
func parseEntry(line string) (Entry, error) {
	agent, glob, found := strings.Cut(line, "|")
	if !found || agent == "" || glob == "" {
		return Entry{}, fmt.Errorf("want agent|glob, got %q", line)
	}
	return Entry{Agent: agent, Glob: glob}, nil
}

// addRelocation parses an `agent|VARIABLE|replaces` line.
func (tables *Tables) addRelocation(line string) error {
	agent, rest, found := strings.Cut(line, "|")
	variable, replaces, foundSecond := strings.Cut(rest, "|")
	if !found || !foundSecond || agent == "" || variable == "" {
		return fmt.Errorf("want agent|VARIABLE|replaces, got %q", line)
	}
	tables.RelocationVars = append(tables.RelocationVars,
		RelocationVar{Agent: agent, Variable: variable, Replaces: replaces})
	return nil
}

// addStartup parses a `scope|glob` line whose scope is home, system or project.
func (tables *Tables) addStartup(line string) error {
	scope, glob, found := strings.Cut(line, "|")
	validScope := scope == "home" || scope == "system" || scope == "project"
	if !found || !validScope || glob == "" {
		return fmt.Errorf("want home|glob, system|glob or project|glob, got %q", line)
	}
	tables.StartupFiles = append(tables.StartupFiles, StartupFile{Scope: scope, Glob: glob})
	return nil
}
