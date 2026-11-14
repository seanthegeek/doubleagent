//go:build parity && windows

package parity

import (
	"os"
	"testing"
)

// windowsCase is one run of smoke.ps1's run list. shared says the case
// builds no fixture and changes nothing in the image, so it runs in parallel
// with the other shared cases; fails says both collectors must exit non-zero
// before writing anything, so there is no output to require.
type windowsCase struct {
	parityCase
	shared bool
	fails  bool
}

// run builds the case's fixtures and compares the two collectors' runs.
func (c windowsCase) run(t *testing.T, img image, reference oracle, binary []string) {
	t.Helper()
	if c.skip != "" {
		t.Skip(c.skip)
	}
	if c.setup != nil {
		c.setup(t, img)
	}
	if !c.shared {
		settleTree(t, img.work)
	}
	if c.fails {
		c.compareFailure(t, img, reference, binary, newWindowsNormaliser())
		return
	}
	c.compare(t, img, reference, binary, newWindowsNormaliser())
}

// compareFailure runs a case both collectors must refuse: each exits
// non-zero and leaves its output directory empty, and the exit codes and the
// error each prints are compared.
func (c windowsCase) compareFailure(t *testing.T, img image, reference oracle, binary []string, normalise normaliser) {
	t.Helper()
	dir, _ := c.outputDir(t, img)
	want := reference(t, c.invocation(img, dir))
	if want.code == 0 {
		t.Fatalf("the PowerShell collector exited 0 where the case expects a refusal: %s", want.stdout)
	}
	requireEmptyDir(t, "PowerShell collector", dir)
	removeOutputs(dir)
	got := execute(t, binary, c.invocation(img, dir))
	requireEmptyDir(t, "binary", dir)
	compareRuns(t, normalise(want, false, c.stderrLog), normalise(got, true, c.stderrLog))
}

// requireEmptyDir fails when a refused run left anything in dir.
func requireEmptyDir(t *testing.T, label, dir string) {
	t.Helper()
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%s: the refused run left %d entries in %s, the first %s", label, len(entries), dir, entries[0].Name())
	}
}

// windowsCases lists every case, in the order of smoke.ps1's run list.
func windowsCases() []windowsCase {
	cases := []windowsCase{
		{parityCase: parityCase{name: "default"}, shared: true},
		{parityCase: parityCase{name: "root71", root: `{WORK}\root71`, args: []string{"--no-docker"},
			setup: setupNonASCIIRoot}},
		{parityCase: parityCase{name: "root486", root: `{WORK}\r[1]`, args: []string{"--no-docker", "--no-projects"},
			setup: setupPatternRoot}},
		{parityCase: parityCase{name: "nosecrets", args: []string{"--no-secrets"}}, shared: true},
		{parityCase: parityCase{name: "full", args: []string{"--full"}}, shared: true},
		{parityCase: parityCase{name: "users", args: []string{"-u", "bob"}}, shared: true},
		{parityCase: parityCase{name: "nodocker", args: []string{"--no-docker"}}, shared: true},
		{parityCase: parityCase{name: "cwd105", cwd: `{WORK}\cwd105`, args: []string{"--no-docker", "--no-projects"}},
			shared: true},
		{parityCase: parityCase{name: "claims106", args: []string{"--no-projects"},
			env: []string{`NESTED_CLAIMS={ROOT}\var\lib\docker\volumes\a0_usr\_data\chats`}}, shared: true},
	}
	cases = append(cases, envScanCases()...)
	cases = append(cases, []windowsCase{
		{parityCase: parityCase{name: "desktop", setup: setupWindowsDockerDesktop}},
		{parityCase: parityCase{name: "fwvolumes", setup: setupFrameworkVolumes}},
		{parityCase: parityCase{name: "fwvolumesnosecrets", args: []string{"--no-secrets"}, setup: setupFrameworkVolumes}},
	}...)
	cases = append(cases, windowsInventoryCases()...)
	cases = append(cases, windowsCase{parityCase: parityCase{name: "noproj31", args: []string{"--no-projects"}}, shared: true},
		windowsCase{parityCase: parityCase{name: "links", root: `{WORK}\linkroot`, args: []string{"--no-docker"},
			setup: setupLinkImage}})
	cases = append(cases, projectAndOutputCases()...)
	cases = append(cases, windowsCase{parityCase: parityCase{name: "fold32", args: []string{"--no-docker"},
		setup: setupWindowsUnreadableCaseVariant}})
	return append(cases, windowsPackageCases()...)
}

