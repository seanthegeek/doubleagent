package analyze

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// runOutputs is what one run of detect, detect --json and timeline over a
// collection printed and wrote, by output.
type runOutputs struct {
	detect, detectJSON, summary, rows, sessions, inventory string
}

// all returns every output joined.
func (o runOutputs) all() string {
	return strings.Join([]string{o.detect, o.detectJSON, o.summary, o.rows, o.sessions, o.inventory}, "\n")
}

// outputsOf writes files as a collection and runs detect, detect --json and
// timeline over it with options, returning what each printed or wrote.
func outputsOf(t *testing.T, files []collectedFile, options ...string) runOutputs {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1")),
		member("projects", stringArray([]string{"/srv/proj"}))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	var printed []string
	for _, args := range [][]string{{"detect"}, {"detect", "--json"}, {"timeline"}} {
		line := append(append(args, options...), collection)
		if args[0] == "timeline" {
			line = append(line, output)
		}
		code, stdout, stderr := runCommand(env, line...)
		if code > ExitNothing || stderr != "" {
			t.Fatalf("%q exited %d: %s", line, code, stderr)
		}
		printed = append(printed, strings.ReplaceAll(stdout, work, "{WORK}"))
	}
	read := func(name string) string {
		data, err := osfile.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return runOutputs{printed[0], printed[1], printed[2], read(timelineFile), read(sessionsFile), read(inventoryFile)}
}

// countLines are the lines of the summary that count rows, sessions and
// dropped duplicates.
var countLines = regexp.MustCompile(`(?m)^(?:dropped|rows|sessions): .*$|^ {11}\S+ +\S+ +[0-9]+$`)

// withheldLeakFixture is a collection in which a secret is identified at one
// place of a parse's rows and the same value stands elsewhere in them, with
// that secret.
type withheldLeakFixture struct {
	name   string
	files  []collectedFile
	secret string
}

// withheldLeakInOneAgentFixtures returns the collections in which the secret
// stands only in the files of one agent.
func withheldLeakInOneAgentFixtures() []withheldLeakFixture {
	const home = "/home/alice"
	vscodeWorkspace := ".config/Code/User/workspaceStorage/h/"
	cody := "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	return []withheldLeakFixture{
		{"a Claude Code bridge record printed whole", []collectedFile{{"alice", home, "claude-code",
			".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
			`{"type":"system","subtype":"bridge_status","url":"https://claude.example/code?token=FIXTURE-BRIDGE"}` +
				"\n" + `{"type":"bridge-session","sessionId":"s1","bridgeSessionId":"b1","lastSequenceNum":1}` + "\n"}},
			"FIXTURE-BRIDGE"},
		{"a VS Code workspace folder used in four places", []collectedFile{
			{"alice", home, "vscode", vscodeWorkspace + "workspace.json",
				`{"folder":"vscode-remote://ci:s3cret@devbox.example/w",` +
					`"workspace":"vscode-remote://ci:s3cret@devbox.example/w.code-workspace"}`},
			{"alice", home, "vscode", vscodeWorkspace + "chatSessions/s1.json",
				`{"version": 3, "sessionId": "s1", "creationDate": 1790845200000, "requests": []}`},
			{"alice", home, "vscode", vscodeWorkspace + "chatEditingSessions/k/state.json",
				`{"recentSnapshot": {"entries": [{"resource": "file:///w/a.go"}]}}`}}, "s3cret"},
		{"a VS Code workspace folder with a short password used in four places", []collectedFile{
			{"alice", home, "vscode", vscodeWorkspace + "workspace.json",
				`{"folder":"vscode-remote://ci:pw7@devbox.example/w",` +
					`"workspace":"vscode-remote://ci:pw7@devbox.example/w.code-workspace"}`},
			{"alice", home, "vscode", vscodeWorkspace + "chatSessions/s1.json",
				`{"version": 3, "sessionId": "s1", "creationDate": 1790845200000, "requests": []}`},
			{"alice", home, "vscode", vscodeWorkspace + "chatEditingSessions/k/state.json",
				`{"recentSnapshot": {"entries": [{"resource": "file:///w/a.go"}]}}`}}, "pw7"},
		{"a Cody account key inside a session id", []collectedFile{{"alice", home, "cody", cody,
			`{"https://ci:s3cret@sg.example/-ana": {"chat": {"c1": {"id": "2026-01-01T00:00:00.000Z", ` +
				`"chatTitle": "t", "interactions": []}}}, "https://sg.example/-bob": {"chat": {"c1": ` +
				`{"id": "2026-01-01T00:00:00.000Z", "chatTitle": "u", "interactions": []}}}}`}}, "s3cret"},
		{"a Claude Desktop fetch grant written as an object", []collectedFile{{"alice", home, "claude-desktop",
			".config/Claude/local-agent-mode-sessions/a/o/local_c0c0c0c0-1111-4222-8333-444444444444.json",
			`{"sessionId": "local_c0c0c0c0-1111-4222-8333-444444444444", "createdAt": 1790845200000, ` +
				`"webFetchAllowedUrls": {"https://ci:s3cret@docs.example/": true}}`}}, "s3cret"},
		{"a Hermes remote backend given by host", []collectedFile{{"alice", home, "hermes",
			".config/Hermes/connections.json", `{"connections": [{"kind": "ssh", "label": "box", ` +
				`"host": "https://u:FIXTURE-PW@box.example", "user": "alice", "port": 22}]}`}}, "FIXTURE-PW"},
		{"an Agent Zero chat that is not JSON", []collectedFile{{"alice", home, "agent-zero",
			"agent-zero/usr/chats/ctx/chat.json", `{"data": {"telegram_bot_cfg": {"token": "FIXTURE-\q"}}}`}},
			"invalid escape sequence"},
		{"a ShellGPT settings line without an equals sign", []collectedFile{{"alice", home, "shellgpt",
			".config/shell_gpt/.sgptrc", "sk-FIXTURE-PASTED\nDEFAULT_MODEL=gpt-4o\n"}}, "sk-FIXTURE-PASTED"},
	}
}

