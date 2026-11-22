package collect

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"runtime"
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
func (noPrograms) Output(_ context.Context, name string, _ ...string) ([]byte, error) {
	return nil, errors.New(name + ": not found")
}

// Available reports that no program exists.
func (noPrograms) Available(string) bool { return false }

// testHost is a host with a pinned clock and no programs.
func testHost(t *testing.T) environment {
	pinned := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return environment{now: func() time.Time { return pinned }, runner: noPrograms{},
		getwd:     func() (string, error) { return syspath.FromNative(t.TempDir()), nil },
		findUsers: users.Find, lastAccess: func() accessUpdates { return accessUnknown },
		confine: follow.NewConfinement}
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
			if got := enclosingHome(homes, testCase.path, nil); got != testCase.want {
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
	// A -p value missing inside the image gets a warning.
	requireOnce(t, "the log", log, "WARNING: project given with -p is not a directory: "+
		native(root+"/home/u/missing")+": ")
	if row := manifestRow(t, outDir, root+"/home/u/missing"); !strings.Contains(row, `"status":"error_read"`) {
		t.Errorf("row %s, want error_read", row)
	}
	requireOnce(t, "collection.json", runOutput(t, outDir, ".collection.json"), "\n  \"projects\": [],\n")
	requireOnce(t, "stdout", stdout.String(), "\nprojects:   0\n")
}

func TestDiscoveryReadsNoSourceThroughALinkOutOfTheRoot(t *testing.T) {
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside = syspath.FromNative(outside)
	if err := os.WriteFile(outside+"/history.jsonl", []byte(`{"project":"/p/proj"}`+"\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	root := writeImage(t, map[string]string{"p/proj/CLAUDE.md": "# proj\n", "home/alice/notes": ""},
		map[string]string{"home/alice/.claude": outside})
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	options := testOptions(root, outDir)
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	native := syspath.ToNative
	requireOnce(t, "the log", runOutput(t, outDir, ".log"), "Discovery [alice] "+
		native(root+"/home/alice/.claude")+
		": the symlink leads out of the image root; nothing behind it was listed or read\n")
	if rowUnder(t, outDir, root+"/p/") {
		t.Error("the manifest holds a row from the project the outside source names")
	}
	requireOnce(t, "stdout", stdout.String(), "\nprojects:   0\n")
}

// outsideTree writes files (path to content) under a new directory outside
// any image and returns it, resolved and canonical.
func outsideTree(t *testing.T, files map[string]string) string {
	t.Helper()
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside = syspath.FromNative(outside)
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(outside+"/"+path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outside+"/"+path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return outside
}

func TestNoNameFromBehindALinkOutOfTheRootReachesTheLog(t *testing.T) {
	copilot := outsideTree(t, map[string]string{
		"session-state/ANALYST-SESSION-1/events.jsonl":   "{}\n",
		"session-state/ANALYST-SESSION-1/workspace.yaml": "cwd: /p/one\n",
		"session-state/ANALYST-SESSION-2/events.jsonl":   "{}\n",
	})
	config := outsideTree(t, map[string]string{
		"Code/User/workspaceStorage/w/workspace.json": `{"folder": "file:///p/two"}`,
		"SecretApp/User/settings.json":                "{}",
	})
	root := writeImage(t, map[string]string{"home/alice/notes": "", "p/one/AGENTS.md": "# 1\n",
		"p/two/AGENTS.md": "# 2\n"},
		map[string]string{"home/alice/.copilot": copilot, "home/alice/.config": config})
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	for _, name := range []string{"ANALYST-SESSION", "SecretApp", "workspaceStorage", "/p/"} {
		if strings.Contains(log, name) {
			t.Errorf("the log holds %q, a name from behind a link out of the root:\n%s", name, log)
		}
	}
	for _, link := range []string{".copilot", ".config"} {
		requireOnce(t, "the log", log, "Discovery [alice] "+syspath.ToNative(root+"/home/alice/"+link)+
			": the symlink leads out of the image root; nothing behind it was listed or read\n")
	}
	if strings.Count(log, "Discovery [") != 2 {
		t.Errorf("the log holds other discovery lines than one per link:\n%s", log)
	}
	requireOnce(t, "stdout", stdout.String(), "\nprojects:   0\n")
}

func TestNoCallerListsBehindALinkOutOfTheRoot(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		link      string
		outside   map[string]string
		wantLines []string
		forbidden []string
	}{
		{"a home parent that leads out names no home",
			map[string]string{"Users/alice/notes": ""}, "home",
			map[string]string{"intruder/.claude/settings.json": "{}"},
			[]string{"user homes: {LINK} leads out of the image root; nothing behind it was" +
				" listed or taken as a home"},
			[]string{"intruder"}},
		{"a volume directory under a linked parent is not listed",
			map[string]string{"home/alice/notes": ""}, "var",
			map[string]string{"lib/docker/volumes/analystvol/_data/x": "x"},
			[]string{"docker: {LINK} leads out of the image root; nothing behind it was listed"},
			[]string{"analystvol", "Docker volumes in"}},
		{"a startup-file glob under a linked parent is not listed",
			map[string]string{"home/alice/notes": ""}, "home/alice/.bashrc.d",
			map[string]string{"analyst.sh": "export CODEX_HOME=/x\n"},
			[]string{"startup files: {LINK} leads out of the image root; nothing behind it was" +
				" listed"},
			[]string{"analyst.sh", "CODEX_HOME"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outside := outsideTree(t, testCase.outside)
			root := writeImage(t, testCase.files, map[string]string{testCase.link: outside})
			outDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), testOptions(root, outDir), testHost(t),
				&stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			log, rows := runOutput(t, outDir, ".log"), runOutput(t, outDir, ".manifest.jsonl")
			summary := runOutput(t, outDir, ".collection.json")
			link := syspath.ToNative(root + "/" + testCase.link)
			for _, line := range testCase.wantLines {
				requireOnce(t, "the log", log, strings.ReplaceAll(line, "{LINK}", link)+"\n")
			}
			for _, text := range testCase.forbidden {
				for what, output := range map[string]string{"log": log, "manifest": rows,
					"collection.json": summary, "stdout": stdout.String()} {
					if strings.Contains(output, text) {
						t.Errorf("the %s holds %q, from behind the link:\n%s", what, text, output)
					}
				}
			}
		})
	}
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
	host.findUsers = func(context.Context, bool, string, string, users.Runner,
		glob.Check, *fsmeta.Opener, func(string) string) users.Report {
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

func TestUnmatchedUserNamesAreLogged(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		found  []string
		want   []string
	}{
		{"each unmatched name gets a line listing the users found", "nobody,Eve", []string{"alice", "bob"},
			[]string{"WARNING: -u nobody matches no user home; homes found for: alice, bob\n",
				"WARNING: -u Eve matches no user home; homes found for: alice, bob\n"}},
		{"no user found reads none", "nobody,Eve", nil,
			[]string{"WARNING: -u nobody matches no user home; homes found for: none\n",
				"WARNING: -u Eve matches no user home; homes found for: none\n"}},
		{"a list naming nobody gets one line", ",", nil, []string{"WARNING: -u lists no names\n"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			host := testHost(t)
			host.findUsers = func(context.Context, bool, string, string, users.Runner,
				glob.Check, *fsmeta.Opener, func(string) string) users.Report {
				return users.Report{Found: test.found, Unmatched: []string{"nobody", "Eve"}}
			}
			outDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			options := testOptions(syspath.FromNative(fixtureImage(t)), outDir)
			options.UsersFilter = test.filter
			if code := runCollection(context.Background(), options, host, &stdout, &stderr); code != exitOK {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			log := runOutput(t, outDir, ".log")
			position := strings.Index(log, "WARNING: no user home directories found\n")
			if position < 0 {
				t.Fatalf("the log lacks the no-home warning:\n%s", log)
			}
			for _, line := range test.want {
				found := strings.Index(log[position:], line)
				if found < 0 {
					t.Fatalf("the log lacks %q in order:\n%s", line, log)
				}
				position += found + len(line)
			}
		})
	}
}

func TestTheRunsOpenerCarriesTheDecision(t *testing.T) {
	cases := []struct {
		name         string
		state        accessUpdates
		set          bool
		wantReadOnly bool
		wantOff      bool
	}{
		{"updates on, no option", accessOn, false, false, false},
		{"updates unknown, no option", accessUnknown, false, false, false},
		{"updates off, no option", accessOff, false, false, true},
		{"updates on, --no-preserve-atime", accessOn, true, true, false},
		{"updates off, --no-preserve-atime", accessOff, true, true, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			host := testHost(t)
			host.lastAccess = func() accessUpdates { return test.state }
			state, err := prepareRun(Options{OutDir: t.TempDir(), NoPreserveAtime: test.set}, host,
				io.Discard, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			opener := state.newCollector("").Stager.Opener
			if opener != state.opener || opener.ReadOnly != test.wantReadOnly ||
				opener.NTFSUpdatesOff != test.wantOff {
				t.Errorf("opener %+v (the run's: %v), want ReadOnly %v, NTFSUpdatesOff %v",
					opener, opener == state.opener, test.wantReadOnly, test.wantOff)
			}
		})
	}
}

func TestAccessUpdatesOf(t *testing.T) {
	cases := []struct {
		name    string
		value   uint64
		isDWORD bool
		want    accessUpdates
	}{
		{"0 means updates are on", 0, true, accessOn},
		{"1 means updates are off", 1, true, accessOff},
		{"user managed, on", 0x80000000, true, accessOn},
		{"user managed, off", 0x80000001, true, accessOff},
		{"system managed, on", 0x80000002, true, accessOn},
		{"system managed, off", 0x80000003, true, accessOff},
		{"another number is unknown", 2, true, accessUnknown},
		{"a high bit beyond the known ones is unknown", 0x80000004, true, accessUnknown},
		{"a 64-bit value is unknown", 1, false, accessUnknown},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := accessUpdatesOf(test.value, test.isDWORD); got != test.want {
				t.Errorf("accessUpdatesOf(%#x, %v) = %v, want %v", test.value, test.isDWORD, got, test.want)
			}
		})
	}
}

func TestAccessDecisionLine(t *testing.T) {
	cases := []struct {
		name  string
		state accessUpdates
		set   bool
		want  string
	}{
		{"updates off", accessOff, false, "last-access updates are off for NTFS on this system; " +
			"files on NTFS are opened read-only, others with the right to hold their access time"},
		{"updates on", accessOn, false,
			"last-access updates are on; files are opened with the right to hold their access time"},
		{"updates unknown", accessUnknown, false, "last-access updates could not be determined; " +
			"files are opened with the right to hold their access time"},
		{"the option wins over the setting", accessOn, true,
			"--no-preserve-atime: files are opened read-only; reading may change their access time on disk"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := accessDecisionLine(test.state, test.set); got != test.want {
				t.Errorf("line %q, want %q", got, test.want)
			}
		})
	}
}

