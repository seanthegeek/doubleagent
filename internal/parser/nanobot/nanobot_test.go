package nanobot

import (
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows of every file are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests cover the helpers the
// analyzer's tests call directly, with its results.

func TestDecodeStemReadsOnlyAnExactEncoding(t *testing.T) {
	tests := []struct {
		stem string
		want string
	}{
		{"dGVsZWdyYW06MTIzNDU2Nzg5", "telegram:123456789"},
		{"Y2xpOmRpcmVjdA", "cli:direct"},
		{"cli_direct", "cli_direct"},
		{"YQ==", "YQ=="},
		{"YQ", "a"},
		{"YR", "YR"},
		{"Y", "Y"},
		{"_w", "_w"},
		{"__8", "__8"},
		{"w6k", "é"},
		{"gA", "gA"},
		{"ab+c", "ab+c"},
	}
	for _, test := range tests {
		t.Run(test.stem, func(t *testing.T) {
			if got := DecodeStem(test.stem); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestRelayTextsNameMessagingPlatformsOnly(t *testing.T) {
	tests := []struct {
		name    string
		session string
		last    string
		want    []string
	}{
		{"a key and a route of two platforms", "telegram:1", "discord:2", []string{
			"session relayed: telegram chat 1 (from session key)",
			"session relayed: discord chat 2 (from metadata.last_channel)"}},
		{"a local key and a route without a chat", "cli:x", "slack:", nil},
		{"a route equal to the key is not repeated", "slack:9", "slack:9", []string{
			"session relayed: slack chat 9 (from session key)"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := relayTexts(test.session, jsonvalue.NewString(test.last))
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheWebUIAndRunRecordsHaveTheWideWindow(t *testing.T) {
	subject := New()
	home := &input.Home{}
	tests := []struct {
		rel       string
		secondary bool
		tolerance float64
	}{
		{".nanobot/webui/websocket_chat-1.jsonl", true, webuiTolerance},
		{".nanobot/workspace/cron/runs/job.json", true, webuiTolerance},
		{".nanobot/history/cli_history", true, parser.DefaultTolerance},
		{".nanobot/memory/history.jsonl", false, parser.DefaultTolerance},
		{".nanobot/sessions/cli_direct.jsonl", false, parser.DefaultTolerance},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel, Home: home}
			if !subject.Wants(artifact) {
				t.Fatal("the parser does not want the file")
			}
			if got := subject.Secondary(artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
			if got := subject.SecondaryTolerance(artifact); got != test.tolerance {
				t.Errorf("tolerance %v, want %v", got, test.tolerance)
			}
		})
	}
}

func TestABareRunRecordIsWantedOnlyInAProject(t *testing.T) {
	subject := New()
	for _, project := range []bool{false, true} {
		artifact := &input.Artifact{Rel: "cron/runs/job.json", Home: &input.Home{Project: project}}
		if got := subject.Wants(artifact); got != project {
			t.Errorf("project %v: wanted %v", project, got)
		}
	}
}

func FuzzDecodeStem(fuzz *testing.F) {
	for _, seed := range []string{"dGVsZWdyYW06MTIzNDU2Nzg5", "cli_direct", "YQ==", "w6k"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, stem string) {
		key := DecodeStem(stem)
		if key != stem && strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(key)), "=") != stem {
			t.Errorf("%q decoded to %q, which does not encode back to it", stem, key)
		}
	})
}
