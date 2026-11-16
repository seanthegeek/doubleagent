package walk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// rowsByPath returns, by native path, each recorded row's agent and status.
func rowsByPath(t *testing.T, recorder *events) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, row := range recorder.rows(t) {
		got[row["path"].(string)] += row["agent"].(string) + " " + row["status"].(string) + ";"
	}
	return got
}

func TestANestedClaimPrunesNoOtherSpellingInACaseSensitiveDirectory(t *testing.T) {
	home := physicalTemp(t)
	caseSensitiveDir(t, home)
	writeFiles(t, home, "cs/Bar/one.txt", "cs/bar/two.txt")
	collector, recorder := windowsCollector(t, "", nil)
	table := []catalog.Entry{{Agent: "outer", Glob: "cs"}, {Agent: "inner", Glob: "cs/Bar"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	got := rowsByPath(t, recorder)
	inner := got[syspath.ToNative(home+"/cs/Bar/one.txt")]
	outer := got[syspath.ToNative(home+"/cs/bar/two.txt")]
	if !strings.HasPrefix(inner, "inner ") || !strings.HasPrefix(outer, "outer ") {
		t.Errorf("rows by path %q; want Bar/one.txt under inner and bar/two.txt, another "+
			"directory that Bar's claim does not cover, under outer", got)
	}
}

func TestNoRowPointsAtAnotherEntrysStagedBytes(t *testing.T) {
	home := physicalTemp(t)
	caseSensitiveDir(t, home)
	writeFiles(t, home, "cs/Notes.txt", "cs/notes.txt", "cs/Dir/f.txt", "cs/dir/f.txt")
	collector, recorder := windowsCollector(t, "", nil)
	table := []catalog.Entry{{Agent: "tool", Glob: "cs"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	seen := map[string]bool{}
	for _, row := range recorder.rows(t) {
		path := row["path"].(string)
		seen[path] = true
		if row["status"] != "collected" {
			continue
		}
		staged, err := os.ReadFile(syspath.ToNative(collector.Stager.Dir) + `\` +
			strings.ReplaceAll(row["archive_path"].(string), "/", `\`))
		sum := sha256.Sum256(staged)
		if err != nil || hex.EncodeToString(sum[:]) != row["sha256"] {
			t.Errorf("row of %s points at %s, which holds %q (%v), not the bytes it hashed",
				path, row["archive_path"], staged, err)
		}
	}
	for _, file := range []string{"Notes.txt", "notes.txt", "Dir/f.txt", "dir/f.txt"} {
		if path := syspath.ToNative(home + "/cs/" + file); !seen[path] {
			t.Errorf("no row of %s in %q", path, recorder.written)
		}
	}
}
