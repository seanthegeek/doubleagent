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
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
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
func (plist fakePlist) Lint(context.Context, string) bool { return plist.lints }

// ExtractString returns the table's string for keyPath.
func (plist fakePlist) ExtractString(_ context.Context, _, keyPath string) (value, reason string,
	found bool,
) {
	value, found = plist.strings[keyPath]
	return value, "", found
}

// writeTree writes each relative path of files under root with its text.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, text := range files {
		path := filepath.Join(root, relative)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err == nil {
			err = os.WriteFile(path, []byte(text), 0o644)
		}
		if invalidFileName(err) {
			t.Skipf("this platform does not accept the file name %q: %v", relative, err)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

// linkTree creates each link of links (relative path to target) under root,
// as makeLink makes one on this platform.
func linkTree(t *testing.T, root string, links map[string]string) {
	t.Helper()
	for relative, target := range links {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		makeLink(t, target, path)
	}
}

// nativeLines returns lines as the log writes them: each path in native form.
// The expected lines hold no slash outside their paths, so the whole line is
// converted.
func nativeLines(lines []string) []string {
	native := make([]string, len(lines))
	for i, line := range lines {
		native[i] = syspath.ToNative(line)
	}
	return native
}

// expandClaims lists the matches of a catalog table under base, as the walk
// does for a table with no case variants.
func expandClaims(base string, table []catalog.Entry) []string {
	var claims []string
	for _, entry := range table {
		claims = append(claims, expandGlob(base, entry.Glob)...)
	}
	return claims
}

// searchFixture builds the tree the Search tests run over and returns its
// root in canonical form. The files whose names hold a newline are left out
// where the platform refuses such a name.
func searchFixture(t *testing.T) string {
	root := syspath.FromNative(t.TempDir())
	export := "export CODEX_HOME=/opt/x\n"
	writeTree(t, root, newlineFixture(export))
	writeTree(t, root, map[string]string{
		"home/alice/.bashrc":                          export,
		"home/alice/Documents/PowerShell/profile.ps1": "$env:codex_home = 1\n",
		"home/alice/.config/environment.d/home.conf":  "CODEX_HOME=$HOME/x\n",
		"home/alice/dotfiles/direnv/direnvrc":         export,
		"home/alice/scripts/deep.sh":                  export,
		"home/alice/scripts/big.txt":                  "# padding past the limit\n" + export,
		"home/alice/.codex/config.toml":               "CODEX_HOME = 1\n",
		"home/alice/AppData/Local/agy/agent-env.sh":   export,
		"home/alice/out/host_run.log":                 export,
		"home/alice/out/.stage-host_run/fs/x":         export,
		"home/alice/out/notes":                        export,
		"home/alice/copied/host_run.manifest.jsonl":   export,
		"home/alice/copied/.stage-host_run/fs/y":      export,
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
		Tables: tables, Root: root, SystemBase: root, Log: log, EnvScanMaxBytes: 32,
		Excluded: (&walk.Collector{Excludes: tables.Excludes}).Excluded,
		Homes: []users.Home{{User: "alice", Path: root + "/home/alice"},
			{User: "svc2", Path: root + "/var/svc"}},
		Projects: []users.Home{{User: "alice", Path: root + "/srv/proj"},
			{User: "", Path: root + "/home"}},
		RunName: "host_run",
		Plutil: fakePlist{available: true, lints: true, strings: map[string]string{
			"EnvironmentVariables.CODEX_HOME": "/opt/d/", "UserName": "svc"}},
		Claims: expandClaims,
		Collect: func(user, home, path string, _ fsmeta.Meta) error {
			*collected = append(*collected, user+"|"+home+"|"+path)
			return nil
		},
	}
}

// startupLog returns the log the startup pass writes over the fixture.
func startupLog(root string) []string {
	alice, system := root+"/home/alice", "relocation hint: "+root
	codex := " names CODEX_HOME (codex-cli)"
	return nativeLines([]string{
		"Relocation hints for alice (" + alice + ")",
		"relocation hint: " + alice + "/.bashrc:1" + codex,
		"relocation hint: " + alice + "/.config/direnv/direnvrc:1" + codex,
		"relocation hint: " + alice + "/.config/environment.d/home.conf:1" + codex + " = " +
			alice + "/x",
		"relocation hint: " + alice + "/Documents/PowerShell/profile.ps1:1" + codex,
		"Relocation hints for svc2 (" + root + "/var/svc)",
		system + "/var/svc/.bashrc:1" + codex,
		"Relocation hints in system files",
		system + "/Library/LaunchDaemons/d.plist:3" + codex,
		system + "/Library/LaunchDaemons/d.plist" + codex + " = " + root + "/opt/d",
		system + "/etc/environment:1" + codex + " = " + root + "/opt/q",
		system + "/etc/environment:2" + codex + "; value not resolved: holds a #",
		system + "/etc/profile:1 names COPILOT_HOME (copilot-cli)",
		system + "/etc/profile:1 names COPILOT_HOME (copilot-desktop)",
		"Relocation hints in project " + root + "/srv/proj",
		system + "/srv/proj/.envrc:1 names CLAUDE_CONFIG_DIR (claude-code)",
	})
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
	daemon := result.Hints[slices.IndexFunc(result.Hints, func(hint Hint) bool {
		return hint.Source == root+"/Library/LaunchDaemons/d.plist" && hint.Line == 0
	})]
	if daemon.User != "svc" || daemon.Value != root+"/opt/d" {
		t.Errorf("daemon plist hint = %+v", daemon)
	}
	wantCollected := "||" + root + "/Library/LaunchDaemons/d.plist"
	if len(collected) != 9 || collected[5] != wantCollected {
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
		swept = append(swept, strings.TrimPrefix(line, "relocation hint: "+syspath.ToNative(alice)))
	}
	hints := []string{"/dotfiles/direnv/direnvrc:1 names CODEX_HOME (codex-cli)",
		"/out/notes:1 names CODEX_HOME (codex-cli)", "/scripts/deep.sh:1 names CODEX_HOME (codex-cli)"}
	// A file whose name, or a directory's, holds a newline is searched like
	// any other.
	for name := range newlineFixture("") {
		hints = append(hints, strings.TrimPrefix(name, "home/alice")+":1 names CODEX_HOME (codex-cli)")
	}
	slices.Sort(hints)
	want := nativeLines(append(append([]string{"Relocation hints for alice (" + alice + ")"},
		hints...), "sweep skipped: home "+root+"/var/svc lies outside the user home roots",
		"sweep skipped: project "+root+"/home holds a home directory"))
	if len(swept) > len(hints) {
		slices.Sort(swept[1 : 1+len(hints)])
	}
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
	// The sweep adds alice's deep.sh, notes, the files whose names or whose
	// directory's name hold a newline where the platform allows one, and the
	// direnvrc the startup pass read through a link, now under its own path,
	// and nothing of the project beyond its .envrc, which the startup pass
	// already searched.
	added := 3 + len(newlineFixture(""))
	if before, after := count(false), count(true); after != before+added {
		t.Errorf("files searched: %d without the sweep, %d with it; want %d", before, after,
			before+added)
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
	want := syspath.ToNative("relocation hint: " + root + "/Library/LaunchDaemons/d.plist:3 " +
		"names CODEX_HOME (codex-cli); value not resolved: no plutil")
	if !slices.Contains(log.lines, want) {
		t.Errorf("log lacks %q:\n%s", want, strings.Join(log.lines, "\n"))
	}
}

func TestSearchTakesTheSystemFilesUnderTheSystemBase(t *testing.T) {
	root := searchFixture(t)
	system := syspath.FromNative(t.TempDir())
	writeTree(t, system, map[string]string{"etc/profile": "export CODEX_HOME=/opt/s\n"})
	cases := []struct {
		name, systemBase, want string
	}{
		{"the image root is the system base in image mode", root,
			"relocation hint: " + root + "/etc/profile:1 names COPILOT_HOME (copilot-cli)"},
		{"a system base set apart is searched in place of the root", system,
			"relocation hint: " + system + "/etc/profile:1 names CODEX_HOME (codex-cli)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			log := &lineLog{}
			deps := searchDeps(t, root, log, &[]string{})
			deps.SystemBase = test.systemBase
			if _, err := Search(context.Background(), deps); err != nil {
				t.Fatal(err)
			}
			if want := syspath.ToNative(test.want); !slices.Contains(log.lines, want) {
				t.Errorf("log lacks %q:\n%s", want, strings.Join(log.lines, "\n"))
			}
		})
	}
}

// expandGlob is glob.Expand without a check, as a live run calls it.
func expandGlob(base, pattern string) []string {
	matches, _ := glob.Expand(base, pattern, nil, nil)
	return matches
}
