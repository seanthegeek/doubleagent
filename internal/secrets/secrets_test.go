package secrets

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
)

// withheld returns the setting of a run with --no-secrets and the default
// placeholder.
func withheld() Setting {
	return Setting{Withhold: true, Placeholder: DefaultPlaceholder}
}

func TestURLsReplacesOnlyTheCredentialsOfEachURL(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"a user name and password", "https://user:pass@host.example:8443/api?x=1",
			"https://REDACTED@host.example:8443/api?x=1"},
		{"a token as the user name", "https://ghp_TOKEN@github.com/org/repo.git",
			"https://REDACTED@github.com/org/repo.git"},
		{"an installation token and its user", "https://x-access-token:ghs_abc@github.com/o/r.git",
			"https://REDACTED@github.com/o/r.git"},
		{"a user name before an IPv6 host", "http://tok@[::1]:8080/x", "http://REDACTED@[::1]:8080/x"},
		{"a password and an IPv6 host", "http://u:p@[2001:db8::1]/x", "http://REDACTED@[2001:db8::1]/x"},
		{"an empty password", "https://user:@host/x", "https://REDACTED@host/x"},
		{"a password holding an at sign", "http://u:p@ss@host/", "http://REDACTED@host/"},
		{"a password holding a percent-escape", "https://u:p%40s%2Fs@host/x", "https://REDACTED@host/x"},
		{"a password holding a colon", "https://u:a:b@host/x", "https://REDACTED@host/x"},
		{"a password holding slashes", "https://AKIA:wJalr/K7MDENG/bPxRf@host/x", "https://REDACTED@host/x"},
		{"a password holding a question mark", "https://x-access-token:ab?cd@github.com/o/r",
			"https://REDACTED@github.com/o/r"},
		{"a password holding a hash", "https://user:p#ss@host/x", "https://REDACTED@host/x"},
		{"a password holding a slash then two at signs", "https://u:p/x@y@host/@path", "https://REDACTED@host/@path"},
		{"a password holding a scheme separator", "https://u:ab://c@host/x", "https://REDACTED@host/x"},
		{"a URL net/url refuses", "https://tok:se%ZZcret@host/", "https://REDACTED@host/"},
		{"a URL with a user name and another scheme", "ssh://git@github.com/o/r.git",
			"ssh://REDACTED@github.com/o/r.git"},
		{"an at sign in the path is kept", "https://host.example/users/@alice", "https://host.example/users/@alice"},
		{"an at sign after a colon in the path is kept", "https://host/a:b/c@d", "https://host/a:b/c@d"},
		{"an at sign in the query is kept", "https://host.example?mail=a@b", "https://host.example?mail=a@b"},
		{"an at sign in the fragment is kept", "https://host.example/#a:b@c", "https://host.example/#a:b@c"},
		{"a port and an at sign in the path are kept", "http://localhost:5173/@vite/client",
			"http://localhost:5173/@vite/client"},
		{"a dotted host's port and an at sign in the path are kept", "https://host.example:80/users/@name",
			"https://host.example:80/users/@name"},
		{"a single-label host with a port and an at sign in the path is withheld", "http://devbox:8080/@x",
			"http://REDACTED@x"},
		{"an IPv6 host with an at sign in its path is kept", "http://[::1]:80/a@b", "http://[::1]:80/a@b"},
		{"every URL in a text", "from https://a:b@one/x to http://c@two/y",
			"from https://REDACTED@one/x to http://REDACTED@two/y"},
		{"a URL in a command line", "git clone https://oauth2:glpat-xyz@gitlab.com/o/r.git dir",
			"git clone https://REDACTED@gitlab.com/o/r.git dir"},
		{"a URL in JSON text", `{"url":"https://u:p@h/x","next":"me@example.com"}`,
			`{"url":"https://REDACTED@h/x","next":"me@example.com"}`},
		{"a URL inside the query of another", "https://h/a?next=https://u:p@i/",
			"https://h/a?next=https://REDACTED@i/"},
		{"a password without a scheme", "user:secret@proxy.example:3128", "REDACTED@proxy.example:3128"},
		{"a password without a scheme after an option", "--proxy=user:secret@proxy:3128",
			"--proxy=REDACTED@proxy:3128"},
		{"a password without a scheme in brackets", "(user:secret@proxy)", "(REDACTED@proxy)"},
		{"the SCP form of a git remote is kept", "git@github.com:o/r.git", "git@github.com:o/r.git"},
		{"a mail address is kept", "alice@host", "alice@host"},
		{"a mailto address is kept", "mailto:alice@host.example", "mailto:alice@host.example"},
		{"a token in the query", "https://host/share/session?token=abc123", "https://host/share/session?token=REDACTED"},
		{"an API key in the query beside other parameters", "https://h/v1?alt=json&key=AIza123&x=1",
			"https://h/v1?alt=json&key=REDACTED&x=1"},
		{"a parameter name in another case", "https://h/?Access_Token=abc#x", "https://h/?Access_Token=REDACTED#x"},
		{"a token in the fragment", "https://h/cb#access_token=abc&token_type=bearer",
			"https://h/cb#access_token=REDACTED&token_type=bearer"},
		{"credentials and a query token together", "https://u:p@h/x?token=t#f", "https://REDACTED@h/x?token=REDACTED#f"},
		{"a parameter whose name only holds a credential name is kept", "https://h/?tokens=1&monkey=2",
			"https://h/?tokens=1&monkey=2"},
		{"an empty credential value is kept", "https://h/?token=&x=1", "https://h/?token=&x=1"},
		{"a query outside a URL is kept", "a?token=b", "a?token=b"},
		{"a URL without credentials is unchanged", "http://localhost:8080", "http://localhost:8080"},
		{"a host string is unchanged", "api.example.com:443", "api.example.com:443"},
		{"empty text is unchanged", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := withheld().URLs(test.text); got != test.want {
				t.Errorf("withheld: got %q, want %q", got, test.want)
			}
			if got := (Setting{}).URLs(test.text); got != test.text {
				t.Errorf("shown: got %q, want the text as written", got)
			}
		})
	}
}

