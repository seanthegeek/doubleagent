//go:build !windows

package input

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// symlink makes link point at target.
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// fifo makes a named pipe at path.
func fifo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatal(err)
	}
}

// linkAndFifoCases need symbolic links and FIFOs.
func linkAndFifoCases() []inputCase {
	return []inputCase{
		{name: "links inside a loose tree are never followed",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"home/alice/.claude/history.jsonl": "{}\\n",
					"outside/secret.jsonl": "{}", "outside/.claude/x": "x"})
				symlink(t, filepath.Join(dir, "outside"), filepath.Join(dir, "home/alice/.claude/linked"))
				symlink(t, filepath.Join(dir, "outside"), filepath.Join(dir, "home/bob"))
				symlink(t, filepath.Join(dir, "outside", ".claude"), filepath.Join(dir, "home/carol/.claude"))
				return filepath.Join(dir, "home")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				for _, artifact := range collection.Artifacts {
					if strings.Contains(artifact.Rel, "linked") || artifact.Home.User != "alice" {
						t.Fatalf("artifact %s of %s through a link", artifact.Rel, artifact.Home.User)
					}
				}
				wantEqual(t, "homes, carol's with only a link", len(collection.Homes), 2)
			}},
		{name: "a FIFO matched by a glob is a problem and one inside a directory an artifact",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"home/carol/.claude/history.jsonl": "{}\\n"})
				fifo(t, filepath.Join(dir, "home/carol/.claude.json"))
				fifo(t, filepath.Join(dir, "home/carol/.claude/pipe"))
				return filepath.Join(dir, "home")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				if len(collection.Problems) != 1 || !strings.HasSuffix(collection.Problems[0], "/home/carol/.claude.json: not a directory") {
					t.Fatalf("problems %q", collection.Problems)
				}
				wantEqual(t, "pipe agent", agentsByRel(collection)[".claude/pipe"], "claude-code")
			}},
	}
}

// chmod sets path's mode, restoring a mode the test directory's removal
// can pass through at the end.
func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
}

// skipAsRoot skips a test that relies on a permission root ignores.
func skipAsRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits")
	}
}

// permissionCases need permission bits or a FIFO as the input.
func permissionCases() []inputCase {
	return []inputCase{
		{name: "an unlistable directory is a problem once",
			build: func(t *testing.T, dir string) string {
				skipAsRoot(t)
				writeTree(t, dir, map[string]string{"home/dave/.claude/locked/x": "x", "home/dave/.claude/y": "y"})
				locked := filepath.Join(dir, "home/dave/.claude/locked")
				chmod(t, locked, 0)
				return filepath.Join(dir, "home")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				if len(collection.Problems) != 1 || !strings.HasSuffix(collection.Problems[0], "/home/dave/.claude/locked: permission denied") {
					t.Fatalf("problems %q", collection.Problems)
				}
				wantEqual(t, "artifacts", len(collection.Artifacts), 1)
			}},
		{name: "an unreadable sidecar is a note",
			build: func(t *testing.T, dir string) string {
				skipAsRoot(t)
				archive := filepath.Join(dir, "c.tar")
				writeFile(t, archive, string(tarBytes(t, false, tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"})))
				writeFile(t, archive+".sha256", "00")
				chmod(t, archive+".sha256", 0)
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				if len(collection.Notes) != 1 || !strings.HasPrefix(collection.Notes[0], "WARNING archive sha256 not checked: ") ||
					!strings.HasSuffix(collection.Notes[0], "c.tar.sha256: permission denied") {
					t.Fatalf("notes %q", collection.Notes)
				}
			}},
		{name: "an xz-compressed archive is refused",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "c.tar.xz")
				writeFile(t, archive, "\\xfd7zXZ\\x00 and more")
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				var inputError *Error
				if !errors.As(err, &inputError) {
					t.Fatalf("error %v", err)
				}
			}},
		{name: "an input that is neither a file nor a directory is refused",
			build: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "pipe")
				fifo(t, path)
				return path
			},
			check: func(t *testing.T, collection *Collection, err error) {
				var inputError *Error
				if !errors.As(err, &inputError) || !strings.HasSuffix(err.Error(), "pipe is neither a regular file nor a directory") {
					t.Fatalf("error %v", err)
				}
			}},
	}
}

// platformCases are the cases that need symbolic links, FIFOs or
// permission bits.
func platformCases() []inputCase {
	return slices.Concat(linkAndFifoCases(), permissionCases())
}
