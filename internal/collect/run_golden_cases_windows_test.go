package collect_test

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// goldenCases lists the Windows cases, the default case first. A case with
// a setup changes the image, and one whose output directory lies in the
// image writes into it, so neither is parallel.
func goldenCases() []goldenCase {
	cases := []goldenCase{
		{name: "default", parallel: true},
		{name: "root71", root: `{WORK}\root71`, args: []string{"--no-docker"},
			setup: func(img imagetest.Image, test testing.TB) { img.NonASCIIRoot(test) }},
		{name: "root486", root: `{WORK}\r[1]`, args: []string{"--no-docker", "--no-projects"},
			setup: func(img imagetest.Image, test testing.TB) { img.PatternRoot(test) }},
		{name: "nosecrets", parallel: true, args: []string{"--no-secrets"}},
		{name: "full", parallel: true, args: []string{"--full"}},
		{name: "users", parallel: true, args: []string{"-u", "bob"}},
		{name: "nodocker", parallel: true, args: []string{"--no-docker"}},
		{name: "cwd105", parallel: true, cwd: `{WORK}\cwd105`, args: []string{"--no-docker", "--no-projects"}},
	}
	cases = append(cases, envScanCases()...)
	cases = append(cases, []goldenCase{
		{name: "desktop", setup: imagetest.Image.DockerDesktop},
		{name: "fwvolumes", setup: imagetest.Image.FrameworkVolumes},
		{name: "fwvolumesnosecrets", args: []string{"--no-secrets"}, setup: imagetest.Image.FrameworkVolumes},
	}...)
	cases = append(cases, inventoryCases()...)
	cases = append(cases, linkAndProjectCases()...)
	return append(cases, packageCases()...)
}

// envScanCases are the --env-scan-home runs and the one with an invalid
// size limit, which the binary refuses.
func envScanCases() []goldenCase {
	return []goldenCase{
		{name: "envscan", parallel: true, args: []string{"--env-scan-home"}},
		{name: "envscanonce", args: []string{"--env-scan-home"}, setup: imagetest.Image.EnvScanOnce},
		{name: "envscanout", args: []string{"--env-scan-home"}, outDir: "{ALICE}"},
		{name: "envscanglob", args: []string{"--env-scan-home"}, outDir: `{BOB}\out [1]`,
			setup: func(img imagetest.Image, test testing.TB) { img.GlobOutput(test) }},
		{name: "envscanroot", parallel: true,
			args: []string{"--env-scan-home", "-p", `{ROOT}\home`, "-p", `{ROOT}\opt\daemonproj`}},
		{name: "envscanbad", parallel: true, args: []string{"--env-scan-max-size", "x"}, fails: true},
	}
}

// linkAndProjectCases are the runs over the link images, the -p run with a
// relative and a missing project, the runs whose output directory cannot be
// used, the run from a working directory holding a tree an exclusion glob
// matches, and the runs over the case-variant root, one with a directory the
// user may not list.
func linkAndProjectCases() []goldenCase {
	return []goldenCase{
		{name: "noproj31", parallel: true, args: []string{"--no-projects"}},
		{name: "links", root: `{WORK}\linkroot`, args: []string{"--no-docker"},
			setup: func(img imagetest.Image, test testing.TB) { img.LinkImage(test) }},
		{name: "brokenjunctionhome", root: `{WORK}\brokenjunctionhome`, args: []string{"--no-docker"},
			setup: func(img imagetest.Image, test testing.TB) { img.BrokenJunctionHome(test) }},
		{name: "projargs", cwd: `{ROOT}\srv`, args: []string{"-p", "relproj", "-p", `{ROOT}\nonexistent-cac`},
			setup: imagetest.Image.RelativeProject},
		{name: "readonly", outDir: `{WORK}\readonly`, args: []string{"--no-docker"}, fails: true,
			setup: func(img imagetest.Image, test testing.TB) { img.ReadOnlyOutput(test) }},
		{name: "file108", parallel: true, outDir: `{WORK}\file108\out`, args: []string{"--no-docker"}, fails: true},
		{name: "cwd109", parallel: true, cwd: `{WORK}\cwd109`, args: []string{"--no-docker", "--no-projects"}},
		{name: "fold32", root: `{WORK}\root584`, args: []string{"--no-docker"},
			setup: imagetest.Image.UnreadableCaseVariant},
		{name: "case584", root: `{WORK}\root584`, args: []string{"--no-docker"},
			setup: func(img imagetest.Image, test testing.TB) { img.CaseVariantRoot(test) }},
	}
}

// packageCases are the three runs over the Claude Desktop MSIX image.
func packageCases() []goldenCase {
	msix := func(img imagetest.Image, test testing.TB) { img.MsixRoot(test) }
	root := `{WORK}\root73`
	return []goldenCase{
		{name: "store73", root: root, args: []string{"--no-docker"}, setup: msix},
		{name: "nosecrets73", root: root, args: []string{"--no-docker", "--no-secrets"}, setup: msix},
		{name: "full73", root: root, args: []string{"--no-docker", "--full"}, setup: msix},
	}
}

// inventoryCases are the stat-only runs: no -o and no staging, one JSON line
// per agent on stdout. inventory-fold32 runs over fold32's fixture, a
// directory the user may not list, with the log on stderr.
func inventoryCases() []goldenCase {
	inventory := imagetest.Image.Inventory
	return []goldenCase{
		{name: "inventory", inventory: true, cwd: "{OUT}", args: []string{"-q"}, setup: inventory},
		{name: "inventory-o", inventory: true, stderrLog: true, args: []string{"-o", `{OUT}\inv-o`}, setup: inventory},
		{name: "inventory-full", inventory: true, args: []string{"-q", "--full"}, setup: inventory},
		{name: "inventory-u", inventory: true, args: []string{"-q", "-u", "bob", "--no-docker"}, setup: inventory},
		{name: "inventory-nodocker", inventory: true, args: []string{"-q", "--no-docker"}, setup: inventory},
		{name: "inventory-fold32", inventory: true, stderrLog: true, root: `{WORK}\root584`,
			args: []string{"--no-docker"}, setup: imagetest.Image.UnreadableCaseVariant},
	}
}
