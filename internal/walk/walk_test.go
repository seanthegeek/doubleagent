//go:build unix

package walk

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// fixture is a test tree: directories, files and links relative to its base.
type fixture struct {
	dirs  []string
	files []string
	links [][2]string // target, name
}

// build makes the fixture under a fresh physical directory and returns it.
func (spec fixture) build(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range spec.dirs {
		mustDo(t, os.MkdirAll(base+"/"+dir, 0o755))
	}
	for _, file := range spec.files {
		mustDo(t, os.MkdirAll(filepath.Dir(base+"/"+file), 0o755))
		mustDo(t, os.WriteFile(base+"/"+file, []byte(file), 0o644))
	}
	for _, link := range spec.links {
		mustDo(t, os.Symlink(link[0], base+"/"+link[1]))
	}
	return base
}

// mustDo fails the test on err.
func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// lineLog keeps log lines in memory.
type lineLog struct{ lines []string }

// Line keeps text.
func (log *lineLog) Line(text string) error {
	log.lines = append(log.lines, text)
	return nil
}

// ignoresCase reports whether dir takes two names that differ only in ASCII
// letter case for one, by making a lower-case probe file, looking it up in
// upper case and removing it.
func ignoresCase(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.WriteFile(dir+"/case-probe", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir + "/case-probe")
	_, err := os.Lstat(dir + "/CASE-PROBE")
	return err == nil
}

// caseInsensitive reports whether the temporary directory's file system
// folds case, so two spellings of a name are one file.
func caseInsensitive(t *testing.T) bool {
	t.Helper()
	return ignoresCase(t, t.TempDir())
}

// testCollector returns a collector over root with a fresh staging directory,
// the manifest in rows and the log in memory.
func testCollector(t *testing.T, root string, excludes []string,
) (*Collector, *bytes.Buffer, *lineLog) {
	t.Helper()
	stageDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rows, log := &bytes.Buffer{}, &lineLog{}
	return &Collector{
		Tables: &catalog.Tables{}, Excludes: excludes, SecretGlobs: []string{"*secret*"},
		Log: log, Manifest: manifest.NewWriter(rows), Root: root, MaxSize: 10,
		Stager:    stage.Stager{Dir: stageDir, Tree: stagingTree(t, stageDir), Root: root, Places: stage.NewPlaces()},
		RootCheck: NewRootCheck(root),
	}, rows, log
}

// field returns the value of a string or number field of each row.
func field(rows *bytes.Buffer, name string) []string {
	var values []string
	for _, row := range strings.Split(strings.TrimSuffix(rows.String(), "\n"), "\n") {
		_, rest, _ := strings.Cut(row, `"`+name+`":`)
		value, _, _ := strings.Cut(strings.TrimPrefix(rest, `"`), `,"`)
		values = append(values, strings.TrimSuffix(value, `"`))
	}
	return values
}

func TestLinkedParent(t *testing.T) {
	parent := fixture{
		dirs: []string{"home/bob/dotfiles/gemini/sub", "outside/crush", "home/bob/real/x"},
		links: [][2]string{{"dotfiles/gemini", "home/bob/.gemini"}, {"../../outside", "home/bob/.cache"},
			{"missing", "home/bob/.gone"}, {"../.gemini", "home/bob/real/up"}},
	}.build(t)
	base := parent + "/home/bob"
	cases := []struct {
		name    string
		path    string
		want    Links
		outside bool
	}{
		{"a path without links meets none", base + "/real/x", Links{}, false},
		{"the path itself is not tested", base + "/.gemini", Links{}, false},
		{"an in-base link is passed and staged at its target", base + "/.gemini/sub",
			Links{Passed: []string{base + "/.gemini"}, From: base + "/.gemini",
				Into: base + "/dotfiles/gemini"}, false},
		{"a link out of the base stops the walk", base + "/.cache/crush",
			Links{Outside: base + "/.cache"}, true},
		{"a dangling link cannot be resolved", base + "/.gone/x", Links{Outside: base + "/.gone"}, true},
		{"a chain of in-base links keeps the deepest", base + "/real/up/sub",
			Links{Passed: []string{base + "/real/up"}, From: base + "/real/up",
				Into: base + "/dotfiles/gemini"}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, outside := LinkedParent(base, test.path, nil)
			if outside != test.outside || !reflect.DeepEqual(got, test.want) {
				t.Errorf("LinkedParent = %+v, %v", got, outside)
			}
		})
	}
}

