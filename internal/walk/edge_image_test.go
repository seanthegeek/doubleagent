//go:build unix

package walk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// edgeRun is what one collection left: its manifest lines, its log lines and
// its staging directory.
type edgeRun struct {
	rows  []string
	log   []string
	stage string
}

// edgeLog keeps log lines in memory.
type edgeLog struct{ lines []string }

// Line keeps text.
func (log *edgeLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// edgeSettings are the collector settings of one run over the edge image.
type edgeSettings struct {
	name      string
	full      bool
	noSecrets bool
	maxSize   int64
}

// edgeRuns are the three runs the edge image is collected with: the default,
// one without secrets and with a 1 MB size cap, and a full one.
var edgeRuns = []edgeSettings{
	{"default", false, false, 256 << 20},
	{"without secrets and a 1 MB cap", false, true, 1 << 20},
	{"full", true, false, 256 << 20},
}

// edgeImage builds an image whose one home holds the walk's edge cases: a
// nested claim, excluded directories, a directory that cannot be listed,
// two empty ones that can be listed but not entered, a FIFO, a link
// to a directory, names holding a quote, a newline or a final `|`, an
// oversized file, a case variant, and catalog paths through symlinked
// directories that stay inside the home and that lead out of it.
func edgeImage(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	root, home := parent+"/root", parent+"/root/home/alice"
	files := map[string]string{
		".claude/projects/p/s1.jsonl": "{}\n", ".claude/a\"b": "quote",
		".claude/c\nd": "newline", ".claude/.credentials.json": "token",
		".claude/cache/c.bin": "cached", ".claude/worktrees/w/f": "tree",
		".claude/locked/hidden": "x", ".claude.json": "{}", ".claude.json.bak\nzzz": "split",
		".claude.json|": "pipe", ".gemini/settings.json": "{}",
		".gemini/antigravity-cli/conversations/a.db": "db", ".config/goose/config.yaml": "a: b",
		"dotlocal/share/crush/crush.json": "{}", "../../outside/crush/x": "out",
	}
	// The home exists before any file, since a path written through `..`
	// resolves it on disk, while filepath.Dir resolves it on paper.
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(home+"/"+name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(home+"/"+name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(home+"/.claude/big.jsonl", make([]byte, 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	links := [][2]string{{"projects", ".claude/linked"}, {"dotlocal", ".local"},
		{"../../outside", ".cache"}}
	for _, link := range links {
		if err := os.Symlink(link[0], home+"/"+link[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(home+"/.claude/pipe", 0o600); err != nil {
		t.Fatal(err)
	}
	// noenter stays empty: for entries of a directory that can be listed but
	// not entered, the sh collector's rows are an artefact of the platform's
	// find (GNU find stats some and hands them on, BSD find none), which the
	// walk does not reproduce, so the fixture holds no entry to compare.
	for _, name := range []string{".claude/empty", ".claude/noenter"} {
		if err := os.Mkdir(home+"/"+name, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	modes := map[string]os.FileMode{".claude/locked": 0, ".claude/noenter": 0o600,
		".claude/empty": 0o600}
	for name, mode := range modes {
		if err := os.Chmod(home+"/"+name, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(home+"/"+name, 0o755) })
	}
	return root
}

// collectEdgeImage collects the homes of root and returns the rows, the log
// lines and the staging directory.
func collectEdgeImage(t *testing.T, root string, settings edgeSettings) edgeRun {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	stageDir := t.TempDir() + "/stage"
	if err := os.MkdirAll(stageDir+"/fs", 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := stage.PhysicalDir(stageDir)
	if err != nil {
		t.Fatal(err)
	}
	var rows bytes.Buffer
	log := &edgeLog{}
	collector := &Collector{
		Tables: tables, Excludes: tables.Excludes, Secret: MatchAny(tables.SecretGlobs),
		Log: log, Manifest: manifest.NewWriter(&rows), Root: root, MaxSize: settings.maxSize,
		NoSecrets: settings.noSecrets,
		Stager:    stage.Stager{Dir: stageDir, Physical: physical, Root: root, Places: stage.NewPlaces()},
	}
	if settings.full {
		collector.Excludes = nil
	}
	homes := users.Enumerate(context.Background(), false, root, "", nil, nil, nil,
		NewRootCheck(root).Reached)
	if err := collector.CollectUserHomes(context.Background(), homes); err != nil {
		t.Fatal(err)
	}
	return edgeRun{rows: strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n"),
		log: log.lines, stage: stageDir}
}

// foldsCase reports whether the temporary directory's file system
// folds case, so two spellings of a name are one file.
func foldsCase(t *testing.T) bool {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/probe", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(dir + "/PROBE")
	return err == nil
}
