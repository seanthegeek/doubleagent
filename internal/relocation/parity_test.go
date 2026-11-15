//go:build parity

package relocation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/hostcmd"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// systemPath puts the system tools first, so the sh collector runs the grep
// and find of the host rather than a replacement earlier on the path.
const systemPath = "PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin"

// smokeImage builds the smoke test's image under a temporary directory and
// returns its root.
func smokeImage(t *testing.T) string {
	t.Helper()
	command := exec.Command("sh", filepath.Join("..", "..", "collectors", "tests", "smoke.sh"), "dash")
	command.Env = []string{systemPath, "SMOKE_IMAGE_ONLY=1", "TMPDIR=" + t.TempDir(),
		"HOME=" + t.TempDir()}
	output, err := command.Output()
	if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines[len(lines)-1] + "/root"
}

// shRun is what one sh collector run left: its user and project lists, run
// name, relocation object and relocation log lines.
type shRun struct {
	homes, projects []users.Home
	name            string
	relocation      string
	logLines        []string
}

// runCollector runs the sh collector over root into outDir with args.
func runCollector(t *testing.T, root, outDir string, args ...string) shRun {
	t.Helper()
	script := filepath.Join("..", "..", "collectors", "collect-agent-artifacts.sh")
	arguments := append([]string{script, "-r", root, "-o", outDir, "-q", "--max-file-size", "1"},
		args...)
	command := exec.Command("dash", arguments...)
	command.Env = []string{systemPath, "COLLECTOR_SH=dash", "HOME=" + t.TempDir()}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	logs, _ := filepath.Glob(filepath.Join(outDir, "*_agent-artifacts.log"))
	if len(logs) != 1 {
		t.Fatalf("want one log in %s, got %q", outDir, logs)
	}
	run := shRun{name: strings.TrimSuffix(filepath.Base(logs[0]), ".log")}
	run.readSummary(t, filepath.Join(outDir, run.name+".collection.json"))
	run.readLog(t, logs[0])
	return run
}

// readSummary takes the users, homes and relocation object of collection.json.
func (run *shRun) readSummary(t *testing.T, path string) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var summary struct{ Users, Homes []string }
	if err := json.Unmarshal(text, &summary); err != nil {
		t.Fatal(err)
	}
	for i, user := range summary.Users {
		run.homes = append(run.homes, users.Home{User: user, Path: summary.Homes[i]})
	}
	_, relocation, _ := strings.Cut(string(text), "\n  \"relocation\": ")
	end := regexp.MustCompile(`"files_skipped_size": [0-9]+\}`).FindStringIndex(relocation)
	run.relocation = relocation[:end[1]]
}

// readLog takes the project list from the `Project` lines and the lines the
// relocation step writes, each without its time.
func (run *shRun) readLog(t *testing.T, path string) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	project := regexp.MustCompile(`^Project (?:\[([^]]*)\] )?(/.*)$`)
	for _, line := range strings.Split(string(text), "\n") {
		_, line, _ = strings.Cut(line, " ")
		if match := project.FindStringSubmatch(line); match != nil {
			run.projects = append(run.projects, users.Home{User: match[1], Path: match[2]})
		}
		for _, prefix := range []string{"Relocation hints ", "relocation hint: ", "sweep skipped: "} {
			if strings.HasPrefix(line, prefix) {
				run.logLines = append(run.logLines, line)
			}
		}
	}
}

// foldedClaims lists a catalog table's matches under base as expand_catalog
// prints them, case variants classified and folded as the sh collector does.
func foldedClaims(base string, table []catalog.Entry) []string {
	var paths []string
	for _, entry := range table {
		paths = append(paths, glob.Expand(base, entry.Glob)...)
	}
	var claims []string
	for i, path := range paths {
		if kept, keep := foldCase(paths, i, path); keep {
			claims = append(claims, kept)
		}
	}
	return claims
}

// foldCase applies classify_case_matches and fold_case_matches to match i.
func foldCase(paths []string, i int, path string) (string, bool) {
	lower := lowerASCII(path)
	for j := range i {
		if lowerASCII(paths[j]) == lower {
			return path, path != paths[j] && !sameFile(path, paths[j])
		}
	}
	parent := -1
	for j, other := range paths {
		otherLower := lowerASCII(other) + "/"
		if j != i && len(lower) > len(otherLower) && strings.HasPrefix(lower, otherLower) &&
			path[:len(other)] != other && (parent < 0 || len(other) < len(paths[parent])) {
			parent = j
		}
	}
	if parent < 0 {
		return path, true
	}
	reference := paths[parent]
	prefix, rest := path[:len(reference)], path[len(reference):]
	if sameFile(prefix, reference) {
		return reference + rest, true
	}
	return prefix + rest, true
}

// sameFile reports whether two paths name one file by device and inode.
func sameFile(first, second string) bool {
	firstInfo, err := os.Lstat(first)
	if err != nil {
		return false
	}
	secondInfo, err := os.Lstat(second)
	return err == nil && os.SameFile(firstInfo, secondInfo)
}