// envScanCases are the --env-scan-home runs and the one with an invalid size
// limit.
func envScanCases() []windowsCase {
	return []windowsCase{
		{parityCase: parityCase{name: "envscan", args: []string{"--env-scan-home"}}, shared: true},
		{parityCase: parityCase{name: "envscanonce", args: []string{"--env-scan-home"}, setup: setupWindowsEnvScanOnce}},
		{parityCase: parityCase{name: "envscanout", args: []string{"--env-scan-home"}, outDir: "{ALICE}"}},
		{parityCase: parityCase{name: "envscanglob", args: []string{"--env-scan-home"}, outDir: `{BOB}\out [1]`,
			setup: setupGlobOutput, expect: dropWildcardLogError}},
		{parityCase: parityCase{name: "envscanroot", args: []string{"--env-scan-home", "-p", `{ROOT}\home`,
			"-p", `{ROOT}\opt\daemonproj`}}, shared: true},
		{parityCase: parityCase{name: "envscanbad", args: []string{"--env-scan-max-size", "x"}}, shared: true,
			fails: true},
	}
}

// projectAndOutputCases are the -p run with a relative and a missing
// project, the runs whose output directory cannot be used (a read-only one
// and one under a file, both exit 2 before collecting) and the run from a
// working directory holding a tree an exclusion glob matches.
func projectAndOutputCases() []windowsCase {
	return []windowsCase{
		{parityCase: parityCase{name: "projargs", cwd: `{ROOT}\srv`,
			args: []string{"-p", "relproj", "-p", `{ROOT}\nonexistent-cac`}, setup: setupWindowsRelativeProject}},
		{parityCase: parityCase{name: "readonly", outDir: `{WORK}\readonly`, args: []string{"--no-docker"},
			setup: setupReadOnlyOutput}, fails: true},
		{parityCase: parityCase{name: "file108", outDir: `{WORK}\file108\out`, args: []string{"--no-docker"}},
			shared: true, fails: true},
		{parityCase: parityCase{name: "cwd109", cwd: `{WORK}\cwd109`, args: []string{"--no-docker", "--no-projects"}},
			shared: true},
	}
}

// windowsPackageCases are the three runs over the Claude Desktop MSIX image.
func windowsPackageCases() []windowsCase {
	root := `{WORK}\root73`
	return []windowsCase{
		{parityCase: parityCase{name: "store73", root: root, args: []string{"--no-docker"}, setup: setupMsixRoot}},
		{parityCase: parityCase{name: "nosecrets73", root: root, args: []string{"--no-docker", "--no-secrets"},
			setup: setupMsixRoot}},
		{parityCase: parityCase{name: "full73", root: root, args: []string{"--no-docker", "--full"},
			setup: setupMsixRoot}},
	}
}

// windowsInventoryCases are the stat-only runs: no -o and no staging, one
// JSON line per agent on stdout. smoke.ps1's unreadable-home inventory run
// is POSIX only; its place is taken by the -NoDocker run of the case-variant evidence check.
// inventory-fold32 runs over fold32's fixture, whose directory the current
// user may not list, with the log on stderr, so the read failed: line both
// collectors log for it is compared.
func windowsInventoryCases() []windowsCase {
	cases := []parityCase{
		{name: "inventory", inventory: true, cwd: "{OUT}", args: []string{"-q"}, setup: setupWindowsInventory},
		{name: "inventory-o", inventory: true, stderrLog: true, args: []string{"-o", `{OUT}\inv-o`},
			setup: setupWindowsInventory},
		{name: "inventory-full", inventory: true, args: []string{"-q", "--full"}, setup: setupWindowsInventory},
		{name: "inventory-u", inventory: true, args: []string{"-q", "-u", "bob", "--no-docker"},
			setup: setupWindowsInventory},
		{name: "inventory-nodocker", inventory: true, args: []string{"-q", "--no-docker"}, setup: setupWindowsInventory},
		{name: "inventory-fold32", inventory: true, stderrLog: true, args: []string{"--no-docker"},
			setup: setupWindowsUnreadableCaseVariant},
	}
	var wrapped []windowsCase
	for _, testCase := range cases {
		wrapped = append(wrapped, windowsCase{parityCase: testCase})
	}
	return wrapped
}
