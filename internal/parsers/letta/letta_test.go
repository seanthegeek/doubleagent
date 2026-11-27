package letta

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The rows of every file kind are checked by the golden output of the
// analysis commands (internal/analyze), over inputs that hold every parsed
// agent; these tests cover the helpers that decide a row's fields.

func TestDecodeKeyReadsBase64URLDirectoryNames(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"Y29udmVyc2F0aW9uOmxvY2FsLWNvbnYtMQ", "conversation:local-conv-1"},
		{"ZGVmYXVsdDphZ2VudC0x", "default:agent-1"},
		{"Y29udmVyc2F0aW9uOmM", "conversation:c"},
		{"Y29udmVyc2F0aW9uOmM=", "conversation:c"},
		{"", ""},
		{"/w", ""},
		{"_-8", ""},
		{"abc", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DecodeKey(test.name); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSanitizeReplacesEachOtherCharacterOnce(t *testing.T) {
	tests := []struct {
		segment, want string
	}{
		{"agent-1.x_y", "agent-1.x_y"},
		{"a b/c", "a_b_c"},
		{"é", "_"},
		{"", "unknown"},
		{"  ", "__"},
		{"ab\ufffd", "ab_"},
		{"ab\xed\xa0", "ab__"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := sanitize(test.segment); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSessionProjectTakesTheNextSessionElseTheLast(t *testing.T) {
	sessions := []projectTime{{"2026-01-01T00:00:00.000Z", "/a"}, {"2026-01-02T00:00:00.000Z", "/b"}}
	tests := []struct {
		timestamp, want string
	}{
		{"", ""},
		{"2025-12-31T00:00:00.000Z", "/a"},
		{"2026-01-01T00:00:00.000Z", "/a"},
		{"2026-01-01T12:00:00.000Z", "/b"},
		{"2026-01-03T00:00:00.000Z", "/b"},
	}
	for _, test := range tests {
		t.Run(test.timestamp, func(t *testing.T) {
			if got := sessionProject(sessions, test.timestamp, parserkit.Work{}); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestWantsTheLettaFilesAndTreatsTranscriptsAsSecondary(t *testing.T) {
	tests := []struct {
		rel             string
		want, secondary bool
	}{
		{".letta/sessions.jsonl", true, false},
		{".letta/settings.json", true, false},
		{".letta/settings.local.json", true, false},
		{".config/letta/settings.json", true, false},
		{".letta/crons.json", true, false},
		{".letta/transcripts/agent-1/conv-1/transcript.jsonl", true, true},
		{".letta/lc-local-backend/conversations/Y29u/messages.jsonl", true, false},
		{".letta/channels/telegram/routing.json", true, false},
		{".letta/channels/telegram/routing.yaml", true, false},
		{".letta/channels/telegram/routing.yml", false, false},
		{".letta/runs/task-1.jsonl", true, false},
		{".letta/listener-state/0123456789abcdef01234567/a_c.json", true, false},
		{".letta/listener-state/0123456789ABCDEF01234567/a_c.json", true, false},
		{".letta/memory/core.md", false, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.want {
				t.Errorf("wants %v, want %v", got, test.want)
			}
			if got := subject.Secondary(artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
		})
	}
}
