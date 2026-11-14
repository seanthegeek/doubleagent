//go:build parity && unix

// Parity exceptions. The comparison is byte for byte after the normalisation
// in normalise_unix_test.go drops what differs by design in every run. A case whose
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
//     exceptions_test.go). Since nothing was collected from it, the binary
//     leaves the project out of collection.json projects, the stdout
//     projects count and the inventory's per-user projects count, where the
//     sh collector lists it.
//   - projargs: under an image root, a -p value not under the root
//     (/nonexistent-cac/proj) is never tested for a directory, since the
//     answer would be the analyst's host's: the binary logs "Project P: is
//     outside the image root; not walked" among the projects and writes no
//     row. The sh collector tests it on the host, where nothing is there: a
//     "WARNING: project given with -p is not a directory" line and an
//     error_read row, which the totals count (outsideArgument in
//     exceptions_test.go).

package parity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// executableSuffix is what a program file name ends in: nothing on Unix.
const executableSuffix = ""

// parityShell is the shell the sh collector runs under: PARITY_SHELL, else dash.
func parityShell() string {
	if shell := os.Getenv("PARITY_SHELL"); shell != "" {
		return shell
	}
	return "dash"
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
	return image{work: work, root: root, alice: root + "/home/alice", bob: root + "/Users/bob"}
}

// shCollector returns the oracle that runs the sh collector script under
// parityShell.
func shCollector(script string) oracle {
	return func(t *testing.T, run invocation) collectorRun {
		t.Helper()
		run.env = append([]string{"COLLECTOR_SH=" + parityShell()}, run.env...)
		return execute(t, []string{parityShell(), script}, run)
	}
}

func TestCollectAgreesWithTheShCollector(t *testing.T) {
	repository := repositoryRoot(t)
	binary := buildBinary(t, repository)
	img := smokeImage(t, repository)
	script := filepath.Join(repository, "collectors/collect-agent-artifacts.sh")
	for _, testCase := range parityCases() {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.skip != "" {
				t.Skip(testCase.skip)
			}
			if testCase.setup != nil {
				testCase.setup(t, img)
			}
			testCase.compare(t, img, shCollector(script), []string{binary, "collect"}, normalise)
		})
	}
}
