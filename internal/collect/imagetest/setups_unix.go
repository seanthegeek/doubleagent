//go:build unix

package imagetest

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The fixture builders of the Unix cases. Each builds what one case adds to
// the main image, or the separate root a case runs over, and takes it away
// when the test ends.

// EnvScanOnce adds one startup file and one file only the --env-scan-home
// sweep reaches (the envscanonce case).
func (img Image) EnvScanOnce(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Root, "home/alice/.bashrc.d/once"), "export CODEX_HOME=/opt/x\n")
	build.file(join(img.Root, "home/alice/scripts/once.sh"), "export CODEX_HOME=/opt/x\n")
}

// RelativeProject builds the project a relative -p names from the root's srv
// directory (the projargs case).
func (img Image) RelativeProject(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Root, "srv/relproj/AGENTS.md"), "# rel\n")
}

// UnreadableHome builds an unreadable home, an unlistable directory and one
// that cannot be entered (the readperm case). It skips the test as root.
func (img Image) UnreadableHome(test testing.TB) {
	requireNonRoot(test)
	build := newFixture(test, img.Work)
	claude := join(img.Alice, ".claude")
	build.dir(join(img.Root, "home/erin/.claude"), join(claude, "locked"), join(claude, "noenter"))
	build.file(join(claude, "locked/f"), "x\n")
	build.done()
	build.chmod(join(img.Root, "home/erin"), 0)
	build.chmod(join(claude, "locked"), 0)
	build.chmod(join(claude, "noenter"), 0o600)
}

// UnreadableCaseVariant builds the case-variant root (CaseVariantRoot) and
// makes one spelling of a catalog directory in it unreadable (the fold32
// case). It skips the test as root.
func (img Image) UnreadableCaseVariant(test testing.TB) {
	requireNonRoot(test)
	root := img.CaseVariantRoot(test)
	newFixture(test, img.Work).chmod(join(root, "home/upper/.config/pearai/User"), 0)
}

// UnreadableVolume builds a Docker volume and a root-only Podman data root
// that a run without root cannot read (the dockerperm case).
func (img Image) UnreadableVolume(test testing.TB) {
	requireNonRoot(test)
	build := newFixture(test, img.Work)
	volumes := join(img.Root, "var/lib/docker/volumes")
	build.file(join(volumes, "hermes_data/_data/sessions/s.json"), "{}\n")
	build.dir(join(img.Root, "var/lib/containers/storage/volumes/x_ollama/_data"))
	build.done()
	build.chmod(join(volumes, "hermes_data/_data"), 0)
	build.chmod(join(img.Root, "var/lib/containers"), 0)
}

// DockerDesktop adds the macOS Docker Desktop data directory (the desktop
// case).
func (img Image) DockerDesktop(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.dir(join(img.Bob, "Library/Containers/com.docker.docker/Data"))
}

// Inventory sets the time the inventory's last field reports and adds a
// directory holding only a nested agent's entry (the inventory cases).
func (img Image) Inventory(test testing.TB) {
	build := newFixture(test, img.Work)
	build.file(join(img.Bob, ".gemini/antigravity-cli/conversations/b1.db"), "db\n")
	build.done()
	build.touch(join(img.Root, "home/ollama/.ollama/history"), time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC))
}

// UnreadableInventoryHome adds, beside Inventory's fixtures, a home the
// inventory cannot read (the inventory-perm case).
func (img Image) UnreadableInventoryHome(test testing.TB) {
	requireNonRoot(test)
	img.Inventory(test)
	build := newFixture(test, img.Work)
	build.dir(join(img.Root, "home/carol/.claude"))
	build.done()
	build.chmod(join(img.Root, "home/carol"), 0)
}