// archiveMembers returns the names of the regular files in the gzipped tar
// archive of the run in outDir, each with the SHA-256 of its contents.
func archiveMembers(t *testing.T, outDir string) map[string]string {
	t.Helper()
	compressed, err := gzip.NewReader(strings.NewReader(runOutput(t, outDir, ".tar.gz")))
	if err != nil {
		t.Fatal(err)
	}
	members := map[string]string{}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return members
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			hash := sha256.New()
			if _, err := io.Copy(hash, reader); err != nil {
				t.Fatal(err)
			}
			members[header.Name] = hex.EncodeToString(hash.Sum(nil))
		}
	}
}

// requireCollectedRow checks that the run in outDir collected path with the
// SHA-256 of content and recorded wantAtime as its access time.
func requireCollectedRow(t *testing.T, outDir, path, content string, wantAtime int64) {
	t.Helper()
	var row struct {
		Status string `json:"status"`
		SHA256 string `json:"sha256"`
		Atime  int64  `json:"atime"`
	}
	if err := json.Unmarshal([]byte(manifestRow(t, outDir, path)), &row); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	if row.Status != "collected" || row.SHA256 != hex.EncodeToString(sum[:]) || row.Atime != wantAtime {
		t.Errorf("row of %s: status %q, sha256 %s, atime %d; want collected, %x, %d",
			path, row.Status, row.SHA256, row.Atime, sum, wantAtime)
	}
}

