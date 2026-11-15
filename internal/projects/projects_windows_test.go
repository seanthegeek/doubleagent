package projects

import (
	"context"
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestDiscoverReadsDrivePathsAndIgnoresCase(t *testing.T) {
	root := fsmeta.OnDiskPath(syspath.FromNative(t.TempDir()))
	writeTree(t, root, map[string]string{
		"home/alice/.claude/history.jsonl": `{"project":"C:\\proj"}` + "\n" +
			`{"project":"/PROJ"}` + "\n" + `{"project":"/HOME/ALICE"}` + "\n" +
			`{"project":"c:/home/alice/inside/"}` + "\n",
		"home/alice/.codex/sessions/s.jsonl": `{"cwd":"/HOME/ALICE/documents/codex/sub"}` + "\n" +
			`{"cwd":"/home/alice/.CODEX/worktrees/w"}` + "\n",
	}, "proj", "home/alice/inside", "home/alice/other", "home/alice/Documents/Codex/sub",
		"home/alice/.codex/worktrees/w")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	extra := []string{root + "/HOME/alice/INSIDE", root + "/HOME/ALICE/other"}
	got, err := Discover(context.Background(), homes, root, extra, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The drive paths land under the image root; a case variant of a found
	// path or of the home is the same directory; the Codex filters fold.
	want := []Project{{"alice", root + "/proj"}, {"alice", root + "/home/alice/inside"},
		{"alice", root + "/HOME/ALICE/other"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverRespellsAProjectAsTheDiskHoldsIt(t *testing.T) {
	root := fsmeta.OnDiskPath(syspath.FromNative(t.TempDir()))
	writeTree(t, root, map[string]string{
		"Users/alice/.claude/history.jsonl": "{\"project\":\"C:\\\\\u00c9mile\"}\n" +
			`{"project":"C:\\IMG"}` + "\n",
	}, "\u00e9mile", "img", "\u0131mg")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, err := Discover(context.Background(), homes, root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The accented name takes the disk's case; IMG names img, never the
	// dotless-i img NTFS keeps beside it.
	want := []Project{{"alice", root + "/\u00e9mile"}, {"alice", root + "/img"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverReadsDriveKeysAndStrings(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		content string
		project string
	}{
		{"a Gemini projects.json drive key", ".gemini/projects.json", `{"projects":{"C:\\gem":"gem"}}`, "gem"},
		{"a pi trust.json drive key", ".pi/agent/trust.json", `{"C:\\pitrust":true}`, "pitrust"},
		{"a muse trust.json drive key", ".config/muse/trust.json",
			`{"schema_version":1,"projects":{"C:\\musetrust":{"decision":"trusted"}}}`, "musetrust"},
		{"a relocated Claude Code home's .claude.json drive key", ".claude-work/.claude.json",
			`{"projects":{"C:\\ccwork":{"allowedTools":[]}}}`, "ccwork"},
		{"the directory a Twinny manifest's drive paths share", ".twinny/embeddings/twinproj/manifest.json",
			`{"files":{"C:\\twinproj\\src\\a.ts":{"hash":"h1"},"C:\\twinproj\\b.ts":{"hash":"h2"}}}`,
			"twinproj"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fsmeta.OnDiskPath(syspath.FromNative(t.TempDir()))
			writeTree(t, root, map[string]string{"Users/alice/" + testCase.source: testCase.content},
				testCase.project)
			homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
			got, err := Discover(context.Background(), homes, root, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := []Project{{"alice", root + "/" + testCase.project}}; !reflect.DeepEqual(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestProjectComparisonsIgnoreCase(t *testing.T) {
	t.Run("an owner's home matches in any case", func(t *testing.T) {
		if got := projectOwner("alice:C:/Users/Alice", "c:/users/alice/p"); got != "alice" {
			t.Errorf("projectOwner = %q, want alice", got)
		}
	})
	t.Run("case variants merge into the first spelling", func(t *testing.T) {
		got := mergeProjectOwners([]Project{{"", "C:/P"}, {"bob", "c:/p"}})
		if want := []Project{{"bob", "C:/P"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a home in another case is not a project", func(t *testing.T) {
		root := fsmeta.OnDiskPath(syspath.FromNative(t.TempDir()))
		writeTree(t, root, nil, "Home")
		got := keepProjectDirs([]Project{{"u", root + "/HOME"}}, "u:"+root+"/home", nil)
		if got != nil {
			t.Errorf("got %q, want none", got)
		}
	})
}
