//go:build unix

package collect

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAVolumeExclusionDoesNotPruneTheSweepOfAHome(t *testing.T) {
	root := writeImage(t, map[string]string{
		"home/alice/bin/helper.sh": "export CODEX_HOME=/opt/x\n"}, nil)
	home := root + "/home/alice"
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.EnvScanHome = true
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	row := manifestRow(t, outDir, home+"/bin/helper.sh")
	if !strings.Contains(row, `"agent":"shared"`) {
		t.Errorf("the sweep did not search bin/helper.sh: row %s", row)
	}
}
