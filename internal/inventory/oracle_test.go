package inventory

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// discardLog drops log lines.
type discardLog struct{}

func (discardLog) Line(string) error { return nil }

var atField = regexp.MustCompile(`"at":"[^"]*",`)

// shInventory runs the sh collector's --inventory over root and returns its
// lines without the at field.
func shInventory(t *testing.T, root string) []string {
	t.Helper()
	shell, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(shell, script, "--inventory", "-r", root, "-q", "--no-projects", "--no-docker")
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash", "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("sh collector: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		lines = append(lines, atField.ReplaceAllString(line, ""))
	}
	return lines
}

// buildFixture writes the image the inventory is taken over and returns its root.
func buildFixture(t *testing.T) string {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for relative, content := range map[string]string{
		"etc/passwd":                           "alice:x:1000:1000::/home/alice:/bin/sh\n",
		"home/alice/.claude/settings.json":     "{}",
		"home/alice/.claude/projects/p/s":      "session",
		"home/alice/.claude.json":              "secret",
		"home/alice/.codex/config.toml":        "x",
		"home/alice/.gemini/g":                 "gem",
		"home/alice/.gemini/antigravity-cli/a": "anti",
	} {
		full := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		past := time.Unix(1700000000+int64(len(content)), 0)
		if err := os.Chtimes(full, past, past); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLinesMatchShInventory(t *testing.T) {
	root := buildFixture(t)
	want := shInventory(t, root)
	if len(want) < 4 {
		t.Fatalf("sh inventory has %d lines; the fixture is wrong: %q", len(want), want)
	}
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	aggregator := NewAggregator()
	stageDir := t.TempDir()
	collector := &walk.Collector{
		Tables: tables, Excludes: tables.Excludes, Secret: walk.MatchAny(tables.SecretGlobs),
		Log: discardLog{}, Manifest: manifest.NewWriter(&strings.Builder{}), Root: root,
		Stager: stage.Stager{Dir: stageDir, Physical: stageDir, Root: root}, MaxSize: 1 << 20,
		Inventory: true, Records: aggregator,
	}
	home := filepath.Join(root, "home/alice")
	if err := collector.CollectCatalog(context.Background(), "alice", home, tables.Catalog, ""); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := aggregator.WriteAgentLines(&out, hostField(t, want[0], "host"), "alice", 0); err != nil {
		t.Fatal(err)
	}
	var hostLine strings.Builder
	host := Host{Host: hostField(t, want[0], "host"), Collector: hostField(t, want[0], "collector"), Mode: "image", At: "t", UsersScanned: 1}
	if err := WriteHostLine(&hostLine, host); err != nil {
		t.Fatal(err)
	}
	if got := atField.ReplaceAllString(strings.TrimSuffix(hostLine.String(), "\n"), ""); got != want[0] {
		t.Errorf("host line = %s, sh = %s", got, want[0])
	}
	if got := strings.Join(want[1:], "\n") + "\n"; out.String() != got {
		t.Errorf("agent lines differ\ngo:\n%s\nsh:\n%s", out.String(), got)
	}
}

// hostField reads a string field of the sh collector's host line.
func hostField(t *testing.T, line, key string) string {
	t.Helper()
	match := regexp.MustCompile(`"` + key + `":"([^"]*)"`).FindStringSubmatch(line)
	if match == nil {
		t.Fatalf("no %s in %q", key, line)
	}
	return match[1]
}
