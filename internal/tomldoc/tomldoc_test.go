package tomldoc

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// render writes a parsed value as compact text: a table as {key=value;...}
// and an array as [value,...], keys and strings in hex, a float as its bits,
// and a temporal value as its kind and its text in hex.
func render(value Value) string {
	switch value.Kind() {
	case KindTable:
		parts := []string{}
		for _, entry := range value.Table().Entries() {
			parts = append(parts, hex.EncodeToString([]byte(entry.Key))+"="+render(entry.Value))
		}
		return "{" + strings.Join(parts, ";") + "}"
	case KindArray:
		parts := []string{}
		for _, item := range value.Items() {
			parts = append(parts, render(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case KindBoolean:
		return fmt.Sprint(value.Bool())
	case KindInteger:
		return fmt.Sprint(value.Int())
	case KindFloat:
		if math.IsNaN(value.Float()) {
			return "fnan"
		}
		return fmt.Sprintf("f%016x", math.Float64bits(value.Float()))
	case KindDatetime:
		return "dt:" + hex.EncodeToString([]byte(value.Text()))
	case KindLocalDatetime:
		return "ndt:" + hex.EncodeToString([]byte(value.Text()))
	case KindLocalDate:
		return "date:" + hex.EncodeToString([]byte(value.Text()))
	case KindLocalTime:
		return "time:" + hex.EncodeToString([]byte(value.Text()))
	}
	return "s" + hex.EncodeToString([]byte(value.Text()))
}

// document is one line of testdata/documents.txt: a label, a TOML document
// and its parsed form as render writes it, or ERR for a document Parse
// refuses.
type document struct {
	label, text, structure string
}

// loadDocuments reads testdata/documents.txt.
func loadDocuments(t testing.TB) []document {
	t.Helper()
	file, err := os.Open("testdata/documents.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var documents []document
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 3 {
			t.Fatalf("bad document line %q", scanner.Text())
		}
		text, err := hex.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("bad document line %q", scanner.Text())
		}
		documents = append(documents, document{parts[0], string(text), parts[2]})
	}
	return documents
}

func TestParseReadsEachDocument(t *testing.T) {
	for _, test := range loadDocuments(t) {
		t.Run(test.label, func(t *testing.T) {
			table, err := Parse(test.text)
			if test.structure == "ERR" {
				if err == nil {
					t.Fatal("want a syntax error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if got := render(Value{kind: KindTable, table: table}); got != test.structure {
				t.Fatalf("got  %s\nwant %s", got, test.structure)
			}
		})
	}
}

func TestParseLimits(t *testing.T) {
	// TOML integers are 64-bit, a leading byte order mark is skipped, and
	// an inline table nested in an array's array has its keys sorted.
	if _, err := Parse("a = 9223372036854775808\n"); err == nil {
		t.Error("an integer beyond int64 should be an error")
	}
	table, err := Parse("\xef\xbb\xbfa = 1\n")
	if err != nil || table.Len() != 1 {
		t.Errorf("a byte order mark: %v", err)
	}
	table, err = Parse("a = [[{ b = 1, a = 2 }]]\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := render(Value{kind: KindTable, table: table}); got != "{61=[[{61=2;62=1}]]}" {
		t.Errorf("nested inline table keys are sorted: %s", got)
	}
}

func TestSyntaxErrorsNameTheLineAndColumn(t *testing.T) {
	_, err := Parse("a = 1\nb = \n")
	if err == nil || !strings.HasPrefix(err.Error(), "invalid TOML at line 2, column ") {
		t.Fatalf("got %v", err)
	}
}

func TestSettingTextFormats(t *testing.T) {
	table, err := Parse("a = true\nb = \"text\"\nc = 7\nd = 1.0\ne = [1, 2.5, \"é\\n\"]\nf = {z = 1, a = {y = false}}\ng = 1e16\nh = 1e-5\ni = 1979-05-27T07:32:00Z\nj = []\nk = {}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"a": "true", "b": "text", "c": "7", "d": "1", "e": `[1,2.5,"é\n"]`,
		"f": `{"z":1,"a":{"y":false}}`, "g": "1e+16", "h": "1e-05",
		"i": "1979-05-27T07:32:00Z", "j": "[]", "k": "{}",
	}
	for _, entry := range table.Entries() {
		if got := SettingText(entry.Value); got != want[entry.Key] {
			t.Errorf("%s: got %q want %q", entry.Key, got, want[entry.Key])
		}
	}
}

func TestAccessorsReadValues(t *testing.T) {
	table, err := Parse("n = 5\nf = 2.5\nb = true\ns = \"x\"\nl = [1]\n[t]\nk = 1\n")
	if err != nil {
		t.Fatal(err)
	}
	number, _ := table.Get("n")
	float, _ := table.Get("f")
	boolean, _ := table.Get("b")
	text, _ := table.Get("s")
	list, _ := table.Get("l")
	nested, _ := table.Get("t")
	if number.Int() != 5 || float.Float() != 2.5 || !boolean.Bool() || text.Text() != "x" ||
		len(list.Items()) != 1 || nested.Table().Len() != 1 || table.Len() != 6 {
		t.Fatal("accessors returned the wrong values")
	}
	if _, ok := table.Get("missing"); ok {
		t.Fatal("Get found a missing key")
	}
}

// seedDocuments returns the table's documents and generated ones, the
// seeds of the fuzz targets.
func seedDocuments(t testing.TB) []string {
	texts := generatedDocuments(500)
	for _, test := range loadDocuments(t) {
		texts = append(texts, test.text)
	}
	return texts
}

// FuzzParse fails when Parse panics, or accepts a document and returns
// values other than the reader's own map of it (differenceFromReader). It
// does not show that a document Parse refuses is invalid: nothing in the
// repository is an oracle for that, and a refusal is not checked.
func FuzzParse(fuzzer *testing.F) {
	for _, text := range seedDocuments(fuzzer) {
		fuzzer.Add(text)
	}
	for _, text := range []string{nestedArray(maxDepth), nestedInline(maxDepth),
		"[" + dottedKey(maxDepth+1) + "]\n", nestedArray(100000)} {
		fuzzer.Add(text)
	}
	fuzzer.Fuzz(func(t *testing.T, text string) {
		table, err := Parse(text)
		if err != nil {
			return
		}
		for _, entry := range table.Entries() {
			SettingText(entry.Value)
		}
		difference := differenceFromReader(Value{kind: KindTable, table: table}, readerMap(t, text), "")
		if difference != "" {
			t.Fatal(difference)
		}
	})
}

func TestSyntaxErrorKeepsTheReadersError(t *testing.T) {
	_, err := Parse("a = \n")
	var parseErr toml.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("got %v, want the reader's ParseError as the cause", err)
	}
}

func TestInlineTableKeyOrder(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"an inline table in a key's array keeps document order", "y = [1, {z=1,a=2}]\n", "{79=[1,{7a=1;61=2}]}"},
		{"a sibling holding a table sorts the keys", "y = [1, {z=1,a=2}, [{c=1}]]\n", "{79=[1,{61=2;7a=1},[{63=1}]]}"},
		{"an inline table holding a table sorts the keys", "y = [{z=1,a={b=1}}]\n", "{79=[{61={62=1};7a=1}]}"},
		{"an array inside an array sorts the keys", "y = [[{z=1,a=2}]]\n", "{79=[[{61=2;7a=1}]]}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table, err := Parse(test.text)
			if err != nil {
				t.Fatal(err)
			}
			if got := render(Value{kind: KindTable, table: table}); got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
