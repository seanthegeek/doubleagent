//go:build parity

package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// recordingTB is a testing.TB that keeps the errors reported to it instead
// of failing, so a test can check what a helper reports.
type recordingTB struct {
	testing.TB
	errors []string
}

// Helper does nothing; the recorded errors carry no file positions.
func (r *recordingTB) Helper() {}

// Errorf keeps the formatted error.
func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// testCorpus writes a corpus under the test's temporary directory: its
// corpus.json, the given blobs and trees, and the case files, each named by
// its path inside the corpus.
type testCorpus struct {
	t         *testing.T
	directory string
	header    Header
}

// newTestCorpus starts an empty corpus with a root length of 160 bytes.
func newTestCorpus(t *testing.T) *testCorpus {
	t.Helper()
	header := Header{Format: corpusFormat, RootLength: 160, RootToken: "{ROOT}", RepoToken: "{REPO}"}
	header.RootPlaceholder = "{parity-root" + strings.Repeat("_", 160-13) + "}"
	built := &testCorpus{t: t, directory: t.TempDir(), header: header}
	data, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	built.write("corpus.json", string(data))
	return built
}

// write writes one file of the corpus.
func (c *testCorpus) write(name, content string) {
	c.t.Helper()
	path := filepath.Join(c.directory, filepath.FromSlash(name))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		c.t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(content), 0o600); err != nil {
		c.t.Fatal(err)
	}
}

// blob stores content as a blob and returns its hash.
func (c *testCorpus) blob(content string) string {
	c.t.Helper()
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	c.write("blobs/"+digest, content)
	return digest
}

