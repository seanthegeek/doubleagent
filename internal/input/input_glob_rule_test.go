package input

import (
	"testing"
)

// A loose tree is attributed by the rule collect expands the catalog with:
// ASCII case ignored, and a `*` never taking a hidden name.
func TestALooseTreeIsMatchedAsCollectMatchesTheCatalog(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, file, rel, want string }{
		{"a directory in another case is found", "home/alice/.config/Goose/config.yaml",
			".config/Goose/config.yaml", "goose"},
		{"a Windows directory copied in another case is found", "Users/bob/AppData/Roaming/Devin/mcp_config.json",
			"AppData/Roaming/Devin/mcp_config.json", "windsurf"},
		{"a star leaves a hidden name alone", "home/carol/.local/share/.hidden/flow_states.db",
			".local/share/.hidden/flow_states.db", ""},
		{"a star takes a name that is not hidden", "home/carol/.local/share/crew/flow_states.db",
			".local/share/crew/flow_states.db", "crewai"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{test.file: "{}"})
			collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			wantEqual(t, "agent of "+test.rel, agentsByRel(collection)[test.rel], test.want)
		})
	}
}
