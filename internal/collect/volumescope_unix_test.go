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
		"home/alice/bin/helper.sh":           "export CODEX_HOME=/opt/x\n",
		"home/alice/Library/notes/helper.sh": "export CODEX_HOME=/opt/x\n"}, nil)
	home := root + "/home/alice"
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.EnvScanHome = true
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	for _, file := range []string{"bin/helper.sh", "Library/notes/helper.sh"} {
		if row := manifestRow(t, outDir, home+"/"+file); !strings.Contains(row, `"agent":"shared"`) {
			t.Errorf("the sweep did not search %s: row %s", file, row)
		}
	}
}
