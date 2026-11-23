package imagetest

import (
	"testing"
	"time"
)

// The fixture builders of the Windows cases. Each builds what one case adds
// to the main image, or the separate root a case runs over, and takes it
// away (or takes its deny rule away) when the test ends.

// EnvScanOnce adds one startup file and one file only the --env-scan-home
// sweep reaches (the envscanonce case).
func (img Image) EnvScanOnce(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Alice, ".bashrc.d/once.sh"), "export CODEX_HOME=/opt/x\n")
	build.file(join(img.Alice, "work/once.sh"), "export CODEX_HOME=/opt/x\n")
}

// GlobOutput makes the output directory whose name holds wildcard
// characters, in bob's home, which the sweep must still leave out (the
// envscanglob case), and returns it.
func (img Image) GlobOutput(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	directory := join(img.Bob, "out [1]")
	build.dir(directory)
	return directory
}

// DockerDesktop adds the Windows Docker Desktop disk directory (the desktop
// case).
func (img Image) DockerDesktop(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Alice, "AppData/Local/Docker/wsl/disk/readme.txt"), "vhdx lives here")
}

// RelativeProject builds the project a relative -p names from the root's srv
// directory (the projargs case).
func (img Image) RelativeProject(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	build.file(join(img.Root, "srv/relproj/AGENTS.md"), "# rel")
}

// Inventory sets the time the inventory's last field reports and gives bob a
// directory holding only a nested agent's entry (the inventory cases).
func (img Image) Inventory(test testing.TB) {
	build := newFixture(test, img.Work)
	build.file(join(img.Bob, ".gemini/antigravity-cli/conversations/b1.db"), "db")
	build.done()
	build.touch(join(img.Bob, ".ollama/history"), time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC))
}

// ReadOnlyOutput makes an output directory in which the current user may
// not create files or directories (the readonly case), and returns it.
func (img Image) ReadOnlyOutput(test testing.TB) string {
	build := newFixture(test, img.Work)
	directory := join(img.Work, "readonly")
	build.dir(directory)
	build.done()
	build.deny(directory, "WD,AD")
	return directory
}

// UnreadableCaseVariant builds the case-variant root (CaseVariantRoot) and
// denies listing one spelling of a catalog directory in it, whose other
// spelling is the same directory on NTFS (the fold32 and inventory-fold32
// cases). It skips the test where the rule does not stop the listing.
func (img Image) UnreadableCaseVariant(test testing.TB) {
	root := img.CaseVariantRoot(test)
	newFixture(test, img.Work).denyListing(join(root, "home/upper/.config/pearai/User"))
}

// LinkImage builds the linkroot image (the links case): a file and a
// directory outside the collection, and in eve's antigravity-cli directory a
// relative, a dangling and a directory symlink and a junction, and returns
// its root. The symlinks are left out, with a note, where the account may
// not create them.
func (img Image) LinkImage(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	root := join(img.Work, "linkroot")
	agent := join(root, "home/eve/.gemini/antigravity-cli")
	build.file(join(agent, "log/real.log"), "LINK-TARGET-BYTES")
	build.file(join(root, "outside/secret.txt"), "OUTSIDE-BYTES")
	links := [][2]string{{`log\real.log`, "cli.log"}, {`log\missing.log`, "dangling.log"},
		{`..\..\..\..\outside`, "out"}}
	for _, link := range links {
		if !build.symlinkOrNote(link[0], join(agent, link[1])) {
			break
		}
	}
	build.junction(join(root, "outside"), join(agent, "jn"))
	return root
}

// BrokenJunctionHome builds the brokenjunctionhome image, whose Users
// directory holds alice's home with one catalog file and bob, a junction to
// a directory that does not exist, and returns its root.
func (img Image) BrokenJunctionHome(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	root := join(img.Work, "brokenjunctionhome")
	build.file(join(root, "Users/alice/.claude/settings.json"), `{"model":"x"}`)
	build.junction(join(root, "gone"), join(root, "Users/bob"))
	return root
}

// NonASCIIRoot builds the root71 image of Latin-1, CJK and Cyrillic names:
// one profile's transcripts, a name holding a lone surrogate (written
// through Go's WTF-8 file names), and a symlink with a Cyrillic name to an
// absolute target with a CJK name, left out with a note where symlinks
// cannot be made. It returns the image's root.
func (img Image) NonASCIIRoot(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	root := join(img.Work, "root71")
	directory := join(root, "Users/alice/.claude/projects/-C-u71")
	build.dir(directory)
	names := []string{"uümlé.jsonl", "日本 語.jsonl", "Журнал.jsonl", "lone\xed\xa0\x80.jsonl"}
	for _, name := range names {
		build.write(join(directory, name), `{"type":"user"}`)
	}
	build.symlinkOrNote(join(directory, names[1]), join(directory, "ссылка.jsonl"))
	return root
}
