package growthtests

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserregistry"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// carriedCase is a file in which one value, long, stands before each of n
// entries in their rows' text: the parser that reads it, where, its text,
// and the name the problem of the cut gives the value.
type carriedCase struct {
	parser, rel, name string
	text              func(long string, n int) string
}

// entries returns n JSON strings joined by commas, each ending in its
// number.
func entries(prefix string, n int) string {
	var items []string
	for i := range n {
		items = append(items, `"`+prefix+strconv.Itoa(i)+`"`)
	}
	return strings.Join(items, ",")
}

// members returns n JSON members joined by commas, each naming its number.
func members(n int, value string) string {
	var items []string
	for i := range n {
		items = append(items, `"m`+strconv.Itoa(i)+`": `+value)
	}
	return strings.Join(items, ",")
}

// carriedCases returns the files of the parsers that write one carried
// value into the text of many rows, each built for a long value and n
// entries.
func carriedCases() []carriedCase {
	return slices.Concat(listCases(), keyCases())
}

// listCases returns the files whose carried value stands before each item
// of a list or a role's blocks.
func listCases() []carriedCase {
	return []carriedCase{
		{"goose", ".config/goose/permission.yaml", "permission category", func(long string, n int) string {
			return long + ":\n  always_allow:\n" + strings.Repeat("    - tool\n", n)
		}},
		{"goose", ".local/share/goose/sessions/20260101_000000.jsonl", "role", func(long string, n int) string {
			return `{"role": "` + long + `", "content": [` + strings.TrimSuffix(strings.Repeat(
				`{"type": "text", "text": "a"},`, n), ",") + `]}` + "\n"
		}},
		{"nanobot", ".nanobot/pairing.json", "approved channel", func(long string, n int) string {
			return `{"approved": {"` + long + `": [` + entries("s", n) + `]}}`
		}},
		{"nanobot", ".nanobot/config.json", "channel name", func(long string, n int) string {
			return `{"channels": {"` + long + `": {"allowFrom": [` + entries("u", n) + `]}}}`
		}},
		{"qwen-code", ".qwen/projects/p/session-organization.v1.json", "group name", func(long string, n int) string {
			return `{"groups": [{"id": "g", "name": "` + long + `"}], "sessions": {` + members(n, `{"groupId": "g"}`) +
				`}}`
		}},
		{"crush", ".config/crush/crush.json", "settings key", func(long string, n int) string {
			return `{"mcp": {"` + long + `": {"disabled_tools": [` + entries("t", n) + `]}}}`
		}},
	}
}

