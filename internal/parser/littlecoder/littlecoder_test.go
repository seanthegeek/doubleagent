package littlecoder

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// The rows of every file kind are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests port the Python test
// that calls the module's patterns directly and check the routing by path.

func TestRecoveredKeysTakeJSONWhitespaceOnly(t *testing.T) {
	subject := New()
	patterns := []struct {
		name     string
		pattern  *regexp.Regexp
		template string
	}{
		{"sessions", subject.sessionsKey, `"sessions"%s:%s[`},
		{"messages", subject.messagesKey, `"messages"%s:%s[`},
		{"session_id", subject.sessionID, `"session_id"%s:%s"s1"`},
		{"saved_at", subject.savedAt, `"saved_at"%s:%s"t"`},
	}
	for _, test := range patterns {
		t.Run(test.name, func(t *testing.T) {
			if text := fmt.Sprintf(test.template, " \r\n", "\t"); !test.pattern.MatchString(text) {
				t.Errorf("%q does not match", text)
			}
			for _, space := range []string{" ", " ", "\u0085"} {
				if text := fmt.Sprintf(test.template, space, ""); test.pattern.MatchString(text) {
					t.Errorf("%q matches", text)
				}
			}
		})
	}
}

func TestWantsLittleCodersOwnFilesAndMarksTheCopiesSecondary(t *testing.T) {
	tests := []struct {
		rel             string
		want, secondary bool
	}{
		{".pi/agent/little-coder-prompt-history.json", true, false},
		{".little-coder/checkpoints/2026-10-01T11-00-00-000Z_0b9e.jsonl/src_a.ts", true, false},
		{".little-coder/checkpoints/2026-10-01T11-00-00-000Z_0b9e.jsonl/sub/src_a.ts", false, false},
		{".little-coder/checkpoints/s1/snapshots.json", true, false},
		{".little-coder/sessions/daily/2026-01-01/s1.json", true, false},
		{".little-coder/sessions/history.json", true, true},
		{".little-coder/sessions/mr_sessions/session_latest.json", true, true},
		{".little-coder/input_history.txt", true, true},
		{".little-coder/config.json", true, false},
		{".little-coder/sessions/notes.txt", false, false},
		{".pi/agent/sessions/--p--/a_b.jsonl", false, false},
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
