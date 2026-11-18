package geminicli

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// referencePatch is a patch applied by copying the message and its tool
// calls each time and searching the calls for each result: the direct
// reading of the recording service's rule, kept to check applyPatch
// against. It returns the message after the patch and the calls it looked
// at.
func referencePatch(message, patch jsonvalue.Value) (jsonvalue.Value, int) {
	compared := 0
	copied := parser.CopyObject(message)
	if content := patch.Get("content"); !content.IsNull() {
		copied.Set("content", content)
	}
	calls := message.Get("toolCalls")
	if patch.Get("toolCalls").IsArray() && parser.IsText(copied.Get("type"), "gemini") && calls.IsArray() {
		items := make([]jsonvalue.Value, 0, calls.Len())
		for _, call := range calls.Items() {
			if call.IsObject() {
				call = parser.CopyObject(call)
			}
			items = append(items, call)
		}
		for _, toolPatch := range patch.Get("toolCalls").Items() {
			result, ok := toolPatch.Lookup("result")
			if !toolPatch.IsObject() || !ok {
				continue
			}
			index := slices.IndexFunc(items, func(call jsonvalue.Value) bool {
				compared++
				return call.IsObject() && jsonvalue.Equal(call.Get("id"), toolPatch.Get("id"))
			})
			if index >= 0 {
				items[index].Set("result", result)
			}
		}
		copied.Set("toolCalls", jsonvalue.NewArray(items...))
	}
	return copied, compared
}

// decoded returns JSON text as a value, failing the test when it is not.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return value
}

// callIDs are the tool call ids generated messages and patches use: equal
// numbers written several ways, strings, and values of other kinds,
// objects among them with their keys in either order.
func callIDs() []string {
	return []string{`"a"`, `"b"`, `"1"`, `1`, `1.0`, `1e0`, `10e-1`, `-0`, `0`, `2`, `2e0`, `1e400`, `10e399`,
		`0.1`, `0.10000000000000001`, `null`, `true`, `false`, `["a"]`, `[1]`, `[1.0]`, `{"k":1}`, `{"k":1e0}`,
		`{"a":1,"b":2}`, `{"b":2,"a":1}`}
}

// randomMessage returns a gemini message, or now and then another kind,
// with tool calls whose ids repeat.
func randomMessage(t *testing.T, source *rand.Rand, line int) jsonvalue.Value {
	kind := `"gemini"`
	if source.IntN(8) == 0 {
		kind = `"user"`
	}
	calls := make([]jsonvalue.Value, source.IntN(6))
	for i := range calls {
		calls[i] = decoded(t, `{"id":`+callIDs()[source.IntN(len(callIDs()))]+`,"name":"n`+strconv.Itoa(i)+`"}`)
		if source.IntN(10) == 0 {
			calls[i] = jsonvalue.NewInt(int64(i))
		}
	}
	message := decoded(t, `{"id":"m","type":`+kind+`,"line":`+strconv.Itoa(line)+`}`)
	message.Set("toolCalls", jsonvalue.NewArray(calls...))
	return message
}

// randomPatch returns a patch of message m: sometimes a content, and tool
// call results for ids that are and are not there.
func randomPatch(t *testing.T, source *rand.Rand, line int) jsonvalue.Value {
	patch := decoded(t, `{"id":"m"}`)
	if source.IntN(2) == 0 {
		patch.Set("content", jsonvalue.NewString("c"+strconv.Itoa(line)))
	}
	if source.IntN(5) > 0 {
		results := make([]jsonvalue.Value, source.IntN(4))
		for i := range results {
			results[i] = decoded(t, `{"id":`+callIDs()[source.IntN(len(callIDs()))]+`,"result":"r`+
				strconv.Itoa(line)+`"}`)
			if source.IntN(8) == 0 {
				results[i] = decoded(t, `{"id":"a"}`)
			}
		}
		patch.Set("toolCalls", jsonvalue.NewArray(results...))
	}
	return patch
}

func TestPatchesChangeTheMessageAsCopyingItEachTimeWould(t *testing.T) {
	for seed := range uint64(300) {
		t.Run("patches from seed "+strconv.FormatUint(seed, 10), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			state := newReplay()
			want := jsonvalue.NewNull()
			for line := 1; line <= 40; line++ {
				if line == 1 || source.IntN(10) == 0 {
					want = randomMessage(t, source, line)
					recorded := want.JSON()
					state.apply(line, want)
					want = decoded(t, recorded)
					continue
				}
				patch := randomPatch(t, source, line)
				state.apply(line, decoded(t, `{"$patch":`+patch.JSON()+`}`))
				want, _ = referencePatch(want, patch)
				if got := state.messages.byID["m"].message.JSON(); got != want.JSON() {
					t.Fatalf("after the patch %s at line %d: %s, want %s", patch.JSON(), line, got, want.JSON())
				}
			}
		})
	}
}

func TestARecordedMessageIsNotChangedByAPatch(t *testing.T) {
	recorded := decoded(t, `{"id":"m","type":"gemini","content":"x","toolCalls":[{"id":"a"}]}`)
	state := newReplay()
	state.apply(1, recorded)
	state.apply(2, decoded(t, `{"$patch":{"id":"m","content":"y","toolCalls":[{"id":"a","result":"r"}]}}`))
	if recorded.JSON() != `{"id":"m","type":"gemini","content":"x","toolCalls":[{"id":"a"}]}` {
		t.Errorf("the recorded message became %s", recorded.JSON())
	}
	if got := state.messages.byID["m"].message.JSON(); got !=
		`{"id":"m","type":"gemini","content":"y","toolCalls":[{"id":"a","result":"r"}]}` {
		t.Errorf("the replayed message is %s", got)
	}
}

func TestEachPatchResultLooksAtOneCall(t *testing.T) {
	const size = 1000
	tests := []struct {
		name         string
		call, result func(i int) string
	}{
		{"string ids", func(i int) string { return `"c` + strconv.Itoa(i) + `"` },
			func(i int) string { return `"c` + strconv.Itoa(i) + `"` }},
		{"number ids patched in another spelling", strconv.Itoa,
			func(i int) string { return strconv.Itoa(i) + `.0e0` }},
		{"object ids patched with their keys in another order", func(i int) string { return `{"a":` + strconv.Itoa(i) + `,"b":0}` },
			func(i int) string { return `{"b":0,"a":` + strconv.Itoa(i) + `}` }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := make([]string, size)
			for i := range calls {
				calls[i] = `{"id":` + test.call(i) + `}`
			}
			state := newReplay()
			state.apply(1, decoded(t, `{"id":"m","type":"gemini","toolCalls":[`+strings.Join(calls, ",")+`]}`))
			for i := range size {
				state.patch(i+2, decoded(t, `{"id":"m","toolCalls":[{"id":`+test.result(size-1-i)+`,"result":"r"}]}`))
			}
			if state.compared != size {
				t.Errorf("%d calls looked at for %d patch results, want one each", state.compared, size)
			}
			for i, call := range state.messages.byID["m"].message.Get("toolCalls").Items() {
				if call.Get("result").StringValue() != "r" {
					t.Fatalf("call %d got no result: %s", i, call.JSON())
				}
			}
		})
	}
}
