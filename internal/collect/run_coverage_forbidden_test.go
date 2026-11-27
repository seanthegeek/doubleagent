package collect_test

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// forbiddenPath is a path below the image root, with "/" between names,
// that the default run over the main image must not collect: no manifest
// row has it as its path (kind "path"), no row's path holds it ("within",
// compared without regard to case when anyCase is set), or no archive
// member's name holds it ("member"). why says what collecting it would get
// wrong.
type forbiddenPath struct {
	kind, part string
	anyCase    bool
	why        string
}

// forbiddenPaths are the paths the main image holds that a default run must
// leave out, a table built for the Unix image; on Windows a path its image
// does not hold is skipped.
var forbiddenPaths = []forbiddenPath{
	{"within", "cache/junk.bin", false, "a cache directory's contents are excluded"},
	{"within", "nonexistent", false, "a passwd home that does not exist is skipped"},
	{"within", ".bash_history", false, "shell histories are the EDR's, not the catalog's"},
	{"path", "home/alice/.env", false, "a home's own .env is in no catalog entry"},
	{"within", "pgdata/_data/x", false, "an unmatched volume gets one row, not its contents"},
	{"within", "volumes/metadata.db", false, "Docker's volume database is no volume"},
	{"within", "/.local/bin/othertool", false, "muse's .local/bin glob leaves other tools alone"},
	{"within", "/.claude-code-router/", false, "claude-code-router is no catalog agent"},
	{"within", "blobs/sha256-abc", false, "Ollama model blobs are excluded"},
	{"within", "/.local/share/myproj/unrelated.txt", false, "crewai's mid-path wildcard names its own files"},
	{"within", "/.camel/4242-status.json", false, "camel-ai's lines leave Apache Camel JBang files alone"},
	{"within", "/9b32b47ec4b3/", false, "Claude Desktop's downloaded CLI is excluded"},
	{"within", "gdocs/bin/gdocs-1", false, "an Antigravity plugin binary is excluded"},
	{"within", "antigravity/bin/webm_encoder", false, "the Antigravity app's binaries are excluded"},
	{"within", "Antigravity/Cache/data_0", false, "Electron caches are excluded"},
	{"within", "Antigravity/Code Cache/js/x", false, "Electron code caches are excluded"},
	{"within", "en-US-10-1.bdic", false, "Electron dictionaries are excluded"},
	{"within", "lib/node_modules/sidecar_sdk/index.mjs", false, "a bundled SDK under node_modules is excluded"},
	{"within", "config/node_modules/sidecar_sdk/index.mjs", false, "node_modules is excluded"},
	{"member", "muse-update.4242.exe", false, "an excluded file gets a row and no archive member"},
	{"within", "antigravity-updater/installer.exe", false, "installers are no catalog entry"},
	{"within", "Programs/antigravity/Antigravity.exe", false, "program directories are no catalog entry"},
	{"within", "web/Codex/component_crx_cache", false, "the browser's component cache is no catalog entry"},
	{"within", ".plugin-appserver/codex.exe", false, "Codex plugin app-server binaries are excluded"},
	{"within", "Computer Use.app/Contents/Info.plist", false, "an app bundle is excluded"},
	{"member", "ipc.fifo", false, "a named pipe gets a row and is never copied"},
	{"member", "daemon-updater.sock", false, "a socket gets a row and is never copied"},
	{"within", "repos/proj/README.md", false, "Copilot Desktop's repository working trees are excluded"},
	{"within", "alice-x/AGENTS.md", false, "Copilot Desktop's worktrees are excluded"},
	{"within", "app-skills/af/SKILL.md", false, "Copilot Desktop's bundled skills are excluded"},
	{"within", "Cache_Data/x", false, "WebView2 caches are excluded"},
	{"within", "component_crx_cache/x", false, "WebView2 component caches are excluded"},
	{"within", "node_modules/m/index.js", false, "a chat project's node_modules is excluded"},
	{"within", "marketplaces/m/.git/HEAD", false, "Copilot CLI marketplace clones are excluded"},
	{"path", "srv/coppool/AGENTS.md", false, "a Copilot pool session's directory is no project"},
	{"within", "agent-cli/.local/share/x", false, "Cursor's embedded agent CLI is excluded"},
	{"within", "node_modules/x.d.ts", false, "Cursor canvases' node_modules is excluded"},
	{"within", "cache/cursor-public/x/0123abcd/mcp.json", false, "Cursor's plugin cache is excluded"},
	{"within", ".openhands/workspaces/run1/f", false, "OpenHands automation workspaces are excluded"},
	{"within", "repo-0123456789/.git/HEAD", false, "CLAI workspace clones are excluded"},
	{"within", "worktrees/a1b2/repo/AGENTS.md", false, "a Codex worktree is no project"},
	{"within", "worktrees/c3d4/repo/AGENTS.md", false, "an Open Interpreter worktree is no project"},
	{"within", "gem/.gemini/worktrees/w1", false, "Gemini CLI project worktrees are excluded"},
	{"within", "gem/.qwen/worktrees/w1", false, "Qwen Code project worktrees are excluded"},
	{"within", "nanows/triggers/inbox", false, "nanobot's trigger queue is no catalog entry"},
	{"within", "compile-cache/x", false, "OpenCode desktop's compile cache is excluded"},
	{"within", "temp/run1/out.txt", false, "Factory temp entries are excluded"},
	{"within", ".cache/amp/runner-desktop/", false, "Amp's cache outside its catalog lines"},
	{"within", ".cache/amp/desktop/", false, "Amp's cache outside its catalog lines"},
	{"within", ".cache/amp/bin/", false, "Amp's cache outside its catalog lines"},
	{"within", ".cache/amp/obelisk-runs/", false, "Amp's cache outside its catalog lines"},
	{"within", "/Library/Caches/ollama", false, "Ollama's Library/Caches is no catalog entry"},
	{"within", "/Library/Caches/com.electron.ollama", false, "Ollama's Electron cache is no catalog entry"},
	{"within", "AppData/Local/devin", true, "Windsurf's devin/bin is no catalog entry"},
	{"within", "hermes-setup.exe", false, "the Hermes updater cache is excluded"},
	{"within", "Roaming/Devin/Cache", true, "the Devin IDE cache is no catalog entry"},
	{"path", "Users/bob/.oh-my-zsh/custom/agents.zsh", false, "a startup file under a symlinked directory is not read"},
	{"within", "opt/dotfiles", false, "no startup file is read through a link"},
	{"path", "Users/bob/.bash_it/custom/agents.bash", false, "a startup file under links leaving the home is not read"},
	{"path", "Users/bob/.zshrc", false, "a symlinked startup file is not followed"},
	{"path", "home/alice/.claude/plugins/nl\nname/node_modules/f.js", false, "node_modules under a name holding a newline is excluded"},
	{"within", "Users/bob/.cache/", false, "a catalog entry under a link leaving the home is not walked"},
	{"within", "opt/linked-cache", false, "the target of a link leaving the home is not walked"},
	{"within", "home/nl/.cache/", false, "a link to a sibling named like the home plus a newline leads out of it"},
	{"within", "home/u1/.cache/", false, "a link whose .. follows a link out of the home leads out of it"},
	{"path", "Users/bob/.pi/agent/little-coder-prompt-history.json", false, "an entry under a symlinked match is not walked"},
	{"within", "home/dot/.pi/agent/", false, "nothing under a link escaping the home is recorded"},
	{"within", "opt/agent-out", false, "an escaping link's target is not walked"},
	{"path", "home/dot/.gemini/settings.json", false, "a link inside the home stages what its target holds, once"},
	{"path", "home/dot/dotfiles/gemini/settings.json", false, "a link inside the home stages what its target holds, once"},
	{"within", "home/dot/.copilot/logs/", false, "a link a claim's walk records is not walked again"},
	{"within", "opt/r1-out", false, "a link under a linked ancestor is recorded, not followed"},
	{"within", "home/dot/.copilot/repos/r1/", false, "a link under a linked ancestor is recorded, not followed"},
	{"path", "home/u[1]/.gemini/tmp/bin/rg", false, "an exclusion applies under a home holding ["},
}

