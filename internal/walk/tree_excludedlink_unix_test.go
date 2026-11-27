//go:build unix

package walk

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// excludedLinkRows collects start, a path under home, with every path named
// "*ex*" excluded, by the entry point named (the walk, or the catalog match
// that is itself the link), and returns the parsed rows.
func excludedLinkRows(t *testing.T, root, home, start string, throughWalk bool,
) ([]map[string]any, string) {
	t.Helper()
	collector, rows, _ := testCollector(t, root, []string{"*ex*"})
	var err error
	if throughWalk {
		_, err = collector.CollectTree(context.Background(), "u", home, "agent", start)
	} else {
		_, err = collector.CollectPath(context.Background(), "u", home, "agent", start)
	}
	mustDo(t, err)
	var parsed []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		mustDo(t, json.Unmarshal([]byte(line), &row))
		parsed = append(parsed, row)
	}
	return parsed, collector.Stager.Dir
}

func TestExcludedLinkIsASymlinkRowNeverFollowed(t *testing.T) {
	outside := fixture{files: []string{"deep/big"}}.build(t)
	cases := []struct {
		name   string
		target string
	}{
		{"an excluded link to a directory", "../dir"},
		{"an excluded link to a file", "../file"},
		{"an excluded dangling link", "../missing"},
		{"an excluded link leading out of the root", outside + "/deep"},
	}
	for _, test := range cases {
		for _, throughWalk := range []bool{true, false} {
			name := test.name + " met by the catalog match"
			if throughWalk {
				name = test.name + " met by the walk"
			}
			t.Run(name, func(t *testing.T) {
				base := fixture{files: []string{"dir/inner", "dir/deep/more", "file", "m/keep"}}.build(t)
				link := base + "/m/excluded"
				mustDo(t, os.Symlink(test.target, link))
				start := link
				if throughWalk {
					start = base + "/m"
				}
				rows, stageDir := excludedLinkRows(t, base, base, start, throughWalk)
				var got map[string]any
				for _, row := range rows {
					if row["path"] == link {
						got = row
					} else if !throughWalk || row["path"] != base+"/m/keep" {
						t.Errorf("unexpected row %v: nothing behind the link may be listed", row)
					}
				}
				meta, err := os.Lstat(link)
				mustDo(t, err)
				if got == nil {
					t.Fatalf("no row of %s in %v", link, rows)
				}
				if got["type"] != "symlink" || got["status"] != "skipped_excluded" ||
					got["target"] != test.target || got["size"] != float64(meta.Size()) ||
					got["archive_path"] != "" {
					t.Errorf("row %v, want a skipped_excluded symlink row of the link itself, "+
						"target %q, size %d, no archive path", got, test.target, meta.Size())
				}
				if names := stagedNames(t, stageDir); names["excluded"] || names["keep"] != throughWalk {
					t.Errorf("staged names %v, want the walk's keep file alone, never the link", names)
				}
			})
		}
	}
}

func TestExcludedRealDirectoryStaysADirectoryRow(t *testing.T) {
	base := fixture{files: []string{"m/exdir/a"}}.build(t)
	rows, _ := excludedLinkRows(t, base, base, base+"/m", true)
	if len(rows) != 1 || rows[0]["type"] != "dir" || rows[0]["status"] != "skipped_excluded" ||
		rows[0]["target"] != "" {
		t.Errorf("rows %v, want one skipped_excluded dir row with no target", rows)
	}
}
