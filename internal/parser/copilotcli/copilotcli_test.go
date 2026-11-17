package copilotcli

import (
	"maps"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows of every file kind are proved against the Python analyzer by the
// parser replay (internal/parity/replay), over the recorded fixtures; these
// tests port what the Python tests call directly. Expected values come from
// running the analyzer's functions.

// decoded returns the value of JSON text.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestArgsTextPrefersTheFirstKnownArgument(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`"raw"`, "raw"},
		{`{"command":"ls","x":1}`, "ls"},
		{`{"url":"u"}`, "u"},
		{`{"x":1,"b":[2]}`, `{"x":1,"b":[2]}`},
		{`{}`, ""},
		{`{"input":0}`, "0"},
		{`null`, ""},
		{`5`, "5"},
		{`[1]`, "[1]"},
		{`{"command":""}`, `{"command":""}`},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := argsText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultTextTakesTheLongerResultAndTheFailure(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"result":{"content":"a","detailedContent":"a b c"}}`, "a b c"},
		{`{"result":{"content":"xxxxx","detailedContent":"s"}}`, "xxxxx"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":0}}`, "ok"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":2}}`, "[exit 2] ok"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":"1"}}`, "[exit 1] ok"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":true}}`, "ok"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":"0"}}`, "ok"},
		{`{"result":{"content":"ok"},"shellExecution":{"exitCode":1.5}}`, "[exit 1.5] ok"},
		{`{"success":false,"error":{"message":"bad"},"result":{"content":"ok"}}`, "ok\nbad"},
		{`{"success":false,"error":{"message":"bad"}}`, "bad"},
		{`{"success":false}`, ""},
		{`{"result":{"content":{"a":1}}}`, `{"a":1}`},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := resultText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestRequestTargetNamesWhatThePermissionAsksAbout(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"serverName":"web","toolName":"fetch"}`, "web/fetch"},
		{`{"kind":"memory"}`, ""},
		{`{"toolName":"t"}`, "t"},
		{`{"serverName":"s"}`, "s/"},
		{`{"url":"u","path":"p"}`, "u"},
		{`{"fullCommandText":"","path":"p"}`, "p"},
		{`{"path":0}`, "0"},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := requestTarget(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDecisionTextAppendsOnlyTheResultKeysThatHaveAValue(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"result":{"kind":"approved"},"decisionSource":"user"}`, "permission decision: approved source=user"},
		{`{"result":{}}`, "permission decision: - source=-"},
		{`{"result":{"kind":"x","approval":{"a":1},"rules":[],"directories":["d"],"feedback":"","reason":0,"path":"p"}}`,
			`permission decision: x source=- approval={"a":1} directories=["d"] reason=0 path=p`},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := decisionText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestHookInfoAndShutdownTexts(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"hook start", hookText("hook.start", decoded(t, `{"hookType":"pre","input":{"b":1,"a":2}}`)),
			`hook pre start: {"b":1,"a":2}`},
		{"hook start without input", hookText("hook.start", decoded(t, `{}`)), "hook - start: {}"},
		{"hook end with an error", hookText("hook.end", decoded(t, `{"success":true,"output":null,"error":{"m":1}}`)),
			`hook - end: success=true output={} error={"m":1}`},
		{"hook end with a success that is not a boolean", hookText("hook.end", decoded(t, `{"success":"yes"}`)),
			"hook - end: success=- output={}"},
		{"info", infoText(decoded(t, `{"infoType":"t","message":"m","url":"u","tip":""}`)),
			"session.info t: m url=u"},
		{"shutdown with files", shutdownText(decoded(t,
			`{"shutdownType":"x","codeChanges":{"filesModified":["a","b"]},"errorReason":"e"}`)),
			"session end: x e files modified: a, b"},
		{"shutdown with no files", shutdownText(decoded(t, `{"codeChanges":{"filesModified":[]}}`)),
			"session end: - files modified: none"},
		{"shutdown with a text of files", shutdownText(decoded(t, `{"codeChanges":{"filesModified":"z"}}`)),
			"session end: - files modified: z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestStrippedSettingsKeepStringsAndTrailingCommasOfJSONSpaceOnly(t *testing.T) {
	for _, space := range []string{" ", "\t", "\n", "\r\n", " // c\n", " /* c */ "} {
		text := `{"a": [1,` + space + `],` + space + `}`
		document, err := jsonvalue.DecodeString(parser.StripJSONC(text))
		if err != nil || document.JSON() != `{"a":[1]}` {
			t.Errorf("%q gave %v, %v", text, document, err)
		}
	}
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		text := "[1," + space + "]"
		if got := parser.StripJSONC(text); got != text {
			t.Errorf("%q became %q", text, got)
		}
	}
	text := `{"x": "a, ]", "y": "b,}"}`
	if got := parser.StripJSONC(text); got != text {
		t.Errorf("%q became %q", text, got)
	}
	document, err := jsonvalue.DecodeString(parser.StripJSONC("{\"a\": \"http://x\", /* c */ \"b\": [1,], // d\n}"))
	if err != nil || document.JSON() != `{"a":"http://x","b":[1]}` {
		t.Errorf("got %v, %v", document, err)
	}
}

