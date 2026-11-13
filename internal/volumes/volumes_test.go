package volumes

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// memoryLog keeps log lines in memory.
type memoryLog struct{ lines []string }

func (log *memoryLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// inventoryRecords keeps what an inventory walk reports.
type inventoryRecords struct{ matches, files []string }

func (records *inventoryRecords) Match(agent, glob string) {
	records.matches = append(records.matches, agent+"|"+glob)
}

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

func newSetup(t *testing.T, inventory bool) setup {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
		Tables: tables, Excludes: tables.Excludes, Secret: walk.MatchAny(tables.SecretGlobs),
		Log: log, Manifest: manifest.NewWriter(rows), Root: root,
		Stager:    stage.Stager{Dir: stageDir, Physical: stageDir, Root: root},
		Inventory: inventory, Records: records,
	}
	return setup{root, collector, rows, log, records}
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
			if err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
				filepath.Join(fixture.root, volumes, "ollama")); err != nil {
				t.Fatal(err)
			}
		}, Counts{Seen: true}, 0, 0, ""},
		{"a symlinked volumes directory is noted and not followed", func(t *testing.T, fixture setup) {
			fixture.mkdir(t, "elsewhere/ollama/_data")
			fixture.mkdir(t, "var/lib/docker")
			if err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
				filepath.Join(fixture.root, "var/lib/docker/volumes")); err != nil {
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
			homes := []users.Home{{User: "alice", Path: filepath.Join(fixture.root, "home/alice")}}
			var notes []string
			got, err := Collect(context.Background(), fixture.collector, fixture.root, homes, &notes)
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

func TestUnmatchedVolumeRow(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	var notes []string
	if _, err := Collect(context.Background(), fixture.collector, fixture.root, nil, &notes); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(fixture.root, "var/lib/docker/volumes/postgres/_data")
	want := `{"user":"docker","home":"` + data + `","agent":"","path":"` + data +
		`","archive_path":"","type":"dir","size":`
	row := fixture.rows.String()
	if !strings.HasPrefix(row, want) ||
		!strings.HasSuffix(row, `,"mtime":0,"atime":0,"ctime":0,"btime":0,"uid":0,"gid":0,`+
			`"mode":"0","sha256":"","secret":false,"status":"skipped_unmatched_volume",`+
			`"target":"","error":""}`+"\n") {
		t.Errorf("row = %s", row)
	}
}

func TestInventoryRecordsMatchedVolumesOnly(t *testing.T) {
	fixture := newSetup(t, true)
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/f", "hi")
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/f", "hi")
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.root, nil, &notes)
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

func TestUnreadableVolumeIsNotedAndCounted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/ollama/_data/f", "x")
	volume := filepath.Join(fixture.root, "var/lib/docker/volumes/ollama")
	if err := os.Chmod(volume, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(volume, 0o755) })
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.root, nil, &notes)
	if err != nil {
		t.Fatal(err)
	}
	want := "docker: volume ollama (ollama) at " + volume + " is not readable; run as root to collect it"
	if got.Unreadable != 1 || got.Collected != 0 || len(notes) != 1 || notes[0] != want {
		t.Errorf("counts = %+v, notes = %q", got, notes)
	}
}

func TestVolumeRuleFirstMatchWins(t *testing.T) {
	table := []catalog.Entry{{Agent: "first", Glob: "*data"}, {Agent: "second", Glob: "my_data"}}
	got, found := volumeRule(table, "my_data")
	if !found || got.Agent != "first" {
		t.Errorf("rule = %+v, %v", got, found)
	}
	if _, found := volumeRule(table, "other"); found {
		t.Error("an unmatched name found a rule")
	}
}

func TestCancelledContextStopsCollection(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/ollama/_data/f", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var notes []string
	if _, err := Collect(ctx, fixture.collector, fixture.root, nil, &notes); err == nil {
		t.Error("a cancelled context gave no error")
	}
}
