package openclaw

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every store are proved against the Python analyzer by the
// parser replay (internal/parity/replay), over the recorded fixture
// databases and transcripts; these tests cover the routing and helpers,
// with expected values taken from running the analyzer's own functions.

func TestWantsAndSecondaryFollowTheFileNames(t *testing.T) {
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
	}{
		{".openclaw/agents/main/agent/openclaw-agent.sqlite", true, false},
		{".openclaw-work/state/openclaw.sqlite", true, false},
		{".clawdbot/sessions/abc.jsonl", true, false},
		{".openclaw/agents/m/sessions/abc.trajectory.jsonl", false, false},
		{".openclaw/sessions/abc.jsonl.reset.2026-10-01T09-30-00.000Z", true, true},
		{".openclaw/sessions/abc.jsonl.deleted.2026-10-02T08-00-00.000Z.zst", true, true},
		{".openclaw/sessions/abc.checkpoint.0b5e1c2d-3f4a-4b5c-8d6e-7f8091a2b3c4.jsonl", true, true},
		{".openclaw/sessions/abc.jsonl.pre-doctor-branch-repair-2026-09-20T08-10-00-000Z.bak", true, true},
		{".openclaw/sessions/cold/" + strings.Repeat("a", 64) + ".jsonl.zst", true, false},
		{".openclaw/sessions/abc.jsonl.bak.1", false, false},
		{".moltbot/state/openclaw.sqlite-wal", false, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.wants {
				t.Errorf("wants %v, want %v", got, test.wants)
			}
			if got := subject.Secondary(artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
		})
	}
}

// decoded returns the JSON value of text, failing the test when it is not
// JSON.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestPolicySettingsGiveTheAnalyzersText(t *testing.T) {
	policy := decoded(t, `{"security": "allowlist", "ask": true, "askFallback": null,`+
		` "autoAllowSkills": {"a": [1]}, "allowlist": [{"pattern": "ls", "argPattern": "-l",`+
		` "source": "user"}, {"pattern": null}, "x", {"pattern": 5}],`+
		` "mcpTools": [{"server": "s"}, {}]}`)
	want := []string{"security=allowlist", "ask=true", `autoAllowSkills={"a":[1]}`,
		"allowlist=ls argPattern=-l source=user", "allowlist=5 source=-", "mcpTools=s/- source=-",
		"mcpTools=-/- source=-"}
	if got := policySettings(policy); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDeliveryRelayNamesOnlyAPlatform(t *testing.T) {
	tests := []struct {
		name     string
		delivery string
		want     string
	}{
		{"an external delivery to a platform",
			`{"kind": "external", "route": {"channel": "telegram"}, "origin": {"accountId": "a1",` +
				` "chatType": "dm", "from": "u", "to": "b"}}`,
			"session relayed: telegram account a1 dm from u to b (from openclaw-agent.sqlite" +
				" session_nodes.entry_json delivery, updated_at)"},
		{"the Control UI is no platform", `{"kind": "external", "route": {"channel": "webchat"}}`, ""},
		{"an internal delivery", `{"kind": "internal", "route": {"channel": "telegram"}}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := deliveryRelay(decoded(t, test.delivery)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func FuzzTranscriptBlob(fuzz *testing.F) {
	fuzz.Add([]byte(`{"type":"session","id":"s1","cwd":"/w"}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"toolCall","arguments":"{}"}]}}`))
	fuzz.Add([]byte(`{"kind":"header","sessionId":"s"}` + "\n" +
		`{"kind":"event","row":{"seq":1,"event_json":"{\"type\":\"label\"}"}}`))
	fuzz.Add([]byte("\x28\xb5\x2f\xfd\x00"))
	fuzz.Fuzz(func(t *testing.T, blob []byte) {
		out := &writer{Parser: New(), artifact: &input.Artifact{}, options: parser.Options{IncludeThinking: true},
			emit: func(timeline.Row) {}}
		data, err := maybeZstd(blob)
		if err != nil {
			return
		}
		var problems parser.Problems
		out.records(newSession(""), blobLines(data, &problems), 0)
	})
}
