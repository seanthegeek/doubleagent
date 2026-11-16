package walk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
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
	// The staging directory is case-insensitive, so the copies of Notes.txt
	// and notes.txt, of Dir and dir, or the recreated links J and j would
	// share one place there.
	home := physicalTemp(t)
	caseSensitiveDir(t, home)
	writeFiles(t, home, "cs/Notes.txt", "cs/notes.txt", "cs/Dir/f.txt", "cs/dir/f.txt",
		"target/t.txt")
	makeJunction(t, home+"/target", home+"/cs/J")
	makeJunction(t, home+"/target", home+"/cs/j")
	collector, recorder := windowsCollector(t, "", nil)
	table := []catalog.Entry{{Agent: "tool", Glob: "cs"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	seen := map[string]bool{}
	refused := 0
	for _, row := range recorder.rows(t) {
		path := row["path"].(string)
		seen[path] = true
		if row["archive_path"] == "" && strings.Contains(row["error"].(string), "differs only in case") {
			refused++
		}
		if row["archive_path"] == "" {
			continue
		}
		// The archive takes its member names from the staging tree's own
		// spelling, which must be the row's.
		member := collector.Stager.Dir + "/" + row["archive_path"].(string)
		if spelled := fsmeta.OnDiskPath(collector.Stager.Dir, member); spelled != member {
			t.Errorf("row of %s points at %s, which the staging tree spells %s", path, member,
				spelled)
		}
		if row["status"] != "collected" {
			continue
		}
		staged, err := os.ReadFile(syspath.ToNative(member))
		sum := sha256.Sum256(staged)
		if err != nil || hex.EncodeToString(sum[:]) != row["sha256"] {
			t.Errorf("row of %s points at %s, which holds %q (%v), not the bytes it hashed",
				path, row["archive_path"], staged, err)
		}
	}
	if refused != 3 {
		t.Errorf("%d rows refused for a taken staged path, want 3 (the later of each pair): %q",
			refused, recorder.written)
	}
	for _, file := range []string{"Notes.txt", "notes.txt", "Dir/f.txt", "dir/f.txt", "J", "j"} {
		if path := syspath.ToNative(home + "/cs/" + file); !seen[path] {
			t.Errorf("no row of %s in %q", path, recorder.written)
		}
	}
}