// BatchRoot builds the root71 image, whose project directory holds names
// the hashers escape, an unreadable file between readable ones and 300
// files, more than one 256-file chunk of a batching stage step (the batch71
// case), and returns its root.
func (img Image) BatchRoot(test testing.TB) string {
	build := newFixture(test, img.Work)
	root := join(img.Work, "root71")
	carol := join(root, "home/carol/.claude")
	directory := join(carol, "projects/-srv-p71")
	build.file(join(root, "etc/passwd"), "carol:x:1002:1002::/home/carol:/bin/sh\n")
	build.dir(directory, join(carol, "file-history/s1"))
	for name, content := range map[string]string{"a.jsonl": "first\n", "nl\nname.jsonl": "newline name\n",
		"back\\slash.jsonl": "backslash name\n", "cr\rname.jsonl": "cr name\n",
		"m-locked.jsonl": "locked\n", "z.jsonl": "last\n"} {
		build.write(join(directory, name), content)
	}
	for index := 0; index < 300; index++ {
		build.write(join(carol, fmt.Sprintf("file-history/s1/f%d@v1", index)), fmt.Sprintf("v%d\n", index))
	}
	build.symlink("tgt\n", join(carol, "nl-target-link"))
	build.dir(join(carol, "plugins/nl\nname/node_modules"))
	build.write(join(carol, "plugins/nl\nname/node_modules/index.js"), strings.Repeat("0", 20000)+"\n")
	build.done()
	if !isRoot() {
		build.chmod(join(directory, "m-locked.jsonl"), 0)
	}
	return root
}

// BadName adds a transcript whose name is not valid UTF-8 to alice's Claude
// Code projects (the badname51 case), and returns that name. It skips the
// test where the file system refuses such a name, as macOS's does; Linux
// file systems take any bytes, so a refusal there fails the test.
func (img Image) BadName(test testing.TB) string {
	const name = "lone\xff\xfe.jsonl"
	build := newFixture(test, img.Work)
	defer build.done()
	directory := join(img.Alice, ".claude/projects/-srv-bad51")
	build.dir(directory)
	if err := os.WriteFile(join(directory, name), []byte("{\"type\":\"user\"}\n"), 0o644); err != nil {
		if runtime.GOOS == "linux" {
			test.Fatalf("Linux refused a name that is not valid UTF-8: %v", err)
		}
		test.Skipf("this file system refuses a name that is not valid UTF-8: %v", err)
	}
	return name
}

// LinkedSourceIntact fails the test when a run left the source file of the
// linkproj497 case, reached through dot's absolute .claude link, without its
// content, and puts the content back so later tests see the image as built.
func (img Image) LinkedSourceIntact(test testing.TB) {
	test.Helper()
	path := join(img.Root, "home/dot/dotfiles/claude/proj/AGENTS.md")
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "x\n" {
		test.Errorf("a failed copy through a recreated link damaged its source %s: content %q, error %v",
			path, content, err)
		newFixture(test, img.Work).write(path, "x\n")
	}
}

// ProjectOutOfHome builds dot's code, a link leading out of the home to
// data/code, which holds proj/AGENTS.md (the projout503 case, and with
// absolute set, whose link target is absolute, the projoutabs503 case).
func (img Image) ProjectOutOfHome(test testing.TB, absolute bool) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Root, "data/code/proj/AGENTS.md"), "# out\n")
	target := "../../data/code"
	if absolute {
		target = join(img.Root, "data/code")
	}
	build.symlink(target, join(img.Root, "home/dot/code"))
}

// LinkedProject builds alice's .claude/proj, a link inside the home to
// dotfiles/proj holding AGENTS.md (the projself503 case, and with absolute
// set, whose link target is absolute, the projselfabs503 case).
func (img Image) LinkedProject(test testing.TB, absolute bool) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Alice, "dotfiles/proj/AGENTS.md"), "# self\n")
	target := "../dotfiles/proj"
	if absolute {
		target = join(img.Alice, "dotfiles/proj")
	}
	build.symlink(target, join(img.Alice, ".claude/proj"))
}

// ProjectHintFile puts a startup file naming a relocation variable in the
// project dot's absolute .claude link leads to (the projhint503 case).
func (img Image) ProjectHintFile(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Root, "home/dot/dotfiles/claude/proj/env.sh"), "CLAUDE_CONFIG_DIR=/opt/x\n")
}

// ProjectOfAnotherUser has alice's Claude Code history name the project
// under dot's home (the projother503 case).
func (img Image) ProjectOfAnotherUser(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Alice, ".claude-u503/history.jsonl"), "{\"project\":\"/home/dot/.claude/proj\"}\n")
}

// ProjectOnTheHost builds alice's .claude/ext, an absolute link out of the
// image root to the work directory's outside503, which holds
// proj/AGENTS.md: a place on the analyst's host, not in the image (the
// projhost503 case).
func (img Image) ProjectOnTheHost(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Work, "outside503/proj/AGENTS.md"), "# host\n")
	build.symlink(join(img.Work, "outside503"), join(img.Alice, ".claude/ext"))
}
