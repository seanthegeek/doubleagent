package goose

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// mergeByCopy is the merge of one chunk written as a copy of the last
// block per chunk, which streamed must agree with.
func mergeByCopy(blocks, added []jsonvalue.Value) []jsonvalue.Value {
	if len(added) != 1 || len(blocks) == 0 || !blocks[len(blocks)-1].IsObject() || !added[0].IsObject() {
		return append(blocks, added...)
	}
	last, block := blocks[len(blocks)-1], added[0]
	kind := last.Get("type")
	switch {
	case parserkit.IsText(kind, "text") && parserkit.IsText(block.Get("type"), "text") &&
		jsonvalue.Equal(audience(last), audience(block)):
		blocks[len(blocks)-1] = withMember(last, "text",
			jsonvalue.NewString(parserkit.Text(last.Get("text"))+parserkit.Text(block.Get("text"))))
	case parserkit.IsText(kind, "thinking") && parserkit.IsText(block.Get("type"), "thinking") &&
		(last.Get("signature").IsEmpty() || jsonvalue.Equal(block.Get("signature"), last.Get("signature"))):
		merged := withMember(last, "thinking",
			jsonvalue.NewString(parserkit.Text(last.Get("thinking"))+parserkit.Text(block.Get("thinking"))))
		if signature := block.Get("signature"); !signature.IsEmpty() {
			merged = withMember(merged, "signature", signature)
		}
		blocks[len(blocks)-1] = merged
	default:
		return append(blocks, added...)
	}
	return blocks
}

// randomChunk returns the blocks of one chunk: mostly one text or thinking
// block with a random audience, signature and member order, sometimes two
// blocks or a block that is not an object.
func randomChunk(t *testing.T, source *rand.Rand) []jsonvalue.Value {
	t.Helper()
	pick := func(choices ...string) string { return choices[source.IntN(len(choices))] }
	block := pick(
		`{"type":"text","text":"t"}`, `{"type":"text","text":"u","annotations":{"audience":["user"]}}`,
		`{"annotations":{"audience":["user"]},"type":"text"}`, `{"type":"text","text":7}`,
		`{"type":"thinking","thinking":"k"}`, `{"type":"thinking","thinking":"m","signature":"s"}`,
		`{"signature":"r","type":"thinking"}`, `{"type":"thinking","thinking":"n","signature":""}`,
		`{"type":"image"}`, `"loose"`)
	text := "[" + block + "]"
	if source.IntN(6) == 0 {
		text = "[" + block + `,{"type":"text","text":"v"}]`
	}
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value.Items()
}

func TestStreamedBlocksAreTheBlocksAMergeByCopyGives(t *testing.T) {
	for seed := range uint64(300) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			first := randomChunk(t, source)
			want := append([]jsonvalue.Value(nil), first...)
			got := &streamed{blocks: append([]jsonvalue.Value(nil), first...)}
			for step := range 12 {
				chunk := randomChunk(t, source)
				want = mergeByCopy(want, chunk)
				got.merge(chunk)
				if source.IntN(4) == 0 || step == 11 {
					if gotText, wantText := jsonvalue.NewArray(got.values()...).JSON(),
						jsonvalue.NewArray(want...).JSON(); gotText != wantText {
						t.Fatalf("after %d chunks: got %s, want %s", step+1, gotText, wantText)
					}
				}
			}
		})
	}
}

// streamWork returns the bytes copied to assemble one message from n text
// chunks of ten bytes.
func streamWork(n int) int {
	chunk := jsonvalue.NewArray(jsonvalue.NewObject(
		jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
		jsonvalue.Member{Key: "text", Value: jsonvalue.NewString(strings.Repeat("x", 10))}))
	work := parserkit.CountingWork()
	blocks := &streamed{blocks: chunk.Items(), work: work}
	for range n - 1 {
		blocks.merge(chunk.Items())
	}
	if len(blocks.values()) != 1 {
		return -1
	}
	return work.Count("bytes copied")
}

func TestAStreamedMessageIsCopiedOnceWhateverItsChunks(t *testing.T) {
	for _, n := range []int{100, 1000} {
		if got := streamWork(n); got != 10*n {
			t.Errorf("%d chunks: copied %d bytes, want %d", n, got, 10*n)
		}
	}
}
