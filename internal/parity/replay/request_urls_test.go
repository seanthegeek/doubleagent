//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// requestURLs are the URLs of the network requests a case's input records,
// by request id: each URL a record of the id gives, once.
type requestURLs map[string][]string

// incomplete stands in a completed recorded row for the URL of a request
// the case's input does not record whole, so the row never agrees.
const incomplete = "{a request the input does not record}"

// requestURL returns the URL the binary is to write for the request
// requestID, whose URL the recording cut to base: the one URL the case's
// input records for the request, when it is base or base and a query or
// fragment, its credentials withheld (withheldURL); otherwise incomplete.
func (c caseRun) requestURL(requestID, base string) string {
	if c.requests == nil {
		return incomplete
	}
	urls := c.requests()[requestID]
	if len(urls) != 1 || urls[0] != base && !strings.HasPrefix(urls[0], base+"?") &&
		!strings.HasPrefix(urls[0], base+"#") {
		return incomplete
	}
	return withheldURL(urls[0])
}

// shownCredential stands in a recorded row for a request URL in which the
// binary's ranges leave out a credential the replay itself requires
// withheld (requiredCredentials), so the row never agrees and the case
// fails.
const shownCredential = "{a credential the binary would show}"

// withheldURL returns url with each of its credentials as replayPlaceholder
// (withheldURLBy, by the ranges secrets.URLCredentialRanges gives, the
// ranges the binary withholds).
func withheldURL(url string) string {
	return withheldURLBy(url, secrets.URLCredentialRanges)
}

// withheldURLBy returns url with each byte range credentialRanges gives as
// replayPlaceholder, so what counts as a credential beyond the replay's own
// minimum is the binary's rule; or shownCredential when a range of
// requiredCredentials lies inside none of them. Ranges that share a byte
// are one, as one secret held inside another goes with it; ranges that
// only touch stay two.
func withheldURLBy(url string, credentialRanges func(text string, emptyPlaceholder bool) [][2]int) string {
	ranges := credentialRanges(url, false)
	for _, required := range requiredCredentials(url) {
		if !slices.ContainsFunc(ranges, func(given [2]int) bool {
			return given[0] <= required[0] && required[1] <= given[1]
		}) {
			return shownCredential
		}
	}
	slices.SortFunc(ranges, func(left, right [2]int) int { return left[0] - right[0] })
	var written strings.Builder
	end := 0
	for i := 0; i < len(ranges); {
		start, stop := ranges[i][0], ranges[i][1]
		for i++; i < len(ranges) && ranges[i][0] < stop; i++ {
			stop = max(stop, ranges[i][1])
		}
		written.WriteString(url[end:start] + replayPlaceholder)
		end = stop
	}
	written.WriteString(url[end:])
	return written.String()
}

// urlScheme is a URL's scheme and separator at the start of a text: a
// letter, then letters, digits, +, - and ., then "://".
var urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// requiredCredentials returns the byte ranges of a request URL that the
// binary must withhold whatever its own table says, the replay's minimum:
// user information holding a colon, unless it is the colon alone, and the
// value of each parameter named exactly key, which the recorded Gemini CLI
// logs carry Google's API key in and the recording cut, so it holds
// nothing to compare. A value is taken up to its first byte other than a
// letter, digit, - or _, where the binary may end one. The URL is read up
// to white space, a double quote, an angle bracket or a backtick.
func requiredCredentials(url string) [][2]int {
	if end := strings.IndexAny(url, " \t\n\v\f\r\"<>`"); end >= 0 {
		url = url[:end]
	}
	scheme := urlScheme.FindString(url)
	if scheme == "" {
		return nil
	}
	var required [][2]int
	authorityEnd := len(url)
	if end := strings.IndexAny(url[len(scheme):], "/?#\\"); end >= 0 {
		authorityEnd = len(scheme) + end
	}
	if lastAt := strings.LastIndexByte(url[:authorityEnd], '@'); lastAt >= len(scheme) {
		if userInfo := url[len(scheme):lastAt]; strings.Contains(userInfo, ":") && userInfo != ":" {
			required = append(required, [2]int{len(scheme), lastAt})
		}
	}
	if query := strings.IndexAny(url[authorityEnd:], "?#"); query >= 0 {
		required = append(required, keyValues(url, authorityEnd+query)...)
	}
	return required
}

