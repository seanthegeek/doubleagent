package tabby

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// The rows of every store are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests cover the helpers,
// with expected values from running the analyzer's functions and Python's
// urlparse and ElementTree on the same inputs.

func TestURLHostnameIsTheHostUrlparseGives(t *testing.T) {
	tests := []struct{ url, want string }{
		{"https://Git.Example.COM:8443/api", "git.example.com"},
		{"http://user:pw@host.x/", "host.x"},
		{"git.example.com", "-"},
		{"", "-"},
		{"  \x01https://A.b", "a.b"},
		{"ht\ttps://x.y", "x.y"},
		{"https://[FE80::1%ZoNe]:80/", "fe80::1%ZoNe"},
		{"//Host", "host"},
		{"1https://x", "-"},
		{"mailto:a@b", "-"},
		{"https://h?q=1", "h"},
		{"https://:80/", "-"},
		{"x+y.z-w://Q/", "q"},
	}
	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			if got := urlHostname(test.url); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestCommandTextReadsAnEntryOfTheVSCodeHistory(t *testing.T) {
	tests := []struct{ item, want string }{
		{`"fix it"`, "fix it"},
		{`""`, ""},
		{`"{\"command\":\"go\",\"context\":[{\"uri\":\"file:///a\"},{\"uri\":\"\"},5,{\"uri\":\"b\"}]}"`,
			"go [context: file:///a; b]"},
		{`"{\"command\":\"\"}"`, `{"command":""}`},
		{`"{\"x\":1}"`, `{"x":1}`},
		{`null`, ""},
		{`0`, "0"},
		{`false`, "false"},
		{`{"command": "c"}`, "c"},
		{`["a"]`, `["a"]`},
		{`"[1]"`, "[1]"},
	}
	for _, test := range tests {
		t.Run(test.item, func(t *testing.T) {
			item, err := pyjson.DecodeString(test.item)
			if err != nil {
				t.Fatal(err)
			}
			if got := commandText(item); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAttachmentPartsDescribeEachKindOfAttachment(t *testing.T) {
	attachment, err := pyjson.DecodeString(`{"code":[{"git_url":"g | x","commit":"abc",` +
		`"filepath":"f.py","start_line":3},{"filepath":"","start_line":1},` +
		`{"git_url":"u","filepath":"p","start_line":""},5],` +
		`"client_code":[{"filepath":"c.py","start_line":0},{"filepath":null}],` +
		`"doc":[{"title":"T","link":"L"},{"sha":"s","page_link":"P"},{}],` +
		`"code_file_list":{"file_list":["a","b"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code g x@abc f.py:3", "code", "code u p", "client_code c.py:0",
		"client_code", "doc T L", "doc s P", "doc", "code_file_list 2 files"}
	if got := attachmentParts(attachment); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestModelTextNamesTheModelAndItsEndpoint(t *testing.T) {
	tests := []struct {
		group, kind, body string
		want              string
	}{
		{"chat", "http", `{"model_name":"m","kind":"openai/chat","api_endpoint":"https://e",` +
			`"supported_models":["a",1,true]}`, "model configured: chat m via http kind=openai/chat " +
			"endpoint=https://e supported_models=a,1,True (from config.toml [model.chat.http], file mtime)"},
		{"completion", "local", `{"model_id":""}`, "model configured: completion - via local " +
			"(from config.toml [model.completion.local], file mtime)"},
		{"chat", "http", `{}`, "model configured: chat - via http (from config.toml " +
			"[model.chat.http], file mtime)"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			body, err := pyjson.DecodeString(test.body)
			if err != nil {
				t.Fatal(err)
			}
			if got := modelText(test.group, test.kind, body); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestXMLThatElementTreeRefusesIsRefused(t *testing.T) {
	tests := []struct {
		document string
		refused  bool
		reason   string
	}{
		{"", true, "no root element: "},
		{"<a>", true, ""},
		{"<a></a><b/>", true, "a second root element: "},
		{"text", true, "text outside the root element: "},
		{"<a></b>", true, ""},
		{"<?xml version=\"1.0\"?>\n<a/>", false, ""},
		{`<a x="1" x="2"/>`, true, "an attribute given twice: "},
		{"<!-- c --><a><b value=\"v\"/></a>\n", false, ""},
	}
	for _, test := range tests {
		t.Run(test.document, func(t *testing.T) {
			_, err := parseXML([]byte(test.document))
			if (err != nil) != test.refused {
				t.Fatalf("got %v, want refused %v", err, test.refused)
			}
			if err != nil && !strings.Contains(err.Error(), ": line ") {
				t.Errorf("error %q has no position", err)
			}
			if err != nil && !strings.HasPrefix(err.Error(), test.reason) {
				t.Errorf("error %q does not start %q", err, test.reason)
			}
		})
	}
}

func TestEditorStateIsWantedOnlyForAnEditorsGlobalState(t *testing.T) {
	tests := []struct {
		agent, rel string
		want       bool
	}{
		{"vscode", "Library/Application Support/Code/User/globalStorage/state.vscdb", true},
		{"cursor", "Library/Application Support/Cursor/User/globalStorage/state.vscdb", true},
		{"vscode", ".tabby/ee/db.sqlite", false},
		{"tabby", ".tabby/ee/db.sqlite", true},
		{"tabby", "ee/dev-db.sqlite", true},
		{"tabby", ".tabby/ee/db.backup-20261005.sqlite", true},
		{"tabby", ".tabby/events/2026-10-01.json", true},
		{"tabby", ".tabby/config.toml", true},
		{"tabby", "Library/Application Support/JetBrains/IntelliJIdea2025.2/options/" +
			"intellij-tabby-command-history.xml", true},
		{"tabby", ".tabby/ee/db.sqlite-wal", false},
	}
	for _, test := range tests {
		t.Run(test.agent+" "+test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Agent: test.agent, Rel: test.rel}
			if got := New().Wants(artifact); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
