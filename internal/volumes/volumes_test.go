package volumes

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// memoryLog keeps log lines in memory.
type memoryLog struct{ lines []string }

// Line keeps the log line.
func (log *memoryLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// inventoryRecords keeps what an inventory walk reports.
type inventoryRecords struct{ matches, files []string }

// Match keeps the agent and glob of a match.
func (records *inventoryRecords) Match(agent, glob string) {
	records.matches = append(records.matches, agent+"|"+glob)
}

// File keeps the agent of a file the walk reports.
func (records *inventoryRecords) File(agent string, _, _ int64) {
	records.files = append(records.files, agent)
}

// setup is a collector over a fresh image root.
type setup struct {
	root      string
	collector *walk.Collector
	rows      *bytes.Buffer
	log       *memoryLog
	records   *inventoryRecords
}

// newSetup returns a collector over a fresh image root, with an inventory
// walk when inventory is set.
func newSetup(t *testing.T, inventory bool) setup {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = syspath.FromNative(root)
	stageDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	rows, log, records := &bytes.Buffer{}, &memoryLog{}, &inventoryRecords{}
	collector := &walk.Collector{
		Tables: tables, Excludes: tables.Excludes, SecretGlobs: tables.SecretGlobs,
		Log: log, Manifest: manifest.NewWriter(rows), Root: root,
		Stager:    stage.Stager{Dir: stageDir, Tree: stagingTree(t, stageDir), Root: root, Places: stage.NewPlaces()},
		Inventory: inventory, Records: records,
	}
	return setup{root, collector, rows, log, records}
}

// bases are the image bases of the fixture's root.
func (fixture setup) bases() Bases {
	return Bases{System: fixture.root, ProgramData: fixture.root + "/ProgramData"}
}

// write creates a file with its parents under the root.
func (fixture setup) write(t *testing.T, relative, content string) {
	t.Helper()
	full := filepath.Join(fixture.root, relative)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkdir makes the directory at relative under the image root, with its
// parents.
func (fixture setup) mkdir(t *testing.T, relative string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(fixture.root, relative), 0o755); err != nil {
		t.Fatal(err)
	}
}

// volumeCase is one fixture and what collecting it should report.
type volumeCase struct {
	name    string
	build   func(t *testing.T, fixture setup)
	want    Counts
	rows    int
	notes   int
	logPart string
}

// volumeCases is the table of TestCollectVolumes.
func volumeCases() []volumeCase {
	return append(findingCases(), linkCases()...)
}

// findingCases are the volumes that are found, matched or not.
func findingCases() []volumeCase {
	const volumes = "var/lib/docker/volumes/"
	return []volumeCase{
		{"a matched volume is collected whole", func(t *testing.T, fixture setup) {
			fixture.write(t, volumes+"a0_usr/_data/x/file", "hi")
		}, Counts{Seen: true, Found: 1, Collected: 1}, 1, 0, "  [agent-zero] "},
		{"an unmatched volume gets one dir row", func(t *testing.T, fixture setup) {
			fixture.write(t, volumes+"postgres/_data/PG_VERSION", "16")
		}, Counts{Seen: true, Found: 1}, 1, 0, ""},
		{"a volume without _data is found but skipped", func(t *testing.T, fixture setup) {
			fixture.mkdir(t, volumes+"ollama")
		}, Counts{Seen: true, Found: 1}, 0, 0, "has no _data directory, skipped"},
	}
}

// linkCases are symlinks, rootless volumes and Docker Desktop data.
func linkCases() []volumeCase {
	const volumes = "var/lib/docker/volumes/"
	return []volumeCase{
		{"a symlinked volume is not counted", func(t *testing.T, fixture setup) {
			fixture.mkdir(t, "elsewhere/_data")
			fixture.mkdir(t, "var/lib/docker/volumes")
			err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
				filepath.Join(fixture.root, volumes, "ollama"))
			if symlinkNeedsPrivilege(err) {
				t.Skipf("creating a symbolic link needs the symbolic-link privilege: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
		}, Counts{Seen: true}, 0, 0, ""},
		{"a symlinked volumes directory is noted and not followed", func(t *testing.T, fixture setup) {
			fixture.mkdir(t, "elsewhere/ollama/_data")
			fixture.mkdir(t, "var/lib/docker")
			err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
				filepath.Join(fixture.root, "var/lib/docker/volumes"))
			if symlinkNeedsPrivilege(err) {
				t.Skipf("creating a symbolic link needs the symbolic-link privilege: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
		}, Counts{Seen: true}, 0, 1, "is a symlink and was not followed"},
		{"a rootless volume of a home is found", func(t *testing.T, fixture setup) {
			fixture.write(t, "home/alice/.local/share/docker/volumes/n8n_data/_data/c", "x")
		}, Counts{Seen: true, Found: 1, Collected: 1}, 1, 0, ""},
		{"docker desktop data is noted", func(t *testing.T, fixture setup) {
			fixture.mkdir(t, "home/alice/Library/Containers/com.docker.docker")
		}, Counts{Seen: true, Desktop: true}, 0, 1, "NOTE: docker: Docker Desktop data"},
		{"nothing found leaves the counts empty", func(t *testing.T, fixture setup) {}, Counts{}, 0, 0, ""},
	}
}

func TestCollectVolumes(t *testing.T) {
	for _, test := range volumeCases() {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSetup(t, false)
			test.build(t, fixture)
			homes := []users.Home{{User: "alice", Path: fixture.root + "/home/alice"}}
			var notes []string
			got, err := Collect(context.Background(), fixture.collector, fixture.bases(), homes, &notes)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("counts = %+v, want %+v", got, test.want)
			}
			if lines := strings.Count(fixture.rows.String(), "\n"); lines < test.rows {
				t.Errorf("rows = %d, want at least %d:\n%s", lines, test.rows, fixture.rows)
			}
			if len(notes) != test.notes {
				t.Errorf("notes = %q, want %d", notes, test.notes)
			}
			if !strings.Contains(strings.Join(fixture.log.lines, "\n"), test.logPart) {
				t.Errorf("log %q lacks %q", fixture.log.lines, test.logPart)
			}
		})
	}
}

