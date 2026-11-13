//go:build parity

package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parityCase is one argument set of the smoke test's `run` and `inventory_run`
// lines. The strings of args, root, cwd, env and outDir may hold {ROOT},
// {WORK}, {ALICE}, {BOB} and {OUT}: the image root, the image's work
// directory, two homes in the image and the run's output directory.
type parityCase struct {
	name      string
	args      []string
	root      string
	cwd       string
	env       []string
	outDir    string
	inventory bool
	stderrLog bool
	setup     func(t *testing.T, img image)
	after     func(t *testing.T, img image)
	skip      string
}

// skipTar names why the tar fallback cases are not run: they patch the
// collector or the PATH, and the binary has no archiver fallbacks.
const skipTar = "needs a patched copy of the collector with a tar shim first on PATH, to force the " +
	"tar | gzip or plain tar fallback; the binary has no archiver fallbacks"

// parityCases lists every case, in the order of the smoke test.
func parityCases() []parityCase {
	cases := []parityCase{
		{name: "default"},
		{name: "root486", root: "{WORK}/r[1]", args: []string{"--no-docker", "--no-projects"}, setup: setupPatternRoot},
		{name: "linkproj497", args: []string{"-p", "{ROOT}/home/dot/.claude/proj"}, after: checkLinkedSourceIntact},
		{name: "nosecrets", args: []string{"--no-secrets"}},
		{name: "envscan", args: []string{"--env-scan-home"}},
		{name: "envscanonce", args: []string{"--env-scan-home"}, setup: setupEnvScanOnce},
		{name: "envscanout", args: []string{"--env-scan-home"}, outDir: "{ALICE}"},
		{name: "envscanglob", args: []string{"--env-scan-home"}, outDir: "{BOB}/out [1]"},
		{name: "envscanroot", args: []string{"--env-scan-home", "-p", "{ROOT}/home", "-p", "{ROOT}/opt/daemonproj"}},
		{name: "full", args: []string{"--full"}},
		{name: "users", args: []string{"-u", "bob"}},
		{name: "nodocker", args: []string{"--no-docker"}},
		{name: "cwd105", cwd: "{WORK}/cwd105", args: []string{"--no-docker", "--no-projects"}},
		{name: "claims106", args: []string{"--no-projects"},
			env: []string{"NESTED_CLAIMS={ROOT}/var/lib/docker/volumes/a0_usr/_data/chats"}},
		{name: "dockerperm", setup: setupUnreadableVolume},
		{name: "desktop", setup: setupDockerDesktop},
		{name: "fwvolumes", setup: setupFrameworkVolumes},
		{name: "fwvolumesnosecrets", args: []string{"--no-secrets"}, setup: setupFrameworkVolumes},
		{name: "tgz31", skip: skipTar},
		{name: "tar31", skip: skipTar},
		{name: "tarfail40", skip: skipTar},
		{name: "projargs", cwd: "{ROOT}/srv", args: []string{"-p", "relproj", "-p", "/nonexistent-cac/proj"},
			setup: setupRelativeProject},
		{name: "readperm", setup: setupUnreadableHome},
		{name: "cwd109", cwd: "{WORK}/cwd109", args: []string{"--no-docker", "--no-projects"}},
		{name: "cdpath110", cwd: "{WORK}", root: "root", env: []string{"CDPATH={WORK}/cdp110"},
			args: []string{"--no-docker", "--no-projects"}},
		{name: "fold32", setup: setupUnreadableCaseVariant},
		{name: "batch71", root: "{WORK}/root71", args: []string{"--no-docker"}, setup: setupBatchRoot},
		{name: "nobatch71", skip: "CAC_NO_BATCH selects the sh collector's per-file stage path, " +
			"which the binary does not have"},
		{name: "shasum71", skip: "needs a patched copy of the collector whose sha256sum probe fails, " +
			"to force shasum; the binary hashes in Go"},
		{name: "nostat480", skip: "needs a stat shim on PATH and a patched copy of the collector; " +
			"the binary has no stat fallback"},
	}
	return append(append(cases, packageCases()...), inventoryCases()...)
}

// packageCases are the three runs over the Claude Desktop MSIX image.
func packageCases() []parityCase {
	return []parityCase{
		{name: "store73", root: "{WORK}/root73", args: []string{"--no-docker"}, setup: setupMsixRoot},
		{name: "nosecrets73", root: "{WORK}/root73", args: []string{"--no-docker", "--no-secrets"},
			setup: setupMsixRoot},
		{name: "full73", root: "{WORK}/root73", args: []string{"--no-docker", "--full"}, setup: setupMsixRoot},
	}
}