func TestNoPreserveAtimeRun(t *testing.T) {
	cases := []struct {
		name      string
		set       bool
		wantLog   string
		wantField bool
	}{
		{"a default run records the option as off", false, " no_preserve_atime=0", false},
		{"a run with --no-preserve-atime records it as on", true, " no_preserve_atime=1", true},
	}
	files := map[string]string{
		"home/alice/.claude/settings.json": "{\"theme\": \"dark\"}\n",
		"home/alice/.claude.json":          "{\"projects\": {}}\n",
		// Read by the startup-file search before its row is built.
		"home/alice/.bashrc": "export CLAUDE_CONFIG_DIR=/home/alice/cc\n",
	}
	// An access time well before the write time, which a volume that
	// records reads would move on the copy's read.
	accessed, written := time.Unix(1_600_000_000, 0), time.Unix(1_700_000_000, 0)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeImage(t, files, nil)
			for path := range files {
				mustChtimes(t, root+"/"+path, accessed, written)
			}
			outDir := t.TempDir()
			options := testOptions(root, outDir)
			options.NoPreserveAtime = test.set
			var stdout, stderr bytes.Buffer
			if code := runCollection(context.Background(), options, testHost(t), &stdout,
				&stderr); code != exitOK {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			members := archiveMembers(t, outDir)
			for path, content := range files {
				requireCollectedRow(t, outDir, root+"/"+path, content, accessed.Unix())
				sum := sha256.Sum256([]byte(content))
				if got := members["./fs/"+path]; got != hex.EncodeToString(sum[:]) {
					t.Errorf("archive member ./fs/%s has sha256 %q, want %x", path, got, sum)
				}
			}
			requireRecorded(t, outDir, test.wantLog, test.wantField)
			requireDecisionLine(t, outDir, test.set)
		})
	}
}

