//go:build unix

package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// runShCollector runs the sh collector in image mode over root and returns
// the manifest lines it wrote. It skips the test when dash is missing.
func runShCollector(t *testing.T, root string) []string {
	t.Helper()
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	command := exec.Command(dash, script, "-r", root, "-o", outDir, "-q",
		"--no-projects", "--no-docker", "--max-file-size", "1")
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "*.manifest.jsonl"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("manifest files = %v, %v", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// buildFixture writes an image with a home holding files of several shapes.
func buildFixture(t *testing.T) (root, home string) {
	t.Helper()
	root = t.TempDir()
	home = filepath.Join(root, "home", "alice")
	claude := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	passwd := "alice:x:1000:1000::" + home[len(root):] + ":/bin/sh\n"
	files := map[string]string{
		filepath.Join(root, "etc", "passwd"):   passwd,
		filepath.Join(claude, "settings.json"): "{\"a\": 1}\n",
		filepath.Join(claude, "CLAUDE.md"):     strings.Repeat("x", 1_500_000),
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(filepath.Join(claude, "settings.json"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(`target "q" \ end`, filepath.Join(claude, "plugins")); err != nil {
		t.Fatal(err)
	}
	return root, home
}

// decodeRow reads a manifest line into a Row with encoding/json, which the
// project may use to read an interface file but never to write one.
func decodeRow(t *testing.T, line string) Row {
	t.Helper()
	var fields struct {
		User        string `json:"user"`
		Home        string `json:"home"`
		Agent       string `json:"agent"`
		Path        string `json:"path"`
		ArchivePath string `json:"archive_path"`
		Type        string `json:"type"`
		Size        int64  `json:"size"`
		Mtime       int64  `json:"mtime"`
		Atime       int64  `json:"atime"`
		Ctime       int64  `json:"ctime"`
		Btime       int64  `json:"btime"`
		UID         uint32 `json:"uid"`
		GID         uint32 `json:"gid"`
		Mode        string `json:"mode"`
		SHA256      string `json:"sha256"`
		Secret      bool   `json:"secret"`
		Status      Status `json:"status"`
		Target      string `json:"target"`
		Error       string `json:"error"`
	}
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil {
		t.Fatalf("decoding %s: %v", line, err)
	}
	return Row(fields)
}

func TestWriterReproducesShRows(t *testing.T) {
	root, _ := buildFixture(t)
	lines := runShCollector(t, root)
	if len(lines) < 3 {
		t.Fatalf("sh collector wrote %d rows, want at least 3: %v", len(lines), lines)
	}
	statuses := map[Status]bool{}
	for _, line := range lines {
		row := decodeRow(t, line)
		statuses[row.Status] = true
		var out bytes.Buffer
		if err := NewWriter(&out).Write(row); err != nil {
			t.Fatal(err)
		}
		if out.String() != line+"\n" {
			t.Errorf("row differs\n got %swant %s", out.String(), line)
		}
	}
	for _, want := range []Status{Collected, Symlink, SkippedSize} {
		if !statuses[want] {
			t.Errorf("fixture produced no %s row; got %v", want, statuses)
		}
	}
}

func TestWriterBuildsShRowFromLstat(t *testing.T) {
	root, home := buildFixture(t)
	source := filepath.Join(home, ".claude", "settings.json")
	meta, err := fsmeta.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	want := Row{
		User: "alice", Home: home, Agent: "claude-code", Path: source,
		ArchivePath: "fs" + strings.TrimPrefix(source, root),
		Type:        "file", Size: meta.Size, Mtime: meta.Mtime, Atime: meta.Atime,
		Ctime: meta.Ctime, Btime: meta.Btime, UID: meta.UID, GID: meta.GID,
		Mode: meta.Mode, SHA256: hex.EncodeToString(digest[:]), Secret: true,
		Status: Collected,
	}
	var expected bytes.Buffer
	if err := NewWriter(&expected).Write(want); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range runShCollector(t, root) {
		if strings.Contains(line, `"path":"`+EscapeJSON(source)+`"`) {
			found = true
			if line+"\n" != expected.String() {
				t.Errorf("row differs\n got %swant %s", expected.String(), line)
			}
		}
	}
	if !found {
		t.Error("the sh collector wrote no row for settings.json")
	}
}
