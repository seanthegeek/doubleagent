package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestTextEqualsTheEmbeddedFile(t *testing.T) {
	want, err := os.ReadFile("catalog.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(Text()) != string(want) {
		t.Fatal("Text() differs from catalog.txt")
	}
}

// countSections counts the table lines of each section of TEXT with the same
// rule the parser documents, written independently of it.
func countSections(text string) map[string]int {
	counts := map[string]int{}
	section := "home"
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if name, ok := sectionHeaders[line]; ok {
				section = name
			}
			continue
		}
		counts[section]++
	}
	return counts
}

func TestLoadCountsEqualTheFileLinesPerSection(t *testing.T) {
	tables, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	counts := countSections(string(Text()))
	cases := []struct {
		name    string
		section string
		got     int
	}{
		{"home artifacts", "home", len(tables.Catalog)},
		{"project artifacts", "project", len(tables.ProjectCatalog)},
		{"excludes", "exclude", len(tables.Excludes)},
		{"secret globs", "secret", len(tables.SecretGlobs)},
		{"docker volumes", "docker", len(tables.DockerVolumes)},
		{"relocation variables", "relocation", len(tables.RelocationVars)},
		{"startup files", "startup", len(tables.StartupFiles)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.got == 0 {
				t.Fatal("table is empty")
			}
			if testCase.got != counts[testCase.section] {
				t.Errorf("parsed %d entries, file has %d lines", testCase.got, counts[testCase.section])
			}
		})
	}
}

func TestParseSectionsAndFields(t *testing.T) {
	text := "# home artifacts (agent|glob relative to home)\n" +
		"# a vendor note\n" +
		"Agent A|.a/*|x\n\n" +
		"# excluded unless --full\n" +
		"# excluded unless --fullish\n" +
		".a/cache\n" +
		"# relocation variables (agent|VARIABLE|home-relative path it replaces)\n" +
		"Agent A|A_HOME|.a\n" +
		"Agent A|A_EXTRA|\n" +
		"# startup files searched for them (scope|glob; scope home, system or project)\n" +
		"system|etc/profile\n"
	tables, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.Catalog) != 1 || tables.Catalog[0] != (Entry{"Agent A", ".a/*|x"}) {
		t.Errorf("home table %v", tables.Catalog)
	}
	if len(tables.Excludes) != 1 || tables.Excludes[0] != ".a/cache" {
		t.Errorf("excludes %v", tables.Excludes)
	}
	if len(tables.RelocationVars) != 2 || tables.RelocationVars[1] != (RelocationVar{"Agent A", "A_EXTRA", ""}) {
		t.Errorf("relocation table %v", tables.RelocationVars)
	}
	if len(tables.StartupFiles) != 1 || tables.StartupFiles[0] != (StartupFile{"system", "etc/profile"}) {
		t.Errorf("startup table %v", tables.StartupFiles)
	}
}

func TestParseRejectsAMalformedLineNamingItsNumber(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"a home line without a bar", "Agent|.a\nno bar here\n", "line 2"},
		{"an empty glob", "Agent|\n", "line 1"},
		{"a relocation line with one bar", "# relocation variables (agent|VARIABLE|home-relative path it replaces)\n\nAgent|VAR\n", "line 3"},
		{"a startup line with an unknown scope", "# startup files searched for them (scope|glob; scope home, system or project)\nuser|x\n", "line 2"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Parse([]byte(testCase.text))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %v, want one naming %q", err, testCase.want)
			}
		})
	}
}
