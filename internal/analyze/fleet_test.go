package analyze

import (
	"bufio"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// fleetText is a saved inventory output with every shape the reader meets;
// the expected rows, junk count and rollup are what fleet reads from it.
const fleetText = "\xef\xbb\xbf" + `{"type":"agent","host":"orphan","user":"u0","agent":"aider","files":1}` + "\r\n" +
	"banner line\r" +
	`{"type":"host","host":"ws1","collector":"1.6.0","mode":"live","at":"2026-10-01T09:00:00Z","users_scanned":2}` + "\n" +
	`{"type":"agent","host":"ws1","user":"alice","agent":"claude-code","files":3,"bytes":12.5,"first":1759300000,` +
	`"last":"2026-10-02 10:00:00+02:00","projects":true,"evidence":"a,b \"q\"\n"}` + "\n" +
	"  \xef\xbb\xbf" + `{"type":"agent","user":"bob","agent":"codex-cli","files":null,"evidence":[1,{"k":"v"}]}  ` + "\n" +
	"\n   \n" +
	`{"type":"other"}` + "\n" +
	`["type","host"]` + "\n" +
	`{"type":"host","host":"ws2","at":"bad"}` + "\n" +
	`{"type":"host","host":"ws3"}` + "\n" +
	`{"type":"agent","agent":"claude-code","user":"carol","last":"2026-11-01T00:00:00Z"}` + "\n" +
	"{bad json\n" +
	"\xff\xfe junk"

func TestFleetRowsReadEachRecordShape(t *testing.T) {
	text := strings.TrimPrefix(textlines.ValidUTF8([]byte(fleetText)), "\uFEFF")
	rows, junk := fleetRows(text)
	want := [][]string{
		{"orphan", "", "", "", "u0", "aider", "1", "", "", "", "", ""},
		{"ws1", "1.6.0", "live", "2026-10-01T09:00:00.000Z", "alice", "claude-code", "3", "12.5",
			"2025-10-01T06:26:40.000Z", "2026-10-02T08:00:00.000Z", "true", "a,b \"q\"\n"},
		{"ws1", "1.6.0", "live", "2026-10-01T09:00:00.000Z", "bob", "codex-cli", "", "", "", "", "",
			`[1,{"k":"v"}]`},
		{"ws2", "", "", "", "", "", "", "", "", "", "", ""},
		{"ws3", "", "", "", "carol", "claude-code", "", "", "", "2026-11-01T00:00:00.000Z", "", ""},
	}
	if !reflect.DeepEqual(rows, want) || junk != 5 {
		t.Errorf("rows %q, junk %d\nwant %q, junk 5", rows, junk, want)
	}
	totals := rollup(rows)
	wantTotals := []agentTotal{{"claude-code", 2, 2, "2026-11-01T00:00:00.000Z"}, {"aider", 1, 1, ""},
		{"codex-cli", 1, 1, ""}}
	if !reflect.DeepEqual(totals, wantTotals) {
		t.Errorf("rollup %+v, want %+v", totals, wantTotals)
	}
}

func TestFleetLinesEndAtLFCRLFOrALoneCR(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"LF and CR LF end a line", "a\nb\r\nc", []string{"a", "b", "c"}},
		{"a lone CR ends a line", "a\rb\n", []string{"a", "b"}},
		{"a CR at the end ends the last line", "a\r", []string{"a"}},
		{"a blank line between CR LF ends is kept", "a\r\n\r\nb", []string{"a", "", "b"}},
		{"a final LF gives no empty line", "a\n\n", []string{"a", ""}},
		{"no text gives no line", "", nil},
		{"a line longer than the scanner's default buffer is one line", strings.Repeat("x", 70000),
			[]string{strings.Repeat("x", 70000)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := inputLines(test.text); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestALineEndSplitAcrossReadsIsOneEnd(t *testing.T) {
	scanner := bufio.NewScanner(iotest.OneByteReader(strings.NewReader("a\r\nb\rc")))
	scanner.Split(scanLineEnds)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(lines, want) {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestFleetWritesTheCSVAndPrintsTheRollup(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "in", "b.txt"), fleetText)
	writeFile(t, filepath.Join(work, "in", "a.txt"), `{"type":"host","host":"ws9"}`+"\n")
	if err := osfile.MkdirAll(filepath.Join(work, "in", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(work, "out", "deeper", "fleet.csv")
	code, stdout, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "in"))
	wantStdout := "rows:   6 from 5 hosts in {ROOT}/out/deeper/fleet.csv\n\n" +
		"agent                   hosts  users  latest\n" +
		"claude-code                 2      2  2026-11-01T00:00:00.000Z\n" +
		"aider                       1      1  \n" +
		"codex-cli                   1      1  \n"
	stdout, stderr = slashRoot(showRoot(stdout, work)), slashRoot(showRoot(stderr, work))
	if code != ExitFound || stdout != wantStdout ||
		stderr != "skipped 5 non-inventory lines in {ROOT}/in/b.txt\n" {
		t.Errorf("exit %d, stdout:\n%s\nstderr: %q", code, stdout, stderr)
	}
	data, err := osfile.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	wantCSV := "host,collector,mode,at,user,agent,files,bytes,first,last,projects,evidence\n" +
		"ws9,,,,,,,,,,,\n" + "orphan,,,,u0,aider,1,,,,,\n"
	if !strings.HasPrefix(string(data), wantCSV) || strings.Contains(string(data), "\r") {
		t.Errorf("CSV:\n%q", data)
	}
}

func TestFleetRefusesInputsItCannotUse(t *testing.T) {
	work := t.TempDir()
	tests := []struct {
		name   string
		inputs []string
		code   int
		stderr string
	}{
		{"a missing input", []string{filepath.Join(work, "nope.txt")}, ExitError,
			"error: {ROOT}/nope.txt: no such file or directory\n"},
		{"a file without records exits 1", []string{filepath.Join(work, "junk.txt")}, ExitNothing,
			"skipped 1 non-inventory line in {ROOT}/junk.txt\n"},
	}
	writeFile(t, filepath.Join(work, "junk.txt"), "hello\n")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"fleet", "-o", filepath.Join(work, "f.csv")}, test.inputs...)
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != test.code || slashRoot(showRoot(stderr, work)) != test.stderr {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
}

func TestSpreadsheetTextKeepsAFormulaFromRunning(t *testing.T) {
	for _, test := range []struct {
		name, text, want string
	}{
		{"an equals sign", "=cmd|' /C calc'!A0", "'=cmd|' /C calc'!A0"},
		{"a plus sign", "+SUM(1)", "'+SUM(1)"},
		{"a minus sign", "-y", "'-y"},
		{"an at sign", "@x", "'@x"},
		{"a tab", "\t=1", "'\t=1"},
		{"a carriage return", "\r=1", "'\r=1"},
		{"a space before an equals sign starts no formula", " =1", " =1"},
		{"an empty cell", "", ""},
		{"a cell that already starts with a quote", "'=1", "'=1"},
		{"a full-width equals sign starts no formula", "＝1", "＝1"},
		{"a cell of several lines", "=a\nb", "'=a\nb"},
		{"plain text", "claude-code", "claude-code"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := spreadsheetText(test.text); got != test.want {
				t.Errorf("spreadsheetText(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestFleetWritesEvidenceTextAsTextAndNumbersAsNumbers(t *testing.T) {
	rows, _ := fleetRows(`{"type":"host","host":"=cmd|' /C calc'!A0","collector":"+1","mode":"@m"}` + "\n" +
		`{"type":"agent","user":"-u","agent":"a","files":-5,"bytes":"-6","last":"=x","evidence":["@x","-y"]}`)
	want := [][]string{{"'=cmd|' /C calc'!A0", "'+1", "'@m", "", "'-u", "a", "-5", "'-6", "", "", "",
		`["@x","-y"]`}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows %q\nwant %q", rows, want)
	}
}
