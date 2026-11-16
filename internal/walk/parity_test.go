//go:build parity && unix

package walk

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The smoke image's passwd home with a `..` after a link (exception 47):
// frank's /srv/dotlink/../frank, srv/dotlink a link to ../opt/escape, is
// opt/frank, where the Go walk records and stages it. The sh collector
// keeps the dotted spelling and stages the copy at fs/srv/frank.
const (
	dottedHome  = "/srv/dotlink/../frank"
	reachedHome = "/opt/frank"
)

// reachDottedHome returns the sh run with frank's dotted home spelled as the
// path the kernel reaches, in its rows and log, and its staged copy moved to
// that path; nothing else changes.
func reachDottedHome(t *testing.T, run shRun) shRun {
	t.Helper()
	for i, row := range run.rows {
		run.rows[i] = strings.ReplaceAll(row, dottedHome, reachedHome)
	}
	for i, line := range run.log {
		run.log[i] = strings.ReplaceAll(line, dottedHome, reachedHome)
	}
	if _, err := os.Lstat(run.stage + "/fs/srv/frank"); err == nil {
		if err := os.MkdirAll(run.stage+"/fs/opt", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(run.stage+"/fs/srv/frank", run.stage+"/fs"+reachedHome); err != nil {
			t.Fatal(err)
		}
	}
	return run
}

// smokeImage builds the smoke test's main image without running its checks
// and returns its root.
func smokeImage(t *testing.T) string {
	t.Helper()
	command := exec.Command("sh", "../../collectors/tests/smoke.sh")
	command.Env = append(os.Environ(), "SMOKE_IMAGE_ONLY=1", "TMPDIR="+t.TempDir())
	output, err := command.Output()
	if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines[len(lines)-1] + "/root"
}

func TestSmokeImageHomeRowsMatchShCollector(t *testing.T) {
	root := smokeImage(t)
	for _, test := range oracleRuns {
		t.Run(test.name, func(t *testing.T) {
			want := reachDottedHome(t, runShCollector(t, root, test.options))
			got := runGoCollector(t, root, test.settings)
			compareRows(t, want.rows, got.rows)
			compareLog(t, want.log, got.log)
			compareStage(t, want.stage, got.stage)
		})
	}
}
