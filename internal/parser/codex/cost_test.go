package codex

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// textBlocks returns a user message payload of input_text blocks.
func textBlocks(texts ...string) jsonvalue.Value {
	blocks := make([]jsonvalue.Value, 0, len(texts))
	for _, text := range texts {
		blocks = append(blocks, jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("input_text")},
			jsonvalue.Member{Key: "text", Value: jsonvalue.NewString(text)}))
	}
	return jsonvalue.NewObject(jsonvalue.Member{Key: "content", Value: jsonvalue.NewArray(blocks...)})
}

func TestUserPartsJoinNeighbouringBlocksOfOneKind(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
		want  []userPart
	}{
		{"one block", []string{"hi"}, []userPart{{text: "hi"}}},
		{"the person's blocks join", []string{"a", "b", "c"}, []userPart{{text: "a\nb\nc"}}},
		{"an empty block is left out", []string{"a", "", "b"}, []userPart{{text: "a\nb"}}},
		{"a context block between two of the person's",
			[]string{"a", "<environment_context>x</environment_context>", "b"},
			[]userPart{{text: "a"}, {kind: "environment_context", text: "<environment_context>x</environment_context>"},
				{text: "b"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := userParts(textBlocks(test.texts...)); !slices.Equal(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestAUserMessageOfManyBlocksIsJoinedOnce(t *testing.T) {
	const size = 4000
	blocks := make([]userPart, size)
	for i := range blocks {
		blocks[i] = userPart{text: "abcdefgh"}
	}
	calls, written := 0, 0
	got := joinParts(blocks, func(texts []string, separator string) string {
		joined := strings.Join(texts, separator)
		calls, written = calls+1, written+len(joined)
		return joined
	})
	if len(got) != 1 || calls != 1 || written != size*9-1 {
		t.Errorf("%d parts, %d joins writing %d bytes; want one join of %d bytes", len(got), calls, written,
			size*9-1)
	}
}

func TestAGuardianRolloutReadsTheLogDatabasesOnce(t *testing.T) {
	home := t.TempDir()
	rel := ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000001.jsonl"
	lines := []string{`{"type":"session_meta","timestamp":"2026-01-02T00:00:00Z","payload":{` +
		`"id":"0198f0a1-0000-7000-8000-000000000001","thread_source":"guardian_review",` +
		`"parent_thread_id":"parent","timestamp":"2026-01-02T00:00:00.000Z"}}`}
	const completions = 50
	for range completions {
		lines = append(lines, `{"type":"event_msg","timestamp":"2026-01-02T00:00:05Z","payload":{`+
			`"type":"task_complete","last_agent_message":"{\"outcome\":\"allow\"}"}}`)
	}
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := New()
	decisions := 0
	artifact := &input.Artifact{Rel: rel, DiskPath: path, Home: &input.Home{DiskPath: home}}
	subject.parseRollout(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, "permission decision:") {
			decisions++
		}
	})
	if decisions != completions || subject.guardianReads != 1 {
		t.Errorf("%d decisions and %d reads of the logs, want %d and 1", decisions, subject.guardianReads,
			completions)
	}
}
