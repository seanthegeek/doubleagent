//go:build darwin || linux

package walk

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
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
	if !ignoresCase(t, stageDir+"/fs") {
		t.Skipf("%s tells letter case apart", stageDir)
	}
	places := stage.NewPlaces()
	for i, name := range []string{"one", "two"} {
		// The root check resolves the root when it is made.
		if err := os.MkdirAll(parent+"/"+name+"/home/u", 0o755); err != nil {
			t.Fatal(err)
		}
		collector, written, _ := testCollector(t, parent+"/"+name, nil)
		collector.Stager = stage.Stager{Dir: stageDir, Tree: stagingTree(t, stageDir), Root: parent + "/" + name,
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
		!strings.Contains(row["error"].(string), "takes its name for another") {
		t.Errorf("row of l %v; want a symlink row with the collision error and no "+
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
	if err := os.Symlink("two", parent+"/two/home/u/.claude/zed/link"); err != nil {
		t.Fatal(err)
	}
	table := []catalog.Entry{{Agent: "claude", Glob: ".claude"}}
	for i, name := range []string{"one", "two"} {
		if err := collectors[i].CollectCatalog(context.Background(), "u",
			parent+"/"+name+"/home/u", table, ""); err != nil {
			t.Fatal(err)
		}
	}
	link := rowByPath(t, rows[1], parent+"/two/home/u/.claude/zed/link")
	if link == nil || link["archive_path"] != "fs/home/u/.claude/Zed/link" {
		t.Errorf("row of zed/link %v; want archive path fs/home/u/.claude/Zed/link, the member "+
			"as the archive names it", link)
	}
	// zed/two lands in the staged directory Zed, where nothing else sits:
	// it is staged there, its row's archive path spelled as the member is.
	for i, file := range []string{"one/home/u/.claude/Zed/one", "two/home/u/.claude/zed/two"} {
		row := rowByPath(t, rows[i], parent+"/"+file)
		if row == nil || row["status"] != "collected" {
			t.Fatalf("row of %s %v; want collected; rows %s", file, row, rows[i].String())
		}
		archived := row["archive_path"].(string)
		if want := "fs/home/u/.claude/Zed/" + filepath.Base(file); archived != want {
			t.Errorf("row of %s names %s; want %s, the member as the archive names it",
				file, archived, want)
		}
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
		!strings.Contains(row["error"].(string), "takes its name for another") {
		t.Errorf("row of Cursor/User/settings.json %v; want error_copy with the collision", row)
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
	if row == nil || !strings.Contains(row["error"].(string), "takes its name for another") {
		t.Errorf("row of .Config/cursor/User/settings.json %v; want the collision", row)
	}
	log := strings.Join(collectors[1].Log.(*lineLog).lines, "\n")
	if strings.Contains(log, "not removed") {
		t.Errorf("the refused copy was looked for behind the staged link:\n%s", log)
	}
}

func TestALinkThatCannotBeRecreatedIsRecorded(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes any directory")
	}
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	home := root + "/home/u"
	if err := os.MkdirAll(home+"/.claude", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("t", home+"/.claude/L"); err != nil {
		t.Fatal(err)
	}
	collector, rows, log := testCollector(t, root, nil)
	// The link's staged directory exists and cannot be written.
	staged := collector.Stager.Dir + "/fs/home/u/.claude"
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(staged, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(staged, 0o755) })
	table := []catalog.Entry{{Agent: "claude", Glob: ".claude"}}
	if err := collector.CollectCatalog(context.Background(), "u", home, table, ""); err != nil {
		t.Fatal(err)
	}
	row := rowByPath(t, rows, home+"/.claude/L")
	if row == nil || row["archive_path"] != "" ||
		!strings.HasPrefix(row["error"].(string), "not recreated in the archive: ") {
		t.Errorf("row of L %v; want the reason and no archive path", row)
	}
	if text := strings.Join(log.lines, "\n"); !strings.Contains(text,
		"symlink not recreated: "+home+"/.claude/L: ") {
		t.Errorf("the log does not say the link was not recreated:\n%s", text)
	}
}

// variantCase is one directory on disk and catalog lines that differ from
// it, or from each other, only in letter case: want holds the kept matches as
// keptPaths gives them, dropped the globs of the lines dropped with a note.
type variantCase struct {
	name    string
	disk    string
	globs   []string
	want    []string
	dropped []string
}

// keptPaths returns, relative to base, the path of each match of globs,
// each line under its own agent (first, second, third), that ExpandCatalog
// keeps, with its agent, and the globs of the lines dropped with a note,
// failing the test when a note does not name its line and the path kept.
func keptPaths(t *testing.T, base string, globs []string) (kept []string, dropped []string) {
	t.Helper()
	var table []catalog.Entry
	for i, glob := range globs {
		table = append(table, catalog.Entry{Agent: []string{"first", "second", "third"}[i],
			Glob: glob})
	}
	matches := ExpandCatalog(base, table, NewRootCheck(base).Expansion(), nil)
	keptLower := map[string]string{}
	for _, match := range matches {
		if match.Note == "" {
			keptLower[asciiLower(match.Path)] = match.Path
		}
	}
	for _, match := range matches {
		if match.Note != "" {
			want := "catalog line " + match.Glob + " names the same file as " +
				keptLower[asciiLower(match.Path)] + " "
			if keptLower[asciiLower(match.Path)] == "" || !strings.Contains(match.Note, want) {
				t.Errorf("note %q does not name the dropped line %s and the path kept",
					match.Note, match.Glob)
			}
			dropped = append(dropped, match.Glob)
			continue
		}
		kept = append(kept, match.Agent+" "+strings.TrimPrefix(match.Path, base+"/"))
	}
	return kept, dropped
}

func TestCaseVariantsTakeTheSpellingTheDiskHolds(t *testing.T) {
	if !caseInsensitive(t) {
		t.Skip("the temporary directory tells letter case apart, so the lines name two directories")
	}
	cases := []variantCase{
		{"goose on disk with the goose line first keeps the goose line", ".config/goose",
			[]string{".config/goose", ".config/Goose"}, []string{"first .config/goose"},
			[]string{".config/Goose"}},
		{"goose on disk with the Goose line first keeps the goose line", ".config/goose",
			[]string{".config/Goose", ".config/goose"}, []string{"second .config/goose"},
			[]string{".config/Goose"}},
		{"Goose on disk with the goose line first keeps the Goose line", ".config/Goose",
			[]string{".config/goose", ".config/Goose"}, []string{"second .config/Goose"},
			[]string{".config/goose"}},
		{"Goose on disk with the Goose line first keeps the Goose line", ".config/Goose",
			[]string{".config/Goose", ".config/goose"}, []string{"first .config/Goose"},
			[]string{".config/goose"}},
		{"no line spelling goose as the disk does keeps the first line", ".config/goose",
			[]string{".config/Goose", ".config/GOOSE"}, []string{"first .config/goose"},
			[]string{".config/GOOSE"}},
		{"two lines spelling goose as the disk does keep the first of them", ".config/goose",
			[]string{".config/goose", ".config/goo*"}, []string{"first .config/goose"},
			[]string{".config/goo*"}},
		{"two lines spelling goose as the disk does after one that does not keep the first of them",
			".config/goose", []string{".config/Goose", ".config/goose", ".config/goo*"},
			[]string{"second .config/goose"}, []string{".config/Goose", ".config/goo*"}},
		{"cursor on disk spells the enclosing match and the one inside it, enclosing line first",
			".config/cursor/User", []string{".config/cursor", ".config/Cursor/User"},
			[]string{"first .config/cursor", "second .config/cursor/User"}, nil},
		{"cursor on disk spells the enclosing match and the one inside it, nested line first",
			".config/cursor/User", []string{".config/Cursor/User", ".config/cursor"},
			[]string{"first .config/cursor/User", "second .config/cursor"}, nil},
		{"Cursor on disk spells the enclosing match and the one inside it, enclosing line first",
			".config/Cursor/User", []string{".config/cursor", ".config/Cursor/User"},
			[]string{"first .config/Cursor", "second .config/Cursor/User"}, nil},
		{"Cursor on disk spells the enclosing match and the one inside it, nested line first",
			".config/Cursor/User", []string{".config/Cursor/User", ".config/cursor"},
			[]string{"first .config/Cursor/User", "second .config/Cursor"}, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			base := fixture{dirs: []string{test.disk}}.build(t)
			kept, dropped := keptPaths(t, base, test.globs)
			if !slices.Equal(kept, test.want) || !slices.Equal(dropped, test.dropped) {
				t.Errorf("kept %q, dropped %q; want %q kept and %q dropped", kept, dropped,
					test.want, test.dropped)
			}
		})
	}
}

func TestCaseVariantsThatAreTwoDirectoriesKeepTheirOwnSpellings(t *testing.T) {
	if caseInsensitive(t) {
		t.Skip("the temporary directory folds letter case, so it cannot hold goose and Goose apart")
	}
	for _, globs := range [][]string{{".config/goose", ".config/Goose"},
		{".config/Goose", ".config/goose"}} {
		t.Run(globs[0]+" first", func(t *testing.T) {
			base := fixture{dirs: []string{".config/goose", ".config/Goose"}}.build(t)
			kept, dropped := keptPaths(t, base, globs)
			want := []string{"first " + globs[0], "second " + globs[1]}
			if !slices.Equal(kept, want) || len(dropped) != 0 {
				t.Errorf("kept %q, dropped %q; want %q", kept, dropped, want)
			}
		})
	}
}

func TestTheSecretFlagUnderACaseVariantFollowsTheDiskSpelling(t *testing.T) {
	if !caseInsensitive(t) {
		t.Skip("the temporary directory tells letter case apart, so the lines name two directories")
	}
	cases := []struct {
		name  string
		file  string
		globs []string
	}{
		{"goose on disk, Goose line first", ".config/goose/config.yaml",
			[]string{".config/Goose", ".config/goose"}},
		{"Goose on disk, goose line first", ".config/Goose/config.yaml",
			[]string{".config/goose", ".config/Goose"}},
		{"Cursor on disk, enclosing cursor line first", ".config/Cursor/User/config.yaml",
			[]string{".config/cursor", ".config/Cursor/User"}},
		{"cursor on disk, nested Cursor line first", ".config/cursor/User/config.yaml",
			[]string{".config/Cursor/User", ".config/cursor"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := fixture{files: []string{"home/u/" + test.file}}.build(t)
			home := root + "/home/u"
			collector, rows, _ := testCollector(t, root, nil)
			collector.SecretGlobs, collector.MaxSize = []string{test.file}, 1<<10
			var table []catalog.Entry
			for _, glob := range test.globs {
				table = append(table, catalog.Entry{Agent: "agent", Glob: glob})
			}
			if err := collector.CollectCatalog(context.Background(), "u", home, table,
				""); err != nil {
				t.Fatal(err)
			}
			row := rowByPath(t, rows, home+"/"+test.file)
			if row == nil || row["secret"] != true ||
				row["archive_path"] != "fs/home/u/"+test.file {
				t.Errorf("row of %s %v; want it under the disk's spelling, flagged secret; rows %s",
					test.file, row, rows.String())
			}
		})
	}
}

func TestOnlyCaseVariantsOfOneFileCostADirectoryRead(t *testing.T) {
	folds := caseInsensitive(t)
	base := fixture{dirs: []string{".config/goose", ".config/cursor/User", ".local/share/goose"}}.build(t)
	cases := []struct {
		name  string
		globs []string
		want  []string
	}{
		{"lines that do not collide read no directory",
			[]string{".config/goose", ".local/share/goose", ".config/cursor", ".config/cursor/User"}, nil},
		{"two spellings of one directory read the directory holding the name",
			[]string{".config/goose", ".config/Goose"}, []string{base + "/.config"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var read []string
			opener := &fsmeta.Opener{Admit: func(path string, _ uintptr) error {
				read = append(read, path)
				return nil
			}}
			var table []catalog.Entry
			for _, glob := range test.globs {
				table = append(table, catalog.Entry{Agent: "agent", Glob: glob})
			}
			ExpandCatalog(base, table, nil, opener)
			want := test.want
			if !folds {
				// Where case counts, .config/Goose does not exist.
				want = nil
			}
			if !slices.Equal(read, want) {
				t.Errorf("read %q; want %q", read, want)
			}
		})
	}
}

// collectGoose collects the lines in globs under the home u of a fixture
// holding files, with the Goose secret and exclusion globs spelled in lower
// case, and returns the home and the rows.
func collectGoose(t *testing.T, files []string, globs []string) (string, *bytes.Buffer) {
	t.Helper()
	root := fixture{files: files}.build(t)
	home := root + "/home/u"
	collector, rows, _ := testCollector(t, root, []string{".config/goose/mcp-apps-cache"})
	collector.SecretGlobs, collector.MaxSize = []string{".config/goose/secrets.yaml"}, 1<<10
	var table []catalog.Entry
	for _, glob := range globs {
		table = append(table, catalog.Entry{Agent: "goose", Glob: glob})
	}
	if err := collector.CollectCatalog(context.Background(), "u", home, table, ""); err != nil {
		t.Fatal(err)
	}
	return home, rows
}

func TestAGlobSpelledOtherwiseThanAFoldingDiskStillFlagsAndPrunes(t *testing.T) {
	if !caseInsensitive(t) {
		t.Skip("the temporary directory tells letter case apart, so .config/Goose is not .config/goose")
	}
	home, rows := collectGoose(t, []string{"home/u/.config/Goose/secrets.yaml",
		"home/u/.config/Goose/mcp-apps-cache/f"}, []string{".config/Goose"})
	if row := rowByPath(t, rows, home+"/.config/Goose/secrets.yaml"); row == nil ||
		row["secret"] != true {
		t.Errorf("secrets.yaml under Goose is %v; want it flagged secret; rows %s", row, rows)
	}
	if row := rowByPath(t, rows, home+"/.config/Goose/mcp-apps-cache"); row == nil ||
		row["status"] != "skipped_excluded" {
		t.Errorf("mcp-apps-cache under Goose is %v; want it excluded; rows %s", row, rows)
	}
	if row := rowByPath(t, rows, home+"/.config/Goose/mcp-apps-cache/f"); row != nil {
		t.Errorf("a file under the excluded directory has the row %v", row)
	}
}

func TestADirectoryHoldingBothSpellingsKeepsByteMatching(t *testing.T) {
	if caseInsensitive(t) {
		t.Skip("the temporary directory folds letter case, so it cannot hold goose and Goose apart")
	}
	var files []string
	for _, name := range []string{"goose", "Goose"} {
		files = append(files, "home/u/.config/"+name+"/secrets.yaml",
			"home/u/.config/"+name+"/mcp-apps-cache/f")
	}
	home, rows := collectGoose(t, files, []string{".config/goose", ".config/Goose"})
	want := map[string][2]any{
		".config/goose/secrets.yaml":     {"collected", true},
		".config/Goose/secrets.yaml":     {"collected", false},
		".config/goose/mcp-apps-cache":   {"skipped_excluded", false},
		".config/Goose/mcp-apps-cache/f": {"collected", false},
	}
	for path, outcome := range want {
		row := rowByPath(t, rows, home+"/"+path)
		if row == nil || row["status"] != outcome[0] || row["secret"] != outcome[1] {
			t.Errorf("%s is %v; want status %v, secret %v", path, row, outcome[0], outcome[1])
		}
	}
}
