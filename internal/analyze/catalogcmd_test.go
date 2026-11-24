package analyze

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/parsers/chatgptdesktop"
)

// catalogAgents are the home agents of the bundled catalog in table order,
// which the loader sorts by agent name, each listed once.
var catalogAgents = []string{"agent-zero", "aider", "amp", "antigravity", "atlarix", "augment",
	"autogen-studio", "camel-ai", "chatgpt-desktop", "claude-code", "claude-desktop", "cline",
	"codex-cli", "cody", "continue", "copilot", "copilot-cli", "copilot-desktop", "crewai",
	"crush", "cursor", "dify", "factory-droid", "flowise", "gemini-cli", "goose", "hermes",
	"kilo-code", "kiro", "langflow", "letta", "little-coder", "local-deep-research", "metagpt",
	"muse-code", "n8n", "nanobot", "ollama", "open-interpreter", "openclaw", "opencode",
	"openhands", "pearai", "pi", "pydantic-clai", "qwen-code", "qwen-live", "roo-code",
	"shared", "shellgpt", "tabby", "twinny", "vscode", "windsurf", "zed"}

// testEnvironment returns an environment with a fixed clock and a registry
// of the given parsers.
func testEnvironment(list ...func() parserkit.Parser) Environment {
	return Environment{
		Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: func() *parsers.Registry {
			made := make([]parserkit.Parser, len(list))
			for i, construct := range list {
				made[i] = construct()
			}
			return parsers.NewOf(made)
		},
		Version: "0.0.0-test",
	}
}

// runCommand runs one command line and returns its exit code and output.
func runCommand(env Environment, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, env, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCatalogPrintsTheEmbeddedCatalog(t *testing.T) {
	code, stdout, stderr := runCommand(testEnvironment(), "catalog")
	if code != ExitFound || stdout != string(catalog.Text()) || stderr != "" {
		t.Errorf("exit %d, stderr %q, stdout equal to the catalog: %v", code, stderr,
			stdout == string(catalog.Text()))
	}
}

func TestCatalogAgentsNamesEachAgentAndWhetherAParserReadsIt(t *testing.T) {
	env := testEnvironment(func() parserkit.Parser { return chatgptdesktop.New() })
	code, stdout, _ := runCommand(env, "catalog", "--agents")
	var want strings.Builder
	for _, agent := range catalogAgents {
		state := "detect only"
		if agent == "chatgpt-desktop" {
			state = "parser"
		}
		fmt.Fprintf(&want, "%-20s %s\n", agent, state)
	}
	if code != ExitFound || stdout != want.String() {
		t.Errorf("exit %d, output:\n%s\nwant:\n%s", code, stdout, want.String())
	}
}

func TestCatalogHelpAndMistakes(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		code               int
		stdout, stderrPart string
	}{
		{"help goes to stdout", []string{"catalog", "--help"}, ExitFound, catalogUsage, ""},
		{"an extra argument is a usage error", []string{"catalog", "x"}, ExitError, "",
			"doubleagent catalog: unexpected argument: x\nUsage: doubleagent catalog"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(testEnvironment(), test.args...)
			if code != test.code || stdout != test.stdout || !strings.Contains(stderr, test.stderrPart) {
				t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
		})
	}
}