func TestClaimed(t *testing.T) {
	base := fixture{dirs: []string{".gemini/antigravity-cli/x", "real"},
		links: [][2]string{{"real", ".gemini/link"}}}.build(t)
	claims := syspath.NewEntries[struct{}](nil)
	claims.Put(base+"/.gemini", struct{}{})
	claims.Put(base+"/.gemini/antigravity-cli", struct{}{})
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"a claim is claimed", base + "/.gemini/antigravity-cli", true},
		{"a path under a claim through real directories is claimed",
			base + "/.gemini/antigravity-cli/x", true},
		{"a path under a link inside a claim is not", base + "/.gemini/link/y", false},
		{"a path outside every claim is not", base + "/real", false},
		{"the base itself is never tested", base, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := claimed(test.path, base, claims); got != test.want {
				t.Errorf("claimed(%q) = %v", test.path, got)
			}
		})
	}
}

func TestExpandCatalogFoldsCaseVariants(t *testing.T) {
	base := fixture{dirs: []string{".config/goose", ".config/cursor/User"},
		files: []string{"nl\nx"}}.build(t)
	mustDo(t, os.Chmod(base+"/.config/goose", 0))
	t.Cleanup(func() { _ = os.Chmod(base+"/.config/goose", 0o755) })
	folds := caseInsensitive(t)
	table := []catalog.Entry{{Agent: "goose", Glob: ".config/goose"},
		{Agent: "goose", Glob: ".config/Goose"}, {Agent: "pearai", Glob: ".config/goose"},
		{Agent: "cursor", Glob: ".config/cursor"}, {Agent: "cursor", Glob: ".config/Cursor/User"},
		{Agent: "whole", Glob: "nl*"}}
	got := ExpandCatalog(base, table, nil, nil)
	counts, paths := map[string]int{}, map[string]string{}
	var whole []string
	for _, match := range got {
		counts[asciiLower(strings.TrimPrefix(match.Path, base+"/"))]++
		paths[match.Agent+" "+match.Glob] = strings.TrimPrefix(match.Path, base+"/")
		if match.Agent == "whole" {
			whole = append(whole, match.Path)
		}
	}
	variant, variantMatched := paths["cursor .config/Cursor/User"]
	checks := []struct {
		name string
		ok   bool
	}{
		{"a path three lines reach, an unreadable directory, is one match of the first line," +
			" spelled as on disk", counts[".config/goose"] == 1 &&
			paths["goose .config/goose"] == ".config/goose"},
		{"a variant inside another match takes its spelling where they are one directory," +
			" and matches nothing where case tells them apart",
			variantMatched == folds && (!folds || variant == ".config/cursor/User")},
		{"a name holding a newline is one match under its whole name",
			len(whole) == 1 && whole[0] == base+"/nl\nx"},
	}
	for _, check := range checks {
		if !check.ok {
			t.Errorf("%s: %+v", check.name, got)
		}
	}
}

func TestPlanFileCheckOrder(t *testing.T) {
	base := fixture{files: []string{"plain", "big-file", "secret-big"},
		links: [][2]string{{"plain", "secret-link"}}}.build(t)
	mustDo(t, syscall.Mkfifo(base+"/secret-fifo", 0o600))
	cases := []struct {
		name       string
		path       string
		noSecrets  bool
		wantStatus manifest.Status
		wantKind   string
		wantSecret bool
	}{
		{"a symlink is never skipped for its name or size", "secret-link", true,
			manifest.Symlink, "symlink", true},
		{"a special file is special even as a secret under --no-secrets", "secret-fifo", true,
			manifest.SkippedSpecial, "fifo", true},
		{"a secret under --no-secrets is skipped whatever its size", "secret-big", true,
			manifest.SkippedSecret, "file", true},
		{"a secret over the cap is skipped for size by default", "secret-big", false,
			manifest.SkippedSize, "file", true},
		{"a file over the cap is skipped for size", "big-file", false,
			manifest.SkippedSize, "file", false},
		{"anything else is copied", "plain", false, "", "file", false},
		{"a missing path is planned as a copy that will fail", "gone", false, "", "file", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			collector, _, _ := testCollector(t, base, nil)
			collector.MaxSize, collector.NoSecrets = 6, test.noSecrets
			owner := scope{home: base, stager: collector.Stager}
			file := collector.planFile(owner, base+"/"+test.path, false)
			if file.status != test.wantStatus || file.kind != test.wantKind ||
				file.secret != test.wantSecret {
				t.Errorf("planned %+v", file)
			}
		})
	}
}

// eventLog records manifest rows and log lines in the order they are written.
type eventLog struct{ events []string }

