package sqlitefile

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// measuredDefaults is testdata/default_values.jsonl.gz, which
// testdata/default_values.py writes from what SQLite reads from a record
// too short to hold a column: the declared types, and per DEFAULT clause
// one reading per type, typeof and quote of the value (hex of a text), nil
// where SQLite refused the column.
type measuredDefaults struct {
	types    []string
	clauses  []string
	readings [][]*[2]string
}

// loadMeasuredDefaults parses testdata/default_values.jsonl.gz.
func loadMeasuredDefaults(t *testing.T) measuredDefaults {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", "default_values.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	lines, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var measured measuredDefaults
	scanner := bufio.NewScanner(lines)
	var header struct{ Types []string }
	if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &header) != nil {
		t.Fatal("default_values.jsonl.gz has no header line")
	}
	measured.types = header.Types
	for scanner.Scan() {
		var line struct {
			Default string
			Results []*[2]string
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil || len(line.Results) != len(measured.types) {
			t.Fatalf("default_values.jsonl.gz line %q: %v", scanner.Text(), err)
		}
		measured.clauses = append(measured.clauses, line.Default)
		measured.readings = append(measured.readings, line.Results)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return measured
}

// quoteValue writes a value as default_values.jsonl.gz does, with SQLite's
// typeof and quote functions, except a text, written as hex, and a float,
// written by strconv and compared by value.
func quoteValue(value Value) [2]string {
	switch value.Type {
	case Integer:
		return [2]string{"integer", strconv.FormatInt(value.Int, 10)}
	case Float:
		return [2]string{"real", strconv.FormatFloat(value.Float, 'g', -1, 64)}
	case Text:
		return [2]string{"text", strings.ToUpper(hex.EncodeToString(value.Bytes))}
	case Blob:
		return [2]string{"blob", "X'" + strings.ToUpper(hex.EncodeToString(value.Bytes)) + "'"}
	}
	return [2]string{"null", "NULL"}
}

// sameReading reports whether the reader's value is the one SQLite read:
// the same type and quoted text, a float the same number, sign included,
// with quote's 9.0e+999 for infinity.
func sameReading(got [2]string, want [2]string) bool {
	if got[0] != "real" || want[0] != "real" {
		return got == want
	}
	number, _ := strconv.ParseFloat(got[1], 64)
	text := strings.Replace(want[1], "9.0e+999", "Inf", 1)
	expected, err := strconv.ParseFloat(text, 64)
	return err == nil && number == expected && math.Signbit(number) == math.Signbit(expected)
}

func TestAShortRecordReadsTheDefaultSQLiteReads(t *testing.T) {
	measured := loadMeasuredDefaults(t)
	compared := 0
	for column, declared := range measured.types {
		t.Run("a column declared "+strconv.Quote(declared)+" reads each DEFAULT as SQLite does", func(t *testing.T) {
			for i, clause := range measured.clauses {
				want := measured.readings[i][column]
				if want == nil {
					continue
				}
				compared++
				statement := "CREATE TABLE t(id, c " + declared + " DEFAULT " + clause + ")"
				shape, err := parseCreateTable(statement)
				if err != nil {
					t.Errorf("DEFAULT %s: %v", clause, err)
					continue
				}
				got, defaultErr := shape.columns[1].Default, shape.columns[1].defaultErr
				if defaultErr != nil || !sameReading(quoteValue(got), *want) {
					t.Errorf("DEFAULT %s: got %v %v, %v; SQLite reads %v", clause, quoteValue(got)[0],
						quoteValue(got)[1], defaultErr, *want)
				}
			}
		})
	}
	if compared != 25555 {
		t.Errorf("compared %d readings, want every reading of default_values.jsonl.gz", compared)
	}
}

func TestACastThatStartsALargerDefaultIsNull(t *testing.T) {
	// From SQLite 3.53.4 and 3.51.0, each the DEFAULT of a column added by
	// rewriting the schema with PRAGMA writable_schema, since ALTER TABLE
	// refuses a value it does not evaluate: every one reads as NULL.
	for _, clause := range []string{"(CAST(1 AS INT) COLLATE NOCASE)", "(CAST(1 AS INT) + 1)",
		"(CAST('a' AS TEXT) || 'b')", "(- CAST(1 AS INT) + 1)"} {
		t.Run("DEFAULT "+clause, func(t *testing.T) {
			shape, err := parseCreateTable("CREATE TABLE t(id, c DEFAULT " + clause + ")")
			if err != nil || shape.columns[1].defaultErr != nil || shape.columns[1].Default.Type != Null {
				t.Errorf("got %v, %v (%v); want NULL", shape.columns[1].Default, shape.columns[1].defaultErr, err)
			}
		})
	}
}