// matches reports whether path is the forbidden path, or holds it for any
// kind but "path".
func (forbidden forbiddenPath) matches(path string) bool {
	switch {
	case forbidden.kind == "path":
		return path == forbidden.part
	case forbidden.anyCase:
		return strings.Contains(strings.ToLower(path), strings.ToLower(forbidden.part))
	}
	return strings.Contains(path, forbidden.part)
}

// reachable reports whether an image entry gives a run a way to the
// forbidden path: it is the path or holds it, it is a link the path runs
// through, or its content names it.
func (forbidden forbiddenPath) reachable(entry imageEntry) bool {
	path, inRoot := strings.CutPrefix(entry.path, "root/")
	switch {
	case inRoot && forbidden.matches(path):
		return true
	case inRoot && entry.link && strings.Contains(forbidden.part, path+"/"):
		return true
	}
	return strings.Contains(entry.text, forbidden.part)
}

// checkForbiddenPaths fails the test for each forbidden path the run
// collected, naming why it must not be, and for each the image gives no
// way to, whose check would prove nothing: the image must hold the path,
// a link the path runs through, or a file naming it (a passwd home).
// image holds the image's entries below its work directory.
func checkForbiddenPaths(t *testing.T, run coverageRun, root string, image []imageEntry) {
	t.Helper()
	paths := make([]string, 0, len(run.rows))
	for _, row := range run.rows {
		paths = append(paths, filepath.ToSlash(relativeTo(root, row.path)))
	}
	for _, forbidden := range forbiddenPaths {
		t.Run("not collected "+forbidden.part, func(t *testing.T) {
			if !slices.ContainsFunc(image, forbidden.reachable) {
				if runtime.GOOS == "windows" {
					t.Skipf("the Windows image holds no %q; the table was built for the Unix image",
						forbidden.part)
				}
				t.Errorf("the image holds no %q, so its check proves nothing", forbidden.part)
			}
			candidates := paths
			if forbidden.kind == "member" {
				candidates = run.members
			}
			if slices.ContainsFunc(candidates, forbidden.matches) {
				t.Errorf("the default run collected %q (%s): %s", forbidden.part, forbidden.kind, forbidden.why)
			}
		})
	}
}

// imageSnapshot returns the image's entries as manifest lines, to tell
// whether a run changed anything in it.
func imageSnapshot(t *testing.T, work string) []string {
	t.Helper()
	entries, err := imagetest.Read(work)
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, imagetest.Format(entry))
	}
	return lines
}

// checkImageUnchanged fails the test on each entry of the image the runs
// made, removed or changed (content, target, mode or modification time).
func checkImageUnchanged(t *testing.T, before, after []string) {
	t.Helper()
	for _, step := range diffLines(before, after) {
		if step.kind != ' ' {
			t.Errorf("a run changed the image: %c %s", step.kind, step.line)
		}
	}
}
