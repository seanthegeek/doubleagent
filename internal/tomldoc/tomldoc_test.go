package tomldoc

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// render writes a parsed value in the form the oracle script prints.
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

// knownDifferences are the oracle cases where this reader and Python's
// tomllib differ, with what each does.
func knownDifferences() map[string]string {
	return map[string]string{
		"an integer beyond int64":             "Python reads any size of integer; this reader stops at 64 bits",
		"a UTF-8 byte order mark":             "Python rejects a leading byte order mark; this reader skips it",
		"an array with a nested inline array": "keys of a table nested in an array's array are sorted, not in document order",
		"a datetime with a long fraction":     "Python keeps microseconds; this reader keeps every digit the text gives, up to nanoseconds",
	}
}

type oracleCase struct {
	label, text, structure, settings string
}

func loadOracle(t testing.TB) []oracleCase {
	t.Helper()
	file, err := os.Open("testdata/oracle.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var cases []oracleCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		text, err := hex.DecodeString(parts[1])
		if err != nil || len(parts) != 4 {
			t.Fatalf("bad oracle line %q", scanner.Text())
		}
		cases = append(cases, oracleCase{parts[0], string(text), parts[2], parts[3]})
	}
	return cases
}

func TestParseMatchesTheTomllibOracle(t *testing.T) {
	known := knownDifferences()
	for _, test := range loadOracle(t) {
		t.Run(test.label, func(t *testing.T) {
			table, err := Parse(test.text)
			if _, differs := known[test.label]; differs {
				t.Skip(known[test.label])
			}
			if test.structure == "ERR" {
				if err == nil {
					t.Fatal("want a syntax error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			want := oracleTemporals(t, test.structure)
			if got := render(Value{kind: KindTable, table: table}); got != want {
				t.Fatalf("got  %s\nwant %s", got, want)
			}
		})
	}
}

// oracleTemporal is a temporal value in the oracle's rendering: its kind
// and its text in hex.
var oracleTemporal = regexp.MustCompile(`(dt|ndt|date|time):([0-9a-f]*)`)

// oracleTemporals returns the oracle's rendering with each date and time,
// which the oracle wrote as a Python date or time object prints (a space
// between date and clock, an offset as +HH:MM, six fraction digits), in the
// RFC 3339 text temporal gives the same value.
func oracleTemporals(t *testing.T, structure string) string {
	return oracleTemporal.ReplaceAllStringFunc(structure, func(match string) string {
		parts := oracleTemporal.FindStringSubmatch(match)
		text, err := hex.DecodeString(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		layouts := map[string]string{"dt": "2006-01-02 15:04:05.999999999-07:00",
			"ndt": "2006-01-02 15:04:05.999999999", "date": time.DateOnly, "time": "15:04:05.999999999"}
		zones := map[string]string{"ndt": "datetime-local", "date": "date-local", "time": "time-local"}
		moment, err := time.Parse(layouts[parts[1]], string(text))
		if err != nil {
			t.Fatalf("oracle temporal %q: %v", text, err)
		}
		if zone, ok := zones[parts[1]]; ok {
			moment = time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(),
				moment.Minute(), moment.Second(), moment.Nanosecond(), time.FixedZone(zone, 0))
		}
		return parts[1] + ":" + hex.EncodeToString([]byte(temporal(moment).Text()))
	})
}

func TestKnownDifferencesAreRealAndNamedInTheOracle(t *testing.T) {
	cases := map[string]bool{}
	for _, test := range loadOracle(t) {
		cases[test.label] = true
	}
	for label := range knownDifferences() {
		if !cases[label] {
			t.Errorf("no oracle case %q", label)
		}
	}
}

func TestTheDifferencesAreWhatTheyAreSaidToBe(t *testing.T) {
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

func FuzzParse(fuzzer *testing.F) {
	for _, test := range loadOracle(fuzzer) {
		fuzzer.Add(test.text)
	}
	fuzzer.Fuzz(func(t *testing.T, text string) {
		table, err := Parse(text)
		if err != nil {
			return
		}
		for _, entry := range table.Entries() {
			SettingText(entry.Value)
		}
	})
}
