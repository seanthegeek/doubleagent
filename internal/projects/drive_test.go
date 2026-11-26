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
	writeTree(t, root, nil, "Proj", "work", "My Dir", "Users/jos\u00e9/proj")
	got, _ := discoverHistory(t, root, nil, `C:\proj`, "c:/Work/", "file:///C:/my%20dir",
		`C:\Users\JOS`+"\u00c9"+`\Proj`)
	// Each value takes the disk's spelling of every part, whatever the case
	// of its letters.
	var want []Project
	for _, path := range []string{"Proj", "work", "My Dir", "Users/jos\u00e9/proj"} {
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
		{"the directory a Twinny manifest's drive URL and drive path share",
			".twinny/embeddings/twinmix/manifest.json",
			`{"files":{"file:///C:/twinmix/src/a.ts":{"hash":"h1"},"C:\\twinmix\\b.ts":{"hash":"h2"}}}`,
			"twinmix"},
		{"an Antigravity project file's drive URL", ".gemini/config/projects/t.json",
			"{\n  \"folderUri\":  \"file:///c%3A/antiproj\"\n}\n", "antiproj"},
		{"a Hermes project file's drive path with a JSON escape", ".hermes/checkpoints/store/projects/a.json",
			`{"workdir": "C:\\Users\\jos\u00e9"}`, "Users/jos\u00e9"},
		{"a Gemini projects.json drive key with a JSON escape", ".gemini/projects.json",
			`{"projects":{"C:\\caf\u00e9":"gem"}}`, "caf\u00e9"},
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

// windowsRules are the rules a run on Windows reads values by.
func windowsRules() pathRules {
	return pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true, driveJSON: true}
}

func TestADriveURLKeepsItsDriveUnderTheDriveRules(t *testing.T) {
	const folder = `  "folderUri":  "file:///c%3A/Users/x",`
	cases := []struct {
		name, line, root string
		matcher          func(line []byte) []string
		rules            pathRules
		want             string
		ok               bool
	}{
		{"a live run on Windows keeps the URL on its drive", folder, "",
			absStringMatcher(windowsRules()), windowsRules(), "C:/Users/x", true},
		{"a run on Windows over an image puts it under the root", folder, "D:/img",
			absStringMatcher(windowsRules()), windowsRules(), "D:/img/Users/x", true},
		{"a live run on Linux or macOS takes the path below the drive", folder, "",
			absStringMatcher(pathRules{}), pathRules{}, "/Users/x", true},
		{"a Tabby TOML URL keeps its drive on a live run on Windows", `git_url = "file:///c%3A/Users/x"`,
			"", tomlFileURLMatcher("git_url", windowsRules()), windowsRules(), "C:/Users/x", true},
		{"a workspace folder URL keeps its drive on a live run on Windows",
			`{"folder": "file:///c%3A/Users/x"}`, "", workspaceFolderMatcher(windowsRules()), windowsRules(),
			"C:/Users/x", true},
		{"a Tabby TOML URL in an image on Windows is under the root", `git_url = "file:///c%3A/Users/x"`,
			"D:/img", tomlFileURLMatcher("git_url", windowsRules()), windowsRules(), "D:/img/Users/x", true},
		{"a workspace folder URL in an image on Windows is under the root",
			`{"folder": "file:///c%3A/Users/x"}`, "D:/img", workspaceFolderMatcher(windowsRules()),
			windowsRules(), "D:/img/Users/x", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			values := test.matcher([]byte(test.line))
			if len(values) != 1 {
				t.Fatalf("the matcher gave %q, want one value", values)
			}
			got, ok := localProjectPath(values[0], test.root, test.rules)
			if got != test.want || ok != test.ok {
				t.Errorf("localProjectPath(%q) = %q, %t; want %q, %t", values[0], got, ok, test.want, test.ok)
			}
		})
	}
}

func TestADriveURLAndADrivePathShareTheirDirectory(t *testing.T) {
	line := `{"files":{"file:///C:/w/src/a.ts":{},"C:\\w\\b.ts":{}}}`
	got, ok := commonDir(absStringMatcher(windowsRules())([]byte(line)))
	if !ok || got != "C:/w" {
		t.Errorf("commonDir = %q, %t; want C:/w, true", got, ok)
	}
}