// mustChtimes sets the access and write times of path.
func mustChtimes(t *testing.T, path string, accessed, written time.Time) {
	t.Helper()
	if err := os.Chtimes(path, accessed, written); err != nil {
		t.Fatal(err)
	}
}

// requireRecorded checks that the run in outDir ends its log's second start
// line with wantLog and records wantField as options.no_preserve_atime.
func requireRecorded(t *testing.T, outDir, wantLog string, wantField bool) {
	t.Helper()
	lines := strings.SplitN(runOutput(t, outDir, ".log"), "\n", 3)
	if len(lines) < 2 || !strings.HasSuffix(lines[1], wantLog) {
		t.Errorf("log start lines %q, want the second to end with %q", lines, wantLog)
	}
	var document struct {
		Options map[string]any `json:"options"`
	}
	if err := json.Unmarshal([]byte(runOutput(t, outDir, ".collection.json")), &document); err != nil {
		t.Fatal(err)
	}
	if got, ok := document.Options["no_preserve_atime"].(bool); !ok || got != wantField {
		t.Errorf("options.no_preserve_atime %v, want %v", document.Options["no_preserve_atime"], wantField)
	}
}

// requireDecisionLine checks that the run in outDir logged how files are
// opened, for the unknown state testHost gives, where the platform logs it
// (Windows), right after the two start lines, and logged no such line where
// it does not.
func requireDecisionLine(t *testing.T, outDir string, set bool) {
	t.Helper()
	lines := strings.Split(runOutput(t, outDir, ".log"), "\n")
	third := ""
	if len(lines) > 2 {
		third = lines[2]
	}
	want := accessDecisionLine(accessUnknown, set)
	if logsAccessDecision && !strings.HasSuffix(third, " "+want) {
		t.Errorf("third log line %q, want it to end with %q", third, want)
	}
	log := strings.Join(lines, "\n")
	if !logsAccessDecision && (strings.Contains(log, "last-access updates") ||
		strings.Contains(log, "--no-preserve-atime:")) {
		t.Errorf("log names an open decision this platform does not make:\n%s", log)
	}
}