// withheldLeakQuotedByChatFixtures returns the collections in which a chat
// quotes a secret that another agent's file holds.
func withheldLeakQuotedByChatFixtures() []withheldLeakFixture {
	const home = "/home/alice"
	return []withheldLeakFixture{
		{"a Hermes headers object whose bearer token a chat quotes alone", []collectedFile{
			{"alice", home, "hermes", ".config/Hermes/connections.json", `{"connections": [{"kind": "remote", ` +
				`"label": "work", "url": "https://hermes.example/", "headers": {"encoding": "plain", ` +
				`"value": {"Authorization": "Bearer fake-header-secret-0001"}}}]}`},
			{"alice", home, "claude-code", ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
				quoting("the header fake-header-secret-0001 was sent")}}, "fake-header-secret-0001"},
		{"a ShellGPT key pasted with spaces after it that a chat quotes", []collectedFile{
			{"alice", home, "shellgpt", ".config/shell_gpt/.sgptrc", "fake-secret-sgpt-0001  \nDEFAULT_MODEL=gpt-4o\n"},
			{"alice", home, "claude-code", ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
				quoting("pasted fake-secret-sgpt-0001 too")}}, "fake-secret-sgpt-0001"},
		{"a Crush key holding a quote and a backslash that a tool call quotes as JSON", []collectedFile{
			{"alice", home, "crush", ".config/crush/crush.json",
				`{"providers": {"b": {"api_key": "fake-crush-key\"sec\\ret-0002"}}}`},
			{"alice", home, "claude-code", ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
				`{"type":"assistant","sessionId":"s1","uuid":"u2","timestamp":"2026-10-01T00:00:01Z",` +
					`"cwd":"/srv/proj","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1",` +
					`"name":"Custom","input":{"keys":["fake-crush-key\"sec\\ret-0002"]}}]}}` + "\n"}},
			"fake-crush-key"},
		{"a Crush key holding a slash that another JSON writer escapes", crushKeyQuotedBy("fake-slash-key/0039",
			`{\"k\":\"fake-slash-key\\/0039\"}`), "fake-slash-key"},
		{"a Crush key holding & < > that another JSON writer escapes", crushKeyQuotedBy("fake-amp-key&<0040>",
			`{\"k\":\"fake-amp-key\\u0026\\u003c0040\\u003e\"}`), "fake-amp-key"},
		{"a Crush key holding a slash and < that another JSON writer escapes in upper-case hex",
			crushKeyQuotedBy("fake-both-key/<0041",
				`{\"k\":\"fake-both-key\\/\\u003C0041\"}`), "fake-both-key"},
	}
}

