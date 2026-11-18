package tomldoc

import (
	"strconv"
	"strings"
	"testing"
)

// nestedArray returns a document whose one value is an empty array nested
// depth levels deep.
func nestedArray(depth int) string {
	return "a = " + strings.Repeat("[", depth) + strings.Repeat("]", depth) + "\n"
}

// nestedInline returns a document whose one value is an inline table nested
// depth levels deep.
func nestedInline(depth int) string {
	return "a = " + strings.Repeat("{b=", depth) + "1" + strings.Repeat("}", depth) + "\n"
}

// dottedKey returns a key of parts parts.
func dottedKey(parts int) string {
	return strings.TrimSuffix(strings.Repeat("a.", parts), ".")
}

func TestParseAcceptsNestingUpToTheLimit(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		// The key a is one level, so a value under it may nest one fewer.
		{"an array nested to the limit", nestedArray(maxDepth - 1)},
		{"an inline table nested to the limit", nestedInline(maxDepth - 1)},
		{"a table header of as many parts as the limit", "[" + dottedKey(maxDepth) + "]\n"},
		{"an array of tables header of as many parts as the limit", "[[" + dottedKey(maxDepth) + "]]\n"},
		{"a dotted key of as many parts as the limit", dottedKey(maxDepth) + " = 1\n"},
		{"a header and a key that together reach the limit", "[" + dottedKey(maxDepth-2) + "]\nb.c = 1\n"},
		{"a header and an inline table that together reach the limit",
			"[" + dottedKey(maxDepth-3) + "]\nb = {c = [1]}\n"},
		{"a float in an array at the limit is not a key part",
			"a = " + strings.Repeat("[", maxDepth-2) + "1.5" + strings.Repeat("]", maxDepth-2) + "\n"},
		{"a dotted key inside nested inline tables that reaches the limit",
			"a = " + strings.Repeat("{b=", maxDepth/2-1) + "{" + dottedKey(maxDepth/2) + "=1" +
				strings.Repeat("}", maxDepth/2) + "\n"},
		{"a float in the deepest array", nestedArray(maxDepth - 1)[:4+maxDepth-1] + "1.5" + nestedArray(maxDepth - 1)[4+maxDepth-1:]},
		{"a long array of floats on one line", "a = [" + strings.Repeat("1.5, ", 1000) + "1.5]\n"},
		{"a later header starts again from the top",
			"[" + dottedKey(maxDepth) + "]\n[b." + dottedKey(maxDepth-1) + "]\n"},
		{"each element of an array is counted on its own",
			"a = [" + strings.Repeat("{b = [1]}, ", 100) + "{b = 1}]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.text); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestParseRefusesNestingPastTheLimit(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"an array nested one past the limit", nestedArray(maxDepth)},
		{"an inline table nested one past the limit", nestedInline(maxDepth)},
		{"a table header of one part past the limit", "[" + dottedKey(maxDepth+1) + "]\n"},
		{"an array of tables header of one part past the limit", "[[" + dottedKey(maxDepth+1) + "]]\n"},
		{"a dotted key of one part past the limit", dottedKey(maxDepth+1) + " = 1\n"},
		{"a header and a key that together pass the limit", "[" + dottedKey(maxDepth-1) + "]\nb.c = 1\n"},
		{"a header and an inline table that together pass the limit",
			"[" + dottedKey(maxDepth-2) + "]\nb = {c = [1]}\n"},
		{"a dotted key inside nested inline tables",
			"a = " + strings.Repeat("{b=", maxDepth/2) + "{" + dottedKey(maxDepth/2) + "=1" +
				strings.Repeat("}", maxDepth/2+1) + "\n"},
		{"an array hundreds of thousands of levels deep", nestedArray(500000)},
		{"an inline table thousands of levels deep", nestedInline(20000)},
		{"a table header thousands of parts long", "[" + dottedKey(20000) + "]\n"},
		{"nesting after a string that holds a bracket", `a = ["]", ` + nestedArray(maxDepth)[4:]},
		{"nesting after a comment ended by a carriage return", "# [\r" + nestedArray(maxDepth)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.text)
			if err == nil || !strings.Contains(err.Error(), "nests keys, tables and arrays more than") {
				t.Fatalf("got %v, want the nesting refused", err)
			}
		})
	}
}

func TestParseIgnoresBracketsInStringsAndComments(t *testing.T) {
	deep := strings.Repeat("[{", maxDepth)
	tests := []struct {
		name string
		text string
	}{
		{"a basic string", `a = "` + deep + `"`},
		{"a basic string with an escaped quote", `a = "\"` + deep + `\\"`},
		{"a basic string ending in an escaped backslash", `a = ["\\", "` + deep + `"]`},
		{"a multi-line basic string", `a = """` + "\n" + deep + "\n" + `"""`},
		{"a multi-line basic string with quotes before its end", `a = """` + deep + `"" """`},
		{"a multi-line basic string ending in two quotes", `a = ["""x""""", "` + deep + `"]`},
		{"a multi-line basic string with an escaped quote", `a = """\"""` + deep + `"""`},
		{"a multi-line basic string with a line-ending backslash", "a = \"\"\"x \\\n" + deep + `"""`},
		{"a literal string", `a = '` + deep + `'`},
		{"a literal string ending in a backslash", `a = ['\', '` + deep + `']`},
		{"a multi-line literal string", "a = '''\n" + deep + "\n'''"},
		{"a multi-line literal string ending in two quotes", `a = ['''x''''', '` + deep + `']`},
		{"an empty string before another", `a = ["", '', "` + deep + `"]`},
		{"a quoted key", `"` + deep + `" = 1`},
		{"a quoted key part with dots", `a."` + strings.Repeat(".", 100) + `".b = 1`},
		{"a quoted table name", `["` + deep + `"]`},
		{"a comment", "# " + deep + "\na = 1"},
		{"a comment inside an array", "a = [ # " + deep + "\n1]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.text + "\n"); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestParseReadsALargeDocumentOfLittleDepth(t *testing.T) {
	const keys = 200000
	var text strings.Builder
	for i := range keys {
		text.WriteString("k" + strconv.Itoa(i) + " = 1\n")
	}
	if text.Len() < 2<<20 {
		t.Fatalf("the document is only %d bytes", text.Len())
	}
	document, err := Parse(text.String())
	if err != nil {
		t.Fatalf("a flat document of %d bytes: %v", text.Len(), err)
	}
	if got := document.Len(); got != keys {
		t.Errorf("read %d keys, want %d", got, keys)
	}
}
func TestSettingTextWritesAValueAtTheDepthLimit(t *testing.T) {
	table, err := Parse(nestedArray(maxDepth - 1))
	if err != nil {
		t.Fatal(err)
	}
	value, _ := table.Get("a")
	want := strings.Repeat("[", maxDepth-1) + strings.Repeat("]", maxDepth-1)
	if got := SettingText(value); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
