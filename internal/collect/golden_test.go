package collect_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// goldenCase is one run of the binary's collect over the fixture image. The
// strings of args, root, cwd, env and outDir may hold the placeholders
// placeholders replaces. setup builds what the case adds to the image (an
// imagetest builder) and after checks the image once the run is done.
type goldenCase struct {
	name      string
	args      []string
	root      string
	cwd       string
	env       []string
	outDir    string
	inventory bool
	stderrLog bool
	setup     func(imagetest.Image, testing.TB)
	after     func(imagetest.Image, testing.TB)
	// parallel marks a case that runs beside the other parallel cases: it
	// has no setup, writes only into its own temporary output directory and
	// leaves the image as it was.
	parallel bool
	// fails marks a case the binary must refuse: it exits non-zero before
	// writing anything into its output directory.
	fails bool
}

// goldenRun is what every case of one test run shares: the binary and the
// image it collects.
type goldenRun struct {
	binary string
	image  imagetest.Image
}

// invocation returns the run of the case that writes into outDir: -r on the
// case's root (the image root unless it names another), --inventory for an
// inventory run and -o, -q and a 1 MiB size limit for any other, then the
// case's arguments.
func (c goldenCase) invocation(img imagetest.Image, outDir string) invocation {
	expand := placeholders(img, outDir)
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
	run := invocation{outDir: outDir, cwd: expand.Replace(c.cwd)}
	for _, arg := range append(args, c.args...) {
		run.args = append(run.args, expand.Replace(arg))
	}
	for _, setting := range c.env {
		run.env = append(run.env, expand.Replace(setting))
	}
	return run
}

// overMainImage reports whether the case collects the main image's root,
// so its files may be kept as deltas from the default case's.
func (c goldenCase) overMainImage(img imagetest.Image) bool {
	root := placeholders(img, "").Replace(c.root)
	if root == "" {
		return true
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(placeholders(img, "").Replace(c.cwd), root)
	}
	return root == img.Root
}

// outputDir returns the case's output directory: the one the case names,
// whose outputs are removed and whose parent's time is put back when the
// test ends, or a new temporary one.
func (c goldenCase) outputDir(test *testing.T, img imagetest.Image) string {
	test.Helper()
	if c.outDir == "" {
		dir, err := filepath.EvalSymlinks(test.TempDir())
		if err != nil {
			test.Fatal(err)
		}
		return dir
	}
	dir := placeholders(img, "").Replace(c.outDir)
	keepTime(test, filepath.Dir(dir))
	if _, err := os.Lstat(dir); err == nil {
		keepTime(test, dir)
		test.Cleanup(func() { removeOutputs(test, dir) })
	} else {
		test.Cleanup(func() { removeAll(test, dir) })
	}
	return dir
}

// keepTime puts back the modification time path has now when the test
// ends, after what the test's later cleanups remove from it.
func keepTime(test *testing.T, path string) {
	test.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	test.Cleanup(func() {
		if err := os.Chtimes(path, time.Time{}, info.ModTime()); err != nil {
			test.Errorf("putting back the time of %s: %v", path, err)
		}
	})
}

// removeOutputs deletes what a run left in an output directory that is
// part of the image.
func removeOutputs(test *testing.T, dir string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "_agent-artifacts.") || strings.HasPrefix(name, ".stage-") {
			removeAll(test, filepath.Join(dir, name))
		}
	}
}

// removeAll deletes path and everything below it.
func removeAll(test *testing.T, path string) {
	if err := os.RemoveAll(path); err != nil {
		test.Errorf("removing %s: %v", path, err)
	}
}

// execute builds the case's fixtures, runs it and checks the guards that
// are not a comparison, and returns its golden files.
func (c goldenCase) execute(test *testing.T, shared goldenRun) map[string]string {
	test.Helper()
	img := shared.image
	if c.setup != nil {
		c.setup(img, test)
	}
	if !c.parallel {
		// A case that changes the image runs alone, after the parallel
		// ones; on Windows its directory listings must show the times the
		// entries hold before it runs.
		imagetest.Settle(test, img.Work)
	}
	dir := c.outputDir(test, img)
	run := execute(test, shared.binary, c.invocation(img, dir))
	c.checkGuards(test, run, dir)
	if c.after != nil {
		c.after(img, test)
	}
	return newNormaliser(test, img.Work, dir, run.name).normalise(run, c.stderrLog)
}

