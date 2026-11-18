//go:build !windows

package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestALinkedDirectoryInsideADirectoryCollectionIsNeverPassedThrough(t *testing.T) {
	cases := []struct {
		name, link, problem string
		rows                func(root string) []string
	}{
		{"in the middle of a collected file's archive path", "fs/home/a/.gemini",
			"/home/a/.gemini/tmp/slug/shell_history: through a symbolic link, not followed",
			func(string) []string {
				return []string{collectedRow("a", "/home/a", "gemini-cli",
					"/home/a/.gemini/tmp/slug/shell_history", "fs/home/a/.gemini/tmp/slug/shell_history")}
			}},
		{"in the middle of a staged path", "fs/home/a/dotfiles",
			"/home/a/.gemini/tmp/slug/x: through a symbolic link, not followed",
			func(root string) []string {
				writeFile(t, filepath.Join(root, "fs/home/a/.claude/x"), "x")
				return []string{collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x"),
					jsonRow("user", "a", "home", "/home/a", "agent", "gemini-cli", "path", "/home/a/.gemini",
						"archive_path", "fs/home/a/.gemini", "type", "symlink", "status", "collected", "size", 0),
					collectedRow("a", "/home/a", "gemini-cli", "/home/a/.gemini/tmp/slug/x",
						"fs/home/a/dotfiles/gemini/tmp/slug/x")}
			}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			outside := plantOutside(t, dir, "tmp/slug/shell_history")
			writeFile(t, filepath.Join(outside, "gemini/tmp/slug/x"), plantedText)
			root := filepath.Join(dir, "collection")
			writeManifest(t, root, test.rows(root)...)
			symlink(t, outside, filepath.Join(root, filepath.FromSlash(test.link)))
			collection, err := Open(root, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			wantConfined(t, collection)
			wantProblem(t, collection, test.problem)
			for _, artifact := range collection.Artifacts {
				if artifact.Agent == "gemini-cli" {
					t.Errorf("artifact %s read through the link", artifact.DiskPath)
				}
			}
			for _, rel := range []string{".gemini/tmp/slug/shell_history", ".gemini/tmp/slug/x"} {
				if path, ok := collection.Homes[0].PathOf(rel); ok {
					if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), plantedText) {
						t.Errorf("PathOf(%s) passed through the link to %s", rel, path)
					}
				}
			}
		})
	}
}
