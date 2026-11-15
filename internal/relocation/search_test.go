package relocation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// lineLog is a Logger that keeps the lines it is given.
type lineLog struct{ lines []string }

// Line keeps text.
func (log *lineLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// fakePlist is a PlistReader answering from a table keyed by key path.
type fakePlist struct {
	available, lints bool
	strings          map[string]string
}

// Available reports the table's setting.
func (plist fakePlist) Available() bool { return plist.available }

// Lint reports the table's setting.
func (plist fakePlist) Lint(string) bool { return plist.lints }

// ExtractString returns the table's string for keyPath.
func (plist fakePlist) ExtractString(_, keyPath string) (value, reason string, found bool) {
	value, found = plist.strings[keyPath]
	return value, "", found
}

// writeTree writes each relative path of files under root with its text.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, text := range files {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// linkTree creates each symlink of links (relative path to target) under root.
func linkTree(t *testing.T, root string, links map[string]string) {
	t.Helper()
	for relative, target := range links {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
}

// expandClaims lists the matches of a catalog table under base, as the walk
// does for a table with no case variants.
func expandClaims(base string, table []catalog.Entry) []string {
	var claims []string
	for _, entry := range table {
		claims = append(claims, glob.Expand(base, entry.Glob)...)
	}
	return claims
}

// searchFixture builds the tree the Search tests run over and returns its
// root.
func searchFixture(t *testing.T) string {
	root := t.TempDir()
	export := "export CODEX_HOME=/opt/x\n"
	writeTree(t, root, map[string]string{
		"home/alice/.bashrc":                          export,
		"home/alice/Documents/PowerShell/profile.ps1": "$env:codex_home = 1\n",
		"home/alice/.config/environment.d/home.conf":  "CODEX_HOME=$HOME/x\n",
		"home/alice/dotfiles/direnv/direnvrc":         export,
		"home/alice/scripts/deep.sh":                  export,
		"home/alice/scripts/big.txt":                  "# padding past the limit\n" + export,
		"home/alice/scripts/d\nx":                     export,
		"home/alice/scripts/nl\ndir/f.sh":             export,
		"home/alice/.codex/config.toml":               "CODEX_HOME = 1\n",
		"home/alice/AppData/Local/agy/agent-env.sh":   export,
		"home/alice/out/host_run.log":                 export,
		"home/alice/out/.stage-host_run/fs/x":         export,
		"home/alice/out/notes":                        export,
		"opt/z":                                       export,
		"opt/omz/custom/a.zsh":                        export,
		"etc/profile":                                 "export COPILOT_HOME=/opt/x\n",
		"etc/environment":                             "CODEX_HOME=\"/opt/q\"\nCODEX_HOME=/opt/a#b\n",
		"Library/LaunchDaemons/d.plist":               "<plist>\n<dict>\n<key>CODEX_HOME</key>\n",
		"srv/proj/.envrc":                             "export CLAUDE_CONFIG_DIR=/opt/x\n",
		"var/svc/.bashrc":                             export,
	})
	linkTree(t, root, map[string]string{
		"home/alice/.zshrc":         filepath.Join(root, "opt/z"),
		"home/alice/.oh-my-zsh":     filepath.Join(root, "opt/omz"),
		"home/alice/.config/direnv": filepath.Join(root, "home/alice/dotfiles/direnv"),
	})
	return root
}

// searchDeps returns the Deps of a search over the fixture under root.
func searchDeps(t *testing.T, root string, log *lineLog, collected *[]string) Deps {
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return Deps{
		Tables: tables, Root: root, Log: log, EnvScanMaxBytes: 32, Excludes: tables.Excludes,
		Homes: []users.Home{{User: "alice", Path: root + "/home/alice"},
			{User: "svc2", Path: root + "/var/svc"}},
		Projects: []users.Home{{User: "alice", Path: root + "/srv/proj"},
			{User: "", Path: root + "/home"}},
		OutDir: root + "/home/alice/out", RunName: "host_run",
		Plutil: fakePlist{available: true, lints: true, strings: map[string]string{
			"EnvironmentVariables.CODEX_HOME": "/opt/d/", "UserName": "svc"}},
		Claims: expandClaims,
		Collect: func(user, home, path string) error {
			*collected = append(*collected, user+"|"+home+"|"+path)
			return nil
		},
	}
}

// startupLog returns the log the startup pass writes over the fixture.
func startupLog(root string) []string {
	alice, system := root+"/home/alice", "relocation hint: "+root
	codex := " names CODEX_HOME (codex-cli)"
	return []string{
		"Relocation hints for alice (" + alice + ")",
		"relocation hint: " + alice + "/.bashrc:1" + codex,
		"relocation hint: " + alice + "/.config/environment.d/home.conf:1" + codex + " = " +
			alice + "/x",
		"relocation hint: " + alice + "/.config/direnv/direnvrc:1" + codex,
		"relocation hint: " + alice + "/Documents/PowerShell/profile.ps1:1" + codex,
		"Relocation hints for svc2 (" + root + "/var/svc)",
		system + "/var/svc/.bashrc:1" + codex,
		"Relocation hints in system files",
		system + "/etc/profile:1 names COPILOT_HOME (copilot-cli)",
		system + "/etc/profile:1 names COPILOT_HOME (copilot-desktop)",
		system + "/etc/environment:1" + codex + " = " + root + "/opt/q",
		system + "/etc/environment:2" + codex + "; value not resolved: holds a #",
		system + "/Library/LaunchDaemons/d.plist:3" + codex,
		system + "/Library/LaunchDaemons/d.plist" + codex + " = " + root + "/opt/d",
		"Relocation hints in project " + root + "/srv/proj",
		system + "/srv/proj/.envrc:1 names CLAUDE_CONFIG_DIR (claude-code)",
	}
}

func TestSearchStartupFiles(t *testing.T) {
	root := searchFixture(t)
	log, collected := &lineLog{}, []string{}
	result, err := Search(context.Background(), searchDeps(t, root, log, &collected))
	if err != nil {
		t.Fatal(err)
	}
	wantLog := startupLog(root)
	if !slices.Equal(log.lines, wantLog) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(wantLog, "\n"))
	}
	if result.FilesSearched != 9 || result.Resolved != 3 || result.FilesSkippedSize != 0 {
		t.Errorf("counts = %d searched, %d resolved, %d skipped", result.FilesSearched,
			result.Resolved, result.FilesSkippedSize)
	}
	daemon := result.Hints[len(result.Hints)-2]
	if daemon.User != "svc" || daemon.Line != 0 || daemon.Value != root+"/opt/d" {
		t.Errorf("daemon plist hint = %+v", daemon)
	}
	wantCollected := "||" + root + "/Library/LaunchDaemons/d.plist"
	if len(collected) != 9 || collected[7] != wantCollected {
		t.Errorf("collected = %q", collected)
	}
}

