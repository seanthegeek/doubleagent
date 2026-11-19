//go:build unix

package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnUnreadableDirectoryOfARelativeInputIsNamedAsGiven(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "lt", "alice", ".claude", "history.jsonl"), historyLine)
	locked := filepath.Join(work, "lt", "alice", ".claude", "projects")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("this account reads a directory without permission")
	}
	t.Chdir(work)
	want := filepath.Join("lt", "alice", ".claude", "projects") + ": "
	for name, args := range map[string][]string{
		"detect":        {"detect", "lt"},
		"detect --json": {"detect", "--json", "lt"},
		"timeline":      {"timeline", "lt", "out"},
	} {
		t.Run(name, func(t *testing.T) {
			_ = os.RemoveAll(filepath.Join(work, "out"))
			_, stdout, stderr := runCommand(System(), args...)
			printed := stdout + stderr + writtenFiles(t, work)
			if !strings.Contains(printed, want) {
				t.Errorf("the output does not hold %q:\n%s", want, printed)
			}
			wantNoWorkingDirectory(t, printed)
		})
	}
}

func TestAnUnreadableSha256FileOfARelativeArchiveIsNamedAsGiven(t *testing.T) {
	work := t.TempDir()
	writeTar(t, filepath.Join(work, "case", "case.tar"),
		[][2]string{{"alice/.claude/history.jsonl", historyLine}})
	sidecar := filepath.Join(work, "case", "case.tar.sha256")
	writeFile(t, sidecar, "0\n")
	if err := os.Chmod(sidecar, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(sidecar); err == nil {
		t.Skip("this account reads a file without permission")
	}
	t.Chdir(work)
	_, stdout, stderr := runCommand(System(), "detect", filepath.Join("case", "case.tar"))
	want := "note:    WARNING archive sha256 not checked: " + filepath.Join("case", "case.tar.sha256") + ": "
	if !strings.Contains(stdout, want) {
		t.Errorf("the output does not hold %q:\n%s", want, stdout+stderr)
	}
	wantNoWorkingDirectory(t, stdout+stderr)
}
