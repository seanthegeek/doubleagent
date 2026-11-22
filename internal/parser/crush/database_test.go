package crush

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The databases under testdata are written by SQLite from the SQL in
// testdata/fixtures.py.

const databaseRel = "proj/.crush/crush.db"

// parseDatabase copies the named testdata database into a new home at
// databaseRel and returns the text of every row the parser gives for it.
func parseDatabase(t *testing.T, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, databaseRel, data)
}

// parseFile writes data into a new home at rel and returns the text of
// every row the parser gives for it, secrets shown.
func parseFile(t *testing.T, rel string, data []byte) []string {
	t.Helper()
	return parseWith(t, secrets.Setting{}, rel, data)
}

// parseWith writes data into a new home at rel and returns the text of
// every row the parser gives for it under the secrets setting.
func parseWith(t *testing.T, setting secrets.Setting, rel string, data []byte) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "crush",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAFileVersionGivesItsContentsLengthInBytes(t *testing.T) {
	var got []string
	for _, text := range parseDatabase(t, "file_versions.db") {
		if strings.HasPrefix(text, "file version: ") {
			got = append(got, text)
		}
	}
	want := []string{
		"file version: /w/ascii.txt version=1 bytes=11 (from crush.db files)",
		"file version: /w/accents.txt version=1 bytes=17 (from crush.db files)",
		"file version: /w/blob.bin version=1 bytes=4 (from crush.db files)",
		"file version: /w/number.txt version=1 bytes=5 (from crush.db files)",
		"file version: /w/null.txt version=1 bytes=0 (from crush.db files)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A virtual generated column is computed when read and not stored, so
// reading every column of its table leaves it out and reads the rest.
func TestATableWithAVirtualGeneratedColumnIsRead(t *testing.T) {
	rows := parseDatabase(t, "generated_column.db")
	if !slices.ContainsFunc(rows, func(text string) bool { return strings.Contains(text, "hello") }) ||
		slices.ContainsFunc(rows, func(text string) bool { return strings.HasPrefix(text, parser.ProblemPrefix) }) {
		t.Errorf("got %q, want the message and no problem", rows)
	}
}

func TestMessagePartsThatAreNotObjectsAreReportedOnce(t *testing.T) {
	rows := parseDatabase(t, "part_kinds.db")
	want := "parser: message m1 parts holds 1 entry that is not a JSON object"
	if !slices.Contains(rows, want) || !slices.Contains(rows, "hello") {
		t.Errorf("rows %q, want the text and %q", rows, want)
	}
}
