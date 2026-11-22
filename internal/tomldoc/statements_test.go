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
		seams := &checkSeams{visit: func(path []string) { read = append(read, path) }}
		if err := checkDefinitions(text, seams); err != nil {
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
		{"a key without escapes is as written", `A b`, "A b"},
		{"the one-letter escapes", `\b\t\n\f\r\e\"\\`, "\b\t\n\f\r\x1b\"\\"},
		{"a two-digit escape is a code point", `\x41\xe9`, "A\u00e9"},
		{"a four-digit escape is a code point", `\u0061\u00E9`, "a\u00e9"},
		{"an eight-digit escape is a code point", `\U0001F600`, "\U0001F600"},
		{"the largest scalar value is a code point", `\U0010FFFF`, "\U0010FFFF"},
		{"escapes between text", `x\u0061y\tz`, "xay\tz"},
		{"a surrogate is kept as written", `\uD800`, `\uD800`},
		{"a code point past the largest is kept as written", `\U00110000`, `\U00110000`},
		{"the largest eight digits are kept as written", `\UFFFFFFFF`, `\UFFFFFFFF`},
		{"a letter that is no escape is kept as written", `\q`, `\q`},
		{"an escape cut short at the end is kept as written", `a\u00`, `a\u00`},
		{"a backslash as the last byte is kept", `a\`, `a\`},
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

// fastestCheck returns the shortest of up to nine runs of the definition
// check over a document, fewer once the runs have taken a second, and the
// lookups one run makes.
func fastestCheck(t *testing.T, text string) (time.Duration, int) {
	t.Helper()
	fastest, lookups, spent := time.Duration(1<<62), 0, time.Duration(0)
	for run := 0; run < 9 && spent < time.Second; run++ {
		seams := &checkSeams{}
		start := time.Now()
		if err := checkDefinitions(text, seams); err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start)
		fastest, lookups, spent = min(fastest, elapsed), seams.lookups, spent+elapsed
	}
	return fastest, lookups
}

// The growth the cost test allows between a document and one costRatio
// times as large: the lookups, which are counted, in proportion with a
// tenth to spare and a few for the fixed part; the time, which a busy
// machine disturbs, up to timeGrowthLimit times, between the costRatio of
// linear work and the costRatio squared of work growing with its square.
const (
	costRatio       = 4
	timeGrowthLimit = 10
)

func TestTheDefinitionCheckGrowsInStepWithTheDocument(t *testing.T) {
	small, large := costDocuments(maxDocumentBytes/costRatio-1024), costDocuments(maxDocumentBytes-4096)
	for name, text := range small {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(text); err != nil {
				t.Fatalf("the document of the case is refused: %v", err)
			}
			ratio := float64(len(large[name])) / float64(len(text))
			smallTime, smallLookups := fastestCheck(t, text)
			largeTime, largeLookups := fastestCheck(t, large[name])
			if float64(largeLookups) > 1.1*ratio*float64(smallLookups)+64 {
				t.Errorf("%d lookups at %d bytes, %d at %d bytes", smallLookups, len(text),
					largeLookups, len(large[name]))
			}
			if largeTime > timeGrowthLimit*smallTime+2*time.Millisecond {
				t.Errorf("%s at %d bytes, %s at %d bytes", smallTime, len(text), largeTime, len(large[name]))
			}
		})
	}
}

// FuzzTheStatementsReadAreTheReadersKeys fails when, for a document the
// reader accepts, the paths the definition check reads are not the
// reader's key list, or the start of it where the check refuses. It shows
// that the check reads the statements and keys the reader reads; it does
// not show that a refusal is right, for which nothing in the repository is
// an oracle. The seeds are the table's documents and generated ones.
func FuzzTheStatementsReadAreTheReadersKeys(fuzzer *testing.F) {
	for _, text := range seedDocuments(fuzzer) {
		fuzzer.Add(text)
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
		refusal := checkDefinitions(text, &checkSeams{visit: func(path []string) { read = append(read, path) }})
		if refusal == nil && len(read) != len(keys) || len(read) > len(keys) ||
			!slices.EqualFunc(read, keys[:len(read)], slices.Equal) {
			t.Fatalf("read %q (refusal %v), the reader's keys are %q", read, refusal, keys)
		}
	})
}
