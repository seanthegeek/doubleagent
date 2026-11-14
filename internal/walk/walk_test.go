//go:build unix

package walk

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
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
		Tables: &catalog.Tables{}, Excludes: excludes, Secret: MatchAny([]string{"*secret*"}),
		Log: log, Manifest: manifest.NewWriter(rows), Root: root, MaxSize: 10,
		Stager: stage.Stager{Dir: stageDir, Physical: stageDir, Root: root},
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

func TestReadField(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"a path without a pipe is kept", "/h/x", "/h/x"},
		{"one final pipe is dropped", "/h/x|", "/h/x"},
		{"a final pipe after another pipe is kept", "/h/x|y|", "/h/x|y|"},
		{"two final pipes are kept", "/h/x||", "/h/x||"},
		{"a lone pipe becomes empty", "|", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := readField(test.path); got != test.want {
				t.Errorf("readField(%q) = %q", test.path, got)
			}
		})
	}
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
			got, outside := LinkedParent(base, test.path)
			if outside != test.outside || !reflect.DeepEqual(got, test.want) {
				t.Errorf("LinkedParent = %+v, %v", got, outside)
			}
		})
	}
}

func TestClaimed(t *testing.T) {
	base := fixture{dirs: []string{".gemini/antigravity-cli/x", "real"},
		links: [][2]string{{"real", ".gemini/link"}}}.build(t)
	claims := map[string]struct{}{base + "/.gemini": {}, base + "/.gemini/antigravity-cli": {}}
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
		{Agent: "split", Glob: "nl*"}}
	got := ExpandCatalog(base, table)
	notes, paths := map[string]bool{}, map[string]string{}
	var split []string
	for _, match := range got {
		notes[match.Agent+" "+match.Glob] = match.Note != ""
		paths[match.Agent+" "+match.Glob] = strings.TrimPrefix(match.Path, base+"/")
		if match.Agent == "split" {
			split = append(split, match.Path)
		}
	}
	variant, variantMatched := paths["cursor .config/Cursor/User"]
	checks := []struct {
		name string
		ok   bool
	}{
		{"an exact second match of one path is dropped", notes["pearai .config/goose"]},
		{"an unreadable case variant is dropped only where the file system folds case",
			notes["goose .config/Goose"] == folds},
		{"a variant inside another match takes its spelling where they are one directory," +
			" and matches nothing where case tells them apart",
			variantMatched == folds && (!folds || variant == ".config/cursor/User")},
		{"a name holding a newline gives one match per line",
			len(split) == 2 && split[0] == base+"/nl" && split[1] == "x"},
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
			file := collector.planFile(owner, base+"/"+test.path, collector.Secret)
			if file.status != test.wantStatus || file.kind != test.wantKind ||
				file.secret != test.wantSecret {
				t.Errorf("planned %+v", file)
			}
		})
	}
}

func TestCollectTreeWritesEachGroupWhole(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	base := fixture{files: []string{"m/a", "m/skip/x", "m/locked/y", "m/z"}}.build(t)
	mustDo(t, os.Chmod(base+"/m/locked", 0))
	t.Cleanup(func() { _ = os.Chmod(base+"/m/locked", 0o755) })
	collector, rows, log := testCollector(t, base, []string{"m/skip"})
	unreadable, err := collector.CollectTree(context.Background(), "u", base, "agent", base+"/m")
	mustDo(t, err)
	want := []string{"skipped_excluded", "error_read", "collected", "collected"}
	if findRunsLastBatchFirst {
		want = []string{"collected", "collected", "error_read", "skipped_excluded"}
	}
	if got := field(rows, "status"); !reflect.DeepEqual(got, want) || unreadable != 1 {
		t.Errorf("statuses %v, unreadable %d", got, unreadable)
	}
	if len(log.lines) != 2 || !strings.HasPrefix(log.lines[0], "find: "+base+"/m/locked: ") {
		t.Errorf("log %q", log.lines)
	}
}

func TestFindLinesForUnreadDirectories(t *testing.T) {
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
	var finds []string
	for _, line := range log.lines {
		if path, found := strings.CutPrefix(line, "find: "); found {
			finds = append(finds, strings.TrimPrefix(path[:strings.LastIndex(path, ": ")], base))
		}
	}
	want := []string{"/m/locked"}
	if findReportsUnenterableEntries {
		names, err := os.ReadDir(base + "/m/noenter")
		mustDo(t, err)
		for _, name := range names {
			want = append(want, "/m/noenter/"+name.Name())
		}
	}
	// The lines come in directory order, which differs by file system.
	sort.Strings(finds)
	sort.Strings(want)
	if !reflect.DeepEqual(finds, want) || unreadable != len(want) {
		t.Errorf("find lines %q (unreadable %d), want %q", finds, unreadable, want)
	}
	if got := strings.Count(rows.String(), `"status":"error_read"`); got != 3 {
		t.Errorf("%d error_read rows, want one per directory", got)
	}
	wantCopyErrors := 0
	if findReportsUnenterableEntries {
		wantCopyErrors = 2
	}
	copyErrors := strings.Count(rows.String(), `"mode":"0","sha256":"","secret":false,"status":"error_copy"`)
	if copyErrors != wantCopyErrors {
		t.Errorf("%d error_copy rows with no metadata, want %d", copyErrors, wantCopyErrors)
	}
}

func TestUnenterableEntriesFollowFindsExpression(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	if !findReportsUnenterableEntries {
		t.Skip("this platform's find writes nothing for such entries")
	}
	base := fixture{files: []string{"m/noenter/cache/x", "m/noenter/keep", "m/noenter/claimed/y"}}.build(t)
	mustDo(t, os.Chmod(base+"/m/noenter", 0o600))
	t.Cleanup(func() { _ = os.Chmod(base+"/m/noenter", 0o755) })
	collector, rows, _ := testCollector(t, base, []string{"m/noenter/cache"})
	owner := scope{user: "u", home: base, agent: "agent", stager: collector.Stager,
		claims: map[string]struct{}{base + "/m/noenter/claimed": {}}}
	_, err := collector.collectTree(context.Background(), owner, base+"/m")
	mustDo(t, err)
	manifest := rows.String()
	checks := []struct {
		name string
		ok   bool
	}{
		{"an excluded entry gets a skipped_excluded row",
			strings.Contains(manifest, `/m/noenter/cache","archive_path":"","type":"file",`) &&
				strings.Count(manifest, `"status":"skipped_excluded"`) == 1},
		{"a claimed entry gets no row", !strings.Contains(manifest, "/m/noenter/claimed")},
		{"the other entry gets an error_copy row",
			strings.Contains(manifest, `/m/noenter/keep",`) &&
				strings.Count(manifest, `"status":"error_copy"`) == 1},
	}
	for _, check := range checks {
		if !check.ok {
			t.Errorf("%s:\n%s", check.name, manifest)
		}
	}
	collector.Inventory, collector.Records = true, &countingRecords{}
	rows.Reset()
	_, err = collector.collectTree(context.Background(), owner, base+"/m")
	mustDo(t, err)
	if files := collector.Records.(*countingRecords).files; files != 0 {
		t.Errorf("inventory counted %d files under an unenterable directory, want 0", files)
	}
}

// countingRecords counts the inventory records a walk makes.
type countingRecords struct{ matches, files int }

func (records *countingRecords) Match(string, string)      { records.matches++ }
func (records *countingRecords) File(string, int64, int64) { records.files++ }

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
