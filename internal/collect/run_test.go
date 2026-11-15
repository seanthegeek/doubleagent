package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestRun(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		options    Options
		action     Action
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"list prints the catalog", Options{}, ActList, 0, string(catalog.Text()), ""},
		{"version prints the tool and version", Options{}, ActVersion, 0, "doubleagent 0.0.0-dev", ""},
		{"help prints the usage", Options{}, ActHelp, 0, UsageText, ""},
		{"a bad root is fatal", Options{Root: "/no/such/dir"}, ActRun, 2, "",
			"Root is not a directory: /no/such/dir\n"},
		{"an output directory that cannot be created is fatal",
			Options{Root: t.TempDir(), OutDir: file + "/out"}, ActRun, 2, "",
			"Cannot create output dir: " + file + "/out\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), testCase.options, testCase.action, &stdout, &stderr)
			if code != testCase.wantCode {
				t.Errorf("exit code %d, want %d", code, testCase.wantCode)
			}
			if !strings.HasPrefix(stdout.String(), testCase.wantStdout) {
				t.Errorf("stdout %q, want prefix %q", stdout.String(), testCase.wantStdout)
			}
			if !strings.Contains(stderr.String(), testCase.wantStderr) {
				t.Errorf("stderr %q, want it to contain %q", stderr.String(), testCase.wantStderr)
			}
		})
	}
}

// noPrograms is a host that has no program to run.
type noPrograms struct{}

// Output fails: no program exists.
func (noPrograms) Output(name string, _ ...string) ([]byte, error) {
	return nil, errors.New(name + ": not found")
}

// Available reports that no program exists.
func (noPrograms) Available(string) bool { return false }

// testHost is a host with a pinned clock and no programs.
func testHost(t *testing.T) environment {
	pinned := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return environment{now: func() time.Time { return pinned }, runner: noPrograms{},
		getwd:     func() (string, error) { return syspath.FromNative(t.TempDir()), nil },
		findUsers: users.Find}
}

// fixtureImage builds an image with one home holding one agent file and
// returns its root.
func fixtureImage(t *testing.T) string {
	t.Helper()
	root := longTempDir(t)
	dir := filepath.Join(root, "home/alice/.claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// testOptions returns the options of an image run over root into outDir.
func testOptions(root, outDir string) Options {
	return Options{Root: root, OutDir: outDir, Quiet: true, MaxFileBytes: 256 * mebibyte,
		EnvScanMaxBytes: mebibyte}
}

// outputNames lists the names in dir, sorted.
func outputNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRunWritesTheOutputs(t *testing.T) {
	name := hostname() + "_20260102T030405Z_agent-artifacts"
	name = safeHostname(name)
	files := []string{name + ".collection.json", name + ".log", name + ".manifest.jsonl",
		name + ".tar.gz", name + ".tar.gz.sha256"}
	cases := []struct {
		name      string
		change    func(*Options)
		wantNames []string
		wantOut   string
	}{
		{"a run writes the archive and its files and removes the staging directory",
			func(*Options) {}, files, "archive:    "},
		{"-k keeps the staging directory", func(options *Options) { options.KeepStaging = true },
			append([]string{".stage-" + name}, files...), "archive:    "},
		{"--inventory writes nothing and prints the host line",
			func(options *Options) { options.Inventory = true }, nil, `{"type":"host",`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := t.TempDir()
			options := testOptions(fixtureImage(t), outDir)
			testCase.change(&options)
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			if got := outputNames(t, outDir); strings.Join(got, " ") != strings.Join(testCase.wantNames, " ") {
				t.Errorf("outputs %v, want %v", got, testCase.wantNames)
			}
			if !strings.HasPrefix(stdout.String(), testCase.wantOut) {
				t.Errorf("stdout %q, want prefix %q", stdout.String(), testCase.wantOut)
			}
		})
	}
}

// cancelOnText is a standard error that cancels a run once text is written.
type cancelOnText struct {
	text   string
	cancel context.CancelFunc
}

// Write cancels the run when data holds the text.
func (writer *cancelOnText) Write(data []byte) (int, error) {
	if strings.Contains(string(data), writer.text) {
		writer.cancel()
	}
	return len(data), nil
}

func TestCancelledRunLeavesNoStagingDirectory(t *testing.T) {
	outDir := t.TempDir()
	options := testOptions(fixtureImage(t), outDir)
	options.Quiet = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stderr := &cancelOnText{text: "User alice", cancel: cancel}
	var stdout bytes.Buffer
	if code := runCollection(ctx, options, testHost(t), &stdout, stderr); code != exitInterrupted {
		t.Fatalf("exit code %d, want %d", code, exitInterrupted)
	}
	stages, _ := filepath.Glob(outDir + "/.stage-*")
	archives, _ := filepath.Glob(outDir + "/*.tar.gz")
	if len(stages) != 0 || len(archives) != 0 {
		t.Errorf("staging directories %v and archives %v left", stages, archives)
	}
	logs, _ := filepath.Glob(outDir + "/*.log")
	if len(logs) != 1 {
		t.Fatalf("logs %v", logs)
	}
	text, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(text), " interrupted\n") {
		t.Errorf("log does not end with the interrupted line:\n%s", text)
	}
}

