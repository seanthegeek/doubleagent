//go:build parity

package replay

import (
	"regexp"
	"strings"
	"testing"
)

// Difference 260: a Gemini CLI network response or error row names its
// request by the id it carries, and only the request's own row gives the
// request's method and URL, so many answers to one request with a long URL
// never repeat it. The recording repeated both in each answer.

// requestAnswersDifference returns difference 260.
func requestAnswersDifference() Difference {
	return requestAnswersDifferenceIn(caseRun{})
}

// requestAnswersDifferenceIn returns difference 260 as it rewrites the
// texts of a case of run, whose input says which requests it records.
func requestAnswersDifferenceIn(run caseRun) Difference {
	forms := requestAnswerForms()
	return Difference{Number: 260, Summary: "a Gemini CLI network response or error row names its request by " +
		"its id alone: the recording's row, which repeated the method and the URL that the case's input " +
		"records for that id (or - - where the input records no request of that id), is compared with the " +
		"two left out, and the binary's row as written; the request's own row keeps them (difference 230)",
		Text: formsText(forms, recordedSide, run), Binary: formsText(forms, binarySide, run),
		RowText: func(agent, text string) (string, int) {
			return rowRewriteAll(forms, recordedSide, agent, text, run)
		}, RowBinary: func(agent, text string) (string, int) {
			return rowRewriteAll(forms, binarySide, agent, text, run)
		}}
}

// requestAnswerForms returns the one site of difference 260, in the form
// of withheldForms: a recorded response or error row of the Gemini CLI
// parser, its method and URL cut at the query as the recording printed
// them, or "- -" for a request the recording had not met. Its completed
// form is the row without them when the case's input records that URL as
// the request of the row's id, or no request of that id for "- -", and
// otherwise a row the binary never writes, so the case fails.
func requestAnswerForms() []withheldForm {
	return []withheldForm{{scope: parserScope, agent: "gemini-cli", name: "network answer's request",
		recorded: regexp.MustCompile(`(network (?:response|error): )(?:[A-Z]+ (https?://[^ ?#"\n]*?)|- -)` +
			`(:? (?:[^"\n]*? )?\(request ([^ )"\n]*)\))`),
		completed: func(match []string, run caseRun) string {
			rest := strings.TrimPrefix(strings.TrimPrefix(match[3], ":"), " ")
			if !run.recordsAnswered(match[4], match[2]) {
				return match[1] + incomplete + " " + rest
			}
			return match[1] + rest
		}}}
}

// recordsAnswered reports whether the case's input records base as the URL
// of the request requestID, as requestURL reads it, or, for an empty base,
// records no request of that id.
func (c caseRun) recordsAnswered(requestID, base string) bool {
	if c.requests == nil {
		return false
	}
	if base == "" {
		return len(c.requests()[requestID]) == 0
	}
	_, ok := c.recordedURL(requestID, base)
	return ok
}

// answerCase is a recorded row of the Gemini CLI parser, the binary's row,
// and whether the two agree under differences 102, 230 and 260 in a case
// whose input records sampleRequests.
type answerCase struct {
	name, want, got string
	agree           bool
}

// answerCases returns the cases of difference 260 and of the request rows
// beside it.
func answerCases() []answerCase {
	streamed := "network response: POST https://h.example/v1:stream status 200 in 900 ms (request k3x9q)"
	streamedBinary := "network response: status 200 in 900 ms (request k3x9q)"
	failed := "network error: POST https://oauth2.googleapis.com/token: socket hang up (request t4r8z)"
	failedBinary := "network error: socket hang up (request t4r8z)"
	unmet := "network response: - - status - in - ms (request zzzzz)"
	keyed := "network request: POST https://oauth2.googleapis.com/token (request t4r8z)"
	keyedBinary := strings.Replace(keyed, "/token", "/token?key="+replayPlaceholder, 1)
	return []answerCase{
		{"a response without its request's method and URL passes", streamed, streamedBinary, true},
		{"an error without its request's method and URL passes", failed, failedBinary, true},
		{"a response to a request the input does not record passes", unmet,
			"network response: status - in - ms (request zzzzz)", true},
		{"a response still giving its request's URL fails", streamed,
			strings.Replace(streamed, ":stream", ":stream?alt=sse", 1), false},
		{"a response giving the recorded row as written fails", streamed, streamed, false},
		{"an error still giving its request's URL fails", failed,
			strings.Replace(failed, "/token", "/token?key="+replayPlaceholder, 1), false},
		{"a response giving its request's method alone fails", streamed,
			"network response: POST status 200 in 900 ms (request k3x9q)", false},
		{"a response naming another request fails", streamed,
			strings.Replace(streamedBinary, "k3x9q", "g1h2j", 1), false},
		{"a response leaving out its status fails", streamed, "network response: in 900 ms (request k3x9q)", false},
		{"a recorded response whose URL is not its request's fails", strings.Replace(streamed, "/v1", "/v2", 1),
			streamedBinary, false},
		{"a recorded response of an unmet request the input records fails",
			"network response: - - status 200 in 900 ms (request k3x9q)", streamedBinary, false},
		{"a request row keeping its URL, its key withheld, passes", keyed, keyedBinary, true},
		{"a request row without its URL fails", keyed, "network request: POST (request t4r8z)", false},
		{"a request row showing its key fails", keyed,
			strings.Replace(keyedBinary, replayPlaceholder, "AIzaFIXTURE", 1), false},
	}
}

// answerAgreeIn reports whether a recorded and a binary row of the Gemini
// CLI parser agree under differences 102, 230 and 260 in a case of run.
func answerAgreeIn(t *testing.T, run caseRun, want, got string) bool {
	t.Helper()
	reporter := &recordingTB{TB: t}
	normalWant, normalGot := normaliseRowsOf(reporter, withheldIn(Differences(), run), "n", []int{102, 230, 260},
		[]Row{{Agent: "gemini-cli", Text: Text(want)}}, []Row{{Agent: "gemini-cli", Text: Text(got)}})
	return normalWant[0].Text == normalGot[0].Text
}

func TestDifference260LeavesOnlyTheRequestsOwnRowItsMethodAndURL(t *testing.T) {
	for _, test := range answerCases() {
		t.Run(test.name, func(t *testing.T) {
			if agree := answerAgreeIn(t, caseRun{requests: sampleRequests()}, test.want, test.got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestDifference260NeverAgreesWithNoInputKnown(t *testing.T) {
	recorded := "network response: POST https://h.example/v1:stream status 200 in 900 ms (request k3x9q)"
	if answerAgreeIn(t, caseRun{}, recorded, "network response: status 200 in 900 ms (request k3x9q)") {
		t.Error("a response agrees with no input known")
	}
}

func TestDifference260RewritesACommandsTimelineOnlyWhereTheBinarysParsersRan(t *testing.T) {
	recorded := `{"agent":"gemini-cli","text":"network response: POST https://h.example/v1:stream status 200 ` +
		`in 900 ms (request k3x9q)"}`
	binary := `{"agent":"gemini-cli","text":"network response: status 200 in 900 ms (request k3x9q)"}`
	tests := []struct {
		name   string
		source runSource
		agree  bool
	}{
		{"a run of the binary's parsers", parsedRun, true},
		{"a run over the recording's rows", recordedRowsRun, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := requestAnswersDifferenceIn(caseRun{source: test.source, requests: sampleRequests()})
			want, _ := entry.Text(recorded)
			got, _ := entry.Binary(binary)
			if agree := want == got; agree != test.agree {
				t.Errorf("agree %v, want %v: %q and %q", agree, test.agree, want, got)
			}
		})
	}
}