func TestSearchSweep(t *testing.T) {
	root := searchFixture(t)
	log, collected := &lineLog{}, []string{}
	deps := searchDeps(t, root, log, &collected)
	deps.EnvScanHome = true
	result, err := Search(context.Background(), deps)
	if err != nil {
		t.Fatal(err)
	}
	alice := root + "/home/alice"
	var swept []string
	for _, line := range log.lines[16:] {
		swept = append(swept, strings.TrimPrefix(line, "relocation hint: "+alice))
	}
	want := []string{
		"Relocation hints for alice (" + alice + ")",
		"/dotfiles/direnv/direnvrc:1 names CODEX_HOME (codex-cli)",
		"/out/notes:1 names CODEX_HOME (codex-cli)",
		"/scripts/deep.sh:1 names CODEX_HOME (codex-cli)",
		"sweep skipped: home " + root + "/var/svc lies outside the user home roots",
		"sweep skipped: project " + root + "/home holds a home directory",
	}
	slices.Sort(swept[1:4])
	if !slices.Equal(swept, want) {
		t.Errorf("sweep log:\n%s\nwant:\n%s", strings.Join(swept, "\n"), strings.Join(want, "\n"))
	}
	if result.FilesSkippedSize != 1 {
		t.Errorf("files skipped for size = %d, want 1", result.FilesSkippedSize)
	}
}

func TestSearchSweepSearchesEachFileOnce(t *testing.T) {
	root := searchFixture(t)
	count := func(sweep bool) int {
		deps := searchDeps(t, root, &lineLog{}, &[]string{})
		deps.EnvScanHome = sweep
		result, err := Search(context.Background(), deps)
		if err != nil {
			t.Fatal(err)
		}
		return result.FilesSearched
	}
	// The sweep adds alice's deep.sh, notes, the file under a directory whose
	// name holds a newline (counted, never recorded) and the direnvrc the
	// startup pass read through a link, now under its own path, and nothing of
	// the project beyond its .envrc, which the startup pass already searched.
	if before, after := count(false), count(true); after != before+4 {
		t.Errorf("files searched: %d without the sweep, %d with it; want %d", before, after,
			before+4)
	}
}

func TestSearchStopsWhenCancelled(t *testing.T) {
	root := searchFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Search(ctx, searchDeps(t, root, &lineLog{}, &[]string{}))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestSearchWithoutPlutil(t *testing.T) {
	root := searchFixture(t)
	log := &lineLog{}
	deps := searchDeps(t, root, log, &[]string{})
	deps.Plutil = fakePlist{}
	if _, err := Search(context.Background(), deps); err != nil {
		t.Fatal(err)
	}
	want := "relocation hint: " + root + "/Library/LaunchDaemons/d.plist:3 names CODEX_HOME " +
		"(codex-cli); value not resolved: no plutil"
	if !slices.Contains(log.lines, want) {
		t.Errorf("log lacks %q:\n%s", want, strings.Join(log.lines, "\n"))
	}
}
