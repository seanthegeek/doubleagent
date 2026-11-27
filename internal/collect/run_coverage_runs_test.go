package collect_test

import (
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// manifestRow is the part of a manifest row the coverage rules read; rel is
// the path below the row's base, the home, project or volume it was
// collected from, with "/" between names.
type manifestRow struct {
	agent  string
	path   string
	rel    string
	home   string
	status string
	secret bool
}

// relocationHint is one hint of collection.json: the file it was read
// from and the variable and agent it names.
type relocationHint struct {
	source, variable, agent string
}

// coverageRun is what one run gave: its option (none, --no-secrets or
// --full), its root, its rows, archive member names and hints, and the
// homes and projects it collected.
type coverageRun struct {
	option   string
	root     string
	rows     []manifestRow
	members  []string
	hints    []relocationHint
	homes    []string
	projects []string
}

// coverageImage is one image the runs collect: the root a run names, or ""
// for the main image's, and the builder that makes it.
type coverageImage struct {
	root  string
	build func(imagetest.Image, testing.TB)
}

// coverageEvidence is what the coverage runs saw: the runs, the image's
// entries once every builder had run, the entries the builders added, and
// the image.
type coverageEvidence struct {
	runs     []coverageRun
	entries  []imageEntry
	builders []imageEntry
	image    imagetest.Image
}

// collectForCoverage runs each of the three options over each image and
// returns what they saw. It fails the test when a run changes the image.
func collectForCoverage(t *testing.T) coverageEvidence {
	t.Helper()
	shared := newGoldenRun(t)
	built := imageEntries(t, shared.image.Work)
	images := []coverageImage{{}, {build: imagetest.Image.FrameworkVolumes},
		{root: "{WORK}/root73", build: func(img imagetest.Image, test testing.TB) { img.MsixRoot(test) }},
		{root: "{WORK}/root584", build: func(img imagetest.Image, test testing.TB) { img.CaseVariantRoot(test) }}}
	evidence := coverageEvidence{image: shared.image}
	for _, image := range images {
		if image.build != nil {
			image.build(shared.image, t)
		}
		before := imageSnapshot(t, shared.image.Work)
		for _, option := range []string{"", "--no-secrets", "--full"} {
			testCase := goldenCase{root: filepath.FromSlash(image.root), args: optionArgs(option)}
			if image.root != "" {
				testCase.args = append(testCase.args, "--no-docker")
			}
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			run := readCoverageRun(t, execute(t, shared.binary, testCase.invocation(shared.image, dir)))
			run.option = option
			evidence.runs = append(evidence.runs, run)
		}
		checkImageUnchanged(t, before, imageSnapshot(t, shared.image.Work))
	}
	evidence.entries = imageEntries(t, shared.image.Work)
	before := map[string]bool{}
	for _, entry := range built {
		before[entry.path] = true
	}
	for _, entry := range evidence.entries {
		if !before[entry.path] {
			evidence.builders = append(evidence.builders, entry)
		}
	}
	return evidence
}

// optionArgs returns the arguments of an option, none for "".
func optionArgs(option string) []string {
	if option == "" {
		return nil
	}
	return []string{option}
}

// readCoverageRun reads the rows, members, hints, homes and projects of a
// run.
func readCoverageRun(t *testing.T, run collectorRun) coverageRun {
	t.Helper()
	if run.code != 0 || run.name == "" {
		t.Fatalf("a coverage run exited %d and wrote %q: %s", run.code, run.name, run.stderr)
	}
	var result coverageRun
	for _, line := range splitLines(run.files[".manifest.jsonl"]) {
		row, err := jsonvalue.DecodeString(line)
		if err != nil {
			t.Fatalf("manifest row is not JSON: %v", err)
		}
		home, path := row.Get("home").StringValue(), row.Get("path").StringValue()
		secret, _ := row.Get("secret").Bool()
		result.rows = append(result.rows, manifestRow{agent: row.Get("agent").StringValue(), path: path,
			rel: relativeTo(home, path), home: home, status: row.Get("status").StringValue(), secret: secret})
	}
	for _, member := range run.members {
		result.members = append(result.members, member.name)
	}
	summary, err := jsonvalue.DecodeString(run.files[".collection.json"])
	if err != nil {
		t.Fatalf("collection.json is not JSON: %v", err)
	}
	result.root = summary.Get("root").StringValue()
	for _, hint := range summary.Get("relocation").Get("hints").Items() {
		result.hints = append(result.hints, relocationHint{source: hint.Get("source").StringValue(),
			variable: hint.Get("variable").StringValue(), agent: hint.Get("agent").StringValue()})
	}
	for _, home := range summary.Get("homes").Items() {
		result.homes = append(result.homes, home.StringValue())
	}
	for _, project := range summary.Get("projects").Items() {
		result.projects = append(result.projects, project.StringValue())
	}
	return result
}

// relativeTo returns path below base with "/" between names, or "" when
// path does not lie below base.
func relativeTo(base, path string) string {
	rest, found := strings.CutPrefix(path, base+string(filepath.Separator))
	if !found || base == "" {
		return ""
	}
	return filepath.ToSlash(rest)
}

// imageEntry is one entry of an image below its work directory: its path
// with "/" between names, a file's content or a link's target, and whether
// it is a link.
type imageEntry struct {
	path, text string
	link       bool
}

// imageEntries returns the entries of the image under work as it is now.
func imageEntries(t *testing.T, work string) []imageEntry {
	t.Helper()
	entries, err := imagetest.Read(work)
	if err != nil {
		t.Fatal(err)
	}
	return workEntries(entries)
}

// manifestEntries returns the entries of the image manifest at name.
func manifestEntries(t *testing.T, name string) []imageEntry {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries, err := imagetest.Parse(file)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return workEntries(entries)
}

// workEntries returns the paths, contents and link kinds of image entries.
func workEntries(entries []imagetest.Entry) []imageEntry {
	paths := make([]imageEntry, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, imageEntry{path: entry.Path, text: entry.Text,
			link: entry.Kind == imagetest.KindLink || entry.Kind == imagetest.KindDirLink ||
				entry.Kind == imagetest.KindJunction})
	}
	return paths
}

// runRoots returns the roots the runs named, below the work directory.
func runRoots(runs []coverageRun, work string) []string {
	var roots []string
	for _, run := range runs {
		roots = append(roots, relativeTo(work, run.root))
	}
	slices.Sort(roots)
	return slices.Compact(roots)
}

// runBases returns the bases the runs collected from, below the work
// directory: their homes and projects and the volume data directories of
// their rows.
func runBases(runs []coverageRun, work string) []string {
	var bases []string
	for _, run := range runs {
		for _, path := range append(slices.Clone(run.homes), run.projects...) {
			bases = append(bases, relativeTo(work, path))
		}
		for _, row := range run.rows {
			bases = append(bases, relativeTo(work, row.home))
		}
	}
	slices.Sort(bases)
	return slices.DeleteFunc(slices.Compact(bases), func(base string) bool { return base == "" })
}

// caseFold returns the fold the coverage rules compare names after: ASCII
// lower case where the image's file system takes two names differing only in
// case for one (macOS and Windows, where the image holds one spelling of
// such a pair, and a row the spelling its walk met), else none.
func caseFold(t *testing.T, work string) func(string) string {
	t.Helper()
	probe := filepath.Join(work, "case-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	if _, err := os.Lstat(filepath.Join(work, "CASE-PROBE")); err != nil {
		return func(text string) string { return text }
	}
	return textlines.LowerASCII
}