// checkGuards fails the case on what holds of every run whatever its
// output: a refused run exits non-zero and writes nothing, an inventory run
// writes nothing into its output directory, and every archive path of the
// manifest names an archive member byte for byte.
func (c goldenCase) checkGuards(test *testing.T, run collectorRun, dir string) {
	test.Helper()
	if c.fails && run.code == 0 {
		test.Errorf("the run exited 0 where the case expects a refusal: %s", run.stdout)
	}
	if c.fails || c.inventory {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			test.Errorf("the run left %d entries in its output directory %s, the first %s", len(entries),
				dir, entries[0].Name())
		}
		return
	}
	if run.name == "" {
		test.Fatalf("the run wrote no output (exit %d): %s", run.code, run.stderr)
	}
	for _, archived := range archivePathsNamingNoMember(test, run) {
		test.Errorf("a manifest row names %q, which the archive holds under no such name", archived)
	}
}

func TestCollectWritesItsGoldenOutput(t *testing.T) {
	shared := newGoldenRun(t)
	cases := goldenCases()
	checkNoStaleCases(t, cases)
	var defaults map[string]string
	t.Run("default", func(t *testing.T) {
		defaults = compareGoldenCase(t, cases[0], shared, nil)
	})
	if defaults == nil {
		t.Fatal("the default case has no expected files, which every delta needs")
	}
	t.Run("shared image", func(t *testing.T) {
		for _, testCase := range cases[1:] {
			if testCase.parallel {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					compareGoldenCase(t, testCase, shared, defaults)
				})
			}
		}
	})
	for _, testCase := range cases[1:] {
		if !testCase.parallel {
			t.Run(testCase.name, func(t *testing.T) { compareGoldenCase(t, testCase, shared, defaults) })
		}
	}
}

// compareGoldenCase runs one case and compares its golden files with the
// expected ones, or rewrites those when updating, and returns the expected
// files with every delta applied.
func compareGoldenCase(test *testing.T, testCase goldenCase, shared goldenRun,
	defaults map[string]string) map[string]string {
	test.Helper()
	got := testCase.execute(test, shared)
	overMain := defaults != nil && testCase.overMainImage(shared.image)
	if updating() {
		storeCase(test, testCase.name, storedForm(got, defaults, overMain))
	}
	stored, found := expectedCase(test, testCase.name)
	if !found {
		test.Fatalf("no expected files for the case %s on %s: run the test with %s=1 on this platform, "+
			"read the files it writes and commit them", testCase.name, runtime.GOOS, updateGoldenVariable)
	}
	want := expandCase(test, stored, defaults)
	compareCase(test, want, got)
	return want
}

// checkNoStaleCases fails the test on an expected-files directory that
// names no case of this platform, and removes it when updating.
func checkNoStaleCases(test *testing.T, cases []goldenCase) {
	test.Helper()
	names := make([]string, 0, len(cases))
	for _, testCase := range cases {
		names = append(names, testCase.name)
	}
	if len(slices.Compact(slices.Sorted(slices.Values(names)))) != len(names) {
		test.Fatal("two golden cases share a name")
	}
	for _, platform := range []string{platformDirectory(), otherUnixPlatform(), sharedDirectory} {
		if platform == "" || (platform == sharedDirectory && otherUnixPlatform() == "") {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(goldenRoot, platform))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			test.Fatal(err)
		}
		for _, entry := range entries {
			if slices.Contains(names, entry.Name()) {
				continue
			}
			if updating() {
				removeCase(test, caseDirectory(platform, entry.Name()))
			} else {
				test.Errorf("%s names no golden case; remove it or add the case", caseDirectory(platform, entry.Name()))
			}
		}
	}
}

// platformDirectory names the platform's own directory of expected files.
func platformDirectory() string {
	return runtime.GOOS
}

// otherUnixPlatform names the other platform that shares unix/ with this
// one, or "" where none does.
func otherUnixPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "linux"
	case "linux":
		return "darwin"
	}
	return ""
}
