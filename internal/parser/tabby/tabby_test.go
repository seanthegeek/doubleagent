package tabby

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every store are proved against the recorded cases by the
// parser replay (internal/parity/replay); these tests cover the helpers
// and the XML reader.

func TestURLHostnameIsTheLowerCaseHostOfAURL(t *testing.T) {
	tests := []struct{ url, want string }{
		{"https://Git.Example.COM:8443/api", "git.example.com"},
		{"http://user:pw@host.x/", "host.x"},
		{"git.example.com", "-"},
		{"", "-"},
		{"  \x01https://A.b", "-"},
		{"ht\ttps://x.y", "-"},
		{"https://[FE80::1%25ZoNe]:80/", "fe80::1%zone"},
		{"https://[FE80::1%ZoNe]:80/", "-"},
		{"https://[::1]:80/", "::1"},
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
			item, err := jsonvalue.DecodeString(test.item)
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
	attachment, err := jsonvalue.DecodeString(`{"code":[{"git_url":"g | x","commit":"abc",` +
		`"filepath":"f.go","start_line":3},{"filepath":"","start_line":1},` +
		`{"git_url":"u","filepath":"p","start_line":""},5],` +
		`"client_code":[{"filepath":"c.go","start_line":0},{"filepath":null}],` +
		`"doc":[{"title":"T","link":"L"},{"sha":"s","page_link":"P"},{}],` +
		`"code_file_list":{"file_list":["a","b"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code g x@abc f.go:3", "code", "code u p", "client_code c.go:0",
		"client_code", "doc T L", "doc s P", "doc", "code_file_list 2 files"}
	if got := attachmentParts(nil, attachment); strings.Join(got, "|") != strings.Join(want, "|") {
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
			"endpoint=https://e supported_models=a,1,true (from config.toml [model.chat.http], file mtime)"},
		{"completion", "local", `{"model_id":""}`, "model configured: completion - via local " +
			"(from config.toml [model.completion.local], file mtime)"},
		{"chat", "http", `{}`, "model configured: chat - via http (from config.toml " +
			"[model.chat.http], file mtime)"},
		{"chat", "http", `{"model_name":"m","api_endpoint":"https://user:s3cret@e/v1"}`,
			"model configured: chat m via http endpoint=https://user:s3cret@e/v1 (from config.toml " +
				"[model.chat.http], file mtime)"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			body, err := jsonvalue.DecodeString(test.body)
			if err != nil {
				t.Fatal(err)
			}
			if got := modelText(nil, test.group, test.kind, body); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestXMLThatIsNotOneWellFormedDocumentIsRefused(t *testing.T) {
	tests := []struct {
		name, document string
		refused        bool
		reason         string
	}{
		{"an empty document has no root", "", true, "no root element: "},
		{"an unclosed element is refused", "<a>", true, ""},
		{"a second root element is refused", "<a></a><b/>", true, "a second root element: "},
		{"text outside the root is refused", "text", true, "text outside the root element: "},
		{"a mismatched end tag is refused", "<a></b>", true, ""},
		{"a declaration before the root is read", "<?xml version=\"1.0\"?>\n<a/>", false, ""},
		{"an attribute given twice is refused", `<a x="1" x="2"/>`, true, "an attribute given twice: "},
		{"a comment before the root is read", "<!-- c --><a><b value=\"v\"/></a>\n", false, ""},
		{"elements nested to the limit are read", nestedElements(maxXMLDepth), false, ""},
		{"elements nested past the limit are refused", nestedElements(maxXMLDepth + 1), true,
			"elements nested more than " + strconv.Itoa(maxXMLDepth) + " deep: "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

// nestedElements returns a document of depth elements, each inside the
// last.
func nestedElements(depth int) string {
	return strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth)
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

// parseFile writes text into a new home at rel and returns the text of
// every row the parser gives for it, secrets shown.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	return parseWith(t, secrets.Setting{}, rel, text)
}

// parseWith writes text into a new home at rel and returns the text of
// every row the parser gives for it under the secrets setting.
func parseWith(t *testing.T, setting secrets.Setting, rel, text string) []string {
	t.Helper()
	return parseAs(t, setting, "tabby", rel, text)
}

// parseAs is parseWith for an artifact the catalog gives agent.
func parseAs(t *testing.T, setting secrets.Setting, agent, rel, text string) []string {
	t.Helper()
	return parseAsBy(t, parsertest.Parse, setting, agent, rel, text)
}

// parseAsBy is parseAs parsing through parse, a function of package
// parsertest.
func parseAsBy(t *testing.T, parse func(testing.TB, parser.Parser, *input.Artifact, parser.Options,
	parser.Emit) error, setting secrets.Setting, agent, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: agent,
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}
