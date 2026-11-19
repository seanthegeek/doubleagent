//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"maps"
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

// withheldURL returns url with each of its credentials as replayPlaceholder:
// the byte ranges secrets.URLCredentialRanges gives, the ranges the binary
// withholds, so what counts as a credential (user information, the
// parameters of the query and fragment named in the binary's table) is the
// binary's own rule. Ranges that share a byte are one, as one secret held
// inside another goes with it; ranges that only touch stay two.
func withheldURL(url string) string {
	ranges := secrets.URLCredentialRanges(url, false)
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
