package collect

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
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
		getwd: func() (string, error) { return t.TempDir(), nil }}
}

// fixtureImage builds an image with one home holding one agent file and
// returns its root.
func fixtureImage(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
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

func TestRunCreatesFilesForTheOwnerOnly(t *testing.T) {
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	options := testOptions(fixtureImage(t), outDir)
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	for _, name := range outputNames(t, outDir) {
		info, err := os.Lstat(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s has mode %o, want 600", name, info.Mode().Perm())
		}
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

func TestOutputDirectoryThatCannotBeWrittenIsFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may write any directory")
	}
	outDir := t.TempDir()
	if err := os.Chmod(outDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(outDir, 0o700)
	var stdout, stderr bytes.Buffer
	options := testOptions(fixtureImage(t), outDir)
	code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr)
	want := "Output dir not writable: " + outDir + "\n"
	if code != exitFatal || stderr.String() != want {
		t.Errorf("exit code %d, stderr %q; want 2 and %q", code, stderr.String(), want)
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
// a new image root and returns it, resolved.
func writeImage(t *testing.T, files, links map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(root+"/"+path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root+"/"+path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, target := range links {
		if err := os.Symlink(strings.ReplaceAll(target, "{ROOT}", root), root+"/"+path); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// manifestRow returns the manifest row of path from the run in outDir.
func manifestRow(t *testing.T, outDir, path string) string {
	t.Helper()
	manifests, _ := filepath.Glob(outDir + "/*.manifest.jsonl")
	if len(manifests) != 1 {
		t.Fatalf("manifests %v", manifests)
	}
	text, err := os.ReadFile(manifests[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range strings.Split(string(text), "\n") {
		if strings.Contains(row, `"path":"`+path+`"`) {
			return row
		}
	}
	t.Fatalf("no row of %s in:\n%s", path, text)
	return ""
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
	if err := os.MkdirAll(outside+"/exists/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	root = writeImage(t, map[string]string{"home/u/.claude/settings.json": "{}\n"},
		map[string]string{"home/u/ext": outside + "/exists/", "home/u/gone": "/no-such-host-dir"})
	return root, outside
}

// requireOnce fails unless text holds line exactly once.
func requireOnce(t *testing.T, what, text, line string) {
	t.Helper()
	if strings.Count(text, line) != 1 {
		t.Errorf("%s lacks %q exactly once:\n%s", what, line, text)
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
	log, manifestText := runOutput(t, outDir, ".log"), runOutput(t, outDir, ".manifest.jsonl")
	for _, link := range []string{"ext", "gone"} {
		requireOnce(t, "the log", log, "Project [u] "+root+"/home/u/"+link+"/proj: under the symlinked directory "+
			root+"/home/u/"+link+", which leads out of the image root; not walked\n")
		if strings.Contains(manifestText, root+"/home/u/"+link+"/") {
			t.Errorf("the manifest holds a row under %s:\n%s", link, manifestText)
		}
	}
	requireOnce(t, "the log", log, "Project "+outside+"/exists/proj: is outside the image root; not walked\n")
	if strings.Contains(manifestText, outside) {
		t.Errorf("a row for a project out of the root:\n%s", manifestText)
	}
	// A -p value missing inside the image keeps the sh collector's warning.
	requireOnce(t, "the log", log, "WARNING: project given with -p is not a directory: "+root+"/home/u/missing: ")
	if row := manifestRow(t, outDir, root+"/home/u/missing"); !strings.Contains(row, `"status":"error_read"`) {
		t.Errorf("row %s, want error_read", row)
	}
	requireOnce(t, "collection.json", runOutput(t, outDir, ".collection.json"), "\n  \"projects\": [],\n")
	requireOnce(t, "stdout", stdout.String(), "\nprojects:   0\n")
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
