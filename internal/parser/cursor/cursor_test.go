package cursor

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every store are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the helpers that read a field's own format.

func TestTagTimeReadsTheTranscriptsTimeTag(t *testing.T) {
	for _, test := range []struct{ tag, want string }{
		{"Thursday, Oct 1, 2026, 6:00 AM (UTC-4)", "2026-10-01T10:00:00.000Z"},
		{"Thursday, Oct 1, 2026, 12:05 AM (UTC)", "2026-10-01T00:05:00.000Z"},
		{"Thursday, Oct 1, 2026, 12:05 PM (UTC+5:30)", "2026-10-01T06:35:00.000Z"},
		{"Wednesday, Sep 30, 2026, 9:30 PM (UTC-4)", "2026-10-01T01:30:00.000Z"},
		{"yesterday", ""},
		{"Thursday, Foo 1, 2026, 6:00 AM (UTC-4)", ""},
		{"Thu, Feb 30, 2026, 1:00 AM (UTC)", ""},
		{"Thu, Oct. 1, 2026, 1:60 AM (UTC)", ""},
		{"Thu, October 1, 2026, 13:05 pm (UTC+0530)", "2026-10-01T07:35:00.000Z"},
		{"Thu, Oct 1, 0000, 1:00 AM (UTC)", ""},
		{" Thu,  Oct 1,  2026,  1:00am(UTC+1)  ", "2026-10-01T00:00:00.000Z"},
		{"Thu, Feb 29, 2028, 11:59 PM (UTC-12)", "2028-03-01T11:59:00.000Z"},
		{"Thursday, Oct 1, 2026, 6:00\u202fAM (UTC-4)", "2026-10-01T10:00:00.000Z"},
		{"Thursday, Oct 1, 2026, 6:00\u00a0AM (UTC-4)", "2026-10-01T10:00:00.000Z"},
	} {
		t.Run(test.tag, func(t *testing.T) {
			if got := tagTime(test.tag); got != test.want {
				t.Errorf("tagTime(%q) = %q, want %q", test.tag, got, test.want)
			}
		})
	}
}

func TestUnwrapQueryTakesTheQueryAndItsTime(t *testing.T) {
	for _, test := range []struct{ text, query, when string }{
		{"<timestamp>Thursday, Oct 1, 2026, 6:00 AM (UTC-4)</timestamp>\n<user_query>a\nb</user_query>",
			"a\nb", "2026-10-01T10:00:00.000Z"},
		{"<timestamp>soon</timestamp>\n<user_query>q</user_query>",
			"<timestamp>soon</timestamp>\n<user_query>q</user_query>", ""},
		{"<timestamp>Thursday, Oct 1, 2026, 6:00 AM (UTC-4)</timestamp>\u00a0<user_query>q</user_query>",
			"<timestamp>Thursday, Oct 1, 2026, 6:00 AM (UTC-4)</timestamp>\u00a0<user_query>q</user_query>", ""},
	} {
		t.Run(test.text, func(t *testing.T) {
			if query, when := unwrapQuery(test.text); query != test.query || when != test.when {
				t.Errorf("unwrapQuery = %q, %q; want %q, %q", query, when, test.query, test.when)
			}
		})
	}
}

func TestGithubRepositoryComparesTheParsedHost(t *testing.T) {
	for _, test := range []struct{ url, want string }{
		{"github.com/owner/demo", "owner/demo"}, {"https://github.com/owner/demo", "owner/demo"},
		{"https://github.com/owner/demo/", "owner/demo"}, {"github.com", ""},
		{"origin.cursor.com/git/ns/tmp-0123456789abcdef", ""}, {"github.com.example.net/owner/demo", ""},
		{"example.net/github.com/owner/demo", ""}, {"https://example.net/?u=github.com/owner/demo", ""},
		{"", ""}, {"https://github.com/o/r.git", "o/r"}, {"https://user:pw@github.com/o/r", "o/r"},
		{"https://github.com:443/o/r", "o/r"}, {"notgithub.com/o/r", ""}, {"https://github.com@evil.example/o/r", ""},
		{"git@github.com:owner/demo", ""}, {"ssh://git@github.com/owner/demo", "owner/demo"},
		{"https://user:pw@GitHub.com:443/o/r/?x#y", "o/r"}, {"HTTPS://github.com/o/r", "o/r"},
		{"1x://github.com/o/r", ""}, {"github.com:22/o/r", "o/r"}, {"//github.com/o/r", ""},
		{"  github.com/o/r", ""}, {"gith\tub.com/o/r", ""}, {"github.com/o/r.git/", "o/r"},
	} {
		t.Run(test.url, func(t *testing.T) {
			if got := githubRepository(test.url); got != test.want {
				t.Errorf("githubRepository(%q) = %q, want %q", test.url, got, test.want)
			}
		})
	}
}