// inventoryCases are the stat-only runs: no -o and no staging, one JSON line
// per agent on stdout.
func inventoryCases() []parityCase {
	return []parityCase{
		{name: "inventory", inventory: true, cwd: "{OUT}", args: []string{"-q"}, setup: setupInventory},
		{name: "inventory-o", inventory: true, stderrLog: true, args: []string{"-o", "{OUT}/inv-o"},
			setup: setupInventory},
		{name: "inventory-full", inventory: true, args: []string{"-q", "--full"}, setup: setupInventory},
		{name: "inventory-u", inventory: true, args: []string{"-q", "-u", "bob", "--no-docker"},
			setup: setupInventory},
		{name: "inventory-perm", inventory: true, args: []string{"-q"}, setup: setupUnreadableInventoryHome},
	}
}

// expand returns text with the placeholders of a case replaced.
func expand(text string, img image, outDir string) string {
	return strings.NewReplacer("{ROOT}", img.root, "{WORK}", img.work,
		"{ALICE}", img.root+"/home/alice", "{BOB}", img.root+"/Users/bob", "{OUT}", outDir).Replace(text)
}

// invocation returns the run of the case that writes into outDir.
func (c parityCase) invocation(img image, outDir string) invocation {
	root := c.root
	if root == "" {
		root = "{ROOT}"
	}
	args := []string{"-r", root}
	if c.inventory {
		args = append(args, "--inventory")
	} else {
		args = append(args, "-o", "{OUT}", "-q", "--max-file-size", "1")
	}
	run := invocation{outDir: outDir, cwd: expand(c.cwd, img, outDir)}
	for _, arg := range append(args, c.args...) {
		run.args = append(run.args, expand(arg, img, outDir))
	}
	for _, setting := range c.env {
		run.env = append(run.env, expand(setting, img, outDir))
	}
	return run
}

// outputDir returns the case's own output directory, creating it as the
// collector would when the case names one, or a new temporary directory.
func (c parityCase) outputDir(t *testing.T, img image) (dir string, shared bool) {
	t.Helper()
	if c.outDir != "" {
		dir = expand(c.outDir, img, "")
		if missing := firstMissing(dir); missing != "" {
			fixture{t}.created(missing)
		}
		t.Cleanup(func() { removeOutputs(dir) })
		return dir, true
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir, false
}

// requireOutput fails the case when the sh collector left nothing to compare,
// which would make every comparison pass on two empty runs.
func (c parityCase) requireOutput(t *testing.T, want collectorRun) {
	t.Helper()
	if want.stdout == "" {
		t.Fatalf("the sh collector wrote no standard output to compare (exit %d): %s", want.code, want.stderr)
	}
	if !c.inventory && (want.files[".manifest.jsonl"] == "" || len(want.members) == 0) {
		t.Fatalf("the sh collector wrote no manifest or archive to compare (exit %d): %s", want.code, want.stderr)
	}
}

// requireNoOutput fails when an inventory run wrote anything into dir.
func (c parityCase) requireNoOutput(t *testing.T, label, dir string) {
	t.Helper()
	if !c.inventory {
		return
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%s: the inventory run wrote %d entries into its output directory", label, len(entries))
	}
}

func TestCollectAgreesWithTheShCollector(t *testing.T) {
	repository := repositoryRoot(t)
	binary := buildBinary(t, repository)
	img := smokeImage(t, repository)
	script := filepath.Join(repository, "collectors/collect-agent-artifacts.sh")
	for _, testCase := range parityCases() {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.skip != "" {
				t.Skip(testCase.skip)
			}
			if testCase.setup != nil {
				testCase.setup(t, img)
			}
			testCase.compare(t, img, []string{parityShell(), script}, []string{binary, "collect"})
		})
	}
}

// compare runs the sh collector and the binary and compares what they wrote.
func (c parityCase) compare(t *testing.T, img image, shCollector, binary []string) {
	t.Helper()
	shDir, shared := c.outputDir(t, img)
	want := execute(t, shCollector, c.invocation(img, shDir))
	c.requireNoOutput(t, "sh collector", shDir)
	c.requireOutput(t, want)
	goDir := shDir
	if shared {
		removeOutputs(shDir)
	} else {
		goDir, _ = c.outputDir(t, img)
	}
	got := execute(t, binary, c.invocation(img, goDir))
	c.requireNoOutput(t, "binary", goDir)
	compareRuns(t, normalise(want, false, c.stderrLog), normalise(got, true, c.stderrLog))
	if c.after != nil {
		c.after(t, img)
	}
}
