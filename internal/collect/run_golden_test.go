package collect_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
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

// removeAll deletes path and everything below it.
func removeAll(test *testing.T, path string) {
	if err := os.RemoveAll(path); err != nil {
		test.Errorf("removing %s: %v", path, err)
	}
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

// add records the outputs of one case.
func (results *goldenResults) add(name string, got caseTexts) {
	results.lock.Lock()
	defer results.lock.Unlock()
	results.got[name] = got
}

// goldenResults collects the outputs of the cases an update runs, which
// the parallel ones add to at once.
type goldenResults struct {
	lock sync.Mutex
	got  map[string]caseTexts
}

// platformDirectory names the platform's own directory of expected files.
func platformDirectory() string {
	return runtime.GOOS
}

// storeAndCheck rewrites the expected files from the cases' outputs, then
// reads them back and fails a case whose expected files do not give its
// outputs.
func storeAndCheck(t *testing.T, got map[string]caseTexts, cases []goldenCase, img imagetest.Image) {
	t.Helper()
	overMain := map[string]bool{}
	for _, testCase := range cases {
		overMain[testCase.name] = testCase.name != "default" && testCase.overMainImage(img)
	}
	storeViews(t, got, overMain)
	stored := loadViews(t)
	for name, texts := range got {
		want, _ := stored.view(platformDirectory(), name)
		t.Run("stored "+name, func(t *testing.T) { compareCase(t, want, texts) })
	}
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

func TestCollectWritesItsGoldenOutput(t *testing.T) {
	shared := newGoldenRun(t)
	cases := goldenCases()
	checkNoStaleCases(t, cases)
	expected := loadViews(t)
	results := goldenResults{got: map[string]caseTexts{}}
	run := func(t *testing.T, testCase goldenCase) {
		got := testCase.execute(t, shared)
		if updating() {
			results.add(testCase.name, got)
			return
		}
		want, found := expected.view(platformDirectory(), testCase.name)
		if !found {
			t.Fatalf("no expected files for the case %s on %s: run the test with %s=1 on this platform, "+
				"read the files it writes and commit them", testCase.name, runtime.GOOS, updateGoldenVariable)
		}
		compareCase(t, want, got)
	}
	// go test splits a -run pattern at each slash, so a parallel case is
	// selected with -run 'TestCollectWritesItsGoldenOutput/shared_image/^<case>$'.
	t.Run("shared image", func(t *testing.T) {
		for _, testCase := range cases {
			if testCase.parallel {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					run(t, testCase)
				})
			}
		}
	})
	for _, testCase := range cases {
		if !testCase.parallel {
			t.Run(testCase.name, func(t *testing.T) { run(t, testCase) })
		}
	}
	if updating() {
		storeAndCheck(t, results.got, cases, shared.image)
	}
}
