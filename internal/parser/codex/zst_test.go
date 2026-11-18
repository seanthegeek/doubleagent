package codex

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The frames under testdata are built by testdata/gen_zst.py; the cut
// tables below are what a reader of the first n bytes of each file must
// give: from the cut n up to the next row's, the line numbers read and the
// problems as line:kind, where kind is zstd for a decoding problem and line
// for a line that is not JSON.

// cutRun is one run of cut lengths that read alike.
type cutRun struct {
	from     int
	lines    string
	problems string
}

// readTestdata returns a file under testdata.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// summary returns the line numbers of records and the problems as the cut
// tables write them.
func summary(records []numberedRecord, problems parser.Problems) (string, string) {
	var lines, kinds []string
	for _, record := range records {
		lines = append(lines, strconv.Itoa(record.line))
	}
	for _, problem := range problems {
		kind := "line"
		if strings.HasPrefix(problem.Message, "zstd: ") {
			kind = "zstd"
		}
		kinds = append(kinds, fmt.Sprintf("%d:%s", problem.Line, kind))
	}
	orDash := func(parts []string) string {
		if len(parts) == 0 {
			return "-"
		}
		return strings.Join(parts, ",")
	}
	return orDash(lines), orDash(kinds)
}

func TestACutStreamKeepsTheLinesOfItsWholeBlocks(t *testing.T) {
	files := []struct {
		name string
		runs []cutRun
	}{
		{"block-per-line.jsonl.zst", []cutRun{{0, "-", "-"}, {1, "-", "1:zstd"},
			{103, "1", "2:zstd"}, {121, "1,2", "3:zstd"}, {139, "1,2,3", "4:zstd"},
			{157, "1,2,3,4", "5:zstd"}, {175, "1,2,3,4,5", "6:zstd"}, {178, "1,2,3,4,5", "-"}}},
		{"two-frames.jsonl.zst", []cutRun{{0, "-", "-"}, {1, "-", "1:zstd"},
			{120, "1,2,3", "-"}, {121, "1,2,3", "4:zstd"}, {237, "1,2,3,4,5", "-"}}},
	}
	for _, file := range files {
		data := readTestdata(t, file.name)
		run := 0
		for n := 0; n <= len(data); n++ {
			for run+1 < len(file.runs) && file.runs[run+1].from <= n {
				run++
			}
			want := file.runs[run]
			t.Run(fmt.Sprintf("%s cut at %d bytes", file.name, n), func(t *testing.T) {
				var problems parser.Problems
				records := zstRecordsOf(bytes.NewReader(data[:n]), "cut.jsonl.zst", &problems)
				lines, kinds := summary(records, problems)
				if lines != want.lines || kinds != want.problems {
					t.Errorf("got lines %s problems %s, want %s and %s", lines, kinds, want.lines,
						want.problems)
				}
			})
		}
	}
}

// failingReader gives its data on the first read and an input/output error
// on every later one, as a failing disk does.
type failingReader struct {
	data []byte
	read bool
}

// Read returns the data once, then the error.
func (f *failingReader) Read(buffer []byte) (int, error) {
	if f.read {
		return 0, syscall.EIO
	}
	f.read = true
	return copy(buffer, f.data), nil
}

func TestAReadThatFailsKeepsTheEarlierLinesAndNamesTheFile(t *testing.T) {
	var problems parser.Problems
	reader := &failingReader{data: readTestdata(t, "two-lines.jsonl.zst")}
	records := zstRecordsOf(reader, filepath.Join("home", "rollout.jsonl.zst"), &problems)
	if lines, _ := summary(records, nil); lines != "1,2" {
		t.Errorf("got lines %s, want 1,2", lines)
	}
	if len(problems) != 1 || problems[0].Line != 2 ||
		!strings.HasPrefix(problems[0].Message, "rollout.jsonl.zst could not be read (") {
		t.Errorf("got problems %+v, want one unreadable problem at line 2", problems)
	}
}

func TestDecodingProblemsReadAsTheBinarysOwnZstdMessages(t *testing.T) {
	data := readTestdata(t, "two-lines.jsonl.zst")
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"text that is no zstd frame", []byte("not a zstd frame at all"), "zstd: corrupt data ("},
		{"a frame cut short", data[:len(data)-5], "zstd: the file ends inside a zstd frame"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parser.Problems
			zstRecordsOf(bytes.NewReader(test.input), "x.jsonl.zst", &problems)
			if len(problems) != 1 || problems[0].Line != 1 ||
				!strings.HasPrefix(problems[0].Message, test.want) {
				t.Errorf("got %+v, want one problem at line 1 starting %q", problems, test.want)
			}
		})
	}
}