func TestMCPKeyNameDecodesTheKeysBase64(t *testing.T) {
	for _, test := range []struct{ suffix, scope, field string }{
		{"W3Nydjo6eF0gbWNwX3NlcnZlcl91cmw", "srv::x", "mcp_server_url"},
		{"W2FdIGI=", "a", "b"}, {"W2FdIGI", "a", "b"}, {"W2EgYl0gYw", "a b", "c"}, {"W2EgYl0gYwo=", "", ""},
		{"W2FdIGIgYw==", "", ""}, {"W8OpXSBm", "\u00e9", "f"}, {"W2FdIP8=", "", ""},
		{"W2Fd!IGI", "", ""}, {"W2FdIGI=junk", "", ""}, {"abc", "", ""}, {"W2Fd IGI", "", ""},
		{"W3hdXSB5XSB6", "x]] y", "z"}, {"", "", ""}, {"\u00e9", "", ""},
	} {
		t.Run(test.suffix, func(t *testing.T) {
			scope, field, ok := mcpKeyName("p."+test.suffix, "p.")
			if scope != test.scope || field != test.field || ok != (test.scope != "") {
				t.Errorf("mcpKeyName = %q, %q, %v; want %q, %q", scope, field, ok, test.scope, test.field)
			}
		})
	}
}

func TestRunModeNamesTheUIsLabel(t *testing.T) {
	for _, test := range []struct {
		mode    string
		sandbox bool
		want    string
	}{
		{`{"autoRun": false}`, true, "Ask Every Time"},
		{`{"autoRun": true}`, false, "Allowlist"},
		{`{"autoRun": true}`, true, "Allowlist (with Sandbox)"},
		{`{"autoRun": true, "smartModeAutoRun": true}`, false, "Auto-review"},
		{`{"autoRun": true, "smartModeAutoRun": true}`, true, "Auto-review (with Sandbox)"},
		{`{"autoRun": false, "fullAutoRun": true}`, true, "Run Everything"},
	} {
		t.Run(test.want, func(t *testing.T) {
			mode, _ := jsonvalue.DecodeString(test.mode)
			if got := runMode(mode, test.sandbox); got != test.want {
				t.Errorf("runMode(%s, %v) = %q, want %q", test.mode, test.sandbox, got, test.want)
			}
		})
	}
}

// A plugin scope's path is written as the run spells every project path:
// a drive or network path with backslashes, a POSIX path as it is.
func TestAPluginScopesPathIsSpelledAsTheRunSpellsAProjectPath(t *testing.T) {
	for _, test := range []struct{ name, uri, want string }{
		{"a drive path with an escaped colon", "file:///c%3A/Users/alice/proj/demo", "C:\\Users\\alice\\proj\\demo"},
		{"a drive path", "file:///c:/Users/alice/proj", "C:\\Users\\alice\\proj"},
		{"a drive written as the host", "file://C:/proj/demo", "C:\\proj\\demo"},
		{"localhost is this machine", "file://localhost/c:/proj", "C:\\proj"},
		{"a POSIX path keeps its slashes", "file:///home/alice/my%20proj", "/home/alice/my proj"},
		{"a network share keeps its server", "file://fileserver/share/proj",
			"\\\\fileserver\\share\\proj"},
		{"a server alone names no folder", "file://fileserver", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := timeline.CanonicalProjectPath(uriPath(test.uri)); got != test.want {
				t.Errorf("the path of %q is spelled %q, want %q", test.uri, got, test.want)
			}
		})
	}
}

func FuzzKeyNamesURLsAndTimeTags(fuzz *testing.F) {
	fuzz.Add("W3Nydjo6eF0gbWNwX3NlcnZlcl91cmw")
	fuzz.Fuzz(func(t *testing.T, text string) {
		_, _, _ = mcpKeyName("p."+text, "p.")
		_ = githubRepository(text)
		_ = tagTime(strings.ToValidUTF8(text, ""))
	})
}

func TestAChunkKeyNamesItsComposerByItsNumber(t *testing.T) {
	tests := []struct {
		name, key, want string
		ok              bool
	}{
		{"a plain id", "composerConvHeaders:a:000000", "a", true},
		{"an id holding a colon", "composerConvHeaders:a:b:000001", "a:b", true},
		{"an empty id", "composerConvHeaders::000000", "", true},
		{"a number that is not digits", "composerConvHeaders:a:b", "", false},
		{"no number", "composerConvHeaders:a:", "", false},
		{"no colon after the id", "composerConvHeaders:a", "", false},
		{"a number holding a sign", "composerConvHeaders:a:-1", "", false},
		{"another key", "bubbleId:a:000000", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := chunkComposer(test.key); got != test.want || ok != test.ok {
				t.Errorf("got %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
