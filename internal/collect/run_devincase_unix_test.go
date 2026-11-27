//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestACommandLineDirectoryLineTakesTheEditorsDirectoryWithoutItsCaches(t *testing.T) {
	cases := []struct{ editor, cli, file string }{
		{".config/Devin", ".config/devin", "mcp_config.json"},
		{".config/Cursor", ".config/cursor", "auth.json"},
	}
	for _, test := range cases {
		t.Run(test.editor, func(t *testing.T) {
			root := writeImage(t, map[string]string{
				"home/alice/" + test.editor + "/Cache/data_0":       "cache\n",
				"home/alice/" + test.editor + "/Cookies":            "cookies\n",
				"home/alice/" + test.editor + "/User/settings.json": "{}\n",
				"home/alice/" + test.cli + "/" + test.file:          "{}\n"}, nil)
			home := root + "/home/alice"
			if _, err := os.Lstat(home + "/" + test.editor + "/" + test.file); err == nil {
				t.Skip("the image directory folds case, so it cannot hold the two directories apart")
			}
			outDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), testOptions(root, outDir), testHost(t),
				&stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			editor := home + "/" + test.editor
			if row := manifestRow(t, outDir, editor+"/Cache"); !strings.Contains(row,
				`"status":"skipped_excluded"`) {
				t.Errorf("the editor's Cache is not excluded: row %s", row)
			}
			if rowUnder(t, outDir, editor+"/Cache/") {
				t.Error("a file under the editor's Cache has a row")
			}
			if row := manifestRow(t, outDir, editor+"/Cookies"); !strings.Contains(row, `"secret":true`) {
				t.Errorf("the editor's Cookies is not flagged secret: row %s", row)
			}
			for _, path := range []string{editor + "/User/settings.json",
				home + "/" + test.cli + "/" + test.file} {
				if row := manifestRow(t, outDir, path); !strings.Contains(row, `"status":"collected"`) {
					t.Errorf("%s is not collected: row %s", path, row)
				}
			}
		})
	}
}
