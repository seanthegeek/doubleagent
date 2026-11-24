//go:build unix

package collect

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTheDevinCommandLineConfigLineLeavesTheEditorsCacheUncollected(t *testing.T) {
	root := writeImage(t, map[string]string{
		"home/alice/.config/Devin/Cache/data_0":       "cache\n",
		"home/alice/.config/Devin/mcp_config.json":    "{}\n",
		"home/alice/.config/Devin/User/settings.json": "{}\n"}, nil)
	directory := root + "/home/alice/.config/Devin"
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	options := testOptions(root, outDir)
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	if rowUnder(t, outDir, directory+"/Cache") {
		t.Error("the editor's Cache was collected through the lower-case line")
	}
	if row := manifestRow(t, outDir, directory+"/mcp_config.json"); !strings.Contains(row, `"secret":true`) {
		t.Errorf("mcp_config.json is not flagged secret: row %s", row)
	}
	manifestRow(t, outDir, directory+"/User/settings.json")
}
