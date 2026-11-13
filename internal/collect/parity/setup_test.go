//go:build parity

package parity

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Each setup function builds the fixtures one smoke test case builds before
// its run, and registers their removal for the end of the case, so the image
// is as the smoke test built it for the next case.

// setupPatternRoot builds an image root whose name holds "[", which finds its
// homes through home/* with no passwd file.
func setupPatternRoot(t *testing.T, img image) {
	fixture{t}.file(img.work+"/r[1]/home/carol/.gemini/projects.json", "{}\n")
}

// setupEnvScanOnce adds one startup file and one file only the sweep reaches.
func setupEnvScanOnce(t *testing.T, img image) {
	fixture{t}.file(img.root+"/home/alice/.bashrc.d/once", "export CODEX_HOME=/opt/x\n")
	fixture{t}.file(img.root+"/home/alice/scripts/once.sh", "export CODEX_HOME=/opt/x\n")
}

// setupRelativeProject builds the project a relative -p names.
func setupRelativeProject(t *testing.T, img image) {
	fixture{t}.file(img.root+"/srv/relproj/AGENTS.md", "# rel\n")
}

// setupUnreadableHome builds an unreadable home, an unlistable directory and
// one that cannot be entered.
func setupUnreadableHome(t *testing.T, img image) {
	requireNonRoot(t)
	build := fixture{t}
	alice := img.root + "/home/alice"
	build.dir(img.root+"/home/erin/.claude", alice+"/.claude/locked", alice+"/.claude/noenter")
	build.file(alice+"/.claude/locked/f", "x\n")
	build.chmod(img.root+"/home/erin", 0)
	build.chmod(alice+"/.claude/locked", 0)
	build.chmod(alice+"/.claude/noenter", 0o600)
}

// setupUnreadableCaseVariant makes one spelling of a catalog directory unreadable.
func setupUnreadableCaseVariant(t *testing.T, img image) {
	requireNonRoot(t)
	fixture{t}.chmod(img.root+"/home/alice/.config/pearai/User", 0)
}

// setupUnreadableVolume builds a Docker volume and a root-only data root that
// a non-root run cannot read.
func setupUnreadableVolume(t *testing.T, img image) {
	requireNonRoot(t)
	build := fixture{t}
	volumes := img.root + "/var/lib/docker/volumes"
	build.file(volumes+"/hermes_data/_data/sessions/s.json", "{}\n")
	build.chmod(volumes+"/hermes_data/_data", 0)
	build.dir(img.root + "/var/lib/containers/storage/volumes/x_ollama/_data")
	build.chmod(img.root+"/var/lib/containers", 0)
}

// setupDockerDesktop adds the macOS Docker Desktop data directory.
func setupDockerDesktop(t *testing.T, img image) {
	fixture{t}.dir(img.root + "/Users/bob/Library/Containers/com.docker.docker/Data")
}

// setupInventory sets the mtime the inventory's last field reports and adds a
// directory holding only a nested agent's entry.
func setupInventory(t *testing.T, img image) {
	build := fixture{t}
	build.touch(img.root+"/home/ollama/.ollama/history", time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC))
	build.file(img.root+"/Users/bob/.gemini/antigravity-cli/conversations/b1.db", "db\n")
}

// setupUnreadableInventoryHome adds a home the inventory cannot read.
func setupUnreadableInventoryHome(t *testing.T, img image) {
	requireNonRoot(t)
	setupInventory(t, img)
	fixture{t}.dir(img.root + "/home/carol/.claude")
	fixture{t}.chmod(img.root+"/home/carol", 0)
}