// crushKeyQuotedBy returns the collection of a Crush file holding key and a
// Claude Code prompt that says quoted, JSON text as another writer escaped
// it (the characters of quoted are those of the prompt, in its own JSON
// string).
func crushKeyQuotedBy(key, quoted string) []collectedFile {
	const home = "/home/alice"
	return []collectedFile{
		{"alice", home, "crush", ".config/crush/crush.json",
			`{"providers": {"b": {"api_key": ` + jsonvalue.NewString(key).JSON() + `}}}`},
		{"alice", home, "claude-code", ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
			quoting("the tool said " + quoted)}}
}

// withheldLeakInCodexRemoteFixtures returns the collection of a Codex remote
// whose rows differ only in their credentials.
func withheldLeakInCodexRemoteFixtures(t *testing.T) []withheldLeakFixture {
	t.Helper()
	database, err := osfile.ReadFile(filepath.Join("..", "parsers", "codex", "testdata", "secrets.db"))
	if err != nil {
		t.Fatal(err)
	}
	return []withheldLeakFixture{
		{"a Codex remote whose rows differ only in their credentials", []collectedFile{
			{"alice", "/home/u", "codex-cli", ".codex/state_5.sqlite", string(database)},
			{"alice", "/home/u", "codex-cli", ".codex/sessions/rollout-t1.jsonl",
				`{"timestamp": "2026-01-02T03:04:05Z", "type": "session_meta", "payload": {"id": "t1", ` +
					`"cwd": "/w/proj", "git": {"repository_url": "https://ci:OTHER@github.com/o/r.git"}}}` + "\n"}},
			"s3cret"},
	}
}

// quoting returns a Claude Code transcript line of a user prompt that says
// text.
func quoting(text string) string {
	return `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-10-01T00:00:00Z",` +
		`"cwd":"/srv/proj","message":{"role":"user","content":"` + text + `"}}` + "\n"
}

func TestAWithheldRunWritesNoIdentifiedSecretAndCountsAsTheDefaultRunDoes(t *testing.T) {
	fixtures := slices.Concat(withheldLeakInOneAgentFixtures(), withheldLeakQuotedByChatFixtures(),
		withheldLeakInCodexRemoteFixtures(t))
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			shown := outputsOf(t, fixture.files)
			withheld := outputsOf(t, fixture.files, "--no-secrets")
			if !strings.Contains(shown.all(), fixture.secret) {
				t.Fatalf("the default run does not show %q:\n%s", fixture.secret, shown.all())
			}
			if strings.Contains(withheld.all(), fixture.secret) {
				t.Errorf("the withheld run writes %q:\n%s", fixture.secret, withheld.all())
			}
			shownCounts := strings.Join(countLines.FindAllString(shown.summary, -1), "\n")
			if withheldCounts := strings.Join(countLines.FindAllString(withheld.summary, -1), "\n"); withheldCounts !=
				shownCounts {
				t.Errorf("the withheld run counts\n%s\nthe default run\n%s", withheldCounts, shownCounts)
			}
			for _, pair := range [][2]string{{shown.rows, withheld.rows}, {shown.sessions, withheld.sessions}} {
				if strings.Count(pair[0], "\n") != strings.Count(pair[1], "\n") {
					t.Errorf("the withheld run wrote %d lines where the default run wrote %d",
						strings.Count(pair[1], "\n"), strings.Count(pair[0], "\n"))
				}
			}
		})
	}
}

