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
// directory, two homes in the image and the run's output directory. expect,
// when set, rewrites the reference collector's run into the run the binary writes
// by design, before both are normalised: one of the parity exceptions listed
// at the top of harness_unix_test.go.
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
	expect    func(t *testing.T, img image, want *collectorRun)
	skip      string
	// parallel marks a case that runs beside the other parallel cases, in the
	// harness's "shared image" group, before the serial ones. It has no
	// setup or after, writes only to its own temporary output directory and
	// leaves the image unchanged.
	parallel bool
}

// expand returns text with the placeholders of a case replaced.
func expand(text string, img image, outDir string) string {
	return strings.NewReplacer("{ROOT}", img.root, "{WORK}", img.work,
		"{ALICE}", img.alice, "{BOB}", img.bob, "{OUT}", outDir).Replace(text)
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

// requireOutput fails the case when the reference collector (the sh one on
// Unix, the PowerShell one on Windows) left nothing to compare,
// which would make every comparison pass on two empty runs.
func (c parityCase) requireOutput(t *testing.T, want collectorRun) {
	t.Helper()
	if want.stdout == "" {
		t.Fatalf("the reference collector wrote no standard output to compare (exit %d): %s", want.code, want.stderr)
	}
	if !c.inventory && (want.files[".manifest.jsonl"] == "" || len(want.members) == 0) {
		t.Fatalf("the reference collector wrote no manifest or archive to compare (exit %d): %s", want.code, want.stderr)
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

// oracle runs the reference collector with the invocation and reads what it
// wrote.
type oracle func(t *testing.T, run invocation) collectorRun

// compare runs the reference collector and the binary and compares what they
// wrote, after the normaliser has dropped what differs by design.
func (c parityCase) compare(t *testing.T, img image, reference oracle, binary []string, normalise normaliser) {
	t.Helper()
	referenceDir, shared := c.outputDir(t, img)
	want := reference(t, c.invocation(img, referenceDir))
	c.requireNoOutput(t, "reference collector", referenceDir)
	c.requireOutput(t, want)
	if c.expect != nil {
		c.expect(t, img, &want)
	}
	goDir := referenceDir
	if shared {
		removeOutputs(referenceDir)
	} else {
		goDir, _ = c.outputDir(t, img)
	}
	got := execute(t, binary, c.invocation(img, goDir))
	c.requireNoOutput(t, "binary", goDir)
	want.foldsCase = goDir != "" && foldsCase(t, goDir)
	got.foldsCase = want.foldsCase
	compareRuns(t, normalise(want, false, c.stderrLog), normalise(got, true, c.stderrLog))
	if c.after != nil {
		c.after(t, img)
	}
}

// foldsCase reports whether dir, an output directory, takes two names that
// differ only in letter case for one, by making a lower-case probe file in
// it, looking it up in upper case and removing it.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "case-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(dir, "CASE-PROBE"))
	return err == nil
}