// keyCases returns the files whose carried value is a key or name written
// before each member below it, or a value each later record repeats.
func keyCases() []carriedCase {
	return []carriedCase{
		{"codex-cli", ".codex/config.toml", "settings key", func(long string, n int) string {
			var out strings.Builder
			out.WriteString("[mcp_servers." + long + ".tools]\n")
			for i := range n {
				out.WriteString("t" + strconv.Itoa(i) + " = { approval_mode = \"approve\" }\n")
			}
			return out.String()
		}},
		{"antigravity", ".gemini/config/config.json", "settings key", func(long string, n int) string {
			return `{"sidecars": {"` + long + `": {"resolvedPermissions": {"accessGrants": [` + entries("g", n) +
				`]}}}}`
		}},
		{"copilot-cli", ".copilot/permissions-config.json", "locations key", func(long string, n int) string {
			return `{"locations": {"` + long + `": {"tool_approvals": [` + entries("t", n) + `]}}}`
		}},
		{"kiro", ".aws/amazonq/cli-agents/a.json", "tool key", func(long string, n int) string {
			return `{"toolsSettings": {"` + long + `": {` + members(n, "true") + `}}}`
		}},
		{"cline", ".cline/data/settings/cline_mcp_settings.json", "server name", func(long string, n int) string {
			return `{"mcpServers": {"` + long + `": {"autoApprove": [` + entries("t", n) + `]}}}`
		}},
		{"roo-code", ".vscode-mock/global-storage/settings/mcp_settings.json", "server name",
			func(long string, n int) string {
				return `{"mcpServers": {"` + long + `": {"alwaysAllow": [` + entries("t", n) + `]}}}`
			}},
		{"kilo-code", ".config/kilo/kilo.json", "permission key", func(long string, n int) string {
			return `{"permission": {"` + long + `": {` + members(n, `"allow"`) + `}}}`
		}},
		{"zed", ".config/zed/settings.json", "settings profile", func(long string, n int) string {
			return `{"profiles": {"` + long + `": {"settings": {"agent": {"tool_permissions": {` + members(n, "true") +
				`}}}}}}`
		}},
		{"cody", ".config/Code/User/settings.json", "settings key", func(long string, n int) string {
			return `{"cody.chat.agenticContext": {"` + long + `": {` + members(n, "true") + `}}}`
		}},
		{"cody", ".config/Cody-nodejs/JetBrains-globalState/cody-model-preferences", "endpoint",
			func(long string, n int) string {
				return `{"` + long + `": {"selected": {` + members(n, `"model"`) + `}}}`
			}},
		{"nanobot", ".nanobot/sessions/0123456789abcdef0123456789abcdef/.webui_session_index.json", "index version",
			func(long string, n int) string {
				var sessions []string
				for i := range n {
					sessions = append(sessions, `{"key": "k`+strconv.Itoa(i)+`"}`)
				}
				return `{"version": "` + long + `", "sessions": [` + strings.Join(sessions, ",") + `]}`
			}},
		{"twinny", ".config/Code/User/globalStorage/rjmacarthy.twinny/agent-changes/c.json", "reply id",
			func(long string, n int) string {
				return `{"id": "` + long + `", "files": [` + strings.TrimSuffix(strings.Repeat(`{"path": "p"},`, n), ",") +
					`]}`
			}},
	}
}

// carriedOutput parses text at rel with the registry's parser called name
// as a run does (parserkit.Collect) and returns its rows and their bytes.
func carriedOutput(t *testing.T, name, rel, text string) ([]timeline.Row, int) {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: name,
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	size := 0
	for _, each := range parserregistry.All() {
		if each.Name() != name {
			continue
		}
		if !each.Wants(artifact) {
			t.Fatalf("%s does not read %s", name, rel)
		}
		if _, err := parserkit.Collect(each, artifact, parserkit.Options{}, func(row timeline.Row) {
			rows = append(rows, row)
			for _, field := range row.Strings() {
				size += len(field)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	return rows, size
}

// One value of 50,000 bytes written before each of n entries is cut in
// every row, with one problem naming it, so twice the entries cost twice
// the rows' bytes at most, never the value's length again.
func TestACarriedValueIsCutInTheTextOfEveryRowItStandsIn(t *testing.T) {
	long := "v" + strings.Repeat("a", 50_000)
	for _, test := range carriedCases() {
		t.Run(test.parser+" "+test.name, func(t *testing.T) {
			var sizes []int
			for _, n := range []int{20, 40} {
				rows, size := carriedOutput(t, test.parser, test.rel, test.text(long, n))
				sizes = append(sizes, size)
				problems, whole := 0, 0
				for _, row := range rows {
					if strings.HasPrefix(row.Text, parserkit.ProblemPrefix+test.name+" longer than") {
						problems++
					}
					if strings.Contains(row.Text, long) {
						whole++
					}
				}
				// The value is written whole only where it is the element's own, once at most.
				if len(rows) < n || problems != 1 || whole > 1 {
					t.Errorf("%d entries gave %d rows, %d problems naming %s and %d whole values, want %d rows, "+
						"one problem and one whole value at most", n, len(rows), problems, test.name, whole, n)
				}
			}
			if growth := sizes[1] - sizes[0]; growth > 20*(2*parserkit.CarriedLimit+1024) {
				t.Errorf("20 more entries cost %d bytes", growth)
			}
		})
	}
}