// goSearch runs Search over root with what the sh run used.
func goSearch(t *testing.T, root, outDir string, run shRun, sweep bool) (Result, []string) {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	log := &lineLog{}
	result, err := Search(context.Background(), Deps{Tables: tables, Root: root, Homes: run.homes,
		Projects: run.projects, EnvScanHome: sweep, EnvScanMaxBytes: 1 << 20,
		Excludes: tables.Excludes, OutDir: outDir, RunName: run.name, Log: log,
		Plutil: hostcmd.NewPlutil(hostcmd.System()), Claims: foldedClaims,
		Collect: func(string, string, string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	return result, log.lines
}

// parityCase is one sh collector run the search is compared with.
type parityCase struct {
	name   string
	outDir func() string
	args   []string
	setup  func() func()
}

// parityCases returns the --env-scan-home runs of the smoke test and a
// default run, over the image at root.
func parityCases(t *testing.T, root string) []parityCase {
	alice := root + "/home/alice"
	addOnceFiles := func() func() {
		onceFiles := []string{alice + "/.bashrc.d/once", alice + "/scripts/once.sh"}
		for _, path := range onceFiles {
			if err := os.WriteFile(path, []byte("export CODEX_HOME=/opt/x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return func() {
			for _, path := range onceFiles {
				os.Remove(path)
			}
		}
	}
	sweep := []string{"--env-scan-home"}
	return []parityCase{
		{"a default run", t.TempDir, nil, nil},
		{"a sweep", t.TempDir, sweep, nil},
		{"a sweep with a project holding the homes", t.TempDir,
			[]string{"--env-scan-home", "-p", root + "/home", "-p", root + "/opt/daemonproj"}, nil},
		{"a sweep writing into a home", func() string { return alice }, sweep, nil},
		{"a sweep with one file more of each pass", t.TempDir, sweep, addOnceFiles},
	}
}

func TestSearchAgreesWithTheShCollector(t *testing.T) {
	root := smokeImage(t)
	for _, testCase := range parityCases(t, root) {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				defer testCase.setup()()
			}
			outDir := testCase.outDir()
			run := runCollector(t, root, outDir, testCase.args...)
			if len(run.logLines) == 0 || !strings.Contains(run.relocation, `"source": `) {
				t.Fatal("the sh run recorded no hint to compare with")
			}
			sweep := slices.Contains(testCase.args, "--env-scan-home")
			result, logLines := goSearch(t, root, outDir, run, sweep)
			if outDir == root+"/home/alice" {
				removeRunFiles(t, outDir, run.name)
			}
			if got := JSON(result); got != run.relocation {
				t.Errorf("relocation object differs:\n%s", lineDiff(got, run.relocation))
			}
			if !slices.Equal(logLines, run.logLines) {
				t.Errorf("log lines differ:\n%s", lineDiff(strings.Join(logLines, "\n"),
					strings.Join(run.logLines, "\n")))
			}
		})
	}
}

// removeRunFiles removes what a run writing into dir left there.
func removeRunFiles(t *testing.T, dir, name string) {
	t.Helper()
	leftovers, _ := filepath.Glob(filepath.Join(dir, name+".*"))
	leftovers = append(leftovers, filepath.Join(dir, ".stage-"+name))
	for _, path := range leftovers {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}

// lineDiff lists the lines of got and want that differ, by position.
func lineDiff(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	var diff []string
	for i := range max(len(gotLines), len(wantLines)) {
		var gotLine, wantLine string
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if gotLine != wantLine {
			diff = append(diff, "go: "+gotLine, "sh: "+wantLine)
		}
		if len(diff) > 20 {
			break
		}
	}
	return strings.Join(diff, "\n")
}

// JSON returns the `relocation` object of collection.json as the sh collector
// writes it for result, for the byte comparison below; the binary's own
// writer is the summary package's, the one writer of collection.json.
func JSON(result Result) string {
	var text strings.Builder
	text.WriteString(`{"hints": [`)
	for i, hint := range result.Hints {
		if i > 0 {
			text.WriteString(",")
		}
		fmt.Fprintf(&text, "\n    {\"user\": \"%s\", \"source\": \"%s\", \"line\": %d, ",
			manifest.EscapeJSON(hint.User), manifest.EscapeJSON(hint.Source), hint.Line)
		fmt.Fprintf(&text, "\"variable\": \"%s\", \"agent\": \"%s\", \"value\": \"%s\"}",
			manifest.EscapeJSON(hint.Variable), manifest.EscapeJSON(hint.Agent),
			manifest.EscapeJSON(hint.Value))
	}
	if len(result.Hints) > 0 {
		text.WriteString("\n  ")
	}
	text.WriteString(`], "registry": {"read": false, "reason": "sh collector", "users_read": [], `)
	text.WriteString(`"users_not_loaded": [], "users_denied": []}, `)
	fmt.Fprintf(&text, `"files_searched": %d, "files_skipped_size": %d}`,
		result.FilesSearched, result.FilesSkippedSize)
	return text.String()
}
