package projects

import (
	"reflect"
	"testing"
)

// matcherCase is one line given to a reader and the values it must give.
type matcherCase struct {
	name string
	line string
	want []string
}

// checkMatcher runs every case through extract.
func checkMatcher(t *testing.T, extract func(line []byte) []string, cases []matcherCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := extract([]byte(test.line))
			if len(got) != 0 || len(test.want) != 0 {
				if !reflect.DeepEqual(got, test.want) {
					t.Errorf("got %q, want %q", got, test.want)
				}
			}
		})
	}
}

func TestJSONValues(t *testing.T) {
	checkMatcher(t, jsonValueMatcher("cwd"), []matcherCase{
		{"a smoke fixture line gives its value", `{"cwd":"/srv/ampproj","prompt":"hi"}`,
			[]string{"/srv/ampproj"}},
		{"blanks after the colon are allowed", `{"cwd":   "/a"}`, []string{"/a"}},
		{"an escaped slash is kept for the stream to undo", `{"cwd":"\/srv\/x"}`,
			[]string{`\/srv\/x`}},
		{"every match on a line counts", `{"cwd":"/a"}{"cwd":"/b"}`, []string{"/a", "/b"}},
		{"a value stops at the first quote, escaped or not", `{"cwd":"/a\"b"}`,
			[]string{`/a\`}},
		{"a blank before the colon is no match", `{"cwd" : "/a"}`, nil},
		{"a longer key ending in the key matches", `{"xcwd":"/a"}`, nil},
		{"a value not starting with a slash is still read", `{"cwd":"rel"}`, []string{"rel"}},
	})
}

func TestJSONPathKeys(t *testing.T) {
	checkMatcher(t, jsonPathKeyMatcher(`\{`), []matcherCase{
		{"keys followed by an object", `{"projects":{"/srv/a": {"x":1},"/srv/b":{}}}`,
			[]string{"/srv/a", "/srv/b"}},
		{"a relative key is no match", `{"rel": {}}`, nil},
		{"a key followed by a string is no match", `{"/srv/a": "x"}`, nil},
	})
	checkMatcher(t, jsonPathKeyMatcher(`[tf]`), []matcherCase{
		{"keys set to true or false", `{"/srv/pitrust": true, "/x": false, "/y": null}`,
			[]string{"/srv/pitrust", "/x"}},
	})
	checkMatcher(t, jsonPathKeyMatcher(`"`), []matcherCase{
		{"keys set to a string", `{"/srv/gem": "gem", "/x": 1}`, []string{"/srv/gem"}},
	})
}

func TestAbsStrings(t *testing.T) {
	checkMatcher(t, absStringMatcher(), []matcherCase{
		{"a Windows drive URI with an encoded colon keeps its path",
			`        "folderUri":  "file:///c%3A/srv/agywin"`, []string{"/srv/agywin"}},
		{"a file URI loses its scheme", `"folderUri":  "file:///srv/agyapp"`,
			[]string{"/srv/agyapp"}},
		{"a drive URI with a colon keeps its path", `"file:///C:/x/y"`, []string{"/x/y"}},
		{"an encoded space is decoded", `"file:///srv/sp%20ace"`, []string{"/srv/sp ace"}},
		{"only %20 is decoded", `"/a%2Fb%20c"`, []string{"/a%2Fb c"}},
		{"plain paths are found among other strings", `["/a", "rel", "/b"]`,
			[]string{"/a", "/b"}},
		{"a URI with a host is no match", `"file://host/x"`, nil},
	})
}

func TestLocalPath(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{"file:///c%3a/x", "/x"},
		{"file:///C%3A/", "/"},
		{"file:///C%3Ax", "/C%3Ax"},
		{"file:///1%3A/x", "/1%3A/x"},
		{"file:///srv/x", "/srv/x"},
		{"/a%20b%20c", "/a b c"},
	} {
		if got := localPath(test.value); got != test.want {
			t.Errorf("localPath(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestYAMLValues(t *testing.T) {
	checkMatcher(t, yamlValueMatcher("cwd"), []matcherCase{
		{"a smoke fixture line gives its value", "cwd: /srv/copproj", []string{"/srv/copproj"}},
		{"one quote is removed at each end", `cwd: '/srv/cop'`, []string{"/srv/cop"}},
		{"the quotes need not pair", `cwd:"/a'`, []string{"/a"}},
		{"a carriage return keeps the closing quote", "cwd: '/a'\r", []string{"/a'\r"}},
		{"an indented key is no match", "  cwd: /a", nil},
		{"a longer key is no match", "cwdx: /a", nil},
	})
}

func TestJSON5Values(t *testing.T) {
	checkMatcher(t, json5ValueMatcher("workspace"), []matcherCase{
		{"a smoke fixture line gives its value",
			`{agents:{defaults:{workspace:"/srv/clawprofile"}}}`, []string{"/srv/clawprofile"}},
		{"the key and value may be single-quoted", `{'workspace': '/a'}`, []string{"/a"}},
		{"a key at the start of the line matches", `workspace: "/a"`, []string{"/a"}},
		{"a key inside a longer name is no match", `{myworkspace: "/a"}`, nil},
		{"the prefix is cut at the key's last occurrence",
			`{"workspace": "/a/workspace: 'b"}`, []string{"b"}},
	})
}

func TestTOMLFileURLs(t *testing.T) {
	checkMatcher(t, tomlFileURLMatcher("git_url"), []matcherCase{
		{"a smoke fixture line gives its path", `git_url = "file:///srv/tabbyproj"`,
			[]string{"/srv/tabbyproj"}},
		{"single quotes and an encoded space", `  git_url='file:///a%20b'`, []string{"/a b"}},
		{"a remote URL is no match", `git_url = "https://example.invalid/x"`, nil},
	})
}

func TestSessionAndEditorReaders(t *testing.T) {
	checkMatcher(t, sessionCwdMatcher(), []matcherCase{
		{"a rollout line gives its cwd", `{"payload":{"cwd":"/srv/oiproj"}}`,
			[]string{"/srv/oiproj"}},
		{"a blank after the colon is no match", `{"cwd": "/a"}`, nil},
	})
	checkMatcher(t, workspaceFolderMatcher(), []matcherCase{
		{"a smoke fixture line gives its folder", `{"folder":"file:///Users/bob/Documents/app"}`,
			[]string{"/Users/bob/Documents/app"}},
		{"an encoded space is decoded", `{"folder": "file:///a%20b"}`, []string{"/a b"}},
	})
	checkMatcher(t, additionalDirectoryMatcher(), []matcherCase{
		{"absolute items of the list", `{"acp_additional_directories": ["/a", "rel", "/b"]}`,
			[]string{"/a", "/b"}},
		{"a list not closed on the line is no match", `{"acp_additional_directories": ["/a",`,
			nil},
	})
}

func TestGooseWorkingDirs(t *testing.T) {
	for _, test := range []struct {
		name  string
		lines []string
		want  []string
	}{
		{"a smoke fixture project", []string{"---", "name: demo", "description: ''",
			"workingDirs:", "- /srv/gooseproj", "---"}, []string{"/srv/gooseproj"}},
		{"quotes and an empty item", []string{"---", "name: quoted", "workingDirs:", "- ",
			"- '/srv/goosequoted'", "---"}, []string{"/srv/goosequoted"}},
		{"trailing blanks and a carriage return", []string{"workingDirs: \r", "- \"/a\" \t"},
			[]string{"/a"}},
		{"mismatched quotes stay", []string{"workingDirs:", `- '/a"`}, []string{`'/a"`}},
		{"another line ends the list", []string{"workingDirs:", "- /a", "x: y", "- /b"},
			[]string{"/a"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := gooseWorkingDirs(test.lines); !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestCommonDir(t *testing.T) {
	for _, test := range []struct {
		name  string
		paths []string
		want  string
		found bool
	}{
		{"the smoke Twinny manifest", []string{"/srv/twinproj/src/a.ts", "/srv/twinproj/b.ts"},
			"/srv/twinproj", true},
		{"one file gives its directory", []string{"/a/b/c.ts"}, "/a/b", true},
		{"a file at the top level shares too little", []string{"/a.ts"}, "", false},
		{"paths sharing only the root", []string{"/a/x", "/b/y"}, "", false},
		{"numeric components compare as numbers", []string{"/a/1/x", "/a/01/y"}, "/a/1", true},
		{"no paths", nil, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, found := commonDir(test.paths)
			if got != test.want || found != test.found {
				t.Errorf("got %q, %v, want %q, %v", got, found, test.want, test.found)
			}
		})
	}
}
