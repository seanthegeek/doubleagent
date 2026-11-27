package catalog_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/catalog/catalogtest"
)

// parseText parses TEXT for a test, failing the test when it does not parse.
func parseText(t *testing.T, text []byte) *catalog.Tables {
	t.Helper()
	tables, err := catalog.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestTheOrderOfTheFileLinesChangesNoTable(t *testing.T) {
	embedded := parseText(t, catalog.Text())
	reordered := map[string][]byte{"reversed": catalogtest.Reversed(catalog.Text())}
	for _, seed := range []uint64{1, 583, 20261123} {
		reordered["shuffled with seed "+strconv.FormatUint(seed, 10)] =
			catalogtest.Shuffled(catalog.Text(), seed)
	}
	for name, text := range reordered {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if string(text) == string(catalog.Text()) {
				t.Fatal("the reordered text is the embedded text")
			}
			if got := parseText(t, text); !reflect.DeepEqual(got, embedded) {
				t.Error("the reordered text parses to other tables than the embedded text")
			}
		})
	}
}

func TestParseSortsEachTableByItsOwnKey(t *testing.T) {
	text := "pi-x|.b\npi|.b\npi|.a\n" +
		"# project artifacts (relative to each discovered project)\nz|.z\na|.a\n" +
		"# excluded unless --full\nb/*\na/*\n" +
		"# credential files (flagged secret, skipped with --no-secrets)\nz.json\na.json\n" +
		"# docker volumes (agent|volume name glob)\nz|*z\na|*a\n" +
		"# relocation variables (agent|VARIABLE|home-relative path it replaces)\n" +
		"a|B_DIR|\na|A_DIR|.b\na|A_DIR|.a\n" +
		"# startup files searched for them (scope|glob; scope home, system or project)\n" +
		"system|/etc/a\nhome|.b\nhome|.a\n"
	want := &catalog.Tables{
		Catalog:        []catalog.Entry{{Agent: "pi", Glob: ".a"}, {Agent: "pi", Glob: ".b"}, {Agent: "pi-x", Glob: ".b"}},
		ProjectCatalog: []catalog.Entry{{Agent: "a", Glob: ".a"}, {Agent: "z", Glob: ".z"}},
		Excludes:       []string{"a/*", "b/*"},
		SecretGlobs:    []string{"a.json", "z.json"},
		DockerVolumes:  []catalog.Entry{{Agent: "a", Glob: "*a"}, {Agent: "z", Glob: "*z"}},
		RelocationVars: []catalog.RelocationVar{{Agent: "a", Variable: "A_DIR", Replaces: ".a"},
			{Agent: "a", Variable: "A_DIR", Replaces: ".b"}, {Agent: "a", Variable: "B_DIR"}},
		StartupFiles: []catalog.StartupFile{{Scope: "home", Glob: ".a"}, {Scope: "home", Glob: ".b"},
			{Scope: "system", Glob: "/etc/a"}},
	}
	if got := parseText(t, []byte(text)); !reflect.DeepEqual(got, want) {
		t.Errorf("tables %+v, want %+v", got, want)
	}
}