// setupBatchRoot builds an image whose project directory holds names the
// hashers escape, an unreadable file between readable ones and 300 files,
// more than one 256-file chunk of the sh collector's stage worker.
func setupBatchRoot(t *testing.T, img image) {
	build := fixture{t}
	root := img.work + "/root71"
	directory := root + "/home/carol/.claude/projects/-srv-p71"
	build.file(root+"/etc/passwd", "carol:x:1002:1002::/home/carol:/bin/sh\n")
	build.dir(directory, root+"/home/carol/.claude/file-history/s1")
	build.write(directory+"/a.jsonl", "first\n")
	build.write(directory+"/nl\nname.jsonl", "newline name\n")
	build.write(directory+"/back\\slash.jsonl", "backslash name\n")
	build.write(directory+"/cr\rname.jsonl", "cr name\n")
	build.write(directory+"/m-locked.jsonl", "locked\n")
	build.write(directory+"/z.jsonl", "last\n")
	for index := 0; index < 300; index++ {
		build.write(fmt.Sprintf("%s/home/carol/.claude/file-history/s1/f%d@v1", root, index),
			fmt.Sprintf("v%d\n", index))
	}
	build.symlink("tgt\n", root+"/home/carol/.claude/nl-target-link")
	build.dir(root + "/home/carol/.claude/plugins/nl\nname/node_modules")
	build.write(root+"/home/carol/.claude/plugins/nl\nname/node_modules/index.js",
		strings.Repeat("0", 20000)+"\n")
	setupLockedBatchFile(t, directory+"/m-locked.jsonl")
}

// setupLockedBatchFile makes the file unreadable, unless root would read it anyway.
func setupLockedBatchFile(t *testing.T, path string) {
	if !isRoot() {
		fixture{t}.chmod(path, 0)
	}
}

// setupMsixRoot builds one Windows profile holding the Claude Desktop MSIX
// build's virtualised data, its logs and its Claude-Data directory.
func setupMsixRoot(t *testing.T, img image) {
	build := fixture{t}
	root := img.work + "/root73"
	spaces := "11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222"
	local := root + "/Users/bob/AppData/Local"
	pkg := local + "/Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude"
	sessions := pkg + "/local-agent-mode-sessions/" + spaces
	build.dir(pkg+"/claude-code/2.1.295/abc", pkg+"/Cache", pkg+"/Network", sessions+"/local_33333333-3333-4333-8333-333333333333",
		local+"/Claude/logs", local+"/Claude-Data/claude_pzs8sxrjxfjjc",
		local+"/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude")
	build.write(pkg+"/config.json", "{\"oauth:tokenCache\":\"enc\"}\n")
	build.write(pkg+"/Local State", "{\"os_crypt\":{}}\n")
	build.write(pkg+"/claude_desktop_config.json", "{\"mcpServers\":{}}\n")
	build.write(pkg+"/Network/Cookies", "cookies\n")
	build.dir(pkg + "/claude-code-sessions/" + spaces)
	build.write(pkg+"/claude-code-sessions/"+spaces+"/local_44444444-4444-4444-8444-444444444444.json",
		"{\"sessionId\":\"local_44444444-4444-4444-8444-444444444444\",\"cwd\":\"C:\\\\proj\"}\n")
	build.write(sessions+"/remote-session-spaces.json",
		"{\"entries\":[{\"sessionId\":\"0123456789abcdef0123456789abcdef\",\"folders\":[]}]}\n")
	build.write(sessions+"/local_33333333-3333-4333-8333-333333333333/.audit-key", "key\n")
	build.write(pkg+"/claude-code/2.1.295/abc/claude.exe", "MZ\n")
	build.write(pkg+"/Cache/data_0", "cache\n")
	build.write(local+"/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude/config.json",
		"{\"oauth:tokenCache\":\"enc\"}\n")
	build.write(local+"/Claude/logs/main.log", "[MSIX] Filesystem virtualization active\n")
	build.write(local+"/Claude-Data/claude_pzs8sxrjxfjjc/config.json", "{\"oauth:tokenCache\":\"enc\"}\n")
}

// checkLinkedSourceIntact fails when a run left the source file of the
// recreated-link case without its content, as the smoke test's check does,
// and puts the content back so later cases see the image as built.
func checkLinkedSourceIntact(t *testing.T, img image) {
	t.Helper()
	path := img.root + "/home/dot/dotfiles/claude/proj/AGENTS.md"
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "x\n" {
		t.Errorf("a failed copy through a recreated link damaged its source %s: content %q, error %v",
			path, content, err)
		fixture{t}.write(path, "x\n")
	}
}
