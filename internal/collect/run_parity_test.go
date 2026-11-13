//go:build parity

package collect

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// shCollector is the sh collector the binary is compared with.
const shCollector = "../../collectors/collect-agent-artifacts.sh"

// collectorRun is what one collector run left: its exit code, its standard
// streams and the text of each output file, keyed by the file's suffix after
// the run name.
type collectorRun struct {
	code   int
	stdout string
	stderr string
	files  map[string]string
}

// parityShell is the shell the sh collector runs under: PARITY_SHELL, else dash.
func parityShell() string {
	if shell := os.Getenv("PARITY_SHELL"); shell != "" {
		return shell
	}
	return "dash"
}

// buildBinary builds doubleagent into a temporary directory.
func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "doubleagent")
	command := exec.Command("go", "build", "-o", binary, "../../cmd/doubleagent")
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	return binary
}

// smokeImage builds the smoke test's main image without running its checks
// and returns its root, resolved as the collectors resolve -r.
func smokeImage(t *testing.T) string {
	t.Helper()
	command := exec.Command("sh", "../../collectors/tests/smoke.sh")
	command.Env = append(os.Environ(), "SMOKE_IMAGE_ONLY=1", "TMPDIR="+t.TempDir())
	output, err := command.Output()
	if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	root, err := filepath.EvalSymlinks(lines[len(lines)-1] + "/root")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// runCollector runs program with args and the common options over root into a
// new output directory, and reads what it wrote with the output directory and
// the run name replaced by OUT and NAME.
func runCollector(t *testing.T, program []string, root string, args []string) collectorRun {
	t.Helper()
	outDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	line := append(append(append([]string{}, program[1:]...), "-r", root, "-o", outDir, "-q", "--max-file-size", "1"),
		args...)
	command := exec.Command(program[0], line...)
	command.Env = append(os.Environ(), "COLLECTOR_SH="+parityShell())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	run := collectorRun{files: map[string]string{}}
	var exitErr *exec.ExitError
	if err := command.Run(); errors.As(err, &exitErr) {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	logs, _ := filepath.Glob(outDir + "/*.log")
	name := "NAME"
	if len(logs) == 1 {
		name = strings.TrimSuffix(filepath.Base(logs[0]), ".log")
	}
	clean := strings.NewReplacer(outDir, "OUT", name, "NAME")
	run.stdout, run.stderr = clean.Replace(stdout.String()), clean.Replace(stderr.String())
	for _, suffix := range []string{".manifest.jsonl", ".collection.json", ".log"} {
		data, _ := os.ReadFile(outDir + "/" + name + suffix)
		run.files[suffix] = clean.Replace(string(data))
	}
	return run
}

// Patterns of what the comparison drops: the volatile manifest fields, the
// collection.json keys that differ by design, the log's times, the start
// line's tool and capabilities and the error texts of failure lines.
var (
	manifestVolatile = regexp.MustCompile(`"atime":[0-9]+,|"error":"(\\.|[^"\\])*"`)
	summaryVolatile  = regexp.MustCompile(`(?m)^  "(started|finished|archive|tool|version|capabilities)": .*\n`)
	registryReason   = regexp.MustCompile(`"reason": "[^"]*"`)
	logTime          = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)
	logStart         = regexp.MustCompile(`^\S+ \S+ starting on `)
	logCapability    = regexp.MustCompile(`(hash|stat|worker_sh)=\S+ ?`)
	logDone          = regexp.MustCompile(`^(done: \S+) .*`)
	logReason        = regexp.MustCompile(`^((?:find|read failed|copy failed): [^:]*): .*?` +
		`(; staged path resolves outside the staging directory, not removed)?$`)
	stdoutVolatile    = regexp.MustCompile(`(?m)^(size|sha256): .*\n`)
	inventoryVolatile = regexp.MustCompile(`"(at|collector)":"[^"]*"`)
)

// normalLog returns the log's timestamped lines without their times and with
// what differs by design dropped. An untimestamped line is a program's own
// error output, which the sh collector lets into the log; the binary has none,
// so one there is kept to fail the comparison.
func normalLog(text string, keepUntimed bool) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if !logTime.MatchString(line) {
			if keepUntimed && line != "" {
				lines = append(lines, "untimed: "+line)
			}
			continue
		}
		line = logTime.ReplaceAllString(line, "")
		line = logStart.ReplaceAllString(line, "TOOL starting on ")
		line = strings.TrimSpace(logCapability.ReplaceAllString(line, ""))
		line = logDone.ReplaceAllString(line, "$1")
		line = logReason.ReplaceAllString(line, "$1$2")
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// normalise returns the run with what differs by design dropped.
func normalise(run collectorRun, binary bool) collectorRun {
	files := map[string]string{
		".manifest.jsonl": manifestVolatile.ReplaceAllString(run.files[".manifest.jsonl"], ""),
		".collection.json": registryReason.ReplaceAllString(
			summaryVolatile.ReplaceAllString(run.files[".collection.json"], ""), `"reason": R`),
		".log": normalLog(run.files[".log"], binary),
	}
	stdout := inventoryVolatile.ReplaceAllString(stdoutVolatile.ReplaceAllString(run.stdout, ""),
		`"$1":X`)
	return collectorRun{code: run.code, stdout: stdout, stderr: run.stderr, files: files}
}

// compareRuns reports each part of the two normalised runs that differs, with
// the first differing line.
func compareRuns(t *testing.T, want, got collectorRun) {
	t.Helper()
	if want.code != got.code {
		t.Errorf("exit code %d, sh collector %d", got.code, want.code)
	}
	parts := map[string][2]string{"stdout": {want.stdout, got.stdout},
		"stderr": {want.stderr, got.stderr}}
	for suffix, text := range want.files {
		parts[suffix] = [2]string{text, got.files[suffix]}
	}
	for part, texts := range parts {
		if texts[0] == texts[1] {
			continue
		}
		wantLines, gotLines := strings.Split(texts[0], "\n"), strings.Split(texts[1], "\n")
		for i := 0; i < max(len(wantLines), len(gotLines)); i++ {
			if i >= len(wantLines) || i >= len(gotLines) || wantLines[i] != gotLines[i] {
				t.Errorf("%s differs at line %d:\n sh: %q\ngot: %q", part, i+1,
					lineAt(wantLines, i), lineAt(gotLines, i))
				break
			}
		}
	}
}

// lineAt returns line i of lines, or a marker past the end.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end>"
}