func TestInventoryRecordsMatchedVolumesOnly(t *testing.T) {
	fixture := newSetup(t, true)
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/f", "hi")
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/f", "hi")
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.bases(), nil, &notes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found != 2 || got.Collected != 1 || fixture.rows.Len() != 0 {
		t.Errorf("counts = %+v, rows = %q", got, fixture.rows)
	}
	if strings.Join(fixture.records.matches, ",") != "agent-zero|*a0_usr" ||
		strings.Join(fixture.records.files, ",") != "agent-zero" {
		t.Errorf("records = %+v", fixture.records)
	}
}

// embeddedDockerTable returns the DOCKER_VOLUMES table of the embedded catalog.
func embeddedDockerTable(t *testing.T) []catalog.Entry {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return tables.DockerVolumes
}

// overlappingNames returns, for every two lines of table, a name both can
// match: the first glob with its first `*` replaced by the second's literal
// text and its other `*` dropped.
func overlappingNames(table []catalog.Entry) []string {
	var names []string
	for _, outer := range table {
		for _, inner := range table {
			literal := strings.ReplaceAll(inner.Glob, "*", "")
			names = append(names, strings.ReplaceAll(strings.Replace(outer.Glob, "*", literal, 1), "*", ""))
		}
	}
	return names
}

// tiedLines returns how many lines of table match name and are as specific
// as the line volumeRule picks; above one, table order decides.
func tiedLines(table []catalog.Entry, name string) int {
	best, found := volumeRule(table, name)
	if !found {
		return 0
	}
	bestStars, bestLiterals := globSpecificity(best.Glob)
	count := 0
	for _, entry := range table {
		stars, literals := globSpecificity(entry.Glob)
		if stars == bestStars && literals == bestLiterals && glob.MatchPath(entry.Glob, name) {
			count++
		}
	}
	return count
}

// The reversed and sorted tables stand for any order: a name whose most
// specific line is unique goes to the same agent in each.
func TestVolumeRuleDoesNotDependOnTableOrder(t *testing.T) {
	table := embeddedDockerTable(t)
	reversed := slices.Clone(table)
	slices.Reverse(reversed)
	sorted := slices.Clone(table)
	slices.SortStableFunc(sorted, func(left, right catalog.Entry) int {
		return cmp.Or(cmp.Compare(left.Agent, right.Agent), cmp.Compare(left.Glob, right.Glob))
	})
	for _, name := range append(overlappingNames(table), "local-deep-research_ollama_data") {
		if tiedLines(table, name) > 1 {
			continue
		}
		want, wantFound := volumeRule(table, name)
		for order, other := range map[string][]catalog.Entry{"reversed": reversed, "sorted": sorted} {
			got, found := volumeRule(other, name)
			if found != wantFound || got != want {
				t.Errorf("%s: %s table gives %+v, table order gives %+v", name, order, got, want)
			}
		}
	}
}

func TestLocalDeepResearchOllamaVolumeGoesToOllama(t *testing.T) {
	table := embeddedDockerTable(t)
	slices.Reverse(table)
	for _, name := range []string{"local-deep-research_ollama_data", "ollama_data", "ollama"} {
		got, found := volumeRule(table, name)
		if !found || got.Agent != "ollama" {
			t.Errorf("%s: rule = %+v, %v", name, got, found)
		}
	}
	got, found := volumeRule(table, "local-deep-research_ldr_data")
	if !found || got.Agent != "local-deep-research" {
		t.Errorf("local-deep-research_ldr_data: rule = %+v, %v", got, found)
	}
}

