package analyze

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The texts planted outside the test collections; none may reach an output.
const (
	plantedManifest = "PLANTED-KILO-MANIFEST"
	plantedHistory  = "PLANTED-TASK-HISTORY"
)

// writeDisagreeingCollection writes, under dir, a collection whose manifest
// rows give deep paths but shallow archive paths, and the side files their
// parsers look for one or two directories above the archived file, planted
// outside the collection. It returns the collection.
func writeDisagreeingCollection(t *testing.T, dir string) string {
	t.Helper()
	collection := filepath.Join(dir, "collection")
	writeFile(t, filepath.Join(collection, "x.md"),
		"# Session s1\nUpdated: 2026-01-02T03:04:05Z\nTopic: t\n## Summary\nhello\n")
	writeFile(t, filepath.Join(collection, "fs", "ui_messages.json"),
		`[{"ts":1767322800000,"type":"say","say":"text","text":"hi"}]`)
	writeFile(t, filepath.Join(collection, "manifest.jsonl"),
		`{"path":"/home/u/.local/share/kilo/memory/F/sessions/x.md","home":"/home/u","user":"u",`+
			`"agent":"kilo-code","archive_path":"x.md","type":"file","status":"collected","size":10}`+"\n"+
			`{"path":"/home/u/.cline/data/tasks/T1/ui_messages.json","home":"/home/u","user":"u",`+
			`"agent":"cline","archive_path":"fs/ui_messages.json","type":"file","status":"collected","size":10}`+"\n")
	writeFile(t, filepath.Join(collection, "collection.json"), `{"hostname":"h"}`)
	writeFile(t, filepath.Join(dir, "manifest.json"), `{"canonical":"`+plantedManifest+`"}`)
	writeFile(t, filepath.Join(dir, "state", "taskHistory.json"),
		`[{"id":"T1","cwdOnTaskInitialization":"`+plantedHistory+`"}]`)
	return collection
}

func TestNeighbourLookupsNeverLeaveTheCollection(t *testing.T) {
	dir := t.TempDir()
	collection := writeDisagreeingCollection(t, dir)
	output := filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"timeline", collection, output}, System(),
		&stdout, &stderr)
	if code != ExitFound {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	written := stdout.String()
	for _, name := range []string{TimelineFile, SessionsFile, InventoryFile} {
		data, err := os.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		written += string(data)
	}
	for _, planted := range []string{plantedManifest, plantedHistory} {
		if strings.Contains(written, planted) {
			t.Errorf("%s reached the output:\n%s", planted, written)
		}
	}
	for _, agent := range []string{`"agent":"kilo-code"`, `"agent":"cline"`} {
		if !strings.Contains(written, agent) {
			t.Errorf("no row of %s: the artifacts were not parsed", agent)
		}
	}
}

func TestAnAgentDirectoryInputReadsNothingBesideIt(t *testing.T) {
	const planted = "PLANTED-BESIDE-THE-INPUT"
	dir := t.TempDir()
	tree := filepath.Join(dir, "case", ".little-coder")
	writeFile(t, filepath.Join(tree, "checkpoints", "2026-01-01_abc.jsonl", "a.txt"), "hi\n")
	writeFile(t, filepath.Join(dir, "case", ".pi", "agent", "sessions", "proj", "2026-01-01_abc.jsonl"),
		`{"type":"session","version":3,"id":"`+planted+`","timestamp":"2026-01-01T00:00:00Z",`+
			`"cwd":"/`+planted+`"}`+"\n")
	output := filepath.Join(dir, "out")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"timeline", tree, output}, System(), &stdout, &stderr)
	if code != ExitFound {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(output, TimelineFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), planted) || !strings.Contains(string(data), `"session_id":"abc"`) {
		t.Errorf("the timeline reads the file beside the input, or not the input:\n%s", data)
	}
}
