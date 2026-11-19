package secrets

import (
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
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

// identified runs identify as one parse of a run with setting and returns
// text as the output writes it as the text of a row of that parse, and the
// secrets the parse recorded.
func identified(setting Setting, text string, identify func(recorder *Recorder)) (string, []string) {
	recorder := NewRecorder(setting)
	found := recorder.Begin()
	identify(recorder)
	recorder.End()
	return recorder.RowText(text, found), found.Secrets()
}

// urlText returns text as the output writes it with setting, and the
// credentials of its URLs, sorted, when the setting withholds.
func urlText(setting Setting, text string) (string, []string) {
	recorder := NewRecorder(setting)
	var found []string
	if recorder != nil {
		found = URLCredentials(text)
		slices.Sort(found)
	}
	return recorder.Text(text), found
}

func TestURLsIdentifiesOnlyTheCredentialsOfEachURL(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"a user name and password", "https://user:pass@host.example:8443/api?x=1",
			"https://REDACTED@host.example:8443/api?x=1"},
		{"a token as the user name", "https://ghp_FIXTURE@github.com/org/repo.git",
			"https://REDACTED@github.com/org/repo.git"},
		{"an installation token and its user", "https://x-access-token:ghs_abc@github.com/o/r.git",
			"https://REDACTED@github.com/o/r.git"},
		{"a short user name alone is kept", "ssh://git@github.com/o/r.git", "ssh://git@github.com/o/r.git"},
		{"a seven-byte user name alone is kept", "https://ci-user@h/x", "https://ci-user@h/x"},
		{"an eight-byte user name alone is withheld", "https://ci-users@h/x", "https://REDACTED@h/x"},
		{"a user name before an IPv6 host", "http://tok@[::1]:8080/x", "http://tok@[::1]:8080/x"},
		{"a password and an IPv6 host", "http://u:pw9@[2001:db8::1]/x", "http://REDACTED@[2001:db8::1]/x"},
		{"an empty password", "https://user:@host/x", "https://REDACTED@host/x"},
		{"the last at sign ends the user information", "http://u:p@ss@host/", "http://REDACTED@host/"},
		{"a password holding a percent-escape", "https://u:p%40s%2Fs@host/x", "https://REDACTED@host/x"},
		{"a password holding a colon", "https://u:a:b@host/x", "https://REDACTED@host/x"},
		{"a slash ends the authority, so what follows is a path",
			"https://AKIA:wJalr/K7MDENG/bPxRf@host/x", "https://AKIA:wJalr/K7MDENG/bPxRf@host/x"},
		{"a URL net/url refuses", "https://tok:se%ZZcret@host/", "https://REDACTED@host/"},
		{"an at sign in the path is kept", "https://host.example/users/@alice", "https://host.example/users/@alice"},
		{"an at sign after a colon in the path is kept", "https://host/a:b/c@d", "https://host/a:b/c@d"},
		{"an at sign in the query is kept", "https://host.example?mail=a@b", "https://host.example?mail=a@b"},
		{"an at sign in the fragment is kept", "https://host.example/#a:b@c", "https://host.example/#a:b@c"},
		{"every URL in a text", "from https://a:b@one/x to http://c:d@two/y",
			"from https://REDACTED@one/x to http://REDACTED@two/y"},
		{"a URL in a command line", "git clone https://oauth2:glpat-xyz@gitlab.com/o/r.git dir",
			"git clone https://REDACTED@gitlab.com/o/r.git dir"},
		{"a URL in JSON text", `{"url":"https://u:pw9@h/x","next":"me@example.com"}`,
			`{"url":"https://REDACTED@h/x","next":"me@example.com"}`},
		{"a URL written with JSON's escaped slashes", `https:\/\/u:pw9@h\/x`, `https:\/\/REDACTED@h\/x`},
		{"a URL inside the query of another", "https://h/a?next=https://u:pw9@i/",
			"https://h/a?next=https://REDACTED@i/"},
		{"a separator without a scheme is no URL", "://u:p@h/", "://u:p@h/"},
		{"credentials without a scheme are kept", "user:secret@proxy.example:3128", "user:secret@proxy.example:3128"},
		{"credentials without a scheme after a URL are kept", "https://a/x,user:pw@proxy:3128",
			"https://a/x,user:pw@proxy:3128"},
		{"a time before an at sign is kept", "meet at 12:30@home", "meet at 12:30@home"},
		{"a package version is kept", "node:18@latest", "node:18@latest"},
		{"an image digest is kept", "image:tag@sha256:abc", "image:tag@sha256:abc"},
		{"a mail header is kept", "From:alice@example.com", "From:alice@example.com"},
		{"an error naming an address is kept", "Error:foo@bar", "Error:foo@bar"},
		{"the SCP form of a git remote is kept", "git@github.com:o/r.git", "git@github.com:o/r.git"},
		{"a token in the query", "https://host/share/session?token=abc123", "https://host/share/session?token=REDACTED"},
		{"an API key in the query beside other parameters", "https://h/v1?alt=json&key=AIza123&x=1",
			"https://h/v1?alt=json&key=REDACTED&x=1"},
		{"parameters split at semicolons", "https://h/?x=1;sig=abc;y=2", "https://h/?x=1;sig=REDACTED;y=2"},
		{"a parameter name in another case", "https://h/?Access_Token=abc#x", "https://h/?Access_Token=REDACTED#x"},
		{"a parameter name with a percent-escape", "https://h/?%74oken=abc", "https://h/?%74oken=REDACTED"},
		{"a signed URL's AWS parameters", "https://b.s3.example/k?X-Amz-Credential=AKIAFIXTURE&X-Amz-Signature=f00d",
			"https://b.s3.example/k?X-Amz-Credential=REDACTED&X-Amz-Signature=REDACTED"},
		{"a name folded outside ASCII is kept", "https://h/?\u212aey=abc", "https://h/?\u212aey=abc"},
		{"a token in the fragment", "https://h/cb#access_token=abc&token_type=bearer",
			"https://h/cb#access_token=REDACTED&token_type=bearer"},
		{"credentials and a query token together", "https://u:pw9@h/x?token=t0k#f", "https://REDACTED@h/x?token=REDACTED#f"},
		{"a parameter whose name only holds a credential name is kept", "https://h/?tokens=1&monkey=2",
			"https://h/?tokens=1&monkey=2"},
		{"an empty credential value is kept", "https://h/?token=&x=1", "https://h/?token=&x=1"},
		{"a query outside a URL is kept", "a?token=b", "a?token=b"},
		{"punctuation after a URL is not part of it", "POST https://h/t?key=k1: failed, see https://u:pw9@h/.",
			"POST https://h/t?key=REDACTED: failed, see https://REDACTED@h/."},
		{"a short parameter value is replaced only where the URL holds it",
			`{"cmd":"curl https://h/x?auth=true","enabled":true,"note":"true story"}`,
			`{"cmd":"curl https://h/x?auth=REDACTED","enabled":true,"note":"true story"}`},
		{"a short code is kept elsewhere in the string", "https://h/?code=1 and 1+1=2",
			"https://h/?code=REDACTED and 1+1=2"},
		{"a short password is kept elsewhere in the string", "https://pw:pw@h/ pw", "https://REDACTED@h/ pw"},
		{"user information that is only a colon is none", "see https://:@host/ at 10:30: done",
			"see https://:@host/ at 10:30: done"},
		{"a URL without credentials is unchanged", "http://localhost:8080", "http://localhost:8080"},
		{"empty text is unchanged", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, _ := urlText(withheld(), test.text); got != test.want {
				t.Errorf("withheld: got %q, want %q", got, test.want)
			}
			if got, recorded := urlText(Setting{}, test.text); got != test.text || recorded != nil {
				t.Errorf("shown: got %q and recorded %q, want the text as written and nothing", got, recorded)
			}
		})
	}
}

