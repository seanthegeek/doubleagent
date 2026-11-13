//go:build parity

package parity

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// image is the smoke test's main image: its work directory and the image
// root inside it, both resolved as the collectors resolve -r.
type image struct {
	work string
	root string
}

// collectorRun is what one collector run left: its exit code, its standard
// streams, the text of each output file keyed by its suffix after the run
// name, and the archive's members.
type collectorRun struct {
	code    int
	stdout  string
	stderr  string
	files   map[string]string
	members map[string]string
}

// invocation is one collector run: its arguments after the program, its
// working directory and extra environment, and the directory its output
// files land in ("" for a run that writes none).
type invocation struct {
	args   []string
	cwd    string
	env    []string
	outDir string
}

// parityShell is the shell the sh collector runs under: PARITY_SHELL, else dash.
func parityShell() string {
	if shell := os.Getenv("PARITY_SHELL"); shell != "" {
		return shell
	}
	return "dash"
}

// repositoryRoot returns the absolute path of the repository, two levels
// above internal/collect/parity's parent.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// buildBinary builds doubleagent into a temporary directory.
func buildBinary(t *testing.T, repository string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "doubleagent")
	command := exec.Command("go", "build", "-o", binary, "./cmd/doubleagent")
	command.Dir = repository
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	return binary
}

// smokeImage builds the smoke test's main image without running its checks.
func smokeImage(t *testing.T, repository string) image {
	t.Helper()
	command := exec.Command("sh", filepath.Join(repository, "collectors/tests/smoke.sh"))
	command.Env = append(os.Environ(), "SMOKE_IMAGE_ONLY=1", "TMPDIR="+t.TempDir())
	output, err := command.Output()
	if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	work, err := filepath.EvalSymlinks(lines[len(lines)-1])
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(work + "/root")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeTree(work) })
	return image{work: work, root: root}
}

// execute runs program with the invocation and reads what it wrote, with the
// output directory and the run name replaced by OUT and NAME.
func execute(t *testing.T, program []string, run invocation) collectorRun {
	t.Helper()
	command := exec.Command(program[0], append(append([]string{}, program[1:]...), run.args...)...)
	command.Dir = run.cwd
	command.Env = append(os.Environ(), append([]string{"COLLECTOR_SH=" + parityShell()}, run.env...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	result := collectorRun{files: map[string]string{}}
	var exitErr *exec.ExitError
	if err := command.Run(); errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	name := runName(run.outDir)
	replacements := []string{}
	if run.outDir != "" {
		replacements = append(replacements, run.outDir, "OUT")
	}
	if name != "" {
		replacements = append(replacements, name, "NAME")
	}
	clean := strings.NewReplacer(replacements...)
	result.stdout, result.stderr = clean.Replace(stdout.String()), clean.Replace(stderr.String())
	if name != "" {
		readOutputs(t, &result, run.outDir, name, clean)
	}
	return result
}

// runName returns the run name of the collector output in dir: the log's
// base name without its suffix, or "" when the directory holds none. It lists
// the directory instead of matching a pattern, since the directory's own name
// may hold pattern characters.
func runName(dir string) string {
	names := outputNames(dir, ".log")
	if len(names) != 1 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".log")
}

// outputNames returns the names in dir that end in suffix and hold the
// collector's "_agent-artifacts" marker.
func outputNames(dir, suffix string) []string {
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "_agent-artifacts.") && strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	return names
}

// readOutputs fills the run with the text of the output files and the
// archive's members.
func readOutputs(t *testing.T, run *collectorRun, dir, name string, clean *strings.Replacer) {
	t.Helper()
	for _, suffix := range []string{".manifest.jsonl", ".collection.json", ".log"} {
		data, _ := os.ReadFile(filepath.Join(dir, name+suffix))
		run.files[suffix] = clean.Replace(string(data))
	}
	members, err := readArchive(filepath.Join(dir, name+".tar.gz"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("archive: %v", err)
	}
	run.members = members
}

// removeOutputs deletes what a run left in a shared output directory, so the
// next run into it starts clean.
func removeOutputs(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "_agent-artifacts.") || strings.HasPrefix(name, ".stage-") {
			removeTree(filepath.Join(dir, name))
		}
	}
}
