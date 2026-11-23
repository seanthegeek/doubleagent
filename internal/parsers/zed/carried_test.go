package zed

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// carriedRows writes data at rel under a new home and returns the rows a
// run gives it (parsertest, showing secrets) and their bytes: every string
// of every row.
func carriedRows(t *testing.T, rel string, data []byte) ([]timeline.Row, int) {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "zed",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	size := 0
	err := parsertest.ParseWithoutSecrets(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) {
		rows = append(rows, row)
		for _, field := range row.Strings() {
			size += len(field)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows, size
}

// A settings key of 100,000 bytes stands before each of n leaves: each
// row holds it cut, so the output grows by one row of 4,231 bytes per
// leaf, not by the key's length.
func TestALongSettingsKeyIsCutInEachOfItsLeavesRows(t *testing.T) {
	key := strings.Repeat("k", 100_000)
	tests := []struct {
		leaves, size int
	}{
		{200, 846_318},
		{400, 1_692_518},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.leaves)+" leaves", func(t *testing.T) {
			items := strings.TrimSuffix(strings.Repeat(`"x",`, test.leaves), ",")
			document := `{"agent": {"tool_permissions": {"` + key + `": [` + items + `]}}}`
			rows, size := carriedRows(t, ".config/zed/settings.json", []byte(document))
			problem := parserkit.ProblemPrefix + parserkit.CarriedProblem("settings key", test.leaves)
			if size != test.size || len(rows) != test.leaves+1 || rows[len(rows)-1].Text != problem {
				t.Errorf("%d rows of %d bytes, last %q; want %d rows of %d bytes and %q", len(rows), size,
					rows[len(rows)-1].Text, test.leaves+1, test.size, problem)
			}
		})
	}
}

// An archived worktree of three 10,000-byte columns linked to n threads,
// and a 10,000-byte main repository of n created worktrees, are cut in
// each of the rows the joins give: 20,963 bytes more per link, where the
// whole values would give 50,483.
func TestJoinedWorktreeValuesAreCutInEachRow(t *testing.T) {
	tests := []struct {
		links, size int
	}{
		{50, 1_063_115},
		{100, 2_111_270},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.links)+" links", func(t *testing.T) {
			data, err := osfile.ReadFile(filepath.Join("testdata", "carried_"+strconv.Itoa(test.links)+".db"))
			if err != nil {
				t.Fatal(err)
			}
			rows, size := carriedRows(t, ".local/share/zed/db/0-stable/db.sqlite", data)
			if size != test.size {
				t.Errorf("%d rows of %d bytes, want %d bytes", len(rows), size, test.size)
			}
			for _, row := range rows {
				if len(row.Text) > 6*parserkit.CarriedLimit {
					t.Errorf("a row of %d bytes: %q", len(row.Text), row.Text[:80])
				}
			}
		})
	}
}
