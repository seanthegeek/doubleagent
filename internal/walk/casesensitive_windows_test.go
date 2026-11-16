package walk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
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
	// A run marks its staging directory case-sensitive, so the copies of
	// Notes.txt and notes.txt, of Dir and dir, and the recreated links J and
	// j are all kept. Where the mark cannot be set the directory folds case,
	// and the later of each pair is refused rather than staged over the
	// earlier one.
	for _, marked := range []bool{true, false} {
		t.Run(fmt.Sprintf("staging directory marked case-sensitive %v", marked), func(t *testing.T) {
			stagesBothOrRefuses(t, marked)
		})
	}
}

// stagesBothOrRefuses walks a case-sensitive directory holding three pairs
// of entries differing only in case, into a staging directory marked
// case-sensitive or not, and checks the rows against the staged tree.
func stagesBothOrRefuses(t *testing.T, marked bool) {
	home := physicalTemp(t)
	caseSensitiveDir(t, home)
	writeFiles(t, home, "cs/Notes.txt", "cs/notes.txt", "cs/Dir/f.txt", "cs/dir/f.txt",
		"cs/Zed/one", "cs/zed/two", "cs/p", "cs/r", "cs/R/x", "target/t.txt")
	makeJunction(t, home+"/target", home+"/cs/J")
	makeJunction(t, home+"/target", home+"/cs/j")
	// A file whose staged place, unmarked, holds a link or a directory of
	// another case is refused as a case collision too.
	makeJunction(t, home+"/target", home+"/cs/P")
	collector, recorder := windowsCollector(t, "", nil)
	mustDo(t, os.Mkdir(syspath.ToNative(collector.Stager.Dir+"/fs"), 0o755))
	if marked {
		mustDo(t, stage.MarkCaseSensitive(collector.Stager.Dir+"/fs"))
	}
	table := []catalog.Entry{{Agent: "tool", Glob: "cs"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	seen := map[string]bool{}
	members := map[string]bool{}
	refused := 0
	var respelled []string // rows whose member the staging tree spells otherwise
	for _, row := range recorder.rows(t) {
		path := row["path"].(string)
		seen[path] = true
		if archived, _ := row["archive_path"].(string); archived != "" {
			members[archived] = true
		}
		if row["archive_path"] == "" && strings.Contains(row["error"].(string), "differs only in case") {
			refused++
		}
		if row["archive_path"] == "" {
			continue
		}
		// The archive takes its member names from the staging tree's own
		// spelling; only a copy staged in an empty spot of a directory spelled
		// in another case may differ, and only in case.
		member := collector.Stager.Dir + "/" + row["archive_path"].(string)
		if spelled := fsmeta.OnDiskPath(collector.Stager.Dir, member); spelled != member {
			respelled = append(respelled, path)
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
	wantRefused, wantMembers := 5, 7
	if marked {
		wantRefused, wantMembers = 0, 12
	}
	// Unmarked, the later of Zed/one and zed/two (in listing order) lands in
	// the staged directory of the earlier, where nothing else sits: staged
	// there with its own bytes, its archive path spelled its own way.
	zed := []string{syspath.ToNative(home + "/cs/Zed/one"), syspath.ToNative(home + "/cs/zed/two")}
	if marked && len(respelled) != 0 ||
		!marked && (len(respelled) != 1 || !slices.Contains(zed, respelled[0])) {
		t.Errorf("rows whose member the staging tree spells otherwise: %q; want none marked, "+
			"one of %q unmarked", respelled, zed)
	}
	if refused != wantRefused || len(members) != wantMembers {
		t.Errorf("%d rows refused for a taken staged path and %d archive paths, want %d and %d:"+
			" %q", refused, len(members), wantRefused, wantMembers, recorder.written)
	}
	for _, file := range []string{"Notes.txt", "notes.txt", "Dir/f.txt", "dir/f.txt", "J", "j",
		"Zed/one", "zed/two", "p", "P", "r", "R/x"} {
		if path := syspath.ToNative(home + "/cs/" + file); !seen[path] {
			t.Errorf("no row of %s in %q", path, recorder.written)
		}
	}
}
