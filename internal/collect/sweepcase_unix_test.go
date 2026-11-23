//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/walk"
)

func TestTheSweepMatchesExclusionsByByteWhileTheWalkFoldsThem(t *testing.T) {
	root := writeImage(t, map[string]string{
		"home/alice/Library/notes/helper.sh": "export CODEX_HOME=/opt/x\n"}, nil)
	home := root + "/home/alice"
	if _, err := os.Lstat(home + "/library"); err != nil {
		t.Skip("the temporary directory tells letter case apart, so library is not Library here")
	}
	collector := &walk.Collector{Excludes: []string{"library"}}
	if !collector.Excluded(home, home+"/Library") {
		t.Error("the walk's exclusion test does not fold library onto a home's Library")
	}
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.EnvScanHome = true
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	row := manifestRow(t, outDir, home+"/Library/notes/helper.sh")
	if !strings.Contains(row, `"agent":"shared"`) {
		t.Errorf("the sweep did not search Library/notes/helper.sh: row %s", row)
	}
}
