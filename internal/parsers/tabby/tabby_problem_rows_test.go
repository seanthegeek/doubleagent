package tabby

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseRowsOf writes data into a new home at rel and returns the rows the
// parser gives for it.
func parseRowsOf(t *testing.T, rel string, data []byte) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// A problem row of a database record has no time and keeps the record's
// line; the record's other rows keep the record's time.
func TestADatabaseRecordsProblemRowHasNoTime(t *testing.T) {
	const created = "2026-01-02 03:04:05"
	const stamp = "2026-01-02T03:04:05.000Z"
	tests := []struct {
		name, problem string
		timed         int
		tables        []sqlitetest.Table
	}{
		{"a user event whose payload does not parse", "parser: user_event 1 payload did not parse", 1,
			[]sqlitetest.Table{{Name: "user_events", SQL: "CREATE TABLE user_events (id INTEGER, kind TEXT, " +
				"user_id INTEGER, created_at TIMESTAMP, payload BLOB)", Rows: [][]any{
				{1, "completion", 1, created, "not json"}, {2, "view", 1, created, `{"view": {}}`}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := sqlitetest.Build(test.tables...)
			if err != nil {
				t.Fatal(err)
			}
			found, timed := 0, 0
			for _, row := range parseRowsOf(t, ".tabby/ee/db.sqlite", data) {
				switch {
				case strings.HasPrefix(row.Text, test.problem):
					found++
					if row.TimestampUTC != "" || row.SourceLine != 1 {
						t.Errorf("problem row %+v", row)
					}
				case row.TimestampUTC == stamp:
					timed++
				}
			}
			if found == 0 || timed < test.timed {
				t.Errorf("%d problem rows and %d rows at %s, want some and %d", found, timed, stamp, test.timed)
			}
		})
	}
}

// A config.toml that is not valid TOML gives a problem row with no time; one
// that is valid gives its model row at the file's modification time.
func TestAConfigProblemRowHasNoTimeAndModelRowsKeepTheFilesTime(t *testing.T) {
	for _, test := range []struct {
		name, content, text string
		timed               bool
	}{
		{"invalid", "model = [", "parser: config.toml is not valid TOML", false},
		{"valid", "[model.chat.http]\nmodel_name = \"m\"\n", "model configured: chat", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := parseRowsOf(t, ".tabby/config.toml", []byte(test.content))
			if len(rows) != 1 || !strings.HasPrefix(rows[0].Text, test.text) {
				t.Fatalf("rows %+v", rows)
			}
			if (rows[0].TimestampUTC != "") != test.timed {
				t.Errorf("time %q", rows[0].TimestampUTC)
			}
		})
	}
}
