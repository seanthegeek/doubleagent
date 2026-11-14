//go:build parity

// Parity exceptions. The comparison is byte for byte after the normalisation
// in normalise_test.go drops what differs by design in every run. A case whose
// run differs further by design names an expect function, which rewrites the
// sh collector's rows, log lines, totals and archive members into what the
// binary writes, and fails when the sh text it rewrites is not there exactly
// once; the rest of the case still compares byte for byte. The exceptions
// encoded that way:
//
//   - linkproj497, projhint503, projother503, projself503, projselfabs503,
//     projout503, projoutabs503: a project the run lists whose own path runs
//     through a symlinked directory is collected where its files physically
//     are, with its recorded path in path (physicalProject in
//     exceptions_test.go, the cases in projects_cases_test.go). The binary
//     resolves the links of the project's path, its last component included,
//     in the first home of any user that holds it; a link leading out of that
//     home is resolved to the project's physical path and gets no row. The
//     binary ends the project's log line with "(through the symlinked
//     directory X, archived under Y)". The sh collector stages the files under
//     their recorded path. Where a walk recreated the link in the staging tree
//     (dot's absolute .claude in linkproj497, projhint503 and projother503;
//     alice's .claude/proj, which her home's walk records, in the projself
//     cases) the copy fails: error_copy rows beside copy failed lines, where
//     the binary writes collected rows and the totals follow. Where none did
//     (the link out of dot's home in the projout cases) the sh collector
//     collects the files at their recorded archive path, where the binary
//     archives them at the physical one. In projhint503 the project's hint
//     file env.sh, which the relocation step stages, fails the same way.
//   - projhost503: under an image root, a project whose own path leads out of
//     the root (alice's .claude/ext, an absolute link to the image's work
//     directory, which her home's walk records) is not walked, since its files
//     are on the analyst's host: the binary logs "Project [alice] P: under the
//     symlinked directory X, which leads out of the image root; not walked"
//     and writes no row for its files, and the relocation step does not
//     search it. The sh collector tries the copy through the recreated link,
//     whose target leads out of the staging directory: an error_copy row
//     beside a copy failed line, and the totals count it (hostProject in
//     exceptions_test.go).

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