// Line keeps a log line.
func (log *eventLog) Line(text string) error {
	log.events = append(log.events, "log "+text)
	return nil
}

// Write keeps each manifest row the writer gives it.
func (log *eventLog) Write(data []byte) (int, error) {
	for _, row := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		log.events = append(log.events, "row "+row)
	}
	return len(data), nil
}

// summary returns each event as "row <path> <status>" or "log <text up to
// the path's end>", paths relative to base.
func (log *eventLog) summary(base string) []string {
	var shown []string
	for _, event := range log.events {
		if row, isRow := strings.CutPrefix(event, "row "); isRow {
			buffer := bytes.NewBufferString(row)
			path := strings.TrimPrefix(field(buffer, "path")[0], base+"/")
			shown = append(shown, "row "+path+" "+field(bytes.NewBufferString(row), "status")[0])
			continue
		}
		line, _, _ := strings.Cut(strings.ReplaceAll(event, base+"/", ""), ": open")
		shown = append(shown, line)
	}
	return shown
}

// listingOrder returns the names in directory in the order its listing
// returns them, the order the walk visits them.
func listingOrder(t *testing.T, directory string) []string {
	t.Helper()
	handle, err := os.Open(directory)
	mustDo(t, err)
	defer handle.Close()
	names, err := handle.Readdirnames(-1)
	mustDo(t, err)
	return names
}

func TestRowsAndTheirLinesFollowTheWalk(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file and directory")
	}
	base := fixture{files: []string{"m/a", "m/locked", "m/skip/x", "m/noread/y", "m/sub/b"}}.build(t)
	for _, name := range []string{"m/locked", "m/noread"} {
		mustDo(t, os.Chmod(base+"/"+name, 0))
		t.Cleanup(func() { _ = os.Chmod(base+"/"+name, 0o755) })
	}
	collector, _, _ := testCollector(t, base, []string{"m/skip"})
	recorder := &eventLog{}
	collector.Log, collector.Manifest = recorder, manifest.NewWriter(recorder)
	_, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	events := map[string][]string{
		"a":      {"row m/a collected"},
		"locked": {"log copy failed: m/locked", "row m/locked error_copy"},
		"skip":   {"row m/skip skipped_excluded"},
		"noread": {"row m/noread error_read", "log read failed: m/noread"},
		"sub":    {"row m/sub/b collected"},
	}
	var want []string
	for _, name := range listingOrder(t, base+"/m") {
		want = append(want, events[name]...)
	}
	if got := recorder.summary(base); !reflect.DeepEqual(got, want) {
		t.Errorf("events in order\n%q\nwant, in the listing's order\n%q", got, want)
	}
}

func TestADirectoryThatCannotBeListedHasItsRowItsLineAndItsCount(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	base := fixture{files: []string{"m/locked/f", "m/noenter/a", "m/noenter/sub/b"},
		dirs: []string{"m/empty"}}.build(t)
	modes := map[string]os.FileMode{"m/locked": 0, "m/noenter": 0o600, "m/empty": 0o600}
	for name, mode := range modes {
		mustDo(t, os.Chmod(base+"/"+name, mode))
		t.Cleanup(func() { _ = os.Chmod(base+"/"+name, 0o755) })
	}
	collector, rows, log := testCollector(t, base, nil)
	unreadable, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	var failed []string
	for _, line := range log.lines {
		if path, found := strings.CutPrefix(line, "read failed: "); found {
			directory, _, _ := strings.Cut(path, ": ")
			failed = append(failed, strings.TrimPrefix(directory, base))
		} else {
			t.Errorf("log line %q, want only read failed: lines", line)
		}
	}
	slices.Sort(failed)
	want := []string{"/m/empty", "/m/locked", "/m/noenter"}
	if !reflect.DeepEqual(failed, want) || unreadable != 1 {
		t.Errorf("read failed lines %q (unreadable %d), want %q and one that cannot be listed",
			failed, unreadable, want)
	}
	var paths []string
	for _, path := range field(rows, "path") {
		paths = append(paths, strings.TrimPrefix(path, base))
	}
	slices.Sort(paths)
	if got := strings.Count(rows.String(), `"status":"error_read"`); got != 3 ||
		!reflect.DeepEqual(paths, want) {
		t.Errorf("%d error_read rows for %q, want one per directory", got, paths)
	}
}