func TestAReadersDetailIsReplacedOnlyInItsOwnParse(t *testing.T) {
	const phrase = "unexpected end of JSON input"
	transcript := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-10-01T00:00:00Z",` +
		`"cwd":"/srv/proj","message":{"role":"user","content":"we saw ` + phrase + ` again"}}` + "\n"
	files := []collectedFile{
		{"alice", "/home/alice", "crush", ".config/crush/crush.json",
			`{"providers": {"openai": {"api_key": "fake-secret-value-0001"`},
		{"alice", "/home/alice", "claude-code",
			".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl", transcript}}
	withheld := outputsOf(t, files, "--no-secrets")
	if !strings.Contains(withheld.rows, "we saw "+phrase+" again") {
		t.Errorf("another parse's row lost the reader's wording:\n%s", withheld.rows)
	}
	for _, line := range strings.Split(withheld.rows, "\n") {
		if strings.Contains(line, `"agent":"crush"`) && strings.Contains(line, phrase) {
			t.Errorf("the reader's detail stands in its own parse's row: %s", line)
		}
	}
}

func TestAShortKeyIsReplacedOnlyInTheTextOfItsParsesRows(t *testing.T) {
	files := []collectedFile{{"alice", "/home/alice", "crush", ".config/crush/crush.json",
		`{"providers": {"openai": {"api_key": "1"}}}`}}
	withheld := outputsOf(t, files, "--no-secrets")
	// The time, the host (h1) and every other field keep their 1s: the
	// one placeholder is the key's, in the text.
	if !strings.Contains(withheld.rows, `"host":"h1"`) || strings.Count(withheld.rows, "REDACTED") != 1 ||
		!strings.Contains(withheld.rows, "api_key=REDACTED") {
		t.Errorf("the withheld run wrote\n%s", withheld.rows)
	}
	if strings.Contains(withheld.sessions+withheld.inventory, "REDACTED") {
		t.Errorf("the key is replaced outside the rows' text:\n%s\n%s", withheld.sessions, withheld.inventory)
	}
}

func TestAnOptionsCredentialsAreReplacedInTheErrorItGives(t *testing.T) {
	const url = "https://u:fake-secret-value-0001@h.example/"
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	work := t.TempDir()
	input, output := filepath.Join(work, "missing"), filepath.Join(work, "out")
	tests := []struct {
		name, prefix string
		args         []string
	}{
		{"a pattern that does not compile", "error: ", []string{"timeline", "--match", url + "(", input, output}},
		{"an agent no parser reads", "error: ", []string{"timeline", "--agent", url, input, output}},
		{"a time that cannot be read", "error: ", []string{"timeline", "--since", url, input, output}},
		{"an argument after timeline's output", "doubleagent timeline: unexpected argument: ",
			[]string{"timeline", input, output, url}},
		{"an option after timeline's input", "doubleagent timeline: options go before the input; ",
			[]string{"timeline", input, "-" + url, output}},
		{"an argument after detect's input", "doubleagent detect: unexpected argument: ",
			[]string{"detect", input, url}},
		{"an option after detect's input", "doubleagent detect: options go before the input; ",
			[]string{"detect", input, "-" + url}},
		{"a timeline input path holding a URL", "error: ", []string{"timeline", work + "/" + url + "x", output}},
		{"a detect input path holding a URL", "error: ", []string{"detect", work + "/" + url + "x"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withheld := append([]string{test.args[0], "--no-secrets"}, test.args[1:]...)
			code, _, stderr := runCommand(env, withheld...)
			if code != ExitError || !strings.HasPrefix(stderr, test.prefix) {
				t.Fatalf("exited %d: %q", code, stderr)
			}
			if strings.Contains(stderr, "fake-secret-value-0001") {
				t.Errorf("the error shows the credential: %q", stderr)
			}
			if _, _, shown := runCommand(env, test.args...); !strings.Contains(shown, "fake-secret-value-0001") {
				t.Errorf("without --no-secrets the error does not show the argument as typed: %q", shown)
			}
		})
	}
}

