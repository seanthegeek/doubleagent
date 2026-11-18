package openclaw

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// writeHomeFile writes text at rel under home.
func writeHomeFile(t *testing.T, home, rel, text string) {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// parseHomeFile returns the texts of the rows subject gives the file at
// rel under home.
func parseHomeFile(t *testing.T, subject *Parser, home, rel string) []string {
	t.Helper()
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/home/u/" + rel,
		Rel: rel, Agent: "openclaw", Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// Every transcript beside a legacy sessions.json asks it for its session
// key, so the file is read once per state of it, not once per transcript.
func TestALegacySessionsFileIsReadOncePerStateOfIt(t *testing.T) {
	const transcripts = 12
	home := t.TempDir()
	var members []string
	for i := range transcripts {
		members = append(members, `"agent:main:k`+strconv.Itoa(i)+`": {"sessionId": "s`+strconv.Itoa(i)+`"}`)
		writeHomeFile(t, home, ".openclaw/sessions/s"+strconv.Itoa(i)+".jsonl",
			`{"type":"session","id":"s`+strconv.Itoa(i)+`","timestamp":"2026-01-02T03:04:05.000Z"}`+"\n")
	}
	writeHomeFile(t, home, ".openclaw/sessions/sessions.json", "{"+strings.Join(members, ", ")+"}")
	subject := New()
	subject.Work = parser.CountingWork()
	for i := range transcripts {
		rows := parseHomeFile(t, subject, home, ".openclaw/sessions/s"+strconv.Itoa(i)+".jsonl")
		if key := "key=agent:main:k" + strconv.Itoa(i); !strings.Contains(strings.Join(rows, "\n"), key) {
			t.Errorf("transcript %d gave %q, want its session's %s", i, rows, key)
		}
	}
	if reads := subject.Work.Count("sessions.json reads"); reads != 1 {
		t.Errorf("sessions.json read %d times for %d transcripts, want once", reads, transcripts)
	}
	writeHomeFile(t, home, ".openclaw/sessions/sessions.json", "[]")
	got := parseHomeFile(t, subject, home, ".openclaw/sessions/s0.jsonl")
	if reads := subject.Work.Count("sessions.json reads"); reads != 2 {
		t.Errorf("a changed sessions.json gave %d reads in all, want 2", reads)
	}
	if want := "parser: sessions.json is not a JSON object"; !strings.Contains(strings.Join(got, "\n"), want) {
		t.Errorf("rows %q, want the changed file's %q", got, want)
	}
}
