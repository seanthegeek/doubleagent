//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

// These tests need POSIX permission bits.
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

func TestUnixHostDescription(t *testing.T) {
	if got := identityText(summary.Identity{UID: 501, RunAs: "ignored"}); got != "uid=501" {
		t.Errorf("identityText = %q, want uid=501", got)
	}
	if !privileged(summary.Identity{UID: 0}) || privileged(summary.Identity{UID: 501, Admin: true}) {
		t.Error("privileged must follow the user ID alone")
	}
	want := volumes.Bases{System: "/img", ProgramData: "/img/ProgramData"}
	if got := systemBases("/img"); got != want {
		t.Errorf("systemBases = %+v, want %+v", got, want)
	}
	if live := (volumes.Bases{ProgramData: "/ProgramData"}); systemBases("") != live {
		t.Errorf("systemBases of a live run = %+v, want %+v", systemBases(""), live)
	}
	if !liveRoot("/") || liveRoot("/img") || registryReader() != nil {
		t.Error("only / is live and Unix has no registry")
	}
}

// longTempDir returns a new temporary directory as the test framework names
// it, a symlinked parent included, which the resolution of -r goes through.
func longTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// These compare runs whose link targets differ in kind and permission bits,
// which only Unix sets on a symlink's target without changing the link.
// outsideTargets are what a link out of the image root may point at, each
// set up at target in turn: a directory with an agent file in it, a file, a
// directory nobody may read, and nothing.
func outsideTargets(t *testing.T, target string) map[string]func() {
	t.Helper()
	reset := func() {
		_ = os.Chmod(target, 0o700)
		if err := os.RemoveAll(target); err != nil {
			t.Fatal(err)
		}
	}
	directory := func() {
		reset()
		writeTree(t, target, "_data/x", ".claude/settings.json", "intruder/.claude/settings.json")
	}
	return map[string]func(){
		"a directory": directory,
		"a file": func() {
			reset()
			if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"an unreadable directory": func() {
			directory()
			if err := os.Chmod(target, 0); err != nil {
				t.Fatal(err)
			}
		},
		"nothing": reset,
	}
}

// writeTree writes each file, empty, under dir.
func writeTree(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(dir+"/"+file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/"+file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// comparableOutput runs a collection of root and returns its stdout, log and
// manifest with what differs between two runs of one image removed: the
// output directory, the archive's size and hash, and access times.
func comparableOutput(t *testing.T, root string) string {
	t.Helper()
	outDir := t.TempDir()
	return comparableRun(t, testOptions(root, outDir), testHost(t))
}

// comparableRun runs a collection with options on host and returns its
// stdout, log, manifest and collection.json as comparableOutput does.
func comparableRun(t *testing.T, options Options, host environment) string {
	t.Helper()
	outDir := options.OutDir
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, host, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	text := stdout.String() + runOutput(t, outDir, ".log") + runOutput(t, outDir, ".manifest.jsonl") +
		runOutput(t, outDir, ".collection.json")
	text = strings.ReplaceAll(text, outDir, "{OUT}")
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "sha256:") || strings.HasPrefix(line, "size:") ||
			strings.Contains(line, " done: ") {
			continue
		}
		kept = append(kept, atimeField.ReplaceAllString(line, `"atime":0`))
	}
	return strings.Join(kept, "\n")
}

// atimeField matches a manifest row's access time, which reading a link may
// move.
var atimeField = regexp.MustCompile(`"atime":[0-9]+`)

func TestOutputsDoNotDependOnWhatALinkOutOfTheRootPointsAt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable directory")
	}
	cases := []struct {
		name, link string
		files      map[string]string
		wantLine   string
	}{
		{"a home that is a link out of the root", "home/alice",
			map[string]string{"Users/bob/notes": "", "home/.keep": ""},
			"user homes: {LINK} leads out of the image root; nothing behind it was listed or taken" +
				" as a home"},
		{"a volume's _data that is a link out of the root", "var/lib/docker/volumes/v1/_data",
			map[string]string{"home/bob/notes": "", "var/lib/docker/volumes/v1/.keep": ""},
			"  volume v1 has no _data directory, skipped"},
		{"an excluded directory that is a link out of the root", "home/bob/.claude/cache",
			map[string]string{"home/bob/.claude/settings.json": "{}"},
			`/home/bob/.claude/cache","archive_path":"","type":"file",`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			target := syspath.FromNative(t.TempDir()) + "/target"
			root := writeImage(t, testCase.files, map[string]string{testCase.link: target})
			var first, firstName string
			for name, setUp := range outsideTargets(t, target) {
				setUp()
				output := comparableOutput(t, root)
				line := strings.ReplaceAll(testCase.wantLine, "{LINK}",
					syspath.ToNative(root+"/"+testCase.link))
				if !strings.Contains(output, line) {
					t.Errorf("with %s behind the link the output lacks %q:\n%s", name, line, output)
				}
				if first == "" {
					first, firstName = output, name
				} else if output != first {
					t.Errorf("the output with %s behind the link differs from that with %s:\n%s\n"+
						"versus\n%s", name, firstName, output, first)
				}
			}
			_ = os.Chmod(target, 0o700)
		})
	}
}

func TestARelativeProjectThroughALinkOutOfTheRootIsNotTested(t *testing.T) {
	work := syspath.FromNative(t.TempDir())
	writeTree(t, work, "img/home/alice/notes")
	if err := os.Symlink(work+"/outside", work+"/img/home/alice/out"); err != nil {
		t.Fatal(err)
	}
	host := testHost(t)
	host.getwd = func() (string, error) { return work, nil }
	var outputs []string
	for _, present := range []bool{true, false} {
		if present {
			writeTree(t, work, "outside/AGENTS.md")
		} else if err := os.RemoveAll(work + "/outside"); err != nil {
			t.Fatal(err)
		}
		options := testOptions("img", t.TempDir())
		options.Projects = []string{"img//home/alice/out/"}
		outputs = append(outputs, comparableRun(t, options, host))
	}
	if outputs[0] != outputs[1] {
		t.Errorf("the run depends on what the link points at:\n%s\nversus\n%s", outputs[0], outputs[1])
	}
}
