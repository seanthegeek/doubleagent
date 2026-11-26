package projects

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// imageRoot returns a new image root under the test's temporary directory,
// spelled as the disk holds it.
func imageRoot(t *testing.T) string {
	t.Helper()
	return fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
}

// discoverHistory writes each value, a JSON string's decoded text, as the
// project of one line of alice's Claude Code history in the image at root,
// and returns what Discover finds there with check as the root check.
func discoverHistory(t *testing.T, root string, check RootExit, values ...string) ([]Project, []Refusal) {
	t.Helper()
	var history strings.Builder
	for _, value := range values {
		history.WriteString(`{"project":"` + strings.ReplaceAll(value, `\`, `\\`) + "\"}\n")
	}
	writeTree(t, root, map[string]string{"Users/alice/.claude/history.jsonl": history.String()})
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, refused, err := Discover(context.Background(), homes, root, nil, nil, check, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return got, refused
}

func TestDiscoverPutsEachFormOfADriveValueBelowTheImageRoot(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, nil, "Proj", "work", "My Dir", "cut", "Users/jos\u00e9/proj")
	got, _ := discoverHistory(t, root, nil, `C:\proj`, "c:/Work/", "file:///C:/my%20dir",
		"/c:/Cut", `C:\Users\JOS`+"\u00c9"+`\Proj`)
	// Each value takes the disk's spelling of every part, whatever the case
	// of its letters.
	var want []Project
	for _, path := range []string{"Proj", "work", "My Dir", "cut", "Users/jos\u00e9/proj"} {
		want = append(want, Project{"alice", root + "/" + path})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverRespellsAProjectAsTheDiskHoldsIt(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	writeTree(t, root, map[string]string{
		"Users/alice/.claude/history.jsonl": "{\"project\":\"C:\\\\\u00c9mile\"}\n" +
			`{"project":"C:\\IMG"}` + "\n",
	}, "\u00e9mile", "img", "\u0131mg")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, nil)
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
			root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
			writeTree(t, root, map[string]string{"Users/alice/" + testCase.source: testCase.content},
				testCase.project)
			homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
			got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := []Project{{"alice", root + "/" + testCase.project}}; !reflect.DeepEqual(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