// tree stores entries as a tree object and returns its hash.
func (c *testCorpus) tree(entries ...map[string]any) string {
	c.t.Helper()
	data, err := json.Marshal(map[string]any{"entries": entries})
	if err != nil {
		c.t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	c.write("trees/"+digest+".json", string(data))
	return digest
}

// times stores modification times by root-relative path and returns the
// hash of the times object.
func (c *testCorpus) times(mtimes map[string]int64) string {
	c.t.Helper()
	data, err := json.Marshal(map[string]any{"mtime_ns": mtimes})
	if err != nil {
		c.t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	c.write("times/"+digest+".json", string(data))
	return digest
}

// open opens the written corpus.
func (c *testCorpus) open() *Corpus {
	c.t.Helper()
	corpus, err := OpenDirectory(c.directory)
	if err != nil {
		c.t.Fatal(err)
	}
	return corpus
}

func TestTextReadsPlainAndHexForms(t *testing.T) {
	tests := []struct {
		name string
		json string
		want Text
	}{
		{"a plain string is itself", `"café"`, "café"},
		{"a hex form gives its bytes, a lone surrogate's UTF-8 form kept", `{"hex": "eda0bd61"}`,
			"\xed\xa0\xbda"},
		{"a hex form of a name gives raw bytes", `{"hex":"ff2f"}`, "\xff/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got Text
			if err := json.Unmarshal([]byte(test.json), &got); err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestOpenDirectoryRefusesAnotherFormatOrPlaceholder(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"a newer format is refused", `{"format":2,"root_length":3,"root_placeholder":"abc"}`,
			"format 2"},
		{"a placeholder of the wrong length is refused",
			`{"format":1,"root_length":4,"root_placeholder":"abc"}`, "3 bytes for a root of 4"},
		{"a header without tokens is refused",
			`{"format":1,"root_length":3,"root_placeholder":"abc"}`, "no root or repository token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "corpus.json")
			if err := osfile.WriteFile(path, []byte(test.header), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := OpenDirectory(directory)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("got %v, want an error naming %q", err, test.want)
			}
		})
	}
}

func TestOpenSkipsWithoutACorpus(t *testing.T) {
	t.Setenv("PARITY_CORPUS", "")
	reached := false
	t.Run("the corpus is opened", func(t *testing.T) {
		Open(t)
		reached = true
	})
	if reached {
		t.Error("Open returned without PARITY_CORPUS")
	}
}

func TestCasesAreReadInFileOrderWithHomesResolved(t *testing.T) {
	built := newTestCorpus(t)
	collection := built.blob(`{"kind":"loose","homes":[{"user":"alice"}],"artifacts":[` +
		`{"rel":"a","home":0},{"rel":"b","home":{"user":"bob"}}]}`)
	built.write("inputs.jsonl", `{"node":"n1","seq":1,"path":"p","result_blob":"`+collection+
		`"}`+"\n"+`{"node":"n2","seq":2,"error":{"type":"builtins.OSError","text":"denied"}}`+"\n")
	rows := built.blob(`{"rows":[{"text":"hi","source_line":3}],"completed":true,"error":null}`)
	built.write("parsers/aider.jsonl", `{"node":"n3","method":"wants","artifact":{"rel":"x",`+
		`"stand_in":true},"result":{"value":true}}`+"\n"+`{"node":"n4","method":"parse",`+
		`"result_blob":"`+rows+`"}`+"\n")
	corpus := built.open()
	inputs := corpus.InputCases(t)
	if len(inputs) != 2 || inputs[0].Node != "n1" || inputs[1].Error.Text != "denied" {
		t.Fatalf("got %+v", inputs)
	}
	artifacts := inputs[0].Result.Artifacts
	if artifacts[0].Home.User != "alice" || artifacts[1].Home.User != "bob" {
		t.Errorf("homes not resolved: %+v, %+v", artifacts[0].Home, artifacts[1].Home)
	}
	parsers := corpus.ParserCases(t, "aider")
	if len(parsers) != 2 || !parsers[0].Artifact.StandIn || string(parsers[0].Result.Value) != "true" {
		t.Fatalf("got %+v", parsers)
	}
	if result := parsers[1].Result; !result.Completed || len(result.Rows) != 1 ||
		result.Rows[0].Text != "hi" || result.Rows[0].SourceLine != 3 {
		t.Errorf("parse result not read from its blob: %+v", result)
	}
}

func TestMaterialiseWritesBytesTimesAndTheRootInPlaceOfThePlaceholder(t *testing.T) {
	built := newTestCorpus(t)
	placeholder := built.header.RootPlaceholder
	modified := time.Date(2026, 10, 1, 9, 0, 0, 123456789, time.UTC)
	file := built.blob("path=" + placeholder + "/top/a.txt\n")
	inner := built.tree(map[string]any{"name": "a.txt", "kind": "file", "mode": 0o640,
		"blob": file})
	root := built.tree(map[string]any{"name": "top", "kind": "dir", "mode": 0o755,
		"tree": inner})
	times := built.times(map[string]int64{"top": modified.Add(time.Hour).UnixNano(),
		"top/a.txt": modified.UnixNano()})
	corpus := built.open()
	materialised := corpus.Materialise(t, root, times)
	if len(materialised) != corpus.Header.RootLength {
		t.Errorf("root %q is %d bytes, want %d", materialised, len(materialised),
			corpus.Header.RootLength)
	}
	path := filepath.Join(materialised, "top", "a.txt")
	data, err := osfile.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "path=" + materialised + "/top/a.txt\n"; string(data) != want {
		t.Errorf("content %q, want %q", data, want)
	}
	for name, want := range map[string]time.Time{path: modified,
		filepath.Join(materialised, "top"): modified.Add(time.Hour)} {
		info, err := osfile.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		// NTFS keeps a time in units of 100 nanoseconds, so both are cut to that.
		const unit = 100 * time.Nanosecond
		if !info.ModTime().Truncate(unit).Equal(want.Truncate(unit)) {
			t.Errorf("%s: time %v, want %v", name, info.ModTime(), want)
		}
	}
	if got := corpus.Expand("{ROOT}/top", materialised); got != materialised+"/top" {
		t.Errorf("Expand gave %q", got)
	}
}

func TestDiffRowsNamesTheNodeTheRowAndEachDifferingField(t *testing.T) {
	want := []Row{{Text: "a", SourceLine: 1}, {Text: "b", SourceLine: 2, Model: "m"}}
	tests := []struct {
		name  string
		got   []Row
		agree bool
		parts []string
	}{
		{"equal rows agree", want, true, nil},
		{"a differing field is shown with both values",
			[]Row{{Text: "a", SourceLine: 1}, {Text: "B", SourceLine: 2, Model: "m"}}, false,
			[]string{"node-1:", "row 1 (line 2)", `text: want "b", got "B"`}},
		{"a missing row is shown", want[:1], false, []string{"row 1 missing"}},
		{"an extra row is shown", append(want, Row{Text: "c"}), false, []string{"row 2 extra"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			if agree := DiffRows(reporter, "node-1", want, test.got); agree != test.agree {
				t.Errorf("DiffRows returned %v", agree)
			}
			report := strings.Join(reporter.errors, "\n")
			for _, part := range test.parts {
				if !strings.Contains(report, part) {
					t.Errorf("report %q lacks %q", report, part)
				}
			}
		})
	}
}

func TestDiffTextShowsTheFirstDifferingLines(t *testing.T) {
	reporter := &recordingTB{TB: t}
	if DiffText(reporter, "node-2", "stdout", "a\nb\n", "a\nc\n") {
		t.Fatal("DiffText agreed")
	}
	if report := strings.Join(reporter.errors, ""); !strings.Contains(report,
		"node-2: stdout differs in 1 lines") || !strings.Contains(report, `want "b\n"`) {
		t.Errorf("report %q", report)
	}
}
