package catalog

import (
	"cmp"
	_ "embed"
	"fmt"
	"slices"
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

// volumePrefix opens an EXCLUDES or SECRET_GLOBS line that applies only under
// a Docker or Podman volume's _data directory.
const volumePrefix = "volume|"

// Tables holds the seven tables, each in its defined order (sortTables),
// whatever the order of the lines in the file. Excludes and SecretGlobs hold
// the lines that apply under every collection base, VolumeExcludes and
// VolumeSecretGlobs the `volume|glob` lines, without the prefix.
type Tables struct {
	Catalog           []Entry
	ProjectCatalog    []Entry
	Excludes          []string
	SecretGlobs       []string
	VolumeExcludes    []string
	VolumeSecretGlobs []string
	DockerVolumes     []Entry
	RelocationVars    []RelocationVar
	StartupFiles      []StartupFile
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

// Parse reads the text of a catalog file into its tables and puts each table in
// its defined order (sortTables), so the order of the file's lines changes
// nothing the program does. A line that does not have the fields its table
// needs is an error naming the line number.
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
	tables.sortTables()
	return tables, nil
}

// sortTables puts every table in its defined order, comparing bytes field by
// field: the home, project and Docker tables by agent, then glob; relocation
// variables by agent, variable, then replaced path; exclusions and secret
// globs, each scope apart, by glob; startup files by scope, then glob. Two
// lines equal in every field are the same line, so the order is total.
func (tables *Tables) sortTables() {
	slices.SortFunc(tables.Catalog, compareEntries)
	slices.SortFunc(tables.ProjectCatalog, compareEntries)
	slices.SortFunc(tables.DockerVolumes, compareEntries)
	slices.Sort(tables.Excludes)
	slices.Sort(tables.SecretGlobs)
	slices.Sort(tables.VolumeExcludes)
	slices.Sort(tables.VolumeSecretGlobs)
	slices.SortFunc(tables.RelocationVars, compareRelocationVars)
	slices.SortFunc(tables.StartupFiles, compareStartupFiles)
}

// compareEntries orders two `agent|glob` lines by agent, then glob, byte by
// byte; it returns a negative number, zero or a positive number as
// cmp.Compare does.
func compareEntries(first, second Entry) int {
	return cmp.Or(strings.Compare(first.Agent, second.Agent), strings.Compare(first.Glob, second.Glob))
}

// compareRelocationVars orders two relocation lines by agent, variable, then
// replaced path, byte by byte.
func compareRelocationVars(first, second RelocationVar) int {
	return cmp.Or(strings.Compare(first.Agent, second.Agent),
		strings.Compare(first.Variable, second.Variable),
		strings.Compare(first.Replaces, second.Replaces))
}

// compareStartupFiles orders two startup lines by scope, then glob, byte by
// byte.
func compareStartupFiles(first, second StartupFile) int {
	return cmp.Or(strings.Compare(first.Scope, second.Scope), strings.Compare(first.Glob, second.Glob))
}

// isBlank reports whether LINE holds only ASCII white space.
func isBlank(line string) bool {
	return strings.Trim(line, " \t\r\v\f") == ""
}

// add stores one non-comment LINE in the table SECTION names.
func (tables *Tables) add(section, line string) error {
	switch section {
	case "exclude", "secret":
		return tables.addPathGlob(section, line)
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

// splitPathGlob splits an EXCLUDES or SECRET_GLOBS line into whether it
// carries the volume prefix and its glob. Any other bar is an error, so a
// mistyped scope is never read as part of a glob.
func splitPathGlob(line string) (volume bool, glob string, err error) {
	glob, volume = strings.CutPrefix(line, volumePrefix)
	if glob == "" || strings.Contains(glob, "|") {
		return false, "", fmt.Errorf("want glob or volume|glob, got %q", line)
	}
	return volume, glob, nil
}

// addPathGlob parses an EXCLUDES or SECRET_GLOBS line into the table its
// SECTION and scope name.
func (tables *Tables) addPathGlob(section, line string) error {
	volume, glob, err := splitPathGlob(line)
	if err != nil {
		return err
	}
	switch {
	case section == "exclude" && volume:
		tables.VolumeExcludes = append(tables.VolumeExcludes, glob)
	case section == "exclude":
		tables.Excludes = append(tables.Excludes, glob)
	case volume:
		tables.VolumeSecretGlobs = append(tables.VolumeSecretGlobs, glob)
	default:
		tables.SecretGlobs = append(tables.SecretGlobs, glob)
	}
	return nil
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