func TestAListingThatFailsAfterTheCheckHasItsRowItsLineAndItsCount(t *testing.T) {
	base := fixture{files: []string{"m/a", "m/b"}}.build(t)
	handle, err := os.Open(base + "/m")
	mustDo(t, err)
	listed, err := handle.ReadDir(-1)
	mustDo(t, errors.Join(err, handle.Close()))
	failure := &fs.PathError{Op: "readdirent", Path: base + "/m", Err: syscall.EIO}
	reason := "readdirent " + base + "/m: input/output error"
	cases := []struct {
		name    string
		entries []fs.DirEntry
		want    []string
	}{
		{"an open that fails visits nothing", nil, []string{"/m error_read"}},
		{"a listing that fails part way visits the entries it read", listed[:1],
			[]string{"/m error_read", "/m/" + listed[0].Name() + " collected"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			collector, rows, log := testCollector(t, base, nil)
			collector.listDirectory = func(directory string) ([]fs.DirEntry, error) {
				if directory != base+"/m" {
					t.Errorf("listed %s, want only the start", directory)
				}
				return testCase.entries, failure
			}
			unreadable, err := collector.CollectTree(context.Background(), "u", base, "agent",
				base+"/m")
			mustDo(t, err)
			var got []string
			for i, path := range field(rows, "path") {
				got = append(got, strings.TrimPrefix(path, base)+" "+field(rows, "status")[i])
			}
			if !reflect.DeepEqual(got, testCase.want) || unreadable != 1 ||
				!strings.Contains(rows.String(), `"error":"`+reason+`"`) {
				t.Errorf("rows %q (unreadable %d), want %q, the first with %q", got, unreadable,
					testCase.want, reason)
			}
			if want := []string{"read failed: " + base + "/m: " + reason}; !reflect.DeepEqual(
				log.lines, want) {
				t.Errorf("log %q, want %q", log.lines, want)
			}
		})
	}
}

