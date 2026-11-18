package codex

import (
	"runtime"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
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