func TestFlatYAMLReadsTopLevelScalarsAsWrittenByTheApp(t *testing.T) {
	tests := []struct {
		name string
		text string
		want map[string]string
	}{
		{"no-break space and line separator belong to the value",
			"cwd: /srv/proj\u00a0\nsummary: a\u2028b\nname: n\n",
			map[string]string{"cwd": "/srv/proj\u00a0", "summary": "a\u2028b", "name": "n"}},
		{"a no-break space after a key belongs to the key", "cwd\u00a0: /srv/proj\n",
			map[string]string{"cwd\u00a0": "/srv/proj"}},
		{"quotes, nulls and CRLF", "id: x\r\ncwd: '/srv'\r\nsummary: ~\r\n",
			map[string]string{"id": "x", "cwd": "/srv", "summary": ""}},
		{"quoted values", "q: \"a\\\"b\"\nr: \"bad\ns: 'it''s'\nt: null\n",
			map[string]string{"q": `a"b`, "r": "bad", "s": "it's", "t": ""}},
		{"nested, list, comment and indented lines are skipped",
			"v:\n  nested: 1\n- item\n# c\n w: 1\nx: y # c\nz: \"ok\" # c\n",
			map[string]string{"v": "", "x": "y # c", "z": `ok" # c`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace.yaml")
			if err := osfile.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			var problems parser.Problems
			if got := readFlatYAML(path, &problems); !maps.Equal(got, test.want) || len(problems) != 0 {
				t.Errorf("got %q and %v, want %q", got, problems, test.want)
			}
		})
	}
}

func TestLogLinePatternsTakeSpaceAndTabAndASCIIDigitsOnly(t *testing.T) {
	subject := New()
	login := subject.signedIn.FindStringSubmatch("Signed in to https://github.com as octo\u00a0 ")
	if login == nil || login[1] != "https://github.com" || login[2] != "octo\u00a0" {
		t.Errorf("login %q", login)
	}
	if login := subject.signedIn.FindStringSubmatch("Signed in to h as a b.  "); login == nil || login[2] != "a b" {
		t.Errorf("login %q", login)
	}
	tests := []struct {
		line string
		want bool
	}{
		{"2026-10-01T08:59:59.400Z [INFO] x", true},
		{"2026-10-01T08:59:59.400Z\t", true},
		{"2026-10-01T08:59:59.400Z\u00a0[INFO] x", false},
		{"2026-10-01T08:59:59.400\u00a0Z [INFO] x", false},
		{"2026-10-01T08h59:59Z [INFO] x", false},
		{"\u0661026-10-01T08:59:59.400Z [INFO] x", false},
	}
	for _, test := range tests {
		if got := subject.logTime.MatchString(test.line); got != test.want {
			t.Errorf("%q gave %v, want %v", test.line, got, test.want)
		}
	}
}

func TestWantsEachFileInEachCopilotHome(t *testing.T) {
	const session = "0123456789abcdef0123456789abcdef0123"
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
		tolerance float64
	}{
		{".copilot/session-state/" + session + "/events.jsonl", true, false, 2},
		{".local/state/.copilot/session-state/" + session + "/workspace.yaml", true, false, 2},
		{".config/.copilot/session-state/" + session + "/rewind-file-snapshots/index.json", true, true, 3600},
		{".copilot/session-store.db", true, true, 3600},
		{".copilot/command-history-state.json", true, true, 2},
		{".copilot/permissions-config.json", true, false, 2},
		{".copilot/settings.json", true, false, 2},
		{".copilot/sidebar-sessions-state/" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.json", true, false, 2},
		{".copilot/logs/process-1-2.log", true, false, 2},
		{".copilot/logs/process-1.log", false, false, 2},
		{".copilot/session-state/short/events.jsonl", false, false, 2},
		{"x/.copilot/settings.json", false, false, 2},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || subject.Secondary(artifact) != test.secondary ||
				subject.SecondaryTolerance(artifact) != test.tolerance {
				t.Errorf("wants %v secondary %v tolerance %v", subject.Wants(artifact),
					subject.Secondary(artifact), subject.SecondaryTolerance(artifact))
			}
		})
	}
}
