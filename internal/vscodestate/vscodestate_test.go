package vscodestate

import (
	"bytes"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// stateCopy copies the sqlitefile package's sites.db, whose ItemTable holds
// JSON values, a value that is not JSON and a secret:// key, to a
// state.vscdb under a new directory.
func stateCopy(t *testing.T) string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "state.vscdb")
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// compactValues writes each value as compact JSON, keyed by its key.
func compactValues(values map[string]jsonvalue.Value) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		out[key] = value.JSON()
	}
	return out
}

func TestReadItemsReturnsEachValueAndProblem(t *testing.T) {
	path := stateCopy(t)
	// From read_items on the same file: values decoded, the value that is not
	// JSON kept as text, the secret never read, problems in key order.
	values, problems, _ := ReadItems(osfile.ListSpellings, path, []string{"cloudAgentRepository.agentsX",
		"cloudAgentRepository.agents.u2", "secret://x", "cloudAgentRepository.agents.u1", "missing",
		"CLOUDAGENTREPOSITORY.agents.u3"})
	want := map[string]string{"CLOUDAGENTREPOSITORY.agents.u3": "[]",
		"cloudAgentRepository.agents.u1": "[1]", "cloudAgentRepository.agents.u2": "[]",
		"cloudAgentRepository.agentsX": `"no"`}
	if got := compactValues(values); !maps.Equal(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0],
		"ItemTable value for cloudAgentRepository.agentsX is not valid JSON (") {
		t.Errorf("problems = %q, want the one value problem", problems)
	}
	if shown := ShownProblem(problems[0]); shown != "ItemTable value for cloudAgentRepository.agentsX is not valid JSON" {
		t.Errorf("ShownProblem() = %q", shown)
	}
	value, problems, _ := ReadItem(osfile.ListSpellings, path, "missing")
	if !value.IsNull() || problems != nil {
		t.Errorf("ReadItem(osfile.ListSpellings, missing) = %v, %q; want null and no problem", value, problems)
	}
}

func TestReadItemsReportsAFileItCannotOpen(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "state.vscdb")
	if values, problems, _ := ReadItems(osfile.ListSpellings, path, []string{"k"}); len(values) != 0 || problems != nil {
		t.Errorf("an absent file gave %v, %q", values, problems)
	}
	if err := osfile.WriteFile(path, []byte("plain text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, problems, _ := ReadItems(osfile.ListSpellings, path, []string{"k"})
	if !slices.Equal(problems, []string{"state.vscdb is not a SQLite database"}) {
		t.Errorf("a text file gave %q", problems)
	}
	if values, problems, _ := ReadItems(osfile.ListSpellings, stateCopy(t), []string{"secret://x"}); len(values) != 0 || problems != nil {
		t.Errorf("only secret keys gave %v, %q; want nothing read", values, problems)
	}
	empty := filepath.Join(directory, "empty.vscdb")
	if err := osfile.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if values, problems, _ := ReadItems(osfile.ListSpellings, empty, []string{"k"}); len(values) != 0 || problems != nil {
		t.Errorf("an empty database gave %v, %q; want nothing", values, problems)
	}
}

func TestShownProblemCutsOnlyAValueDecoderDetail(t *testing.T) {
	cases := []struct{ problem, want string }{
		{"ItemTable value for k is not valid JSON (invalid JSON at line 1, column 1: expected a value)",
			"ItemTable value for k is not valid JSON"},
		{"state.vscdb could not be read (permission denied)", "state.vscdb could not be read (permission denied)"},
		{"state.vscdb-wal could not be read (permission denied)",
			"state.vscdb-wal could not be read (permission denied)"},
	}
	for _, test := range cases {
		t.Run(test.want, func(t *testing.T) {
			if got := ShownProblem(test.problem); got != test.want {
				t.Errorf("ShownProblem(%q) = %q, want %q", test.problem, got, test.want)
			}
		})
	}
}

func TestStateDatabasePathsAndEditors(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		{"User/globalStorage/state.vscdb", true},
		{".config/Code/User/globalStorage/state.vscdb", true},
		{".config/Code/user/GlobalStorage/State.VSCDB", true},
		{"USER/GLOBALSTORAGE/STATE.VSCDB", true},
		{".config/Code/User/globalStorage/state.v\u017fcdb", false},
		{".config/Code/XUser/globalStorage/state.vscdb", false},
		{".config/Code/User/globalStorage/state.vscdb-wal", false},
		{".config/Code/User/workspaceStorage/abc/state.vscdb", false},
	}
	for _, test := range cases {
		t.Run(test.rel, func(t *testing.T) {
			if got := IsStateDatabase(test.rel); got != test.want {
				t.Errorf("IsStateDatabase(%q) = %v, want %v", test.rel, got, test.want)
			}
		})
	}
	editors := EditorAgents()
	editors[0] = "changed"
	if EditorAgents()[0] != "vscode" || len(editors) != 6 {
		t.Error("EditorAgents should return a fresh list of the six editors")
	}
}

func TestReadItemsFindsItemTableInAnyASCIICase(t *testing.T) {
	cases := []struct {
		name, spelling string
		found          bool
	}{
		{"a table named ITEMTABLE is read", "ITEMTABLE", true},
		{"a table named itemtable is read", "itemtable", true},
		{"a table named ItemTablf is not read", "ItemTablf", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := stateCopy(t)
			data, err := osfile.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			renamed := bytes.ReplaceAll(data, []byte("ItemTable"), []byte(test.spelling))
			if err := osfile.WriteFile(path, renamed, 0o600); err != nil {
				t.Fatal(err)
			}
			values, problems, _ := ReadItems(osfile.ListSpellings, path, []string{"cloudAgentRepository.agents.u1"})
			if found := len(values) == 1; found != test.found || problems != nil {
				t.Errorf("ReadItems(osfile.ListSpellings, ) = %v, %q; want found %v", values, problems, test.found)
			}
		})
	}
}
