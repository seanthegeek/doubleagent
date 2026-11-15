package volumes

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// shCollection is the part of collection.json the oracle test compares.
type shCollection struct {
	Docker struct {
		Found      int  `json:"volumes_found"`
		Collected  int  `json:"volumes_collected"`
		Unreadable int  `json:"unreadable"`
		Desktop    bool `json:"docker_desktop"`
	} `json:"docker"`
	Notes []string `json:"notes"`
}

var atimeField = regexp.MustCompile(`"atime":[0-9]+,`)

// runShCollector runs the sh collector in image mode over root and returns its
// parsed collection.json and the manifest rows of user docker, without atime.
func runShCollector(t *testing.T, root string) (shCollection, []string) {
	t.Helper()
	shell, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	command := exec.Command(shell, script, "-r", root, "-o", out, "-q", "--no-projects",
		"--max-file-size", "1")
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash", "LC_ALL=C")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	var summary shCollection
	read := func(pattern string) []byte {
		names, _ := filepath.Glob(filepath.Join(out, pattern))
		if len(names) != 1 {
			t.Fatalf("outputs for %s: %v", pattern, names)
		}
		content, err := os.ReadFile(names[0])
		if err != nil {
			t.Fatal(err)
		}
		return content
	}
	if err := json.Unmarshal(read("*.collection.json"), &summary); err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, row := range strings.Split(string(read("*.manifest.jsonl")), "\n") {
		if strings.HasPrefix(row, `{"user":"docker"`) {
			rows = append(rows, atimeField.ReplaceAllString(row, ""))
		}
	}
	return summary, rows
}

// buildOracleFixture writes the image the oracle test collects.
func buildOracleFixture(t *testing.T, fixture setup) {
	fixture.write(t, "etc/passwd", "alice:x:1000:1000::/home/alice:/bin/sh\n")
	fixture.mkdir(t, "home/alice")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/x/file", "hi")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/tmp/playwright/cache", "junk")
	fixture.write(t, "var/lib/docker/volumes/a0_usr/_data/secrets.env", "KEY=1")
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	fixture.write(t, "var/lib/docker/volumes/ollama_data/_data/models/blob", "m")
	fixture.mkdir(t, "var/lib/docker/volumes/langflow-data")
	fixture.mkdir(t, "var/lib/docker/volumes/n8n_data/other")
	fixture.write(t, "home/alice/.local/share/docker/volumes/hermes1/_data/f", "x")
	fixture.mkdir(t, "elsewhere")
	fixture.mkdir(t, "var/lib/containers/storage")
	if err := os.Symlink(filepath.Join(fixture.root, "elsewhere"),
		filepath.Join(fixture.root, "var/lib/containers/storage/volumes")); err != nil {
		t.Fatal(err)
	}
	fixture.mkdir(t, "ProgramData/DockerDesktop")
	fixture.mkdir(t, "home/alice/Library/Containers/com.docker.docker")
}

func TestCollectMatchesShCollector(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.collector.MaxSize = 1 << 20
	buildOracleFixture(t, fixture)
	wantSummary, wantRows := runShCollector(t, fixture.root)
	if len(wantRows) < 4 {
		t.Fatalf("the sh collector wrote %d docker rows; the fixture is wrong", len(wantRows))
	}
	homes := []users.Home{{User: "alice", Path: filepath.Join(fixture.root, "home/alice")}}
	var notes []string
	got, err := Collect(context.Background(), fixture.collector, fixture.root, homes, &notes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found != wantSummary.Docker.Found || got.Collected != wantSummary.Docker.Collected ||
		got.Unreadable != wantSummary.Docker.Unreadable || got.Desktop != wantSummary.Docker.Desktop {
		t.Errorf("counts = %+v, sh = %+v", got, wantSummary.Docker)
	}
	if strings.Join(notes, "\n") != strings.Join(wantSummary.Notes, "\n") {
		t.Errorf("notes = %q, sh = %q", notes, wantSummary.Notes)
	}
	var gotRows []string
	for _, row := range strings.Split(strings.TrimSuffix(fixture.rows.String(), "\n"), "\n") {
		gotRows = append(gotRows, atimeField.ReplaceAllString(row, ""))
	}
	if strings.Join(gotRows, "\n") != strings.Join(wantRows, "\n") {
		t.Errorf("rows differ\ngo:\n%s\nsh:\n%s", strings.Join(gotRows, "\n"), strings.Join(wantRows, "\n"))
	}
}
