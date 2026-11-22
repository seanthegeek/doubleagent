//go:build unix

package collect_test

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// goldenCases lists the Unix cases, the default case first. A case with a
// setup changes the image, and one whose output directory lies in the image
// writes into it, so neither is parallel.
func goldenCases() []goldenCase {
	cases := []goldenCase{
		{name: "default", parallel: true},
		{name: "root486", root: "{WORK}/r[1]", args: []string{"--no-docker", "--no-projects"},
			setup: func(img imagetest.Image, test testing.TB) { img.PatternRoot(test) }},
		{name: "linkproj497", args: []string{"-p", "{ROOT}/home/dot/.claude/proj"},
			after: imagetest.Image.LinkedSourceIntact},
	}
	cases = append(cases, projectLinkCases()...)
	cases = append(cases, []goldenCase{
		{name: "nosecrets", parallel: true, args: []string{"--no-secrets"}},
		{name: "envscan", parallel: true, args: []string{"--env-scan-home"}},
		{name: "envscanonce", args: []string{"--env-scan-home"}, setup: imagetest.Image.EnvScanOnce},
		{name: "envscanout", args: []string{"--env-scan-home"}, outDir: "{ALICE}"},
		{name: "envscanglob", args: []string{"--env-scan-home"}, outDir: "{BOB}/out [1]"},
		{name: "envscanroot", parallel: true,
			args: []string{"--env-scan-home", "-p", "{ROOT}/home", "-p", "{ROOT}/opt/daemonproj"}},
		{name: "full", parallel: true, args: []string{"--full"}},
		{name: "users", parallel: true, args: []string{"-u", "bob"}},
		{name: "nodocker", parallel: true, args: []string{"--no-docker"}},
		{name: "cwd105", parallel: true, cwd: "{WORK}/cwd105", args: []string{"--no-docker", "--no-projects"}},
		{name: "dockerperm", setup: imagetest.Image.UnreadableVolume},
		{name: "desktop", setup: imagetest.Image.DockerDesktop},
		{name: "fwvolumes", setup: imagetest.Image.FrameworkVolumes},
		{name: "fwvolumesnosecrets", args: []string{"--no-secrets"}, setup: imagetest.Image.FrameworkVolumes},
		{name: "projargs", cwd: "{ROOT}/srv", args: []string{"-p", "relproj", "-p", "/nonexistent-cac/proj"},
			setup: imagetest.Image.RelativeProject},
		{name: "readperm", setup: imagetest.Image.UnreadableHome},
		{name: "cwd109", parallel: true, cwd: "{WORK}/cwd109", args: []string{"--no-docker", "--no-projects"}},
		{name: "cdpath110", parallel: true, cwd: "{WORK}", root: "root", env: []string{"CDPATH={WORK}/cdp110"},
			args: []string{"--no-docker", "--no-projects"}},
		{name: "fold32", setup: imagetest.Image.UnreadableCaseVariant},
		{name: "badname51", setup: func(img imagetest.Image, test testing.TB) { img.BadName(test) }},
		{name: "batch71", root: "{WORK}/root71", args: []string{"--no-docker"},
			setup: func(img imagetest.Image, test testing.TB) { img.BatchRoot(test) }},
	}...)
	return append(append(cases, packageCases()...), inventoryCases()...)
}

// projectLinkCases are the runs over projects whose own path runs through a
// symlinked directory.
func projectLinkCases() []goldenCase {
	outOfHome := func(absolute bool) func(imagetest.Image, testing.TB) {
		return func(img imagetest.Image, test testing.TB) { img.ProjectOutOfHome(test, absolute) }
	}
	linked := func(absolute bool) func(imagetest.Image, testing.TB) {
		return func(img imagetest.Image, test testing.TB) { img.LinkedProject(test, absolute) }
	}
	return []goldenCase{
		{name: "projout503", args: []string{"-p", "{ROOT}/home/dot/code/proj"}, setup: outOfHome(false)},
		{name: "projoutabs503", args: []string{"-p", "{ROOT}/home/dot/code/proj"}, setup: outOfHome(true)},
		{name: "projself503", args: []string{"-p", "{ROOT}/home/alice/.claude/proj"}, setup: linked(false)},
		{name: "projselfabs503", args: []string{"-p", "{ROOT}/home/alice/.claude/proj"}, setup: linked(true)},
		{name: "projhint503", args: []string{"-p", "{ROOT}/home/dot/.claude/proj", "--env-scan-home"},
			setup: imagetest.Image.ProjectHintFile},
		{name: "projhost503", args: []string{"-p", "{ROOT}/home/alice/.claude/ext/proj"},
			setup: imagetest.Image.ProjectOnTheHost},
		{name: "projother503", setup: imagetest.Image.ProjectOfAnotherUser},
	}
}

// packageCases are the three runs over the Claude Desktop MSIX image.
func packageCases() []goldenCase {
	msix := func(img imagetest.Image, test testing.TB) { img.MsixRoot(test) }
	return []goldenCase{
		{name: "store73", root: "{WORK}/root73", args: []string{"--no-docker"}, setup: msix},
		{name: "nosecrets73", root: "{WORK}/root73", args: []string{"--no-docker", "--no-secrets"}, setup: msix},
		{name: "full73", root: "{WORK}/root73", args: []string{"--no-docker", "--full"}, setup: msix},
	}
}

// inventoryCases are the stat-only runs: no -o and no staging, one JSON line
// per agent on stdout.
func inventoryCases() []goldenCase {
	return []goldenCase{
		{name: "inventory", inventory: true, cwd: "{OUT}", args: []string{"-q"}, setup: imagetest.Image.Inventory},
		{name: "inventory-o", inventory: true, stderrLog: true, args: []string{"-o", "{OUT}/inv-o"},
			setup: imagetest.Image.Inventory},
		{name: "inventory-full", inventory: true, args: []string{"-q", "--full"}, setup: imagetest.Image.Inventory},
		{name: "inventory-u", inventory: true, args: []string{"-q", "-u", "bob", "--no-docker"},
			setup: imagetest.Image.Inventory},
		{name: "inventory-perm", inventory: true, args: []string{"-q"}, setup: imagetest.Image.UnreadableInventoryHome},
	}
}
