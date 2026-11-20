package opencode

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionByEveryDirectory finds a session file by the rule stated on
// treeIndex.session, looking in every project directory of the session
// directory in name order: the first where <id>.json is a regular file that
// decodes.
func sessionByEveryDirectory(directory, sessionID string) jsonvalue.Value {
	sessions, _ := parser.DirectoryAt(directory)
	for _, project := range subdirectories(directory) {
		projectPath, ok := sessions.Child(project)
		if !ok {
			continue
		}
		listed, _ := parser.DirectoryAt(projectPath)
		candidate, ok := parser.ListingOf(listed).RegularFile(sessionID + ".json")
		if !ok {
			continue
		}
		if decoded, ok := quietObject(candidate, true); ok {
			return decoded
		}
	}
	return jsonvalue.NewNull()
}

func TestTheSessionFileIsTheOneEveryProjectDirectoryInTurnWouldGive(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{"a name in one directory", map[string]string{"p1/s.json": `{"directory": "/one"}`}},
		{"a name in two directories is the first one's", map[string]string{
			"p1/s.json": `{"directory": "/first"}`, "p2/s.json": `{"directory": "/second"}`}},
		{"a name whose first file does not decode is the next one's", map[string]string{
			"p1/s.json": `{"directory": `, "p2/s.json": `[1]`, "p3/s.json": `{"directory": "/third"}`}},
		{"a directory of the name is passed over", map[string]string{
			"p1/s.json/inner.json": `{}`, "p2/s.json": `{"directory": "/file"}`}},
		{"a name no directory holds is null", map[string]string{"p1/other.json": `{}`, "p2/t.json": `{}`}},
		{"directories in name order, not in the order made", map[string]string{
			"b/s.json": `{"directory": "/b"}`, "a/s.json": `{"directory": "/a"}`, "c/s.json": `{}`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for rel, text := range test.files {
				path := filepath.Join(directory, filepath.FromSlash(rel))
				if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			index := newTreeIndex()
			for _, identifier := range []string{"s", "t", "other", "absent"} {
				want := sessionByEveryDirectory(directory, identifier).JSON()
				if got := index.session(directory, identifier, parser.Work{}).JSON(); got != want {
					t.Errorf("session %s: got %s, want %s", identifier, got, want)
				}
			}
		})
	}
}

// Each message file names its session, so a session directory with as many
// project directories as there are sessions would cost their product if
// every lookup looked in every directory. A lookup looks only where the
// listings hold the file's name.
func TestFindingTheSessionFilesOfManySessionsIsLinear(t *testing.T) {
	const storage = ".local/share/opencode/storage"
	for _, sessions := range []int{10, 80} {
		t.Run(fmt.Sprintf("%d sessions beside %d empty project directories", sessions, sessions), func(t *testing.T) {
			files := map[string]string{storage + "/session/zz/ses_000.json": `{"directory": "/work/x"}`}
			for i := range sessions {
				files[fmt.Sprintf("%s/session/p%03d/.keep", storage, i)] = ``
				files[fmt.Sprintf("%s/message/s/msg_%03d.json", storage, i)] =
					fmt.Sprintf(`{"id": "msg_%03d", "sessionID": "ses_%03d", "role": "user"}`, i, i)
			}
			home := writeTree(t, files)
			subject := New()
			subject.Work = parser.CountingWork()
			for i := range sessions {
				rel := fmt.Sprintf("%s/message/s/msg_%03d.json", storage, i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}}
				if err := subject.Parse(artifact, parser.Options{}, func(timeline.Row) {}); err != nil {
					t.Fatal(err)
				}
			}
			listings := subject.Work.Count("project directory listings")
			steps := subject.Work.Count("session file candidates")
			if listings != sessions+1 || steps != 1 {
				t.Errorf("%d listings and %d candidates looked at, want %d listings and 1 candidate",
					listings, steps, sessions+1)
			}
		})
	}
}