func TestEnclosingHome(t *testing.T) {
	homes := []users.Home{{User: "alice", Path: "/home/alice"}, {User: "bob", Path: "/home/bob"},
		{User: "bob", Path: "/srv/bob"}, {User: "nobody", Path: ""}}
	cases := []struct {
		name string
		path string
		want string
	}{
		{"a project under a home has that home", "/home/alice/p", "/home/alice"},
		{"a project under a user's second home has that home", "/srv/bob/p", "/srv/bob"},
		{"a project under another user's home has that home", "/home/bob/p", "/home/bob"},
		{"a project sharing a home's prefix without its slash has none", "/home/alice2/p", ""},
		{"a project under no home has none, an empty home path holding nothing", "/opt/p", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := enclosingHome(homes, testCase.path); got != testCase.want {
				t.Errorf("enclosingHome = %q, want %q", got, testCase.want)
			}
		})
	}
}

// writeImage builds files (path to content) and links (path to target) under
// a new image root and returns it, resolved and canonical.
func writeImage(t *testing.T, files, links map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = syspath.FromNative(root)
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(root+"/"+path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root+"/"+path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, target := range links {
		err := os.Symlink(strings.ReplaceAll(target, "{ROOT}", root), root+"/"+path)
		if symlinkNeedsPrivilege(err) {
			t.Skipf("creating a symbolic link needs the symbolic-link privilege: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// manifestRows returns the manifest rows of the run in outDir, parsed, with
// the text of each.
func manifestRows(t *testing.T, outDir string) ([]map[string]any, []string) {
	t.Helper()
	var rows []map[string]any
	var texts []string
	for _, text := range strings.Split(runOutput(t, outDir, ".manifest.jsonl"), "\n") {
		if text == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(text), &row); err != nil {
			t.Fatalf("row %s: %v", text, err)
		}
		rows, texts = append(rows, row), append(texts, text)
	}
	return rows, texts
}

// manifestRow returns the manifest row of path, canonical, from the run in
// outDir: the row whose native path field is path's native form.
func manifestRow(t *testing.T, outDir, path string) string {
	t.Helper()
	rows, texts := manifestRows(t, outDir)
	for i, row := range rows {
		if row["path"] == syspath.ToNative(path) {
			return texts[i]
		}
	}
	t.Fatalf("no row of %s in:\n%s", path, strings.Join(texts, "\n"))
	return ""
}

// rowUnder reports whether the run in outDir has a row whose path lies
// under dir, canonical.
func rowUnder(t *testing.T, outDir, dir string) bool {
	t.Helper()
	rows, _ := manifestRows(t, outDir)
	for _, row := range rows {
		if path, _ := row["path"].(string); strings.HasPrefix(path, syspath.ToNative(dir)) {
			return true
		}
	}
	return false
}

// runOutput is the text of a run's file with suffix in outDir.
func runOutput(t *testing.T, outDir, suffix string) string {
	t.Helper()
	paths, _ := filepath.Glob(outDir + "/*" + suffix)
	if len(paths) != 1 {
		t.Fatalf("files %v ending in %s", paths, suffix)
	}
	text, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

// hostLinkImage builds an image whose home u holds two links out of the
// image root, ext to a directory that exists on the analyst's host and gone to
// one that does not, and returns the root and the outside directory.
func hostLinkImage(t *testing.T) (root, outside string) {
	t.Helper()
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside = syspath.FromNative(outside)
	if err := os.MkdirAll(outside+"/exists/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	// The missing target is absolute on the image's drive: on Windows a
	// target that starts with one separator is relative to the link's drive.
	missing := filepath.VolumeName(outside) + "/no-such-host-dir"
	root = writeImage(t, map[string]string{"home/u/.claude/settings.json": "{}\n"},
		map[string]string{"home/u/ext": outside + "/exists/", "home/u/gone": missing})
	return root, outside
}

// requireOnce fails unless text holds line exactly once.
func requireOnce(t *testing.T, what, text, line string) {
	t.Helper()
	if strings.Count(text, line) != 1 {
		t.Errorf("%s lacks %q exactly once:\n%s", what, line, text)
	}
}

func TestInventoryCountsNoProjectThatLeavesTheRoot(t *testing.T) {
	root, _ := hostLinkImage(t)
	options := testOptions(root, t.TempDir())
	options.Inventory, options.Quiet = true, true
	options.Projects = []string{root + "/home/u/ext/proj", root + "/home/u/gone/proj"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"projects":0,`) || strings.Contains(stdout.String(), `"projects":1,`) {
		t.Errorf("the inventory counts a project that leaves the root:\n%s", stdout.String())
	}
}

func TestProjectFilesAreStagedWhereTheProjectIs(t *testing.T) {
	physical := "fs/home/dot/dotfiles/claude/proj/"
	cases := []struct {
		name     string
		files    map[string]string
		links    map[string]string
		project  string
		envScan  bool
		wantRows map[string]string
	}{
		{"a hint file of a project through an absolute home link",
			map[string]string{"home/dot/dotfiles/claude/proj/AGENTS.md": "# a\n",
				"home/dot/dotfiles/claude/proj/env.sh": "CLAUDE_CONFIG_DIR=/x\n"},
			map[string]string{"home/dot/.claude": "{ROOT}/home/dot/dotfiles/claude"},
			"home/dot/.claude/proj", true,
			map[string]string{"home/dot/.claude/proj/AGENTS.md": `"user":"dot"`,
				"home/dot/.claude/proj/env.sh": `"user":"dot"`}},
		{"a project another user's state names through the enclosing home's link",
			map[string]string{"home/dot/dotfiles/claude/proj/AGENTS.md": "# a\n",
				"home/alice/.claude/history.jsonl": "{\"project\":\"/home/dot/.claude/proj\"}\n"},
			map[string]string{"home/dot/.claude": "{ROOT}/home/dot/dotfiles/claude"},
			"", false,
			map[string]string{"home/dot/.claude/proj/AGENTS.md": `"user":"alice"`}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeImage(t, testCase.files, testCase.links)
			outDir := t.TempDir()
			options := testOptions(root, outDir)
			options.EnvScanHome = testCase.envScan
			if testCase.project != "" {
				options.Projects = []string{root + "/" + testCase.project}
			}
			var stdout, stderr bytes.Buffer
			if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			for path, user := range testCase.wantRows {
				row := manifestRow(t, outDir, root+"/"+path)
				archivePath := `"archive_path":"` + physical + filepath.Base(path) + `"`
				if !strings.Contains(row, archivePath) || !strings.Contains(row, `"status":"collected"`) ||
					!strings.Contains(row, user) {
					t.Errorf("row %s, want %s, %s and collected", row, user, archivePath)
				}
			}
		})
	}
}

func TestProjectLeadingOutOfTheRootIsRecordedWhateverTheHostHolds(t *testing.T) {
	root, outside := hostLinkImage(t)
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.Projects = []string{root + "/home/u/ext/proj", root + "/home/u/gone/proj",
		outside + "/exists/proj", root + "/home/u/missing"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	native := syspath.ToNative
	for _, link := range []string{"ext", "gone"} {
		requireOnce(t, "the log", log, "Project [u] "+native(root+"/home/u/"+link+"/proj")+
			": under the symlinked directory "+native(root+"/home/u/"+link)+
			", which leads out of the image root; not walked\n")
		if rowUnder(t, outDir, root+"/home/u/"+link+"/") {
			t.Errorf("the manifest holds a row under %s", link)
		}
	}
	requireOnce(t, "the log", log, "Project "+native(outside+"/exists/proj")+
		": is outside the image root; not walked\n")
	if rowUnder(t, outDir, outside) {
		t.Errorf("a row for a project out of the root")
	}
	// A -p value missing inside the image keeps the sh collector's warning.
	requireOnce(t, "the log", log, "WARNING: project given with -p is not a directory: "+
		native(root+"/home/u/missing")+": ")
	if row := manifestRow(t, outDir, root+"/home/u/missing"); !strings.Contains(row, `"status":"error_read"`) {
		t.Errorf("row %s, want error_read", row)
	}
	requireOnce(t, "collection.json", runOutput(t, outDir, ".collection.json"), "\n  \"projects\": [],\n")
	requireOnce(t, "stdout", stdout.String(), "\nprojects:   0\n")
}

func TestProbeWrite(t *testing.T) {
	writable := syspath.FromNative(t.TempDir())
	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"a directory the run may write is writable", writable, true},
		{"a directory that does not exist is not", writable + "/missing", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := probeWrite(testCase.dir); got != testCase.want {
				t.Errorf("probeWrite = %v, want %v", got, testCase.want)
			}
			if names := outputNames(t, writable); len(names) != 0 {
				t.Errorf("the probe left %v", names)
			}
		})
	}
}

// skippedHomesHost is testHost with a user enumeration that finds no home
// and two candidates it could not see: one its parent lists, one gone.
func skippedHomesHost(t *testing.T, root string) environment {
	host := testHost(t)
	host.findUsers = func(bool, string, string, users.Runner) users.Report {
		return users.Report{Skipped: []users.Skipped{
			{Path: root + "/Users/locked", Reason: "access denied", Denied: true},
			{Path: root + "/Users/gone", Reason: "missing"},
		}}
	}
	return host
}

func TestSkippedHomesAreLoggedAndDeniedOnesGetARow(t *testing.T) {
	root := syspath.FromNative(fixtureImage(t))
	if err := os.MkdirAll(root+"/Users/locked", 0o755); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runCollection(context.Background(), testOptions(root, outDir),
		skippedHomesHost(t, root), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	locked, gone := syspath.ToNative(root+"/Users/locked"), syspath.ToNative(root+"/Users/gone")
	wantOrder := []string{"WARNING: no user home directories found\n",
		"profile not accessible (skipped): " + locked + "\n",
		"profile not accessible (skipped): " + gone + "\n",
		"read failed: " + locked + ": access denied\n"}
	position := 0
	for _, line := range wantOrder {
		found := strings.Index(log[position:], line)
		if found < 0 {
			t.Fatalf("the log lacks %q after byte %d:\n%s", line, position, log)
		}
		position += found + len(line)
	}
	row := manifestRow(t, outDir, root+"/Users/locked")
	for _, part := range []string{`"user":"locked"`, `"agent":""`, `"status":"error_read"`,
		`"error":"access denied"`} {
		if !strings.Contains(row, part) {
			t.Errorf("row %s lacks %s", row, part)
		}
	}
	if rowUnder(t, outDir, root+"/Users/gone") {
		t.Error("a home that is gone got a row")
	}
}

func TestStartLogNamesTheRootAndTheAccount(t *testing.T) {
	root := syspath.FromNative(fixtureImage(t))
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t),
		&stdout, &stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	first, _, _ := strings.Cut(runOutput(t, outDir, ".log"), "\n")
	want := " mode=image root=" + syspath.ToNative(root) + " " + identityText(runIdentity())
	if !strings.HasSuffix(first, want) {
		t.Errorf("first log line %q, want it to end with %q", first, want)
	}
}

func TestNativePaths(t *testing.T) {
	registrySource := `HKU\S-1-5-21-1\Environment\CODEX_HOME`
	report := summary.Summary{Root: "C:/img", Homes: []string{"C:/img/Users/a"},
		Projects: []string{"C:/img/p"}, Options: summary.Options{Projects: []string{"D:/q"}},
		ArchivePath: "C:/out/a.tar.gz", ManifestPath: "C:/out/m", SummaryPath: "C:/out/s",
		LogPath: "C:/out/l", Relocation: summary.Relocation{Hints: []summary.Hint{
			{Source: "C:/img/Users/a/.bashrc"}, {Source: registrySource},
			{Source: `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment\X`}}}}
	got := nativePaths(report)
	native := syspath.ToNative
	checks := []struct{ name, got, want string }{
		{"root", got.Root, native("C:/img")},
		{"home", got.Homes[0], native("C:/img/Users/a")},
		{"project", got.Projects[0], native("C:/img/p")},
		{"-p value", got.Options.Projects[0], native("D:/q")},
		{"archive", got.ArchivePath, native("C:/out/a.tar.gz")},
		{"manifest", got.ManifestPath, native("C:/out/m")},
		{"summary", got.SummaryPath, native("C:/out/s")},
		{"log", got.LogPath, native("C:/out/l")},
		{"file hint source", got.Relocation.Hints[0].Source, native("C:/img/Users/a/.bashrc")},
		{"user registry hint source", got.Relocation.Hints[1].Source, registrySource},
		{"system registry hint source", got.Relocation.Hints[2].Source,
			report.Relocation.Hints[2].Source},
	}
	for _, check := range checks {
		t.Run("the "+check.name+" is in native form", func(t *testing.T) {
			if check.got != check.want {
				t.Errorf("%q, want %q", check.got, check.want)
			}
		})
	}
	if report.Homes[0] != "C:/img/Users/a" || report.Relocation.Hints[0].Source != "C:/img/Users/a/.bashrc" {
		t.Error("nativePaths changed the report it was given")
	}
}