// parityArgs are the argument sets both collectors run with, besides
// -r ROOT -o OUT -q --max-file-size 1.
func parityArgs(root string) []struct {
	name string
	args []string
} {
	return []struct {
		name string
		args []string
	}{
		{"no further options", nil},
		{"--no-secrets", []string{"--no-secrets"}},
		{"--full", []string{"--full"}},
		{"-u bob", []string{"-u", "bob"}},
		{"--no-docker", []string{"--no-docker"}},
		{"--no-projects", []string{"--no-projects"}},
		{"--env-scan-home", []string{"--env-scan-home"}},
		{"two -p projects", []string{"-p", root + "/srv/ccwork", "-p", root + "/srv/cchist"}},
		{"--inventory", []string{"--inventory"}},
	}
}

func TestCollectAgreesWithTheShCollector(t *testing.T) {
	binary := buildBinary(t)
	root := smokeImage(t)
	for _, testCase := range parityArgs(root) {
		t.Run(testCase.name, func(t *testing.T) {
			want := runCollector(t, []string{parityShell(), shCollector}, root, testCase.args)
			got := runCollector(t, []string{binary, "collect"}, root, testCase.args)
			wroteRows := want.files[".manifest.jsonl"] != "" || testCase.name == "--inventory"
			if want.stdout == "" || !wroteRows {
				t.Fatalf("the sh collector wrote no output to compare: %s", want.stderr)
			}
			compareRuns(t, normalise(want, false), normalise(got, true))
		})
	}
}
