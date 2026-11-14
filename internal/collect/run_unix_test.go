//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests need POSIX permission bits, or expect paths spelled with slashes
// only, which Windows does not.
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
