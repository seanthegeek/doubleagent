package codex

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// allocatedBytes returns how many bytes work allocates on the heap, a count
// that does not depend on the machine's speed.
func allocatedBytes(work func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	work()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// growsLinearly reports whether the bytes work allocates for twice the size
// stay under three times those for the size: a reader that rebuilds its
// text per element allocates four times as much.
func growsLinearly(t *testing.T, size int, work func(size int)) {
	t.Helper()
	small := allocatedBytes(func() { work(size) })
	large := allocatedBytes(func() { work(2 * size) })
	if large > 3*small {
		t.Errorf("%d bytes for size %d against %d for size %d: not linear", large, 2*size, small, size)
	}
}

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

func TestAUserMessageOfManyBlocksCostsLinearWork(t *testing.T) {
	growsLinearly(t, 4000, func(size int) {
		texts := make([]string, size)
		for i := range texts {
			texts[i] = "abcdefgh"
		}
		userParts(textBlocks(texts...))
	})
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
