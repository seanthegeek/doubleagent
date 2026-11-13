//go:build parity

package walk

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

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
			want := runShCollector(t, root, test.options)
			got := runGoCollector(t, root, test.settings)
			compareRows(t, want.rows, got.rows)
			compareLog(t, want.log, got.log)
			compareStage(t, want.stage, got.stage)
		})
	}
}