func TestVolumeRuleMostSpecificLineWins(t *testing.T) {
	for _, testCase := range []struct {
		broad, narrow, name string
	}{
		{"*data*", "*_data", "my_data"},       // fewer `*`
		{"*data", "*_data", "my_data"},        // more literal characters
		{"ollam?", "ollama", "ollama"},        // `?` is not a literal
		{"[a-z]y_data", "my_data", "my_data"}, // nor is a bracket list
		{"*[*]*", "[*]x*", "*x"},              // a `*` in a bracket list is not a wildcard
	} {
		table := []catalog.Entry{{Agent: "broad", Glob: testCase.broad}, {Agent: "narrow", Glob: testCase.narrow}}
		for range 2 {
			got, found := volumeRule(table, testCase.name)
			if !found || got.Agent != "narrow" {
				t.Errorf("%v: rule = %+v, %v", table, got, found)
			}
			slices.Reverse(table)
		}
	}
}

func TestEquallySpecificLinesFallToTableOrder(t *testing.T) {
	table := []catalog.Entry{{Agent: "first", Glob: "*data"}, {Agent: "second", Glob: "my_d*"}}
	for range 2 {
		got, found := volumeRule(table, "my_data")
		if !found || got != table[0] {
			t.Errorf("%v: rule = %+v, %v", table, got, found)
		}
		slices.Reverse(table)
	}
	if _, found := volumeRule(table, "other"); found {
		t.Error("an unmatched name found a rule")
	}
}

func TestGlobSpecificity(t *testing.T) {
	for _, testCase := range []struct {
		pattern         string
		stars, literals int
	}{
		{"ollama", 0, 6},
		{"*a0_usr", 1, 6},
		{"a0-launcher-*-usr", 1, 16},
		{"*ollama*", 2, 6},
		{"a?c", 0, 2},
		{"[a-z]x", 0, 1},
		{"[!.]x*", 1, 1},
		{"[]a]b", 0, 1},
		{"[!]*]b", 0, 1},
		{"[*?]*", 1, 0},
	} {
		stars, literals := globSpecificity(testCase.pattern)
		if stars != testCase.stars || literals != testCase.literals {
			t.Errorf("%s: %d stars, %d literals, want %d, %d", testCase.pattern, stars, literals,
				testCase.stars, testCase.literals)
		}
	}
}

func TestCancelledContextStopsCollection(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/ollama/_data/f", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var notes []string
	if _, err := Collect(ctx, fixture.collector, fixture.bases(), nil, &notes); err == nil {
		t.Error("a cancelled context gave no error")
	}
}

func TestUnmatchedVolumeRow(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	var notes []string
	if _, err := Collect(context.Background(), fixture.collector, fixture.bases(), nil, &notes); err != nil {
		t.Fatal(err)
	}
	data := syspath.ToNative(fixture.root + "/var/lib/docker/volumes/postgres/_data")
	var row map[string]any
	if err := json.Unmarshal(fixture.rows.Bytes(), &row); err != nil {
		t.Fatalf("row %s: %v", fixture.rows, err)
	}
	want := map[string]any{"user": "docker", "home": data, "agent": "", "path": data,
		"archive_path": "", "type": "dir", "mtime": 0.0, "atime": 0.0, "ctime": 0.0, "btime": 0.0,
		"uid": 0.0, "gid": 0.0, "mode": fsmeta.ZeroMeta().Mode, "sha256": "", "secret": false,
		"status": "skipped_unmatched_volume", "target": "", "error": ""}
	for name, value := range want {
		if row[name] != value {
			t.Errorf("%s = %#v, want %#v in %s", name, row[name], value, fixture.rows)
		}
	}
}

func TestBasesPlaceTheSystemVolumeDirectories(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "sys/var/lib/docker/volumes/postgres/_data/f", "x")
	fixture.write(t, "data/Docker/volumes/redis/_data/f", "x")
	fixture.mkdir(t, "data/DockerDesktop")
	fixture.write(t, "ProgramData/Docker/volumes/ignored/_data/f", "x")
	bases := Bases{System: fixture.root + "/sys", ProgramData: fixture.root + "/data"}
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, bases, nil, &notes)
	if err != nil {
		t.Fatal(err)
	}
	wantNote := "docker: Docker Desktop data at " + syspath.ToNative(fixture.root+"/data/DockerDesktop") +
		"; volumes inside its virtual machine disk are not collected"
	if got.Found != 2 || !got.Desktop || len(notes) != 1 || notes[0] != wantNote {
		t.Errorf("counts %+v, notes %q; want 2 volumes and the note %q", got, notes, wantNote)
	}
	wantLine := "Docker volumes in " + syspath.ToNative(fixture.root+"/data/Docker/volumes")
	if !strings.Contains(strings.Join(fixture.log.lines, "\n"), wantLine) {
		t.Errorf("log %q lacks %q", fixture.log.lines, wantLine)
	}
}

// stagingTree opens dir as the staging tree of a test, closed when it ends.
func stagingTree(t *testing.T, dir string) *stage.Tree {
	t.Helper()
	tree, err := stage.OpenTree(syspath.FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}
