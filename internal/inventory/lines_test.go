//go:build unix

package inventory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// silentLog drops log lines.
type silentLog struct{}

// Line drops the log line.
func (silentLog) Line(string) error { return nil }

// writeAgentImage writes one user's agent files, each with a fixed
// modification time derived from its size, and returns the image root.
func writeAgentImage(t *testing.T) string {
	t.Helper()
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

// wantInventoryLines are the host line and the agent lines of the image, in
// the order the catalog lists the agents.
const wantInventoryLines = `{"type":"host","host":"testhost","collector":"test-collector","mode":"image","at":"t","users_scanned":1,"users_unreadable":0,"docker_volumes":0}
{"type":"agent","host":"testhost","user":"alice","agent":"antigravity","files":1,"bytes":4,"first":"2023-11-14T22:13:24Z","last":"2023-11-14T22:13:24Z","projects":0,"evidence":".gemini/antigravity-cli"}
{"type":"agent","host":"testhost","user":"alice","agent":"claude-code","files":3,"bytes":15,"first":"2023-11-14T22:13:22Z","last":"2023-11-14T22:13:27Z","projects":0,"evidence":".claude,.claude.json*"}
{"type":"agent","host":"testhost","user":"alice","agent":"codex-cli","files":1,"bytes":1,"first":"2023-11-14T22:13:21Z","last":"2023-11-14T22:13:21Z","projects":0,"evidence":".codex"}
{"type":"agent","host":"testhost","user":"alice","agent":"gemini-cli","files":1,"bytes":3,"first":"2023-11-14T22:13:23Z","last":"2023-11-14T22:13:23Z","projects":0,"evidence":".gemini"}
`

func TestInventoryLinesOfAgentImage(t *testing.T) {
	root := writeAgentImage(t)
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	aggregator := NewAggregator()
	stageDir := t.TempDir()
	collector := &walk.Collector{
		Tables: tables, Excludes: tables.Excludes, SecretGlobs: tables.SecretGlobs,
		Log: silentLog{}, Manifest: manifest.NewWriter(&strings.Builder{}), Root: root,
		Stager:  stage.Stager{Dir: stageDir, Tree: stagingTree(t, stageDir), Root: root, Places: stage.NewPlaces()},
		MaxSize: 1 << 20, Inventory: true, Records: aggregator,
	}
	home := filepath.Join(root, "home/alice")
	if err := collector.CollectCatalog(context.Background(), "alice", home, tables.Catalog, ""); err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	host := Host{Host: "testhost", Collector: "test-collector", Mode: "image", Timestamp: "t", UsersScanned: 1}
	if err := WriteHostLine(&got, host); err != nil {
		t.Fatal(err)
	}
	if err := aggregator.WriteAgentLines(&got, "testhost", "alice", 0); err != nil {
		t.Fatal(err)
	}
	if got.String() != wantInventoryLines {
		t.Errorf("inventory lines:\n%s\nwant:\n%s", got.String(), wantInventoryLines)
	}
}

// stagingTree opens dir as the staging tree of a test, closed when it ends.
func stagingTree(t *testing.T, dir string) *stage.Tree {
	t.Helper()
	tree, err := stage.OpenTree(syspath.FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}
