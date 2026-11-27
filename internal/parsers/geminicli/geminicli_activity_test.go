package geminicli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// activityRows parses log as an activity log and returns its rows' texts.
func activityRows(t *testing.T, log string) []string {
	t.Helper()
	const rel = ".gemini/tmp/0123456789abcdef/logs/session-s1.jsonl"
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "gemini-cli",
		Home: &input.Home{DiskPath: home, Secrets: secrets.NewRecorder(secrets.Setting{})}, Manifest: jsonvalue.NewNull()}
	var got []string
	err := parsertest.Parse(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// networkRecord returns an activity log line of a network record whose
// payload is the JSON members given.
func networkRecord(payload string) string {
	return `{"type": "network", "timestamp": "2026-01-01T00:00:00Z", "sessionId": "s1", "payload": {` +
		payload + "}}\n"
}

func TestOnlyARequestsOwnRowCarriesItsMethodAndURL(t *testing.T) {
	log := networkRecord(`"id": "r1", "method": "POST", "url": "https://h.example/v1?alt=sse"`) +
		networkRecord(`"id": "r1", "response": {"status": 200, "durationMs": 5}`) +
		networkRecord(`"id": "r1", "error": "socket hang up"`) +
		networkRecord(`"id": "z", "response": {}`)
	want := []string{
		"network request: POST https://h.example/v1?alt=sse (request r1)",
		"network response: status 200 in 5 ms (request r1)",
		"network error: socket hang up (request r1)",
		"network response: status - in - ms (request z)",
	}
	if got := activityRows(t, log); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// answeredText returns the bytes of text of the rows of an activity log of
// one request whose URL is 50 bytes per answer, then n answers to it.
func answeredText(t *testing.T, n int) int {
	t.Helper()
	log := networkRecord(`"id": "r", "method": "GET", "url": "https://h.example/`+strings.Repeat("p", 50*n)+`"`) +
		strings.Repeat(networkRecord(`"id": "r", "response": {}`), n)
	total := 0
	for _, text := range activityRows(t, log) {
		total += len(text)
	}
	return total
}

func TestTheTextOfAnActivityLogGrowsWithItsRecordsNotWithTheirSquare(t *testing.T) {
	small, large := answeredText(t, 100), answeredText(t, 200)
	// Twice the answers to a request whose URL is twice as long gives twice
	// the text; repeating the URL in each answer would give four times as
	// much.
	if small == 0 || large > 5*small/2 {
		t.Errorf("text bytes: %d for 100 answers, %d for 200", small, large)
	}
}