func TestAHermesHeaderThatNamesNoCredentialIsKeptInOtherRows(t *testing.T) {
	files := []collectedFile{
		{"alice", "/home/alice", "hermes", ".config/Hermes/connections.json", `{"connections": [{"kind": "remote", ` +
			`"label": "work", "url": "https://hermes.example/", "headers": {"encoding": "plain", "value": ` +
			`{"Authorization": "Bearer fake-hermes-tok-0003", "Content-Type": "application/json"}}}]}`},
		{"alice", "/home/alice", "claude-code", ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
			quoting("secrets: fake-hermes-tok-0003 | application/json")}}
	withheld := outputsOf(t, files, "--no-secrets")
	if !strings.Contains(withheld.rows, `"secrets: REDACTED | application/json"`) ||
		strings.Contains(withheld.all(), "fake-hermes-tok-0003") {
		t.Errorf("the withheld rows are\n%s", withheld.rows)
	}
}

// timelineOf writes files as a collection whose collection.json lists
// projects and runs timeline over it with options, returning
// timeline.jsonl and inventory.json.
func timelineOf(t *testing.T, projects []string, files []collectedFile, options ...string) (string, string) {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1")), member("projects", stringArray(projects))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	if code, _, stderr := runCommand(env, append(append([]string{"timeline"}, options...), collection,
		output)...); code > ExitNothing {
		t.Fatalf("timeline exited %d: %s", code, stderr)
	}
	var written []string
	for _, name := range []string{timelineFile, inventoryFile} {
		data, err := osfile.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		written = append(written, string(data))
	}
	return written[0], written[1]
}

func TestACollectedProjectsURLCredentialsAreReplacedInEveryString(t *testing.T) {
	files := []collectedFile{{"alice", "/home/alice", "claude-code",
		".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl",
		quoting("the pass is fake-secret-value-0001 and fake-secret-value-0002")}}
	projects := []string{"/srv/https://carol:fake-secret-value-0001@h.example/x",
		"C:/w/https://u:fake-secret-value-0002@h/x"}
	rows, inventory := timelineOf(t, projects, files, "--no-secrets")
	if strings.Contains(rows+inventory, "fake-secret-value-") {
		t.Errorf("the withheld run writes a project's credential:\n%s\n%s", rows, inventory)
	}
}

func TestInventoryProjectsKeepTheirOrderWhenWithheld(t *testing.T) {
	projects := []string{"/p/https://z:Z@h", "/p/https://b@h", "/p/https://y:Y@h"}
	keys := func(inventory string) []string {
		document, err := jsonvalue.DecodeString(inventory)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range document.Get("projects").Members() {
			names = append(names, entry.Key)
		}
		return names
	}
	files := []collectedFile{{"alice", "/home/alice", "claude-code",
		".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl", quoting("hello")}}
	_, shown := timelineOf(t, projects, files)
	_, withheld := timelineOf(t, projects, files, "--no-secrets")
	if got := keys(shown); !slices.Equal(got, []string{"/p/https://b@h", "/p/https://y:Y@h", "/p/https://z:Z@h",
		"/srv/proj"}) {
		t.Fatalf("the default run's keys are %q", got)
	}
	// The withheld run's two replaced keys are one name, which a reader
	// keeps once; the name kept as written still comes first.
	user, replaced := strings.Index(withheld, `"/p/https://b@h": {`), strings.Index(withheld, `"/p/https://REDACTED@h": {`)
	if user < 0 || replaced < user || strings.Count(withheld, `"/p/https://REDACTED@h": {`) != 2 {
		t.Errorf("the withheld run's keys are in another order:\n%s", withheld)
	}
}
