package cursor

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// The expected values in this file come from running the Python helpers
// of the same names on the same inputs.

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
		{"git@github.com:owner/demo", "demo"}, {"ssh://git@github.com/owner/demo", "owner/demo"},
		{"https://user:pw@GitHub.com:443/o/r/?x#y", "o/r"}, {"HTTPS://github.com/o/r", "o/r"},
		{"1x://github.com/o/r", ""}, {"github.com:22/o/r", "o/r"}, {"//github.com/o/r", ""},
		{"  github.com/o/r", ""}, {"gith\tub.com/o/r", "o/r"}, {"github.com/o/r.git/", "o/r"},
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
		{"W2FdIGI=", "a", "b"}, {"W2FdIGI", "a", "b"}, {"W2EgYl0gYwo=", "a b", "c"},
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

func TestURIPathWritesDrivePathsWithBackslashes(t *testing.T) {
	for uri, want := range map[string]string{
		"file:///c%3A/Users/alice/proj/demo": "c:\\Users\\alice\\proj\\demo",
		"file:///home/alice/my%20proj":       "/home/alice/my proj",
	} {
		if got := uriPath(uri); got != want {
			t.Errorf("uriPath(%q) = %q, want %q", uri, got, want)
		}
	}
}

func FuzzLenientBase64(fuzz *testing.F) {
	fuzz.Add("W3Nydjo6eF0gbWNwX3NlcnZlcl91cmw")
	fuzz.Fuzz(func(t *testing.T, text string) {
		_, _, _ = mcpKeyName("p."+text, "p.")
		_ = githubRepository(text)
		_ = tagTime(strings.ToValidUTF8(text, ""))
	})
}