func TestAURLsShortCredentialsAreReplacedWhereTheyStandAndLongOnesEverywhere(t *testing.T) {
	recorder := NewRecorder(withheld())
	output := []string{"see https://ci:pw7@h/ and https://ci:FIXTURE-PW@g/", "pw7 ci:FIXTURE-PW"}
	for _, text := range output {
		recorder.Collect(text)
	}
	want := []string{"see https://REDACTED@h/ and https://REDACTED@g/", "pw7 REDACTED"}
	for i, text := range output {
		if got := recorder.Text(text); got != want[i] {
			t.Errorf("%q: got %q, want %q", text, got, want[i])
		}
	}
}

func TestAPasswordIsRecordedWithItsUserAndOnItsOwn(t *testing.T) {
	_, recorded := urlText(withheld(), "https://ci:s3cret@h/x?token=t0k")
	if want := []string{"ci:s3cret", "s3cret", "t0k"}; !slices.Equal(recorded, want) {
		t.Errorf("recorded %q, want %q", recorded, want)
	}
}

func TestAnyPlaceholderStandsWhereTheSecretWasAndIsNotReadAgain(t *testing.T) {
	const text = "https://ci:s3cret@github.com/o/r.git?token=REDACTED"
	tests := []struct {
		name, placeholder, want string
	}{
		{"the default", DefaultPlaceholder, "https://REDACTED@github.com/o/r.git?token=REDACTED"},
		{"an empty placeholder takes the at sign too", "", "https://github.com/o/r.git?token="},
		{"verbs", "%s%v", "https://%s%v@github.com/o/r.git?token=%s%v"},
		{"a line end", "\n", "https://\n@github.com/o/r.git?token=\n"},
		{"a quote", `"`, `https://"@github.com/o/r.git?token="`},
		{"a group reference", "$1", "https://$1@github.com/o/r.git?token=$1"},
		{"a parameter", "&key=V", "https://&key=V@github.com/o/r.git?token=&key=V"},
		{"a whole URL", "https://x:y@z/?token=t", "https://https://x:y@z/?token=t@github.com/o/r.git?token=" +
			"https://x:y@z/?token=t"},
		{"the placeholder when a secret is the placeholder", "REDACTED",
			"https://REDACTED@github.com/o/r.git?token=REDACTED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting := Setting{Withhold: true, Placeholder: test.placeholder}
			if got, _ := urlText(setting, text); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnEmptyPlaceholderGivesTheURLWithoutItsUserInformation(t *testing.T) {
	got, _ := urlText(Setting{Withhold: true}, "https://x-access-token:ghs_FIXTURE@github.com/o/r.git")
	if got != "https://github.com/o/r.git" {
		t.Errorf("got %q", got)
	}
}

func TestTheLongestSecretAtAPositionIsReplacedWhole(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		text    string
		want    string
	}{
		{"a password inside its user and password", []string{"password", "user:password"},
			"user:password and password", "R and R"},
		{"the same, recorded the other way round", []string{"user:password", "password"},
			"user:password and password", "R and R"},
		{"two that overlap are one replaced range", []string{"abcd", "cdef"}, "abcdef", "R"},
		{"a second that starts inside the first and ends after it", []string{"AAAABBBB", "BBBBCCCCDDDD"},
			"x AAAABBBBCCCCDDDD y", "x R y"},
		{"a chain of overlaps is one range", []string{"abc", "cde", "efg"}, "abcdefg!", "R!"},
		{"one inside another is one range", []string{"abcdef", "cd"}, "abcdef cd", "R R"},
		{"a secret repeated", []string{"tok"}, "toktoktok", "RRR"},
		{"a one-letter secret wherever it stands", []string{"p"}, "a pup", "a RuR"},
		{"nothing recorded leaves the text", nil, "user:password", "user:password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := identified(Setting{Withhold: true, Placeholder: "R"}, test.text, func(recorder *Recorder) {
				for _, secret := range test.secrets {
					recorder.Value(secret)
				}
			})
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestEachKindOfSecretIsReplacedOnlyWhereItReaches(t *testing.T) {
	recorder := NewRecorder(withheld())
	first := recorder.Begin()
	recorder.Value("pw1")
	recorder.Value("fake-secret-value-0001")
	recorder.ReaderError(errors.New("unexpected end of JSON input"))
	recorder.End()
	second := recorder.Begin()
	recorder.End()
	recorder.Value("pw2")
	const text = "pw1 fake-secret-value-0001 unexpected end of JSON input pw2"
	tests := []struct {
		name, got, want string
	}{
		{"the text of a row of the parse that found them", recorder.RowText(text, first),
			"REDACTED REDACTED REDACTED pw2"},
		{"a problem line of that parse", recorder.ProblemText(text, first), "pw1 REDACTED REDACTED pw2"},
		{"the text of a row of another parse", recorder.RowText(text, second),
			"pw1 REDACTED unexpected end of JSON input pw2"},
		{"a problem line of another parse", recorder.ProblemText(text, second),
			"pw1 REDACTED unexpected end of JSON input pw2"},
		{"any other string", recorder.Text(text), "pw1 REDACTED unexpected end of JSON input pw2"},
		{"the text of a row of no parse", recorder.RowText(text, nil),
			"pw1 REDACTED unexpected end of JSON input pw2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestARunThatShowsSecretsRecordsNothing(t *testing.T) {
	recorder := NewRecorder(Setting{Placeholder: "x"})
	if recorder != nil {
		t.Fatal("a run that shows secrets has a recorder")
	}
	found := recorder.Begin()
	recorder.Value("sk-FIXTURE")
	recorder.Collect("https://ci:FIXTURE-PW@h/")
	recorder.ReaderError(errors.New("x"))
	recorder.End()
	for _, got := range []string{recorder.Text("sk-FIXTURE"), recorder.RowText("sk-FIXTURE", found),
		recorder.ProblemText("sk-FIXTURE", found)} {
		if got != "sk-FIXTURE" || found.Secrets() != nil {
			t.Errorf("got %q, recorded %q", got, found.Secrets())
		}
	}
	if recorder.Setting() != (Setting{}) {
		t.Errorf("setting %+v", recorder.Setting())
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

func TestAReadersDetailIsRecordedAndItsPositionKept(t *testing.T) {
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
		{"a JSON error keeps its position", withheld(), jsonError, "invalid JSON at line 1, column 13: REDACTED"},
		{"a TOML error keeps its position", withheld(), tomlError, "invalid TOML at line 1, column 11: REDACTED"},
		{"an XML error keeps its line", withheld(), xmlError, "XML syntax error on line 2: REDACTED"},
		{"another placeholder follows the position", Setting{Withhold: true, Placeholder: "***"}, jsonError,
			"invalid JSON at line 1, column 13: ***"},
		{"an empty placeholder leaves the position alone", Setting{Withhold: true}, tomlError,
			"invalid TOML at line 1, column 11"},
		{"a TOML limit is in full", withheld(), limitError, limitError.Error()},
		{"an error of another kind is the placeholder", withheld(), errors.New("sk-live-123"), "REDACTED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := test.err.Error()
			got, _ := identified(test.setting, message, func(recorder *Recorder) { recorder.ReaderError(test.err) })
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
			if quotesTheDocument(got) {
				t.Errorf("the withheld message %q quotes the document", got)
			}
			if shown, _ := identified(Setting{}, message, func(recorder *Recorder) { recorder.ReaderError(test.err) }); shown != message {
				t.Errorf("shown: %q", shown)
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

// counter is a textset.Counter that keeps its counts by name.
type counter map[string]int

// Add counts n units of the named work.
func (c counter) Add(name string, n int) { c[name] += n }

// identifyWork returns the bytes read identifying the URLs of k copies of a
// unit dense in separators, at signs and parameters.
func identifyWork(k int) int {
	work := counter{}
	text := "https://h/" + strings.Repeat("x://a@b?c&d#e=f&token=g;", k) + strings.Repeat("&", k) +
		strings.Repeat("://", k) + strings.Repeat("@", k)
	recorder := NewRecorder(withheld())
	recorder.work = work
	recorder.Collect(text)
	return work["bytes read"]
}

// outputWork returns the bytes read collecting and then writing an output
// of rows strings, each dense in URLs, parameters and recorded secrets.
func outputWork(rows int) int {
	work := counter{}
	recorder := NewRecorder(withheld())
	recorder.work = work
	found := recorder.Begin()
	recorder.Value("sk-FIXTURE-0001")
	recorder.End()
	output := make([]string, rows)
	for i := range output {
		output[i] = fmt.Sprintf("row %06d https://u%06d:pw@h/x?token=t%06d&a=b#key=k x://y@z sk-FIXTURE-0001", i, i, i)
	}
	for _, text := range output {
		recorder.Collect(text)
	}
	for _, text := range output {
		recorder.RowText(text, found)
	}
	return work["bytes read"] + work["bytes searched"]
}

func TestWritingAnOutputCostsItsLengthAFixedNumberOfTimes(t *testing.T) {
	small, double, quadruple := outputWork(1000), outputWork(2000), outputWork(4000)
	// Each row is the same length, so doubling the rows doubles what they
	// cost, exactly, the long secrets collected from them included.
	if quadruple-double != 2*(double-small) || double-small <= 0 {
		t.Errorf("read %d, %d and %d bytes for 1000, 2000 and 4000 rows: not linear", small, double, quadruple)
	}
}

func TestIdentifyingURLsReadsTheTextAFixedNumberOfTimes(t *testing.T) {
	small, double, quadruple := identifyWork(5000), identifyWork(10000), identifyWork(20000)
	// The work is a fixed cost plus a cost per unit: doubling the units
	// doubles what the units cost, exactly.
	if quadruple-double != 2*(double-small) || double-small <= 0 {
		t.Errorf("read %d, %d and %d bytes for 5000, 10000 and 20000 units: not linear", small, double, quadruple)
	}
}

// replaceWork returns the bytes searched and the fallbacks taken replacing
// n recorded secrets in a text holding each of them, once with their parse
// as the scope.
func replaceWork(n int) (int, int, int) {
	work := counter{}
	recorder := NewRecorder(withheld())
	recorder.work = work
	found := recorder.Begin()
	var text strings.Builder
	for i := range n {
		long, short := fmt.Sprintf("sk-FIXTURE-%06d", i), fmt.Sprintf("p%05d", i)
		recorder.Value(long)
		recorder.Value(short)
		text.WriteString(long + " " + short + " plain text between them ")
	}
	recorder.End()
	got := recorder.RowText(text.String(), found)
	return len(text.String()), work["bytes searched"], strings.Count(got, DefaultPlaceholder)
}

func TestReplacingReadsTheTextOncePerRecordWhateverTheNumberOfSecrets(t *testing.T) {
	for _, n := range []int{500, 5000} {
		t.Run(fmt.Sprintf("%d secrets", n), func(t *testing.T) {
			length, searched, replaced := replaceWork(n)
			// Once for the secrets replaced everywhere and once for the
			// parse's short ones.
			if searched != 2*length || replaced != 2*n {
				t.Errorf("searched %d bytes of %d and replaced %d, want %d and %d", searched, length, replaced,
					2*length, 2*n)
			}
		})
	}
}

// reference returns text with every occurrence of each of secrets and
// each of ranges ([start, end) byte ranges) replaced, the occurrences found
// by comparing each secret at each position: ranges that share a byte are
// one, replaced by one placeholder, and two that only touch are two.
func reference(text string, secrets []string, ranges [][2]int, placeholder string) string {
	for i := range len(text) {
		for _, secret := range secrets {
			if secret != "" && strings.HasPrefix(text[i:], secret) {
				ranges = append(ranges, [2]int{i, i + len(secret)})
			}
		}
	}
	slices.SortFunc(ranges, func(left, right [2]int) int { return left[0] - right[0] })
	var out strings.Builder
	written, end := 0, -1
	for _, each := range ranges {
		if each[0] >= end {
			out.WriteString(text[written:each[0]] + placeholder)
		}
		end = max(end, each[1])
		written = end
	}
	out.WriteString(text[written:])
	return out.String()
}

// FuzzTextReplacesExactlyTheRecordedSecrets holds that the output of a row
// is its text with exactly the recorded secrets and the credentials of its
// URLs replaced, the longest at the leftmost position first, whatever the
// secrets, the text and the placeholder.
func FuzzTextReplacesExactlyTheRecordedSecrets(fuzz *testing.F) {
	fuzz.Add("user:password and password", "password\x00user:password", "R")
	fuzz.Add("aaaa", "a\x00aa\x00aaa", "")
	fuzz.Add("x&key=y", "y", "&key=y")
	fuzz.Fuzz(func(t *testing.T, text, joined, placeholder string) {
		secrets := strings.Split(joined, "\x00")
		recorder := NewRecorder(Setting{Withhold: true, Placeholder: placeholder})
		found := recorder.Begin()
		for _, secret := range secrets {
			recorder.Value(secret)
		}
		recorder.End()
		// The output also withholds the credentials of the text's URLs where
		// they stand.
		ranges := URLCredentialRanges(text, placeholder == "")
		if got, want := recorder.RowText(text, found), reference(text, secrets, ranges, placeholder); got != want {
			t.Errorf("%q with %q as %q: got %q, want %q", text, secrets, placeholder, got, want)
		}
	})
}

// FuzzURLsWithholdsBothSecrets holds that for
// scheme://user:SECRET@host/path?token=SECRET2#fragment exactly the user
// information, the password and SECRET2 are recorded and the output is the
// URL with the user information and SECRET2 replaced where they stand, the
// same bytes elsewhere in the URL kept, and that with withholding off
// nothing is recorded. SECRET holds no byte that ends an authority (/, ?, #, a
// backslash) and SECRET2 none that ends a parameter (&, ;, #, a backslash)
// and no "://", which starts another URL; neither holds a delimiter.
func FuzzURLsWithholdsBothSecrets(fuzz *testing.F) {
	for _, seed := range [][2]string{{"s3cret", "t0ken"}, {"tok+abc=", "a?b"}, {"ab@cd", "x/y:z"},
		{"p:ss", "@@"}, {"a@b", "u:p@h"}, {"x:y", "%41"}, {"%40:", "é"}} {
		fuzz.Add("https", seed[0], "host.example", "/path", seed[1], "frag")
	}
	fuzz.Fuzz(func(t *testing.T, scheme, secret, host, path, secret2, fragment string) {
		if !onlyOf(scheme, "abcdefghijklmnopqrstuvwxyz", 1) ||
			!onlyOf(host, "abcdefghijklmnopqrstuvwxyz0123456789.-", 1) ||
			!onlyOf(path, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._~-", 0) ||
			!onlyOf(fragment, "abcdefghijklmnopqrstuvwxyz0123456789", 0) ||
			!printable(secret, `/?#\`) || !printable(secret2, `&;#\`) || strings.Contains(secret2, "://") ||
			secret == "" || secret2 == "" {
			return
		}
		text := scheme + "://user:" + secret + "@" + host + "/" + path + "?token=" + secret2 + "#" + fragment
		if got, recorded := urlText(Setting{}, text); got != text || recorded != nil {
			t.Fatalf("shown: %q gave %q and recorded %q", text, got, recorded)
		}
		want := []string{"user:" + secret, secret, secret2}
		slices.Sort(want)
		for _, placeholder := range []string{DefaultPlaceholder, ""} {
			got, recorded := urlText(Setting{Withhold: true, Placeholder: placeholder}, text)
			userInfo := placeholder + "@"
			if placeholder == "" {
				userInfo = ""
			}
			expected := scheme + "://" + userInfo + host + "/" + path + "?token=" + placeholder + "#" + fragment
			if got != expected || !slices.Equal(recorded, slices.Compact(slices.Clone(want))) {
				t.Errorf("withheld with %q: %q gave %q and recorded %q, want %q and %q", placeholder, text, got,
					recorded, expected, want)
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