func TestAnotherPlaceholderStandsWhereTheCredentialsWere(t *testing.T) {
	tests := []struct {
		name, placeholder, text, want string
	}{
		{"a named placeholder", "[secret]", "https://u:p@h/x?key=k", "https://[secret]@h/x?key=[secret]"},
		{"an empty placeholder drops the at sign", "", "https://x-access-token:ghs_abc@github.com/o/r.git",
			"https://github.com/o/r.git"},
		{"an empty placeholder leaves a query value empty", "", "https://h/?token=abc&x=1", "https://h/?token=&x=1"},
		{"an empty placeholder without a scheme", "", "user:secret@proxy:3128", "proxy:3128"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting := Setting{Withhold: true, Placeholder: test.placeholder}
			if got := setting.URLs(test.text); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestValueIsShownOrReplaced(t *testing.T) {
	tests := []struct {
		name    string
		setting Setting
		value   string
		want    string
	}{
		{"shown by default", Setting{}, "sk-live-123", "sk-live-123"},
		{"replaced when withheld", withheld(), "sk-live-123", "REDACTED"},
		{"replaced by another placeholder", Setting{Withhold: true, Placeholder: "***"}, "sk-live-123", "***"},
		{"replaced by an empty placeholder", Setting{Withhold: true}, "sk-live-123", ""},
		{"an empty value stays empty", withheld(), "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.setting.Value(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// readerErrors returns the errors of a JSON, a TOML and an XML document
// that quote a secret, and of a TOML document refused for its size.
func readerErrors(t *testing.T) (jsonError, tomlError, xmlError, limitError error) {
	t.Helper()
	_, jsonError = jsonvalue.DecodeString(`{"api_key": sk-live-123}`)
	_, tomlError = tomldoc.Parse("api_key = sk-live-123\n")
	decoder := xml.NewDecoder(strings.NewReader("<a>\n<sk-live-123></b>"))
	for xmlError == nil {
		_, xmlError = decoder.Token()
	}
	_, limitError = tomldoc.Parse(strings.Repeat("#", 64<<20) + "\n")
	for _, err := range []error{jsonError, tomlError, xmlError} {
		if !quotesTheDocument(err.Error()) {
			t.Fatalf("the reader's error %q does not quote the document", err)
		}
	}
	return jsonError, tomlError, xmlError, limitError
}

func TestReaderErrorGivesThePositionAndThePlaceholderWhenWithheld(t *testing.T) {
	jsonError, tomlError, xmlError, limitError := readerErrors(t)
	var limit *tomldoc.Error
	if !errors.As(limitError, &limit) || !limit.IsLimit() {
		t.Fatalf("a large document gave %v, not a limit", limitError)
	}
	tests := []struct {
		name    string
		setting Setting
		err     error
		want    string
	}{
		{"a JSON error is in full by default", Setting{}, jsonError, jsonError.Error()},
		{"a JSON error is its position when withheld", withheld(), jsonError,
			"invalid JSON at line 1, column 13: REDACTED"},
		{"a TOML error is in full by default", Setting{}, tomlError, tomlError.Error()},
		{"a TOML error is its position when withheld", withheld(), tomlError,
			"invalid TOML at line 1, column 11: REDACTED"},
		{"an XML error is its line when withheld", withheld(), xmlError, "invalid XML at line 2: REDACTED"},
		{"another placeholder follows the position", Setting{Withhold: true, Placeholder: "***"}, jsonError,
			"invalid JSON at line 1, column 13: ***"},
		{"an empty placeholder leaves the position alone", Setting{Withhold: true}, tomlError,
			"invalid TOML at line 1, column 11"},
		{"a TOML limit is in full when withheld", withheld(), limitError, limitError.Error()},
		{"an error of another kind is the placeholder", withheld(), errors.New("sk-live-123"), "error: REDACTED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.setting.ReaderError(test.err)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
			if test.setting.Withhold && quotesTheDocument(got) {
				t.Errorf("the withheld message %q quotes the document", got)
			}
		})
	}
}

// quotesTheDocument reports whether a reader's message quotes the
// documents of readerErrors as each reader quotes them: the JSON reader
// the secret's first character, the TOML reader its first word, the XML
// reader the element named after it.
func quotesTheDocument(message string) bool {
	return strings.Contains(message, "'s'") || strings.Contains(message, `"sk"`) ||
		strings.Contains(message, "sk-live")
}

// FuzzURLsWithholdsBothSecrets holds that for
// scheme://user:SECRET@host/path?token=SECRET2#fragment the withheld text is
// that URL with REDACTED (or nothing) in place of both secrets, so neither
// survives, and that with withholding off the text is returned byte for
// byte. A secret holds any printable text without the delimiters that end a
// URL in text (white space, quotes, angle brackets, a backtick, a
// backslash); SECRET2 also holds no & or #, which end a parameter's value.
// A SECRET holding an @ followed later by /, ? or # is left out: it reads
// as user information ending at that @ and a path (userInfoEnd).
func FuzzURLsWithholdsBothSecrets(fuzz *testing.F) {
	for _, seed := range [][2]string{{"s3cret", "t0ken"}, {"tok/en+abc=", "a?b"}, {"ab?cd", "x/y:z"},
		{"p#ss", "@@"}, {"a@b", "://"}, {"x:y", "%41"}, {"1234/5678", "u:p@h"}, {"%40/?#:", "é"},
		{"ab://c", "https://u:p@h/?token=q"}} {
		fuzz.Add("https", seed[0], "host.example", "/path", seed[1], "frag")
	}
	fuzz.Fuzz(func(t *testing.T, scheme, secret, host, path, secret2, fragment string) {
		if !onlyOf(scheme, "abcdefghijklmnopqrstuvwxyz", 1) ||
			!onlyOf(host, "abcdefghijklmnopqrstuvwxyz0123456789.-", 1) ||
			!onlyOf(path, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._~-", 0) ||
			!onlyOf(fragment, "abcdefghijklmnopqrstuvwxyz0123456789", 0) ||
			!printable(secret, "") || !printable(secret2, "&#") || len(secret) < 4 || len(secret2) < 4 {
			return
		}
		if atSign := strings.IndexByte(secret, '@'); atSign >= 0 && strings.ContainsAny(secret[atSign:], "/?#") {
			return
		}
		text := scheme + "://user:" + secret + "@" + host + "/" + path + "?token=" + secret2 + "#" + fragment
		if got := (Setting{}).URLs(text); got != text {
			t.Fatalf("shown: %q gave %q", text, got)
		}
		for _, placeholder := range []string{DefaultPlaceholder, ""} {
			want := scheme + "://" + withAtSign(placeholder) + host + "/" + path + "?token=" + placeholder +
				"#" + fragment
			got := Setting{Withhold: true, Placeholder: placeholder}.URLs(text)
			if got != want {
				t.Errorf("withheld with %q: %q gave %q, want %q", placeholder, text, got, want)
			}
		}
	})
}

// onlyOf reports whether text holds at least least bytes, each one of
// allowed.
func onlyOf(text, allowed string, least int) bool {
	return len(text) >= least && strings.Trim(text, allowed) == ""
}

// printable reports whether text is valid UTF-8 of printable characters
// holding no URL delimiter and none of the bytes in banned.
func printable(text, banned string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, character := range text {
		if !unicode.IsPrint(character) || character < utf8.RuneSelf &&
			(isDelimiter(byte(character)) || strings.ContainsRune(banned, character)) {
			return false
		}
	}
	return true
}