func TestZstLinesSkipBlankLinesAndReportEveryOtherLineThatIsNoObject(t *testing.T) {
	// A file of plain JSON lines is no zstd stream, so the lines are given
	// to appendRecord directly, as the decoder hands them over.
	lines := []string{"{\"a\":1}", "", "  ", "[1]", "{bad", "{\"b\":2}\r"}
	var records []numberedRecord
	var problems parser.Problems
	for i, line := range lines {
		records = appendRecord(records, i+1, []byte(line), &problems)
	}
	got, kinds := summary(records, problems)
	if got != "1,6" || kinds != "4:line,5:line" {
		t.Errorf("got lines %s problems %s, want 1,6 and 4:line,5:line", got, kinds)
	}
	if len(problems) == 2 && (problems[0].Raw != "[1]" || problems[1].Raw != "{bad") {
		t.Errorf("got raw %q and %q", problems[0].Raw, problems[1].Raw)
	}
}

func TestACompressedRolloutGivesTheRowsOfThePlainOne(t *testing.T) {
	home := t.TempDir()
	directory := ".codex/sessions/2026/01/02/"
	name := "rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000002.jsonl"
	plain := filepath.Join(home, filepath.FromSlash(directory+name))
	if err := osfile.MkdirAll(filepath.Dir(plain), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(plain, readTestdata(t, "mixed-lines.jsonl"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(plain+".zst", readTestdata(t, "mixed-lines.jsonl.zst"), 0o600); err != nil {
		t.Fatal(err)
	}
	rowsOf := func(rel string) []string {
		var rows []string
		artifact := &input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
			Home: &input.Home{DiskPath: home}}
		New().parseRollout(artifact, parser.Options{}, func(row timeline.Row) {
			rows = append(rows, strconv.Itoa(row.SourceLine)+" "+row.TurnType+" "+row.Text)
		})
		return rows
	}
	want, got := rowsOf(directory+name), rowsOf(directory+name+".zst")
	if !slices.Equal(got, want) || len(want) != 4 {
		t.Errorf("compressed rows %q, plain rows %q: want the same four", got, want)
	}
}

func TestLinesCutAcrossManyChunksAreSearchedOnce(t *testing.T) {
	work := func(chunks int) int {
		var split lineSplitter
		lines := 0
		for range chunks {
			split.add(bytes.Repeat([]byte("a"), 1024), func(int, []byte) { lines++ })
		}
		split.add([]byte("\n"), func(int, []byte) { lines++ })
		if lines != 1 || split.number != 1 {
			t.Fatalf("%d lines numbered up to %d, want one", lines, split.number)
		}
		return split.scanned
	}
	if small, large := work(500), work(1000); small > 500*1024+1 || large > 3*small {
		t.Errorf("%d then %d bytes searched for 500 and 1000 chunks of 1024: not once each", small, large)
	}
}

func TestAMissingZstFileIsOneProblemAtLineZero(t *testing.T) {
	var problems parser.Problems
	path := filepath.Join(t.TempDir(), "rollout-x.jsonl.zst")
	if records := zstRecords(path, &problems); len(records) != 0 {
		t.Errorf("got %d records", len(records))
	}
	if len(problems) != 1 || problems[0].Line != 0 ||
		problems[0].Message != "rollout-x.jsonl.zst could not be read (no such file or directory)" {
		t.Errorf("got %+v", problems)
	}
}

func FuzzZstRecords(fuzz *testing.F) {
	for _, name := range []string{"two-lines.jsonl.zst", "block-per-line.jsonl.zst",
		"two-frames.jsonl.zst"} {
		data, err := osfile.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			fuzz.Fatal(err)
		}
		fuzz.Add(data)
	}
	fuzz.Add([]byte("not zstd"))
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		var problems parser.Problems
		records := zstRecordsOf(bytes.NewReader(data), "fuzz.jsonl.zst", &problems)
		for i := 1; i < len(records); i++ {
			if records[i].line <= records[i-1].line {
				t.Fatalf("line numbers out of order: %d after %d", records[i].line, records[i-1].line)
			}
		}
	})
}