func TestNoWalkStagesTheRunsOwnOutputs(t *testing.T) {
	base := fixture{files: []string{".claude/d1/f", ".claude/m/other", ".claude/m/r.log",
		".claude/m/r.manifest.jsonl", ".claude/m/.stage-r/fs/x", ".claude/m/rx",
		".claude/d2/r.tar.gz.partial", ".claude/d2/.stage-r/fs/y", ".claude/d2/xr.log"}}.build(t)
	collector, rows, log := testCollector(t, base, nil)
	collector.Output = stage.RunOutput{RunName: "r"}
	table := []catalog.Entry{{Agent: "a", Glob: ".claude"}, {Agent: "b", Glob: ".claude/m/r.log"},
		{Agent: "c", Glob: ".claude/m/.stage-r"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", base, table, ""))
	var paths []string
	for _, path := range field(rows, "path") {
		paths = append(paths, strings.TrimPrefix(path, base+"/"))
	}
	slices.Sort(paths)
	want := []string{".claude/d1/f", ".claude/d2/xr.log", ".claude/m/other", ".claude/m/rx"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("rows of %q, want only %q, wherever the run's outputs lie", paths, want)
	}
	if want := []string{"  [a] " + base + "/.claude"}; !reflect.DeepEqual(log.lines, want) {
		t.Errorf("log %q, want only %q", log.lines, want)
	}
}

func TestInventoryLogsADirectoryItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	base := fixture{files: []string{"m/locked/f", "m/g"}}.build(t)
	mustDo(t, os.Chmod(base+"/m/locked", 0))
	t.Cleanup(func() { _ = os.Chmod(base+"/m/locked", 0o755) })
	collector, rows, log := testCollector(t, base, nil)
	collector.Inventory, collector.Records = true, &inventoryRecords{}
	_, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	prefix := "read failed: " + base + "/m/locked: "
	if len(log.lines) != 1 || !strings.HasPrefix(log.lines[0], prefix) {
		t.Errorf("log %q, want one line starting %q", log.lines, prefix)
	}
	if rows.Len() != 0 {
		t.Errorf("inventory wrote rows %q", rows.String())
	}
}

func TestAnExcludedDirectoryRowKeepsItsOwnMetadata(t *testing.T) {
	base := fixture{files: []string{"m/skip/x"}}.build(t)
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	mustDo(t, os.Chtimes(base+"/m/skip", old, old))
	collector, rows, _ := testCollector(t, base, []string{"m/skip"})
	_, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	info, err := os.Lstat(base + "/m/skip")
	mustDo(t, err)
	want := map[string]string{"type": "dir", "status": "skipped_excluded",
		"mtime": strconv.FormatInt(old.Unix(), 10),
		"mode":  strconv.FormatUint(uint64(info.Mode().Perm()), 8)}
	for name, value := range want {
		if got := field(rows, name); len(got) != 1 || got[0] != value {
			t.Errorf("%s = %v, want %q", name, got, value)
		}
	}
	if got := field(rows, "size"); len(got) != 1 || got[0] == "0" {
		t.Errorf("size %v, want the directory's disk use", got)
	}
}

func TestACollectedRowRecordsTheBytesCopied(t *testing.T) {
	file := plannedFile{path: "/h/grew", kind: "file", status: manifest.Collected, written: 7,
		meta: fsmeta.Meta{Kind: fsmeta.File, Size: 3}}
	if row := fileRow(scope{}, file); row.Size != 7 {
		t.Errorf("size %d, want the 7 bytes copied, not the 3 lstat gave", row.Size)
	}
}

func TestANameEndingInABarIsCollectedUnderItsOwnName(t *testing.T) {
	base := fixture{files: []string{".claude.json|"}}.build(t)
	collector, rows, log := testCollector(t, base, nil)
	collector.MaxSize = 0
	table := []catalog.Entry{{Agent: "claude-code", Glob: ".claude.json?"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", base, table, ""))
	if got := field(rows, "path"); !reflect.DeepEqual(got, []string{base + "/.claude.json|"}) {
		t.Errorf("paths %q, want the file's own name", got)
	}
	if got := field(rows, "status"); !reflect.DeepEqual(got, []string{"collected"}) {
		t.Errorf("statuses %q, log %q", got, log.lines)
	}
}

func TestFailedCopyThroughRecreatedLink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable file")
	}
	root := fixture{files: []string{"home/a/sub/f"}}.build(t)
	outside := fixture{files: []string{"f"}}.build(t)
	mustDo(t, os.Chmod(root+"/home/a/sub/f", 0))
	collector, rows, log := testCollector(t, root, nil)
	collector.MaxSize = 0
	mustDo(t, os.MkdirAll(collector.Stager.Dir+"/fs/home/a", 0o700))
	mustDo(t, os.Symlink(outside, collector.Stager.Dir+"/fs/home/a/sub"))
	source := root + "/home/a/sub/f"
	_, err := collector.CollectPath(context.Background(), "a", root+"/home/a", "x", source)
	mustDo(t, err)
	if got := field(rows, "status"); !reflect.DeepEqual(got, []string{"error_copy"}) {
		t.Errorf("statuses %v", got)
	}
	if len(log.lines) != 1 || !strings.HasSuffix(log.lines[0],
		"; staged path resolves outside the staging directory, not removed") {
		t.Errorf("log %q", log.lines)
	}
	if data, _ := os.ReadFile(outside + "/f"); string(data) != "f" {
		t.Errorf("the file outside the staging directory was changed: %q", data)
	}
}

func TestProjectUnderARecreatedLinkLeavesItsSource(t *testing.T) {
	root := fixture{files: []string{"home/dot/dotfiles/claude/proj/AGENTS.md"}}.build(t)
	home := root + "/home/dot"
	mustDo(t, os.Symlink(home+"/dotfiles/claude", home+"/.claude"))
	source := home + "/dotfiles/claude/proj/AGENTS.md"
	content, err := os.ReadFile(source)
	mustDo(t, err)
	collector, rows, log := testCollector(t, root, nil)
	collector.MaxSize = 0
	ctx := context.Background()
	mustDo(t, collector.CollectCatalog(ctx, "dot", home,
		[]catalog.Entry{{Agent: "claude-code", Glob: ".claude"}}, ""))
	mustDo(t, collector.CollectCatalog(ctx, "dot", home+"/.claude/proj",
		[]catalog.Entry{{Agent: "agents-md", Glob: "AGENTS.md"}}, ""))
	staged := home + "/.claude/proj/AGENTS.md"
	failed := "copy failed: " + staged + ": "
	note := "; staged path resolves outside the staging directory, not removed"
	if got := field(rows, "status"); !reflect.DeepEqual(got, []string{"symlink", "error_copy"}) {
		t.Errorf("statuses %v", got)
	}
	if got := field(rows, "archive_path"); len(got) != 2 || got[1] != "" {
		t.Errorf("archive paths %q", got)
	}
	if last := log.lines[len(log.lines)-1]; !strings.HasPrefix(last, failed) ||
		!strings.HasSuffix(last, note) {
		t.Errorf("log %q", log.lines)
	}
	if data, err := os.ReadFile(source); err != nil || !bytes.Equal(data, content) {
		t.Errorf("the source was changed: %q, %v", data, err)
	}
	var staging []string
	mustDo(t, filepath.WalkDir(collector.Stager.Dir, func(path string, _ os.DirEntry, err error) error {
		staging = append(staging, strings.TrimPrefix(path, collector.Stager.Dir))
		return err
	}))
	want := []string{"", "/fs", "/fs/home", "/fs/home/dot", "/fs/home/dot/.claude"}
	if !reflect.DeepEqual(staging, want) {
		t.Errorf("staging tree %q, want %q", staging, want)
	}
}

func TestNamesWithANewlineOrAQuote(t *testing.T) {
	base := fixture{files: []string{"m/a\"b", "m/c\nd"}}.build(t)
	collector, rows, _ := testCollector(t, base, nil)
	_, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	text := rows.String()
	for _, want := range []string{`/m/a\"b","archive_path":"fs`, `/m/c\nd","archive_path":"fs`} {
		if !strings.Contains(text, want) {
			t.Errorf("no row holding %s in\n%s", want, text)
		}
	}
	if got := field(rows, "status"); !reflect.DeepEqual(got, []string{"collected", "collected"}) {
		t.Errorf("statuses %v", got)
	}
}

// inventoryRecords keeps what an inventory walk reports.
type inventoryRecords struct{ lines []string }

// Match keeps an M line.
func (records *inventoryRecords) Match(agent, glob string) {
	records.lines = append(records.lines, "M|"+agent+"|"+glob)
}

// File keeps an F line with the size.
func (records *inventoryRecords) File(agent string, size, _ int64) {
	records.lines = append(records.lines, "F|"+agent+"|"+strconv.FormatInt(size, 10))
}

func TestInventoryRecordsWithoutStaging(t *testing.T) {
	base := fixture{files: []string{".tool/abc", ".tool/skip/x", ".shared/f"},
		links: [][2]string{{"abc", ".tool/link"}}}.build(t)
	collector, rows, _ := testCollector(t, base, []string{".tool/skip"})
	records := &inventoryRecords{}
	collector.Inventory, collector.Records = true, records
	table := []catalog.Entry{{Agent: "tool", Glob: ".tool"}, {Agent: sharedAgent, Glob: ".shared"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", base, table, ""))
	want := []string{"M|tool|.tool", "F|tool|9"}
	if !reflect.DeepEqual(records.lines, want) || rows.Len() != 0 {
		t.Errorf("records %q, rows %q", records.lines, rows.String())
	}
}

func TestInventoryCountsOnlyRegularFiles(t *testing.T) {
	base := fixture{files: []string{"f"}}.build(t)
	mustDo(t, syscall.Mkfifo(base+"/fifo", 0o644))
	collector, _, _ := testCollector(t, base, nil)
	records := &inventoryRecords{}
	collector.Inventory, collector.Records = true, records
	owner := scope{user: "u", home: base, agent: "tool"}
	for _, name := range []string{"f", "fifo", "missing"} {
		collector.recordFile(owner, base+"/"+name)
	}
	want := []string{"F|tool|" + strconv.Itoa(len("f")), "F|tool|0"}
	if !reflect.DeepEqual(records.lines, want) {
		t.Errorf("records %q, want the file and the missing path only", records.lines)
	}
}

func TestCancelledContextStopsTheWalk(t *testing.T) {
	base := fixture{files: []string{"m/a"}}.build(t)
	collector, rows, _ := testCollector(t, base, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := collector.CollectTree(ctx, "u", base, "agent", base+"/m")
	if !errors.Is(err, context.Canceled) || rows.Len() != 0 {
		t.Errorf("err %v, rows %q", err, rows.String())
	}
}

func TestWriteReadErrorWritesTheRowAndTheLogLine(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	mustDo(t, os.Mkdir(project, 0o755))
	collector, rows, log := testCollector(t, root, nil)
	mustDo(t, collector.WriteReadError("alice", project, project, "permission denied"))
	want := map[string]string{"user": "alice", "home": project, "agent": "", "path": project,
		"type": "dir", "size": "0", "status": "error_read"}
	for name, value := range want {
		if got := field(rows, name); len(got) != 1 || got[0] != value {
			t.Errorf("%s = %v, want %q", name, got, value)
		}
	}
	if !strings.HasSuffix(rows.String(), `"error":"permission denied"}`+"\n") {
		t.Errorf("row %q does not end with the error", rows.String())
	}
	if line := "read failed: " + project + ": permission denied"; len(log.lines) != 1 ||
		log.lines[0] != line {
		t.Errorf("log %q, want %q", log.lines, line)
	}
}

// outOfRootTable is the catalog the out-of-root walk tests expand: a literal
// path and a glob, both through .config.
func outOfRootTable() []catalog.Entry {
	return []catalog.Entry{{Agent: "vscode", Glob: ".config/Code/User"},
		{Agent: "editors", Glob: ".config/*/User"}}
}

// walkOutOfRoot walks bob's home under root with the out-of-root table,
// checking the image root unless live, and returns the rows and log lines.
func walkOutOfRoot(t *testing.T, root string, live bool) (string, []string) {
	t.Helper()
	collector, rows, log := testCollector(t, root, nil)
	if live {
		collector.RootCheck = RootCheck{}
	}
	err := collector.CollectCatalog(context.Background(), "bob", root+"/home/bob",
		outOfRootTable(), "User bob")
	mustDo(t, err)
	return rows.String(), log.lines
}

func TestCatalogLooksAtNothingBehindALinkOutOfTheRoot(t *testing.T) {
	root := fixture{dirs: []string{"home/bob"}}.build(t)
	outside := fixture{}.build(t)
	mustDo(t, os.Symlink(outside, root+"/home/bob/.config"))
	emptyRows, emptyLines := walkOutOfRoot(t, root, false)
	for _, dir := range []string{"Code/User", "SecretApp/User"} {
		mustDo(t, os.MkdirAll(outside+"/"+dir, 0o755))
		mustDo(t, os.WriteFile(outside+"/"+dir+"/settings.json", []byte("{}"), 0o644))
	}
	fullRows, fullLines := walkOutOfRoot(t, root, false)
	if fullRows != emptyRows || strings.Join(fullLines, "\n") != strings.Join(emptyLines, "\n") {
		t.Errorf("what the outside holds changed the output:\n%s%q\nversus\n%s%q",
			emptyRows, emptyLines, fullRows, fullLines)
	}
	link := root + "/home/bob/.config"
	wantLine := "  [shared] " + link + ": the symlinked directory leads out of the image root;" +
		" nothing under it was listed, not walked"
	if want := []string{"User bob", wantLine}; !reflect.DeepEqual(fullLines, want) {
		t.Errorf("log %q, want %q", fullLines, want)
	}
	paths, kinds := field(rows(fullRows), "path"), field(rows(fullRows), "type")
	if !reflect.DeepEqual(paths, []string{link}) || !reflect.DeepEqual(kinds, []string{"symlink"}) {
		t.Errorf("rows %s, want one symlink row for %s", fullRows, link)
	}
}

// rows wraps manifest text for field.
func rows(text string) *bytes.Buffer { return bytes.NewBufferString(text) }

func TestCatalogStillExpandsLiveAndThroughALinkInsideTheRoot(t *testing.T) {
	cases := []struct {
		name, target string
		live         bool
		wantLine     string
	}{
		{"a live run lists through the link as before", "{OUT}", true,
			"/home/bob/.config/Code/User: under the symlinked directory"},
		{"a link inside the home is expanded", "{ROOT}/home/bob/dotfiles", false,
			"/home/bob/.config/Code/User (through the symlinked directory"},
		{"a link out of the home inside the image root is listed and refused as before",
			"{ROOT}/dotfiles", false, "/home/bob/.config/Code/User: under the symlinked directory"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fixture{dirs: []string{"home/bob/dotfiles/Code/User", "dotfiles/Code/User"}}.build(t)
			outside := fixture{dirs: []string{"Code/User"}}.build(t)
			target := strings.NewReplacer("{OUT}", outside, "{ROOT}", root).Replace(testCase.target)
			mustDo(t, os.Symlink(target, root+"/home/bob/.config"))
			_, lines := walkOutOfRoot(t, root, testCase.live)
			if !strings.Contains(strings.Join(lines, "\n"), testCase.wantLine) {
				t.Errorf("log %q, want a line holding %q", lines, testCase.wantLine)
			}
		})
	}
}

func TestADirectorySwappedForALinkOutOfTheRootIsNotOpened(t *testing.T) {
	root := fixture{dirs: []string{"home/bob/.tool"}}.build(t)
	outside := fixture{}.build(t)
	link := root + "/home/bob/.tool/swapped"
	mustDo(t, os.Symlink(outside+"/gone", link))
	collector, rows, log := testCollector(t, root, nil)
	found := &treeWalk{start: root + "/home/bob/.tool"}
	owner := scope{user: "bob", home: root + "/home/bob", agent: "tool", stager: collector.Stager}
	// The listing saw a directory where the link now is.
	mustDo(t, collector.visit(context.Background(), owner, found, link, true))
	if found.unreadable != 0 || len(log.lines) != 1 ||
		!strings.HasPrefix(log.lines[0], "read failed: "+link+": ") {
		t.Errorf("unreadable %d, log %q: want the read failed line alone", found.unreadable, log.lines)
	}
	if got := field(rows, "error"); len(got) != 1 || !strings.Contains(got[0], "image root") {
		t.Errorf("rows %q, want one error_read row with the refusal as its reason", rows)
	}
}

// linkAgents returns the agent of each symlink row of rows, by path relative
// to home.
func linkAgents(rows *bytes.Buffer, home string) map[string]string {
	agents := map[string]string{}
	paths, kinds, owners := field(rows, "path"), field(rows, "type"), field(rows, "agent")
	for i, path := range paths {
		if kinds[i] == "symlink" {
			agents[strings.TrimPrefix(path, home+"/")] = owners[i]
		}
	}
	return agents
}

func TestALinkRowNamesTheAgentsWhoseMatchesCrossIt(t *testing.T) {
	t.Parallel()
	amp, goose := catalog.Entry{Agent: "amp", Glob: ".config/amp"},
		catalog.Entry{Agent: "goose", Glob: ".config/goose"}
	inBase := fixture{dirs: []string{"home/bob/dot/amp", "home/bob/dot/goose"},
		links: [][2]string{{"dot", "home/bob/.config"}}}
	outOfBase := fixture{dirs: []string{"home/bob", "dotfiles/amp", "dotfiles/goose"},
		links: [][2]string{{"../../dotfiles", "home/bob/.config"}}}
	nested := fixture{dirs: []string{"home/bob/dot/amp", "home/bob/dot/goose", "home/bob/sess"},
		files: []string{"home/bob/sess/db"},
		links: [][2]string{{"dot", "home/bob/.config"}, {"../../sess", "home/bob/dot/goose/sessions"}}}
	cases := []struct {
		name  string
		image fixture
		table []catalog.Entry
		want  map[string]string
	}{
		{"one agent through a link inside the home gives that agent", inBase,
			[]catalog.Entry{amp}, map[string]string{".config": "amp"}},
		{"two agents through a link inside the home give shared", inBase,
			[]catalog.Entry{amp, goose}, map[string]string{".config": "shared"}},
		{"a shared catalog line counts as one more agent", inBase,
			[]catalog.Entry{amp, {Agent: "shared", Glob: ".config/goose"}},
			map[string]string{".config": "shared"}},
		{"each nested link is judged by the matches that cross it", nested,
			[]catalog.Entry{amp, {Agent: "goose", Glob: ".config/goose/sessions/db"}},
			map[string]string{".config": "shared", ".config/goose/sessions": "goose"}},
		{"two agents through a link out of the home give shared", outOfBase,
			[]catalog.Entry{amp, goose}, map[string]string{".config": "shared"}},
		{"one agent through a link out of the home gives that agent", outOfBase,
			[]catalog.Entry{goose}, map[string]string{".config": "goose"}},
		{"a link a match claims keeps its claimant", inBase,
			[]catalog.Entry{goose, {Agent: "amp", Glob: ".config"}},
			map[string]string{".config": "amp"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := testCase.image.build(t)
			collector, rows, _ := testCollector(t, root, nil)
			home := root + "/home/bob"
			mustDo(t, collector.CollectCatalog(context.Background(), "bob", home, testCase.table, ""))
			if got := linkAgents(rows, home); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("link rows %v, want %v\n%s", got, testCase.want, rows)
			}
		})
	}
}

func TestALinkOutOfTheRootIsSharedEvenWhenOneLineReachesIt(t *testing.T) {
	t.Parallel()
	root := fixture{dirs: []string{"home/bob"}}.build(t)
	outside := fixture{dirs: []string{"amp"}}.build(t)
	link := root + "/home/bob/.config"
	mustDo(t, os.Symlink(outside, link))
	collector, rows, log := testCollector(t, root, nil)
	table := []catalog.Entry{{Agent: "amp", Glob: ".config/amp"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "bob", root+"/home/bob", table, ""))
	if got := linkAgents(rows, root+"/home/bob"); !reflect.DeepEqual(got, map[string]string{".config": "shared"}) {
		t.Errorf("link rows %v, want .config shared\n%s", got, rows)
	}
	if len(log.lines) != 1 || !strings.HasPrefix(log.lines[0], "  [shared] "+link+":") {
		t.Errorf("log %q, want one line starting [shared] %s", log.lines, link)
	}
}
