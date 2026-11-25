//go:build unix

package collect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"crypto/sha256"
	"encoding/hex"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

// These tests need POSIX permission bits.
func TestOutputDirectoryThatCannotBeWrittenIsFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may write any directory")
	}
	image := fixtureImage(t)
	for _, mode := range []os.FileMode{0o500, 0o600} {
		t.Run(fmt.Sprintf("an output directory of mode %o", mode), func(t *testing.T) {
			outDir := t.TempDir()
			if err := os.Chmod(outDir, mode); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(outDir, 0o700)
			var stdout, stderr bytes.Buffer
			options := testOptions(image, outDir)
			code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr)
			want := "Cannot create staging dir: mkdir " + outDir + "/.stage-"
			if code != exitFatal || !strings.HasPrefix(stderr.String(), want) ||
				!strings.HasSuffix(stderr.String(), ": permission denied\n") {
				t.Errorf("exit code %d, stderr %q; want 2 and %q...: permission denied",
					code, stderr.String(), want)
			}
			if err := os.Chmod(outDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if names := outputNames(t, outDir); len(names) != 0 {
				t.Errorf("the run left %v", names)
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
			`/home/bob/.claude/cache","archive_path":"","type":"symlink",`},
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

func TestARelativeProjectOutOfTheRootIsNamedWhereItIs(t *testing.T) {
	work := syspath.FromNative(t.TempDir())
	writeTree(t, work, "img/srv/.keep", "img/home/alice/notes", "outside/proj/AGENTS.md")
	host := testHost(t)
	host.getwd = func() (string, error) { return work + "/img/srv", nil }
	outDir := t.TempDir()
	options := testOptions(work+"/img", outDir)
	options.Projects = []string{"../../outside/proj", "../../outside/proj/"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, host, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	line := "Project " + work + "/outside/proj: is outside the image root; not walked\n"
	if strings.Count(log, "Project ") != 1 || !strings.Contains(log, line) {
		t.Errorf("the log lacks the one line %q:\n%s", line, log)
	}
}

func TestAMissingRelativeProjectThatClimbsIsNotADirectory(t *testing.T) {
	work := syspath.FromNative(t.TempDir())
	writeTree(t, work, "img/srv/.keep", "img/home/alice/notes")
	host := testHost(t)
	host.getwd = func() (string, error) { return work + "/img/srv", nil }
	outDir := t.TempDir()
	options := testOptions(work+"/img", outDir)
	options.Projects = []string{"../../img/missing"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, host, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	line := "WARNING: project given with -p is not a directory: " + work + "/img/missing: "
	if !strings.Contains(log, line) || strings.Contains(log, "cannot be verified") {
		t.Errorf("the log lacks %q, or says the path cannot be verified:\n%s", line, log)
	}
}

// refusalCase is an image whose collection must log refusals worded by
// their kind: wantLines hold {ROOT} for the image root.
type refusalCase struct {
	name      string
	files     map[string]string
	links     map[string]string
	locked    bool
	wantLines []string
}

// refusalCases are the kinds of refusal each log site can produce on Unix.
func refusalCases() []refusalCase {
	const unverified = " cannot be verified to lie inside the image root; " +
		"nothing behind it was looked at"
	return []refusalCase{
		{"a root the check cannot resolve is unverified for users, docker and startup files",
			map[string]string{"home/alice/notes": ""}, nil, true, []string{
				"user homes: {ROOT}" + unverified, "docker: {ROOT}" + unverified,
				"startup files: {ROOT}" + unverified}},
		{"a link loop is unverified for discovery and the catalog walk",
			map[string]string{"home/alice/notes": ""},
			map[string]string{"home/alice/.config": "loop2", "home/alice/loop2": ".config"}, false,
			[]string{"Discovery [alice] {ROOT}/home/alice/.config" + unverified,
				"  [shared] {ROOT}/home/alice/.config" + unverified + ", not walked"}},
		{"a passwd home whose own .. climbs above the root names the path",
			map[string]string{"home/alice/notes": "", "etc/passwd": "eve:x:1:1::/../../tmp:/bin/sh\n"},
			nil, false, []string{"user homes: {ROOT}/../../tmp climbs above the image root with ..; " +
				"nothing behind it was looked at"}},
		{"a link whose target climbs above the root leads out",
			map[string]string{"home/alice/notes": ""},
			map[string]string{"home/alice/.codex": "../../../../../../../../.."}, false,
			[]string{"Discovery [alice] {ROOT}/home/alice/.codex: the symlink leads out of the " +
				"image root; nothing behind it was listed or read"}},
	}
}

func TestEachRefusalIsWordedByItsKind(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may search any directory")
	}
	for _, testCase := range refusalCases() {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeImage(t, testCase.files, testCase.links)
			if testCase.locked {
				if err := os.Chmod(root, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
			}
			outDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t),
				&stdout, &stderr); code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			log := runOutput(t, outDir, ".log")
			for _, line := range testCase.wantLines {
				if want := strings.ReplaceAll(line, "{ROOT}", root) + "\n"; !strings.Contains(log, want) {
					t.Errorf("the log lacks %q:\n%s", want, log)
				}
			}
		})
	}
}

// TestADottedHomeIsStagedWhereTheKernelReachesIt is the image of a home
// given through a link and `..`: frank's passwd home /srv/dotlink/../frank,
// with srv/dotlink a link to b/c, is srv/b/frank; gina's home is srv/frank.
// Kept dotted, frank's copy would land on gina's staged path and his row
// would point at her bytes.
func TestADottedHomeIsStagedWhereTheKernelReachesIt(t *testing.T) {
	files := map[string]string{
		"etc/passwd": "frank:x:1001:1001::/srv/dotlink/../frank:/bin/sh\n" +
			"gina:x:1002:1002::/srv/frank:/bin/sh\n" +
			"frank2:x:1003:1003::/srv/b/frank:/bin/sh\n",
		"srv/b/c/keep":                      "",
		"srv/b/frank/.claude/settings.json": "{\"who\": \"frank\"}\n",
		"srv/frank/.claude/settings.json":   "{\"who\": \"gina\"}\n",
	}
	root := writeImage(t, files, map[string]string{"srv/dotlink": "b/c"})
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	members := archiveMembers(t, outDir)
	rows, texts := manifestRows(t, outDir)
	for _, test := range []struct{ user, path, content string }{
		{"frank", "srv/b/frank/.claude/settings.json", files["srv/b/frank/.claude/settings.json"]},
		{"gina", "srv/frank/.claude/settings.json", files["srv/frank/.claude/settings.json"]},
	} {
		sum := sha256.Sum256([]byte(test.content))
		want := hex.EncodeToString(sum[:])
		if got := members["./fs/"+test.path]; got != want {
			t.Errorf("archive member ./fs/%s has sha256 %q, want %s", test.path, got, want)
		}
		found := 0
		for i, row := range rows {
			if row["path"] != root+"/"+test.path {
				continue
			}
			found++
			if row["user"] != test.user || row["archive_path"] != "fs/"+test.path ||
				row["sha256"] != want {
				t.Errorf("row %s", texts[i])
			}
		}
		if found != 1 {
			t.Errorf("%d rows of %s, want 1:\n%s", found, test.path, strings.Join(texts, "\n"))
		}
	}
	for i, row := range rows {
		for _, field := range []string{"path", "home", "archive_path"} {
			if value, _ := row[field].(string); strings.Contains(value, "..") {
				t.Errorf("row %s holds .. in %s", texts[i], field)
			}
		}
	}
	if log := runOutput(t, outDir, ".log"); strings.Contains(log, "dotlink/..") {
		t.Errorf("the log names the dotted spelling:\n%s", log)
	}
}

// TestAPathReachingTheRootIsNamedByTheRootAsGiven gives the image root
// through a link: eve's passwd home /home/.. reaches the root itself, which
// is skipped as a home, as every home of / is; a -p that
// reaches the root is named by the root as given. No row, log line or
// collection.json value carries the analyst's own path for the root.
func TestAPathReachingTheRootIsNamedByTheRootAsGiven(t *testing.T) {
	root := writeImage(t, map[string]string{
		"etc/passwd":                     "eve:x:1001:1001::/home/..:/bin/sh\n",
		"home/bob/.claude/settings.json": "{}\n",
		"AGENTS.md":                      "# image root\n",
	}, nil)
	via := t.TempDir() + "/via"
	if err := os.Symlink(root, via); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	options := testOptions(via, outDir)
	options.Projects = []string{via + "/home/bob/../.."}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	if strings.Contains(log, "User eve") {
		t.Errorf("the image root was walked as eve's home:\n%s", log)
	}
	requireOnce(t, "the log", log, "Project "+via+"\n")
	for _, suffix := range []string{".log", ".manifest.jsonl", ".collection.json"} {
		if text := runOutput(t, outDir, suffix); strings.Contains(text, root) {
			t.Errorf("%s names the root by the analyst's path %s:\n%s", suffix, root, text)
		}
	}
}

// TestTwoProjectsReachingOnePathAreOne gives -p twice for one path, once
// with a `.`: the run lists it once, warns once and records it once.
func TestTwoProjectsReachingOnePathAreOne(t *testing.T) {
	root := writeImage(t, map[string]string{"srv/file.txt": "x\n"}, nil)
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.Projects = []string{root + "/srv/file.txt", root + "/srv/./file.txt"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	if got := strings.Count(log, "WARNING: project given with -p is not a directory"); got != 1 {
		t.Errorf("%d warnings for one path, want 1:\n%s", got, log)
	}
	if summary := runOutput(t, outDir, ".collection.json"); strings.Count(summary,
		root+"/srv/file.txt") != 1 || strings.Contains(summary, "/./") {
		t.Errorf("collection.json lists the path other than once:\n%s", summary)
	}
}

// TestAHomeReachingAnotherUsersHomeIsLogged has frank's passwd home reach,
// through a link and `..`, the home frank2 also names: the home is walked
// once, as frank's, the user listed first, and one line names both.
func TestAHomeReachingAnotherUsersHomeIsLogged(t *testing.T) {
	root := writeImage(t, map[string]string{
		"etc/passwd": "frank:x:1001:1001::/srv/dotlink/../frank:/bin/sh\n" +
			"frank2:x:1003:1003::/srv/b/frank:/bin/sh\n",
		"srv/b/c/keep":                      "",
		"srv/b/frank/.claude/settings.json": "{}\n",
	}, map[string]string{"srv/dotlink": "b/c"})
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	requireOnce(t, "the log", runOutput(t, outDir, ".log"), "user homes: "+root+
		"/srv/b/frank is the home of frank and of frank2; walked once, as frank's\n")
}

// TestADottedSystemHomeIsWarnedAboutAsAPlainOne is the table of -u against
// a passwd home that reaches, through `..`, the image root itself or /usr,
// with and without another user's home: the run warns as it does for a home
// of / (the system home is no home, so -u names nobody with one).
func TestADottedSystemHomeIsWarnedAboutAsAPlainOne(t *testing.T) {
	for _, test := range []struct {
		name, home string
		others     bool
	}{
		{"reaching the root", "/srv/x/../..", false},
		{"reaching the root, other homes", "/srv/x/../..", true},
		{"reaching /usr", "/srv/x/../../usr", false},
		{"reaching /usr, other homes", "/srv/x/../../usr", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{
				"etc/passwd": "dot1:x:1001:1001::" + test.home + ":/bin/sh\n",
				"srv/x/keep": "", "usr/keep": "",
			}
			found := "none"
			if test.others {
				files["home/alice/.claude/settings.json"] = "{}\n"
				found = "alice"
			}
			root := writeImage(t, files, nil)
			outDir := t.TempDir()
			options := testOptions(root, outDir)
			options.UsersFilter = "dot1"
			var stdout, stderr bytes.Buffer
			if code := runCollection(context.Background(), options, testHost(t), &stdout,
				&stderr); code != exitOK {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			log := runOutput(t, outDir, ".log")
			requireOnce(t, "the log", log,
				"WARNING: -u dot1 matches no user home; homes found for: "+found+"\n")
			if strings.Contains(log, "User dot1") {
				t.Errorf("the system home was walked as dot1's:\n%s", log)
			}
		})
	}
}

// TestAProjectGivenAsTheRootWithASlashIsInsideIt gives -p as the root with a
// trailing slash: it is the root, not outside it.
func TestAProjectGivenAsTheRootWithASlashIsInsideIt(t *testing.T) {
	root := writeImage(t, map[string]string{"AGENTS.md": "# root\n"}, nil)
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.Projects = []string{root + "/"}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	if log := runOutput(t, outDir, ".log"); strings.Contains(log, "outside the image root") {
		t.Errorf("the root with a slash is called outside itself:\n%s", log)
	}
}

// TestTwoHomesReachingOneFileAreNoMerge has two passwd homes reach one file:
// neither is a home, nothing is walked, so no line says one was.
func TestTwoHomesReachingOneFileAreNoMerge(t *testing.T) {
	root := writeImage(t, map[string]string{
		"etc/passwd": "f1:x:1001:1001::/srv/notes:/bin/sh\n" +
			"f2:x:1002:1002::/srv/x/../notes:/bin/sh\n",
		"srv/notes": "not a home\n", "srv/x/keep": "",
	}, nil)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	if log := runOutput(t, outDir, ".log"); strings.Contains(log, "is the home of") {
		t.Errorf("a merge line for a file:\n%s", log)
	}
}

func TestTheLogNotesAnOutputDirectoryThatIgnoresCase(t *testing.T) {
	root := fixtureImage(t)
	outDir := syspath.FromNative(t.TempDir())
	// The note is due where the directory takes letter case or the two
	// Unicode forms of a name (é as one character, or e and an accent) for one.
	var folds error = errors.New("keeps names apart")
	for _, pair := range [][2]string{{"probe", "PROBE"}, {"caf\u00e9", "cafe\u0301"}} {
		if err := os.WriteFile(outDir+"/"+pair[0], nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(outDir + "/" + pair[1]); err == nil {
			folds = nil
		}
		if err := os.Remove(outDir + "/" + pair[0]); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	if got := strings.Contains(log, foldingLine(outDir)+"\n"); got != (folds == nil) {
		t.Errorf("the note is in the log: %v; the output directory folds case: %v\n%s", got,
			folds == nil, log)
	}
}

func TestEveryRowNamesItsMemberOnAnOutputDirectoryThatIgnoresCase(t *testing.T) {
	root := longTempDir(t)
	// On a volume that folds case, the catalog's AppData/Roaming/Devin and
	// AppData/Roaming/devin are this one directory.
	dir := filepath.Join(root, "home/alice/AppData/Roaming/devin")
	for name, text := range map[string]string{"User/settings.json": "{}\n",
		"mcp_config.json": "{\"mcpServers\": {}}\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A link the catalog's Devin/User line reaches, in the directory devin.
	if err := os.Symlink("settings.json", filepath.Join(dir, "User/link")); err != nil {
		t.Fatal(err)
	}
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	requireRowsNameMembers(t, outDir)
}
