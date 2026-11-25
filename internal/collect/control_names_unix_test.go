//go:build unix

package collect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// decoyText is what the files in the working directory hold, which no run
// over an image may read.
const decoyText = "decoy in the working directory\n"

// controlNameImage builds an image whose names hold a newline, a carriage
// return and an escape character, each followed by the name of a decoy that
// lies in the working directory, and returns its root.
func controlNameImage(t *testing.T) string {
	t.Helper()
	root := longTempDir(t)
	files := map[string]string{
		"home/alice/.claude.json.bak\ndecoy.txt":           "image file\n",
		"home/alice/.claude.json.cr\rdecoy.txt":            "carriage return\n",
		"home/alice/.claude.json.esc\x1b[2Kforged":         "escape\n",
		"home/alice/.bashrc.d/env\ndecoy.txt":              "export CODEX_HOME=/opt/codex\n",
		"home/bob\ndecoyhome/.claude.json":                 "bob\n",
		"home/carol\nuser:decoyproject/.claude/x.json":     "{}\n",
		"home/alice/.claude/projects/-srv-p/session.jsonl": "{}\n",
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// plantDecoys fills the working directory with the files a name split at a
// control character would name: a file, a home holding agent state, and a
// home whose state names a project inside the image.
func plantDecoys(t *testing.T, root string) {
	t.Helper()
	work := t.TempDir()
	t.Chdir(work)
	state := `{"projects":{"/srv/decoy-project":{}}}` + "\n"
	files := map[string]string{"decoy.txt": decoyText, "decoyhome/.claude.json": decoyText,
		"decoyproject/.claude.json": state}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(work, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "srv/decoy-project/.claude"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestANameHoldingAControlCharacterIsCollectedUnderItsRealName(t *testing.T) {
	root := controlNameImage(t)
	plantDecoys(t, root)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != ExitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	rows, texts := manifestRows(t, outDir)
	for i, row := range rows {
		if path, _ := row["path"].(string); !strings.HasPrefix(path, root+"/") {
			t.Errorf("a row names %q, outside the image root: %s", path, texts[i])
		}
		if path, _ := row["path"].(string); strings.Contains(path, "decoy-project") {
			t.Errorf("a project named only in the working directory has a row: %s", texts[i])
		}
	}
	decoy := sha256.Sum256([]byte(decoyText))
	members := archiveMembers(t, outDir)
	for name, hash := range members {
		if hash == hex.EncodeToString(decoy[:]) {
			t.Errorf("the archive holds the working directory's decoy as %q", name)
		}
	}
	for _, name := range []string{"./fs/home/alice/.claude.json.bak\ndecoy.txt",
		"./fs/home/alice/.claude.json.cr\rdecoy.txt", "./fs/home/alice/.claude.json.esc\x1b[2Kforged",
		"./fs/home/bob\ndecoyhome/.claude.json", "./fs/home/alice/.bashrc.d/env\ndecoy.txt"} {
		if _, ok := members[name]; !ok {
			t.Errorf("the archive has no member %q", name)
		}
	}
	checkOneLinePerEvent(t, runOutput(t, outDir, ".log"))
}

// logLine is the shape of every log line: a UTC time, a space and text
// holding no control character.
var logLine = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ [^\x00-\x1f\x7f]*$`)

// checkOneLinePerEvent fails the test unless every line of log has the
// shape of a log line and the hostile names are written with their control
// characters escaped.
func checkOneLinePerEvent(t *testing.T, log string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSuffix(log, "\n"), "\n") {
		if !logLine.MatchString(line) {
			t.Errorf("log line %q is not one timed line free of control characters", line)
		}
	}
	var missing []string
	for _, want := range []string{`/home/alice/.claude.json.bak\x0adecoy.txt`,
		`/home/alice/.claude.json.cr\x0ddecoy.txt`, `/home/alice/.claude.json.esc\x1b[2Kforged`,
		`/home/alice/.bashrc.d/env\x0adecoy.txt:1 names CODEX_HOME (codex-cli)`} {
		if !strings.Contains(log, want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		t.Errorf("the log does not name %q:\n%s", missing, log)
	}
}
