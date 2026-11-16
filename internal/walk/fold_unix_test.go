//go:build darwin || linux

package walk

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// foldingCollectors returns two collectors over the image roots one and
// two under a test directory, staging into one staging directory that
// folds case and sharing its record of staged places, as two entries a
// case-sensitive volume keeps apart would; the test is skipped where the
// directory tells case apart.
func foldingCollectors(t *testing.T) (parent string, collectors [2]*Collector,
	rows [2]*bytes.Buffer) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stageDir := parent + "/stage"
	if err := os.MkdirAll(stageDir+"/fs", 0o700); err != nil {
		t.Fatal(err)
	}
	if !stage.FoldsCase(stageDir + "/fs") {
		t.Skipf("%s tells letter case apart", stageDir)
	}
	places := stage.NewPlaces()
	for i, name := range []string{"one", "two"} {
		// The root check resolves the root when it is made.
		if err := os.MkdirAll(parent+"/"+name+"/home/u", 0o755); err != nil {
			t.Fatal(err)
		}
		collector, written, _ := testCollector(t, parent+"/"+name, nil)
		collector.Stager = stage.Stager{Dir: stageDir, Physical: stageDir, Root: parent + "/" + name,
			FoldsCase: true, Places: places}
		collectors[i], rows[i] = collector, written
	}
	return parent, collectors, rows
}

// rowByPath returns the parsed row of path among rows, or nil.
func rowByPath(t *testing.T, rows *bytes.Buffer, path string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row["path"] == path {
			return row
		}
	}
	return nil
}

func TestALinkRefusedForACaseCollisionNamesNoArchivePath(t *testing.T) {
	parent, collectors, rows := foldingCollectors(t)
	for i, link := range []string{"one/home/u/.claude/L", "two/home/u/.claude/l"} {
		if err := os.MkdirAll(filepath.Dir(parent+"/"+link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink([]string{"a", "b"}[i], parent+"/"+link); err != nil {
			t.Fatal(err)
		}
	}
	table := []catalog.Entry{{Agent: "claude", Glob: ".claude"}}
	for i, name := range []string{"one", "two"} {
		if err := collectors[i].CollectCatalog(context.Background(), "u",
			parent+"/"+name+"/home/u", table, ""); err != nil {
			t.Fatal(err)
		}
	}
	row := rowByPath(t, rows[1], parent+"/two/home/u/.claude/l")
	if row == nil || row["archive_path"] != "" || row["status"] != "symlink" ||
		!strings.Contains(row["error"].(string), "differs only in case") {
		t.Errorf("row of l %v; want a symlink row with the case collision error and no "+
			"archive path, since the staged place holds L's link", row)
	}
	if target, err := os.Readlink(parent + "/stage/fs/home/u/.claude/L"); err != nil || target != "a" {
		t.Errorf("staged L reads %q, %v; want L's own target a", target, err)
	}
}

func TestACopyInADirectorySpelledInAnotherCaseKeepsItsBytes(t *testing.T) {
	parent, collectors, rows := foldingCollectors(t)
	files := map[string]string{"one/home/u/.claude/Zed/one": "one",
		"two/home/u/.claude/zed/two": "two"}
	for file, text := range files {
		if err := os.MkdirAll(filepath.Dir(parent+"/"+file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(parent+"/"+file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	table := []catalog.Entry{{Agent: "claude", Glob: ".claude"}}
	for i, name := range []string{"one", "two"} {
		if err := collectors[i].CollectCatalog(context.Background(), "u",
			parent+"/"+name+"/home/u", table, ""); err != nil {
			t.Fatal(err)
		}
	}
	// zed/two lands in the staged directory Zed, where nothing else sits:
	// it is staged there, its row's archive path spelled zed, as documented.
	for i, file := range []string{"one/home/u/.claude/Zed/one", "two/home/u/.claude/zed/two"} {
		row := rowByPath(t, rows[i], parent+"/"+file)
		if row == nil || row["status"] != "collected" {
			t.Fatalf("row of %s %v; want collected; rows %s", file, row, rows[i].String())
		}
		archived := row["archive_path"].(string)
		staged, err := os.ReadFile(parent + "/stage/" + archived)
		if err != nil || string(staged) != files[file] {
			t.Errorf("%s resolves to %q, %v; want its own bytes %q", archived, staged, err,
				files[file])
		}
	}
	entries, err := os.ReadDir(parent + "/stage/fs/home/u/.claude")
	if err != nil || len(entries) != 1 || entries[0].Name() != "Zed" {
		t.Errorf("staged .claude holds %v, %v; want the one directory Zed, so the archive "+
			"member of zed/two is spelled Zed/two", entries, err)
	}
}

func TestALinkOutOfTheStageIsNeverListedThroughAnotherSpelling(t *testing.T) {
	parent, collectors, rows := foldingCollectors(t)
	outside := parent + "/outside"
	if err := os.MkdirAll(outside+"/User", 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, dir := range []string{outside + "/User", outside} {
		// A later modification time: macOS then moves the access time on
		// the next listing.
		if err := os.Chtimes(dir, old, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	link := parent + "/one/home/u/.config/cursor"
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	settings := parent + "/two/home/u/.config/Cursor/User/settings.json"
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	tables := [][]catalog.Entry{{{Agent: "cursor", Glob: ".config/cursor"}},
		{{Agent: "cursor", Glob: ".config/Cursor/User"}}}
	for i, name := range []string{"one", "two"} {
		if err := collectors[i].CollectCatalog(context.Background(), "u",
			parent+"/"+name+"/home/u", tables[i], ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{outside, outside + "/User"} {
		info, err := os.Lstat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if accessTime(info) != old.Unix() {
			t.Errorf("%s was listed through the staged link cursor: access time %d, want %d",
				dir, accessTime(info), old.Unix())
		}
	}
	row := rowByPath(t, rows[1], settings)
	if row == nil || row["status"] != "error_copy" ||
		!strings.Contains(row["error"].(string), "differs only in case") {
		t.Errorf("row of Cursor/User/settings.json %v; want error_copy with the case collision", row)
	}
}

func TestARefusedCopyLooksAtNothingBehindAnotherEntrysLink(t *testing.T) {
	parent, collectors, rows := foldingCollectors(t)
	outside := parent + "/outside"
	if err := os.MkdirAll(outside+"/User", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside+"/User/settings.json", []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := parent + "/one/home/u/.config/cursor"
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	settings := parent + "/two/home/u/.Config/cursor/User/settings.json"
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	tables := [][]catalog.Entry{{{Agent: "cursor", Glob: ".config/cursor"}},
		{{Agent: "cursor", Glob: ".Config/cursor/User"}}}
	for i, name := range []string{"one", "two"} {
		if err := collectors[i].CollectCatalog(context.Background(), "u",
			parent+"/"+name+"/home/u", tables[i], ""); err != nil {
			t.Fatal(err)
		}
	}
	row := rowByPath(t, rows[1], settings)
	if row == nil || !strings.Contains(row["error"].(string), "differs only in case") {
		t.Errorf("row of .Config/cursor/User/settings.json %v; want the case collision", row)
	}
	log := strings.Join(collectors[1].Log.(*lineLog).lines, "\n")
	if strings.Contains(log, "not removed") {
		t.Errorf("the refused copy was looked for behind the staged link:\n%s", log)
	}
}