// keyValueBytes are the bytes of a key's value that requiredCredentials
// takes: letters, digits, - and _.
const keyValueBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// keyValues returns the ranges of the values of the parameters named key in
// url's query and fragment, which start at from, the first ? or # after
// the authority: a name starts after each
// ?, & and ;, and after the first # alone, where every reading of a URL
// starts one.
func keyValues(url string, from int) [][2]int {
	var values [][2]int
	fragment := false
	for i := from; i < len(url); i++ {
		if url[i] == '#' {
			if fragment {
				continue
			}
			fragment = true
		} else if strings.IndexByte("?&;", url[i]) < 0 {
			continue
		}
		if !strings.HasPrefix(url[i+1:], "key=") {
			continue
		}
		start := i + 1 + len("key=")
		end := start
		for end < len(url) && strings.IndexByte(keyValueBytes, url[end]) >= 0 {
			end++
		}
		if end > start {
			values = append(values, [2]int{start, end})
		}
	}
	return values
}

// requestRecord is the part of a line of an activity log that names a
// network request: its id and URL.
type requestRecord struct {
	Payload struct {
		RequestID *string `json:"id"`
		URL       *string `json:"url"`
	} `json:"payload"`
}

// addRequests adds to urls the request of each line of data that is a JSON
// object whose payload gives a request's id and URL as strings.
func addRequests(urls requestURLs, data []byte) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"url"`)) {
			continue
		}
		var record requestRecord
		if json.Unmarshal(line, &record) != nil || record.Payload.RequestID == nil || record.Payload.URL == nil {
			continue
		}
		requestID, url := *record.Payload.RequestID, *record.Payload.URL
		if !slices.Contains(urls[requestID], url) {
			urls[requestID] = append(urls[requestID], url)
		}
	}
}

// fileRequests returns the requests the file at path records, read when
// first asked for.
func fileRequests(path string) func() requestURLs {
	var urls requestURLs
	return func() requestURLs {
		if urls == nil {
			urls = requestURLs{}
			if data, err := osfile.ReadFile(path); err == nil {
				addRequests(urls, data)
			}
		}
		return urls
	}
}

// treeRequests returns the requests the regular files of a case's input
// tree, the files under root by root-relative path, record, read when
// first asked for.
func treeRequests(root string, files map[string]bool) func() requestURLs {
	var urls requestURLs
	return func() requestURLs {
		if urls == nil {
			urls = requestURLs{}
			for _, name := range slices.Sorted(maps.Keys(files)) {
				if data, ok := inputFileBytes(root, name); ok {
					addRequests(urls, data)
				}
			}
		}
		return urls
	}
}

// sampleRequests returns the requests of an activity log of the shape the
// recorded Gemini CLI logs have.
func sampleRequests() func() requestURLs {
	urls := requestURLs{}
	addRequests(urls, []byte(`{"type": "network", "payload": {"id": "t4r8z", "method": "POST", `+
		`"url": "https://oauth2.googleapis.com/token?key=AIzaFIXTURE"}}
{"type": "network", "payload": {"id": "k3x9q", "method": "POST", `+
		`"url": "https://h.example/v1:stream?alt=sse"}}
{"type": "network", "payload": {"id": "g1h2j", "method": "GET", "url": "https://play.example/log"}}
{"type": "network", "payload": {"id": "t4r8z", "response": {"status": 200}}}
{"type": "network", "payload": {"id": "twice", "method": "GET", "url": "https://a.example/x?key=1"}}
{"type": "network", "payload": {"id": "twice", "method": "GET", "url": "https://a.example/x?key=2"}}
{"type": "network", "payload": {"id": "f7m2c", "method": "GET", "url": "https://h.example/cb#access_token=FIXTURE&state=s"}}
{"type": "network", "payload": {"id": "p9w4e", "method": "GET", "url": "https://h.example/v1?apikey=FIXTURE&alt=sse"}}
`))
	return func() requestURLs { return urls }
}

func TestAGeminiRequestRowIsComparedWithTheURLTheInputRecords(t *testing.T) {
	keyed := "network request: POST https://oauth2.googleapis.com/token (request t4r8z)"
	keyedBinary := strings.Replace(keyed, "/token", "/token?key="+replayPlaceholder, 1)
	streamed := "network response: POST https://h.example/v1:stream status 200 in 900 ms (request k3x9q)"
	streamedBinary := strings.Replace(streamed, ":stream", ":stream?alt=sse", 1)
	plain := "network request: GET https://play.example/log (request g1h2j)"
	failed := "network error: POST https://oauth2.googleapis.com/token: socket hang up (request t4r8z)"
	failedBinary := strings.Replace(failed, "/token:", "/token?key="+replayPlaceholder+":", 1)
	fragment := "network request: GET https://h.example/cb (request f7m2c)"
	fragmentBinary := strings.Replace(fragment, "/cb", "/cb#access_token="+replayPlaceholder+"&state=s", 1)
	apiKey := "network request: GET https://h.example/v1 (request p9w4e)"
	apiKeyBinary := strings.Replace(apiKey, "/v1", "/v1?apikey="+replayPlaceholder+"&alt=sse", 1)
	tests := []struct {
		name, want, got string
		agree           bool
	}{
		{"a request's key withheld passes", keyed, keyedBinary, true},
		{"an error of the request with its key withheld passes", failed, failedBinary, true},
		{"a response with the request's query passes", streamed, streamedBinary, true},
		{"a request without a query passes as recorded", plain, plain, true},
		{"a request's query dropped fails", keyed, keyed, false},
		{"a request's key left out for another parameter fails", keyed, strings.Replace(keyed, "/token",
			"/token?alt=sse", 1), false},
		{"a request's key shown fails", keyed, strings.Replace(keyedBinary, replayPlaceholder, "AIzaFIXTURE", 1), false},
		{"a request's key repeated fails", keyed, strings.Replace(keyedBinary, "(request",
			"&key="+replayPlaceholder+" (request", 1), false},
		{"a request's key written as the default placeholder fails", keyed,
			strings.Replace(keyedBinary, replayPlaceholder, secrets.DefaultPlaceholder, 1), false},
		{"a response's query dropped fails", streamed, streamed, false},
		{"a response's query repeated fails", streamed, strings.Replace(streamedBinary, "alt=sse", "alt=sse&alt=sse",
			1), false},
		{"a response given another request's query fails", streamed, strings.Replace(streamed, ":stream",
			":stream?key="+replayPlaceholder, 1), false},
		{"a query the input does not record fails", plain, strings.Replace(plain, "/log", "/log?alt=sse", 1), false},
		{"a request the input does not record fails", strings.Replace(plain, "g1h2j", "zzzzz", 1),
			strings.Replace(plain, "g1h2j", "zzzzz", 1), false},
		{"a request of another path fails", keyed, strings.Replace(keyedBinary, "/token", "/tokens", 1), false},
		{"a token in the fragment withheld passes", fragment, fragmentBinary, true},
		{"a token in the fragment shown fails", fragment, strings.Replace(fragment, "/cb", "/cb#access_token=FIXTURE&state=s",
			1), false},
		{"a token in the fragment withheld with the rest of the fragment fails", fragment,
			strings.Replace(fragment, "/cb", "/cb#"+replayPlaceholder, 1), false},
		{"another parameter of the binary's table withheld passes", apiKey, apiKeyBinary, true},
		{"another parameter of the binary's table shown fails", apiKey, strings.Replace(apiKey, "/v1",
			"/v1?apikey=FIXTURE&alt=sse", 1), false},
		{"a request whose id the input gives two URLs fails",
			"network request: GET https://a.example/x (request twice)",
			"network request: GET https://a.example/x?key=" + replayPlaceholder + " (request twice)", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree := withheldAgreeIn(t, caseRun{requests: sampleRequests()}, "gemini-cli", test.want,
				test.got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestAGeminiRequestRowOfAnUnknownInputNeverAgrees(t *testing.T) {
	recorded := "network request: GET https://play.example/log (request g1h2j)"
	if withheldAgreeIn(t, caseRun{}, "gemini-cli", recorded, recorded) {
		t.Error("a row agrees with no input known")
	}
}

func TestARequestURLIsWithheldAsTheBinarysURLRuleFindsItsCredentials(t *testing.T) {
	tests := []struct {
		name, url, want string
	}{
		{"a key", "https://h/p?key=AIza", "https://h/p?key=" + replayPlaceholder},
		{"a key among other parameters", "https://h/p?alt=sse&Key=AIza&x=1",
			"https://h/p?alt=sse&Key=" + replayPlaceholder + "&x=1"},
		{"a key before a fragment", "https://h/p?key=AIza#f", "https://h/p?key=" + replayPlaceholder + "#f"},
		{"an empty key", "https://h/p?key=&x=1", "https://h/p?key=&x=1"},
		{"a key in the fragment", "https://h/p#key=AIza", "https://h/p#key=" + replayPlaceholder},
		{"an access token in the fragment", "https://h/cb#access_token=t0k&state=s",
			"https://h/cb#access_token=" + replayPlaceholder + "&state=s"},
		{"an apikey", "https://h/p?apikey=AIza", "https://h/p?apikey=" + replayPlaceholder},
		{"a name the table does not hold", "https://h/p?monkey=AIza", "https://h/p?monkey=AIza"},
		{"two credentials", "https://h/p?token=a&sig=b", "https://h/p?token=" + replayPlaceholder + "&sig=" +
			replayPlaceholder},
		{"a user name and a password, withheld once", "https://u:pw@h/p", "https://" + replayPlaceholder + "@h/p"},
		{"a short user name", "https://git@h/p", "https://git@h/p"},
		{"no credential", "https://h/p?alt=sse", "https://h/p?alt=sse"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := withheldURL(test.url); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheReplayRequiresAKeyAndAPasswordWithheldWhateverTheBinarysTable(t *testing.T) {
	tests := []struct {
		name, url string
		want      [][2]int
	}{
		{"a key", "https://h/p?key=AIza", [][2]int{{16, 20}}},
		{"a key after another parameter", "https://h/p?alt=sse&key=AIza", [][2]int{{24, 28}}},
		{"a key after a semicolon", "https://h/p?a=1;key=AIza", [][2]int{{20, 24}}},
		{"a key in the fragment", "https://h/p#key=AIza", [][2]int{{16, 20}}},
		{"a key after the first #", "https://h/p?a=1#key=AIza", [][2]int{{20, 24}}},
		{"a key's value up to a byte that may end it", "https://h/p?key=AI-z_a.", [][2]int{{16, 22}}},
		{"two keys", "https://h/p?key=a&key=b", [][2]int{{16, 17}, {22, 23}}},
		{"an empty key", "https://h/p?key=&x=1", nil},
		{"a name holding key", "https://h/p?apikey=AIza&monkey=b", nil},
		{"a key of another case", "https://h/p?Key=AIza", nil},
		{"a key in the path", "https://h/p;key=AIza", nil},
		{"a user name and a password", "https://u:pw@h/p", [][2]int{{8, 12}}},
		{"a password alone", "https://:pw@h/p", [][2]int{{8, 11}}},
		{"user information up to the last @", "https://a@b:c@h/p", [][2]int{{8, 13}}},
		{"an upper-case scheme", "HTTPS://u:pw@h/p?key=k", [][2]int{{8, 12}, {21, 22}}},
		{"a user name alone", "https://user@h/p", nil},
		{"the colon alone", "https://:@h/p", nil},
		{"an @ after the authority", "https://h/p?a=u:pw@x", nil},
		{"no scheme", "h/p?key=AIza", nil},
		{"a key after white space", "https://h/p ?key=AIza", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := requiredCredentials(test.url); !slices.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestARequestURLWhoseRangesLeaveOutARequiredCredentialIsNeverWritten(t *testing.T) {
	given := func(ranges ...[2]int) func(string, bool) [][2]int {
		return func(string, bool) [][2]int { return ranges }
	}
	tests := []struct {
		name, url string
		ranges    func(string, bool) [][2]int
		want      string
	}{
		{"a key withheld", "https://h/p?key=AIza", given([2]int{16, 20}), "https://h/p?key=" + replayPlaceholder},
		{"a key not withheld", "https://h/p?key=AIza", given(), shownCredential},
		{"a key withheld in part", "https://h/p?key=AIza", given([2]int{16, 18}), shownCredential},
		{"a key withheld within a wider range", "https://h/p?key=AIza", given([2]int{12, 20}),
			"https://h/p?" + replayPlaceholder},
		{"another parameter withheld and the key not", "https://h/p?sig=s&key=AIza", given([2]int{16, 17}),
			shownCredential},
		{"a password not withheld", "https://u:pw@h/p", given(), shownCredential},
		{"a password withheld alone", "https://u:pw@h/p", given([2]int{10, 12}), shownCredential},
		{"user information withheld", "https://u:pw@h/p", given([2]int{8, 12}), "https://" + replayPlaceholder + "@h/p"},
		{"a parameter the replay does not require, as the ranges give", "https://h/p?alt=sse", given([2]int{16, 19}),
			"https://h/p?alt=" + replayPlaceholder},
		{"no credential and no range", "https://h/p?alt=sse", given(), "https://h/p?alt=sse"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := withheldURLBy(test.url, test.ranges); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