func TestReadsWithoutTheHoldAreLoggedOncePerReasonAndCountedOnAnyExit(t *testing.T) {
	outDir := t.TempDir()
	state, err := prepareRun(testOptions(fixtureImage(t), outDir), testHost(t), io.Discard,
		io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer state.cleanup()
	if err := state.openOutputs(); err != nil {
		t.Fatal(err)
	}
	first, second := syspath.FromNative(outDir)+"/a", syspath.FromNative(outDir)+"/b"
	state.opener.Report(first, "hold not honoured, access time restored")
	state.opener.Report(first, "hold not honoured, access time restored")
	state.opener.Report(first, "hold not honoured, restore failed")
	state.opener.Report(second, "right to hold refused")
	// A cancelled run stops before its first step and still writes the count.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := state.execute(ctx); err == nil {
		t.Fatal("a cancelled run reported no error")
	}
	state.log.Close()
	log := runOutput(t, outDir, ".log")
	want := map[string]int{
		" access time not held: " + syspath.ToNative(first) + ": hold not honoured, access time restored\n": 1,
		" access time not held: " + syspath.ToNative(first) + ": hold not honoured, restore failed\n":       1,
		" access time not held: " + syspath.ToNative(second) + ": right to hold refused\n":                  1,
		" access times not held: 2 file(s)\n":                                                               1,
	}
	for line, count := range want {
		if got := strings.Count(log, line); got != count {
			t.Errorf("%q appears %d time(s), want %d:\n%s", line, got, count, log)
		}
	}
}

func TestARootWhosePlaceCannotBeReadRunsOnTheRootCheckAlone(t *testing.T) {
	files := map[string]string{"home/alice/.claude/settings.json": "{}\n"}
	if runtime.GOOS == "windows" {
		files = map[string]string{"Users/alice/.claude/settings.json": "{}\n"}
	}
	root := writeImage(t, files, nil)
	outDir := syspath.FromNative(t.TempDir())
	host := testHost(t)
	host.confine = func(string, func(string)) (*follow.Confinement, error) {
		return nil, errors.New("access denied")
	}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), host, &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	line := "image root: where " + syspath.ToNative(root) + " lies cannot be read (access denied);" +
		" what is opened under it is checked by the root check alone\n"
	requireOnce(t, "the log", runOutput(t, outDir, ".log"), line)
	for path := range files {
		if !strings.Contains(manifestRow(t, outDir, root+"/"+path), `"status":"collected"`) {
			t.Errorf("%s was not collected", path)
		}
	}
}

// requireRowsNameMembers fails unless every row of the run in outDir that
// has an archive path names an archive member of exactly that name: a
// hashed row one holding exactly those bytes, a link row a symlink member
// with the row's target.
func requireRowsNameMembers(t *testing.T, outDir string) {
	t.Helper()
	members, links := archiveMembers(t, outDir), archiveLinks(t, outDir)
	rows, lines := manifestRows(t, outDir)
	for i, row := range rows {
		archived, _ := row["archive_path"].(string)
		if archived == "" {
			continue
		}
		if row["status"] == "symlink" {
			if target, found := links["./"+archived]; !found || target != row["target"] {
				t.Errorf("row %s names link member %s, which the archive holds as %q, %v",
					lines[i], archived, target, found)
			}
			continue
		}
		if hash, _ := row["sha256"].(string); members["./"+archived] != hash {
			t.Errorf("row %s names member %s, which the archive holds as %q", lines[i], archived,
				members["./"+archived])
		}
	}
}

// archiveLinks returns the symlink members of the archive in outDir, each
// with its target.
func archiveLinks(t *testing.T, outDir string) map[string]string {
	t.Helper()
	compressed, err := gzip.NewReader(strings.NewReader(runOutput(t, outDir, ".tar.gz")))
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]string{}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return links
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeSymlink {
			links[header.Name] = header.Linkname
		}
	}
}
