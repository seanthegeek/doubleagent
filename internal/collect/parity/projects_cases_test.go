//go:build parity

package parity

import "testing"

// linkedProject is the linkproj497 project: -p names it through dot's
// .claude, an absolute link to dotfiles/claude inside the home.
var linkedProject = physicalProject{owner: "dot", project: "home/dot/.claude/proj",
	link: "home/dot/.claude", physical: "home/dot/dotfiles/claude/proj", names: []string{"AGENTS.md"}}

// outsideProject is dot's code/proj, through a link out of the home to
// data/code; selfLinkedProject is alice's .claude/proj, itself a link to
// dotfiles/proj inside the home.
var (
	outsideProject = physicalProject{owner: "dot", project: "home/dot/code/proj",
		link: "home/dot/code", physical: "data/code/proj", names: []string{"AGENTS.md"}}
	selfLinkedProject = physicalProject{owner: "alice", project: "home/alice/.claude/proj",
		link: "home/alice/.claude/proj", physical: "home/alice/dotfiles/proj", names: []string{"AGENTS.md"}}
)

// projectLinkCases are the runs over projects whose own path runs through a
// symlinked directory, which the binary stages where their files physically
// are (the parity exceptions at the top of harness_test.go).
func projectLinkCases() []parityCase {
	return []parityCase{
		{name: "projout503", args: []string{"-p", "{ROOT}/home/dot/code/proj"},
			setup: setupProjectOutOfHome("../../data/code"), expect: outsideProject.expect},
		{name: "projoutabs503", args: []string{"-p", "{ROOT}/home/dot/code/proj"},
			setup: setupProjectOutOfHome("{ROOT}/data/code"), expect: outsideProject.expect},
		{name: "projself503", args: []string{"-p", "{ROOT}/home/alice/.claude/proj"},
			setup: setupLinkedProject("../dotfiles/proj"), expect: selfLinkedProject.expect},
		{name: "projselfabs503", args: []string{"-p", "{ROOT}/home/alice/.claude/proj"},
			setup: setupLinkedProject("{ROOT}/home/alice/dotfiles/proj"), expect: selfLinkedProject.expect},
		{name: "projhint503", args: []string{"-p", "{ROOT}/home/dot/.claude/proj", "--env-scan-home"},
			setup: setupProjectHintFile, expect: physicalProject{owner: "dot", project: linkedProject.project,
				link: linkedProject.link, physical: linkedProject.physical,
				names: []string{"AGENTS.md", "env.sh"}}.expect},
		{name: "projhost503", args: []string{"-p", "{ROOT}/home/alice/.claude/ext/proj"},
			setup: setupProjectOnTheHost, expect: hostProject{owner: "alice",
				project: "home/alice/.claude/ext/proj", link: "home/alice/.claude/ext",
				names: []string{"AGENTS.md"}}.expect},
		{name: "projother503", setup: setupProjectOfAnotherUser, expect: physicalProject{owner: "alice",
			project: linkedProject.project, link: linkedProject.link, physical: linkedProject.physical,
			names: linkedProject.names}.expect},
	}
}

// setupProjectOutOfHome builds dot's code, a link to target (which may hold
// {ROOT}) leading out of the home to data/code, which holds proj/AGENTS.md.
func setupProjectOutOfHome(target string) func(t *testing.T, img image) {
	return func(t *testing.T, img image) {
		build := fixture{t}
		build.file(img.root+"/data/code/proj/AGENTS.md", "# out\n")
		build.symlink(expand(target, img, ""), img.root+"/home/dot/code")
	}
}

// setupLinkedProject builds alice's .claude/proj, a link to target (which may
// hold {ROOT}) inside the home, to dotfiles/proj holding AGENTS.md.
func setupLinkedProject(target string) func(t *testing.T, img image) {
	return func(t *testing.T, img image) {
		build := fixture{t}
		build.file(img.root+"/home/alice/dotfiles/proj/AGENTS.md", "# self\n")
		build.symlink(expand(target, img, ""), img.root+"/home/alice/.claude/proj")
	}
}

// setupProjectHintFile puts a startup file naming a relocation variable in
// the linkproj497 project, reached through dot's absolute .claude link.
func setupProjectHintFile(t *testing.T, img image) {
	fixture{t}.file(img.root+"/home/dot/dotfiles/claude/proj/env.sh", "CLAUDE_CONFIG_DIR=/opt/x\n")
}

// setupProjectOfAnotherUser has alice's Claude Code history name the
// linkproj497 project under dot's home.
func setupProjectOfAnotherUser(t *testing.T, img image) {
	fixture{t}.file(img.root+"/home/alice/.claude-u503/history.jsonl",
		"{\"project\":\"/home/dot/.claude/proj\"}\n")
}

// setupProjectOnTheHost builds alice's .claude/ext, an absolute link out of
// the image root to the work directory's outside503, which holds
// proj/AGENTS.md: a place on the analyst's host, not in the image.
func setupProjectOnTheHost(t *testing.T, img image) {
	build := fixture{t}
	build.file(img.work+"/outside503/proj/AGENTS.md", "# host\n")
	build.symlink(img.work+"/outside503", img.root+"/home/alice/.claude/ext")
}
