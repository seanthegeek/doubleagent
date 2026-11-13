//go:build unix

package walk

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// edgeTree builds an image whose one home holds the walk's edge cases: a
// nested claim, excluded directories, an unreadable directory, a FIFO, a link
// to a directory, names holding a quote, a newline or a final `|`, an
// oversized file, a case variant, and catalog paths through symlinked
// directories that stay inside the home and that lead out of it.
func edgeTree(t *testing.T) string {
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
	if err := os.Chmod(home+"/.claude/locked", 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home+"/.claude/locked", 0o755) })
	return root
}

func TestEdgeTreeRowsMatchShCollector(t *testing.T) {
	if _, err := exec.LookPath("dash"); err != nil {
		t.Skip("the oracle needs dash")
	}
	root := edgeTree(t)
	texts := []string{"not walked", "archived under"}
	if caseInsensitive(t) {
		texts = append(texts, "same file as")
	}
	for _, test := range oracleRuns {
		t.Run(test.name, func(t *testing.T) {
			want := runShCollector(t, root, test.options)
			got := runGoCollector(t, root, test.settings)
			compareRows(t, want.rows, got.rows)
			compareLog(t, want.log, got.log)
			compareStage(t, want.stage, got.stage)
			rows, log := strings.Join(got.rows, "\n"), strings.Join(got.log, "\n")
			for _, status := range edgeStatuses(test.settings) {
				if !strings.Contains(rows, `"status":"`+status+`"`) {
					t.Errorf("no %s row", status)
				}
			}
			for _, text := range texts {
				if !strings.Contains(log, text) {
					t.Errorf("no log line holding %q", text)
				}
			}
		})
	}
}

// edgeStatuses returns the statuses the edge tree must give under settings.
func edgeStatuses(settings runSettings) []string {
	statuses := []string{"collected", "symlink", "skipped_special", "error_copy"}
	if !settings.full {
		statuses = append(statuses, "skipped_excluded")
	}
	if settings.noSecrets {
		statuses = append(statuses, "skipped_secret", "skipped_size")
	}
	if os.Geteuid() != 0 {
		statuses = append(statuses, "error_read")
	}
	return statuses
}

// caseInsensitive reports whether the temporary directory's file system
// folds case, so two spellings of a name are one file.
func caseInsensitive(t *testing.T) bool {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/probe", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(dir + "/PROBE")
	return err == nil
}
