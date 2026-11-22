package tomldoc

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

// differenceFromReader returns where a parsed value differs from the
// reader's own map of the same document, or "" when it holds the same
// tables with the same keys, the same arrays with the same lengths and the
// same scalars; only the order of a table's keys may differ.
func differenceFromReader(value Value, raw any, place string) string {
	switch expected := raw.(type) {
	case map[string]any:
		return tableDifference(value, expected, place)
	case []map[string]any:
		return arrayDifference(value, expected, place)
	case []any:
		return arrayDifference(value, expected, place)
	case time.Time:
		if value.kind != temporal(expected).kind || value.text != temporal(expected).text {
			return place + ": a date or time differs"
		}
		return ""
	}
	if !sameScalar(value, raw) {
		return fmt.Sprintf("%s: got %s, the reader has %#v", place, render(value), raw)
	}
	return ""
}

// tableDifference compares a parsed table with the reader's table.
func tableDifference(value Value, raw map[string]any, place string) string {
	if value.kind != KindTable || value.table.Len() != len(raw) {
		return fmt.Sprintf("%s: got %s, the reader has a table of %d keys", place, render(value), len(raw))
	}
	for _, entry := range value.table.Entries() {
		child, ok := raw[entry.Key]
		if !ok {
			return fmt.Sprintf("%s: key %q is not the reader's", place, entry.Key)
		}
		if difference := differenceFromReader(entry.Value, child, place+"."+entry.Key); difference != "" {
			return difference
		}
	}
	return ""
}

// arrayDifference compares a parsed array with the reader's array.
func arrayDifference[Element any](value Value, raw []Element, place string) string {
	if value.kind != KindArray || len(value.items) != len(raw) {
		return fmt.Sprintf("%s: got %s, the reader has an array of %d", place, render(value), len(raw))
	}
	for i, element := range raw {
		if difference := differenceFromReader(value.items[i], element, fmt.Sprintf("%s[%d]", place, i)); difference != "" {
			return difference
		}
	}
	return ""
}

// sameScalar says whether a parsed scalar is the reader's, a NaN matching a
// NaN.
func sameScalar(value Value, raw any) bool {
	switch expected := raw.(type) {
	case string:
		return value.kind == KindString && value.text == expected
	case int64:
		return value.kind == KindInteger && value.integer == expected
	case float64:
		return value.kind == KindFloat && (value.float == expected ||
			math.IsNaN(value.float) && math.IsNaN(expected))
	case bool:
		return value.kind == KindBoolean && value.boolean == expected
	}
	return false
}

// generatedKeys are the keys the generated documents use: each other's
// text, the empty key, and dotted keys made of them.
var generatedKeys = []string{`0`, `1`, `""`, `a`, `0.0`, `"".0`, `a.""`, `0.a`}

// generatedDocuments returns count small documents from a fixed seed: keys
// given arrays of inline tables, inline tables and scalars, under the root,
// a table header or an array of tables header, with the keys of
// generatedKeys colliding with each other and with the arrays' own keys.
func generatedDocuments(count int) []string {
	random := rand.New(rand.NewPCG(570, 1))
	documents := make([]string, count)
	for i := range documents {
		var text strings.Builder
		for range 1 + random.IntN(3) {
			switch random.IntN(4) {
			case 0:
				text.WriteString("[" + generatedKey(random) + "]\n")
			case 1:
				text.WriteString("[[" + generatedKey(random) + "]]\n")
			}
			for range 1 + random.IntN(2) {
				text.WriteString(generatedKey(random) + " = " + generatedValue(random, 3) + "\n")
			}
		}
		documents[i] = text.String()
	}
	return documents
}

// generatedKey returns one of generatedKeys.
func generatedKey(random *rand.Rand) string {
	return generatedKeys[random.IntN(len(generatedKeys))]
}

// generatedValue returns a value nested at most depth levels: mostly an
// array of inline tables.
func generatedValue(random *rand.Rand, depth int) string {
	if depth == 0 {
		return []string{`""`, `1`, `true`, `1979-05-27`}[random.IntN(4)]
	}
	parts := []string{}
	switch random.IntN(5) {
	case 0, 1, 2:
		for range random.IntN(4) {
			parts = append(parts, generatedInline(random, depth-1))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 3:
		return generatedInline(random, depth-1)
	}
	return generatedValue(random, 0)
}

// generatedInline returns an inline table of up to three keys.
func generatedInline(random *rand.Rand, depth int) string {
	parts := []string{}
	for range random.IntN(4) {
		parts = append(parts, generatedKey(random)+" = "+generatedValue(random, depth))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// readerMap returns the reader's own map of a document.
func readerMap(t *testing.T, text string) map[string]any {
	t.Helper()
	var mapping map[string]any
	if _, err := toml.Decode(strings.ReplaceAll(text, "\r\n", "\n"), &mapping); err != nil {
		t.Fatalf("the reader refuses what Parse accepted: %v", err)
	}
	return mapping
}

func TestParseGivesTheReadersValuesWithTheDocumentsOrder(t *testing.T) {
	texts := generatedDocuments(4000)
	for _, test := range loadDocuments(t) {
		texts = append(texts, test.text)
	}
	accepted := 0
	for _, text := range texts {
		table, err := Parse(text)
		if err != nil {
			continue
		}
		accepted++
		difference := differenceFromReader(Value{kind: KindTable, table: table}, readerMap(t, text), "")
		if difference != "" {
			t.Errorf("document %q: %s", text, difference)
		}
	}
	if accepted < len(texts)/8 {
		t.Fatalf("only %d of %d documents were accepted, too few to test", accepted, len(texts))
	}
}

func TestATableWhoseOrderTheWalkCannotGiveHasItsKeysSorted(t *testing.T) {
	tests := []struct {
		name   string
		raw    map[string]any
		walked *Table
		want   []string
	}{
		{"no walk sorts the keys", map[string]any{"b": int64(1), "a": int64(2)}, nil, []string{"a", "b"}},
		{"a walk with a key the reader lacks sorts the keys", map[string]any{"b": int64(1), "a": int64(2)},
			tableOf("b", "a", "c"), []string{"a", "b"}},
		{"a walk missing a key sorts the keys", map[string]any{"b": int64(1), "a": int64(2)},
			tableOf("b"), []string{"a", "b"}},
		{"a walk with the same number of other keys sorts the keys",
			map[string]any{"b": int64(1), "a": int64(2)}, tableOf("b", "c"), []string{"a", "b"}},
		{"a walk of exactly the keys keeps its order", map[string]any{"b": int64(1), "a": int64(2)},
			tableOf("b", "a"), []string{"b", "a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := orderedTable(test.raw, test.walked)
			keys := []string{}
			for _, entry := range table.Entries() {
				keys = append(keys, entry.Key)
			}
			if !slices.Equal(keys, test.want) {
				t.Fatalf("got keys %q, want %q", keys, test.want)
			}
			if difference := differenceFromReader(Value{kind: KindTable, table: table}, test.raw, ""); difference != "" {
				t.Fatal(difference)
			}
		})
	}
}

// tableOf returns a table of the keys, in that order, each holding 0.
func tableOf(keys ...string) *Table {
	table := &Table{}
	for _, key := range keys {
		table.set(key, Value{kind: KindInteger})
	}
	return table
}
