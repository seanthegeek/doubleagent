package claudedesktop

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// writeEmpty writes an empty file at the slash-separated rel below root,
// making its directories.
func writeEmpty(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestARecordsIDsThatAreNotFileNamesNameNothingCollected(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		cliID     string
		outside   []string
	}{
		{"a session id of .. is not the account directory", "..", "",
			[]string{cowork + "/../audit.jsonl"}},
		{"a session id holding a slash is not a directory below the org", "a/b", "",
			[]string{cowork + "/a/b/audit.jsonl"}},
		{"a CLI id climbing out of imported-staging reads nothing there", "", "../x",
			[]string{cowork + "/imported-staging/.keep", cowork + "/x.jsonl"}},
		{"a CLI id climbing out of a project slug reads nothing there", "", "../x",
			[]string{".claude/projects/p/.keep", ".claude/projects/x.jsonl"}},
		{"a CLI id of .. reads nothing", "", "..", []string{cowork + "/imported-staging/...jsonl"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			for _, rel := range test.outside {
				writeEmpty(t, home, rel)
			}
			rel := cowork + "/" + session + ".json"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			record := jsonvalue.NewObject(
				jsonvalue.Member{Key: "sessionId", Value: jsonvalue.NewString(test.sessionID)},
				jsonvalue.Member{Key: "cliSessionId", Value: jsonvalue.NewString(test.cliID)})
			if New().sessionCollected(artifact, record) {
				t.Error("a file outside the session's places counted as collected")
			}
		})
	}
}
