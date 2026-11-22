package tomldoc

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func TestTheStatementsReadAreTheReadersKeys(t *testing.T) {
	texts := generatedDocuments(4000)
	for _, test := range loadDocuments(t) {
		texts = append(texts, test.text)
	}
	compared := 0
	for _, text := range texts {
		if _, err := Parse(text); err != nil {
			continue
		}
		text = strings.ReplaceAll(text, "\r\n", "\n")
		var mapping map[string]any
		meta, err := toml.Decode(text, &mapping)
		if err != nil {
			t.Fatalf("the reader refuses what Parse accepted: %v", err)
		}
		var read []toml.Key
		if err := checkDefinitions(text, func(path []string) { read = append(read, path) }); err != nil {
			t.Fatalf("document %q: %v", text, err)
		}
		if !slices.EqualFunc(read, meta.Keys(), slices.Equal) {
			t.Errorf("document %q: read %q, the reader's keys are %q", text, read, meta.Keys())
		}
		compared++
	}
	if compared < len(texts)/8 {
		t.Fatalf("only %d of %d documents were accepted, too few to test", compared, len(texts))
	}
}

func TestAStatementTheScannerCannotFollowEndsTheCheckWithoutARefusal(t *testing.T) {
	tests := []struct {
		name, text string
	}{
		{"a line with no key", "= 1\nx.y = 0\nx = 0\n"},
		{"a key with no equals sign", "x\nx.y = 0\nx = 0\n"},
		{"a header that is not closed", "[a\nx.y = 0\nx = 0\n"},
		{"an array that is not closed", "a = [1,\n"},
		{"an inline table that is not closed", "a = {b = 1,\n"},
		{"a quoted key that is not closed", "\"a = 1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := checkDefinitions(test.text, nil); err != nil {
				t.Fatalf("got %v, want the check to stop without refusing", err)
			}
		})
	}
}

func TestAQuotedKeyIsTheTextItsEscapesStandFor(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"a key without escapes is as written", `a b`, "a b"},
		{"the one-letter escapes", `\b\t\n\f\r\e\"\\`, "\b\t\n\f\r\x1b\"\\"},
		{"a two-digit escape is a code point", `\x41\xe9`, "Aé"},
		{"a four-digit escape is a code point", `aé`, "aé"},
		{"an eight-digit escape is a code point", `\U0001F600`, "😀"},
		{"escapes between text", `xay\tz`, "xay\tz"},
		{"an escape the reader would refuse is kept as written", `\q\u00`, `\q\u00`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decodeBasicKey(test.body); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// costDocuments returns, for each shape the definition check reads, a
// document of about size bytes.
func costDocuments(size int) map[string]string {
	fill := func(prefix string, line func(int) string) string {
		var text strings.Builder
		text.WriteString(prefix)
		for i := 0; text.Len() < size; i++ {
			text.WriteString(line(i))
		}
		return text.String()
	}
	longPart := strings.Repeat("p", size/maxDepth)
	return map[string]string{
		"many distinct keys": fill("", func(i int) string { return fmt.Sprintf("k%d = 1\n", i) }),
		"many tables":        fill("", func(i int) string { return fmt.Sprintf("[t%d]\nk = 1\n", i) }),
		"many keys of one inline table": fill("a = {", func(i int) string {
			return fmt.Sprintf("k%d = 1, ", i)
		}) + "}\n",
		"many tables of an array": fill("", func(i int) string { return fmt.Sprintf("[[a]]\nb.c%d = 1\n", i%7) }),
		"one long dotted key":     strings.TrimSuffix(strings.Repeat(longPart+".", maxDepth), ".") + " = 1\n",
		"inline tables nested to the depth limit": fill("", func(i int) string {
			return fmt.Sprintf("a%d = ", i) + strings.Repeat("{b.c = 1, d = ", maxDepth/2-1) + "1" +
				strings.Repeat("}", maxDepth/2-1) + "\n"
		}),
	}
}

// fastestCheck returns the shortest of several runs of the definition
// check over a document.
func fastestCheck(t *testing.T, text string) time.Duration {
	t.Helper()
	fastest := time.Duration(1 << 62)
	for range 5 {
		start := time.Now()
		if err := checkDefinitions(text, nil); err != nil {
			t.Fatal(err)
		}
		fastest = min(fastest, time.Since(start))
	}
	return fastest
}

func TestTheDefinitionCheckGrowsInStepWithTheDocument(t *testing.T) {
	small, large := costDocuments(256<<10), costDocuments(512<<10)
	for name, text := range small {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(text); err != nil {
				t.Fatalf("the document of the case is refused: %v", err)
			}
			smallTime, largeTime := fastestCheck(t, text), fastestCheck(t, large[name])
			// Linear work doubles; work that grows with the square of the
			// document grows by four. A millisecond covers the clock's grain.
			if largeTime > 3*smallTime+time.Millisecond {
				t.Fatalf("%s at %d bytes, %s at %d bytes", smallTime, len(text), largeTime, len(large[name]))
			}
		})
	}
}

// FuzzTheStatementsReadAreTheReadersKeys fails when, for a document the
// reader accepts, the paths the definition check reads are not the
// reader's key list, or the start of it where the check refuses.
func FuzzTheStatementsReadAreTheReadersKeys(fuzzer *testing.F) {
	for _, test := range loadDocuments(fuzzer) {
		fuzzer.Add(test.text)
	}
	fuzzer.Fuzz(func(t *testing.T, text string) {
		if checkShape(text) != nil {
			return
		}
		text = strings.ReplaceAll(text, "\r\n", "\n")
		mapping, keys, err := decode(text)
		if err != nil || mapping == nil {
			return
		}
		var read []toml.Key
		refusal := checkDefinitions(text, func(path []string) { read = append(read, path) })
		if refusal == nil && len(read) != len(keys) || len(read) > len(keys) ||
			!slices.EqualFunc(read, keys[:len(read)], slices.Equal) {
			t.Fatalf("read %q (refusal %v), the reader's keys are %q", read, refusal, keys)
		}
	})
}
