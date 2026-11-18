package pi

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every file kind are proved by the parser replay
// (internal/parity/replay) over the recorded fixtures; these tests cover the
// helpers the rows are built from, the ones other parsers share, and the
// bounds of the readers of evidence.

// decoded returns JSON text decoded, failing the test on an error.
func decoded(t testing.TB, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestArgsSummaryPicksTheMostIdentifyingArgument(t *testing.T) {
	tests := []struct {
		name, arguments, want string
	}{
		{"the path of an edit", `{"edits":[{"oldText":"a","newText":"b"}],"path":"src/a.ts"}`, "src/a.ts"},
		{"empty and null values are passed over", `{"command":"","path":null,"pattern":0}`, "0"},
		{"false is a value", `{"url":false}`, "false"},
		{"no known key gives the sorted object", `{"b":2,"a":"é"}`, `{"b":2,"a":"é"}`},
		{"an empty object", `{}`, "{}"},
		{"null is empty", `null`, ""},
		{"the empty string is empty", `""`, ""},
		{"a string is itself", `"ls -la"`, "ls -la"},
		{"a list is JSON", `[1,"x"]`, `[1,"x"]`},
		{"zero is JSON", `0`, "0"},
		{"false alone is JSON", `false`, "false"},
		{"a float as written", `1.50`, "1.50"},
		{"an object argument is sorted JSON", `{"query":{"z":1,"a":[true]}}`, `{"z":1,"a":[true]}`},
		{"spaces are kept", `{"command":"  "}`, "  "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ArgsSummary(decoded(t, test.arguments)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestModelTextNamesTheProviderAndTheModelAskedFor(t *testing.T) {
	seen := parser.Tracker{}
	steps := []struct{ key, model, provider, requested, want string }{
		{"s", "m1", "p", "", "model: m1 provider=p"},
		{"s", "m1", "p", "", ""},
		{"s", "m2", "", "m1", "model: m1 -> m2 requested=m1"},
		{"s", "m2", "p", "m2", ""},
		{"t", "m2", "p", "x", "model: m2 provider=p requested=x"},
		{"s", "", "p", "", ""},
	}
	for i, step := range steps {
		if got := ModelText(seen, step.key, step.model, step.provider, step.requested); got != step.want {
			t.Errorf("step %d: got %q, want %q", i, got, step.want)
		}
	}
}

func TestSwitchTextKeepsARowForTheModelInForce(t *testing.T) {
	seen := parser.Tracker{}
	steps := []struct{ key, model, provider, want string }{
		{"s", "m1", "p", "model: m1 provider=p"},
		{"s", "m1", "p", "model: m1 provider=p"},
		{"s", "m1", "", "model: m1"},
		{"s", "", "", ""},
		{"s", "m2", "", "model: m1 -> m2"},
	}
	for i, step := range steps {
		if got := SwitchText(seen, step.key, step.model, step.provider); got != step.want {
			t.Errorf("step %d: got %q, want %q", i, got, step.want)
		}
	}
}

func TestBugReportTextNamesTheDelivery(t *testing.T) {
	source := " (from session entry pi.bug-report, entry timestamp)"
	tests := []struct {
		name, data, want string
	}{
		{"an upload", `{"delivery":"upload","id":"r1","sessionIncluded":true,"summaryIncluded":false}`,
			"session synced: bug report r1 uploaded to radius.pi.dev transcript=true summary=false" + source},
		{"a zip", `{"delivery":"zip","id":"","path":"/p/x.zip"}`,
			"bug report exported: - path=/p/x.zip" + source},
		{"another delivery", `{"delivery":"other"}`, ""},
		{"data that is not an object", `[]`, ""},
		{"an upload without fields", `{"delivery":"upload"}`,
			"session synced: bug report - uploaded to radius.pi.dev transcript=- summary=-" + source},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BugReportText(decoded(t, test.data)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestBase64IsReadOnlyInItsPaddedFormWithoutLineBreaks(t *testing.T) {
	tests := []struct {
		text, want string
		valid      bool
	}{
		{"QUJD", "ABC", true},
		{"QQ==", "A", true},
		{"QR==", "A", true},
		{"QUI=", "AB", true},
		{"", "", true},
		{"QQ=", "", false},
		{"QQ", "", false},
		{"QUJDQ", "", false},
		{"QQ===", "", false},
		{"QQ==QQ==", "", false},
		{"Q===", "", false},
		{"QU JD", "", false},
		{"QU\nJD", "", false},
		{"QU\r\nJD", "", false},
		{"====", "", false},
		{"=QUJD", "", false},
		{"QUJD=", "", false},
		{"QQ=A", "", false},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			got, err := decodeBase64([]byte(test.text))
			if (err == nil) != test.valid || string(got) != test.want {
				t.Errorf("got %q, %v; want %q, valid %v", got, err, test.want, test.valid)
			}
		})
	}
}

// htmlPage returns an export page whose session-data element holds body.
func htmlPage(body string) []byte {
	return []byte("<html><script id=\"session-data\" type=\"application/json\">" + body +
		"</script></html>")
}

func TestHTMLSessionDataSaysWhyItCannotBeRead(t *testing.T) {
	encode := func(text string) string { return base64.StdEncoding.EncodeToString([]byte(text)) }
	tests := []struct {
		name string
		page []byte
		want string
	}{
		{"an object", htmlPage("\n " + encode(`{"header":{}}`) + " \n"), ""},
		{"no element", []byte("<html></html>"), "holds no session data"},
		{"no closing tag", []byte(`<script id="session-data" type="application/json">QUJD`),
			"session data has no closing tag"},
		{"not base64", htmlPage("QU JD"), "session data is not base64 (invalid base64 data at byte 2)"},
		{"not UTF-8", htmlPage(encode("\xff{}")), "session data is not JSON (not UTF-8 text)"},
		{"not JSON", htmlPage(encode("{")), "session data is not JSON ("},
		{"not an object", htmlPage(encode("[]")), "session data is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlSessionData(test.page)
			got := ""
			if err != nil {
				got = err.Error()
			}
			if !strings.HasPrefix(got, test.want) || (test.want == "") != (err == nil) {
				t.Errorf("got %v, want %q", err, test.want)
			}
		})
	}
}

// zipOf returns a deflated zip archive of the members, in order.
func zipOf(t testing.TB, members ...[2]string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, member := range members {
		file, err := writer.Create(member[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(member[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestZipMembersStopAtTheDecompressionBudget(t *testing.T) {
	data := zipOf(t, [2]string{"report.json", "{}"}, [2]string{"diagnostics.json", "{}"},
		[2]string{"session.jsonl", strings.Repeat("x", 4096)})
	tests := []struct {
		name     string
		budget   int
		members  []string
		problems []string
	}{
		{"a budget that holds every member", 4100, []string{"diagnostics.json", "report.json",
			"session.jsonl"}, nil},
		{"a budget the transcript passes", 4099, []string{"diagnostics.json", "report.json"},
			[]string{"r.zip member session.jsonl could not be read (" + errTooLarge.Error() + ")"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parser.Problems
			members := zipMembers("r.zip", data, test.budget, &problems)
			var names, messages []string
			for _, name := range bugMembers() {
				if _, ok := members[name]; ok {
					names = append(names, name)
				}
			}
			for _, problem := range problems {
				messages = append(messages, problem.Message)
			}
			if strings.Join(slices.Sorted(slices.Values(names)), ",") != strings.Join(test.members, ",") ||
				strings.Join(messages, "|") != strings.Join(test.problems, "|") {
				t.Errorf("got %v and %v, want %v and %v", names, messages, test.members, test.problems)
			}
		})
	}
}

func TestZipMembersReadTheLastOfTwoMembersWithOneName(t *testing.T) {
	data := zipOf(t, [2]string{"report.json", `{"id":"first"}`}, [2]string{"report.json", `{"id":"last"}`},
		[2]string{"diagnostics.json", "{}"})
	var problems parser.Problems
	members := zipMembers("r.zip", data, maxInflated, &problems)
	if got := string(members["report.json"]); got != `{"id":"last"}` || len(problems) != 0 {
		t.Errorf("got %q and %v", got, problems)
	}
}

func TestWantsTheFilesPiWrites(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{".pi/agent/sessions/--srv-proj--/2026-10-01T11-00-00-000Z_0b9e.jsonl", true},
		{".pi/agent/legacy.jsonl", true},
		{".pi/agent/sessions/a/b/c.jsonl", false},
		{".pi/agent/trust.json", true},
		{".pi/agent/settings.json", true},
		{".pi/agent/pi-debug.log", true},
		{".pi/agent/crashes.json", true},
		{".pi/agent/pi-tui-crash.log", true},
		{".pi/agent/pi-crash.log", true},
		{".pi/agent/experimental/sessions/s1/meta.json", true},
		{"pi-session-2026-10-02T08-00-00-000Z_x.html", true},
		{"session-2026-10-02T08-00-00.jsonl", true},
		{"session-26-10.jsonl", false},
		{"pi-bug-report-0199.zip", true},
		{"sub/pi-bug-report-0199.zip", false},
		{".pi/agent/trust.json.bak", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func FuzzHTMLSessionData(fuzz *testing.F) {
	fuzz.Add(htmlPage(base64.StdEncoding.EncodeToString([]byte(`{"header":{"type":"session"}}`))))
	fuzz.Add(htmlPage("QU JD"))
	fuzz.Fuzz(func(t *testing.T, page []byte) {
		value, err := htmlSessionData(page)
		if (err == nil) != value.IsObject() {
			t.Errorf("error %v with a value of kind %v", err, value.Kind())
		}
	})
}

func FuzzZipMembers(fuzz *testing.F) {
	fuzz.Add(zipOf(fuzz, [2]string{"report.json", "{}"}, [2]string{"session.jsonl", "{}\n"}))
	fuzz.Add([]byte("PK\x05\x06" + strings.Repeat("\x00", 18)))
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		var problems parser.Problems
		total := 0
		for _, content := range zipMembers("r.zip", data, 1<<20, &problems) {
			total += len(content)
		}
		if total > 1<<20 {
			t.Errorf("%d bytes read past the budget", total)
		}
	})
}

func TestFirstSeenKeepsDistinctTextsInOrderAtOneLookupEach(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
		want  []string
	}{
		{"none", nil, nil},
		{"repeats are kept once, first place", []string{"lc-b", "lc-a", "lc-b", "lc-a", "lc-c"}, []string{"lc-b", "lc-a", "lc-c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seen := newFirstSeen(parser.CountingWork())
			for _, text := range test.texts {
				seen.add(text)
			}
			if !slices.Equal(seen.order, test.want) {
				t.Errorf("order %q, want %q", seen.order, test.want)
			}
		})
	}
	for _, count := range []int{1000, 2000, 4000} {
		t.Run(strconv.Itoa(count)+" distinct texts", func(t *testing.T) {
			seen := newFirstSeen(parser.CountingWork())
			for i := range count {
				seen.add("lc-" + strconv.Itoa(i))
			}
			if seen.work.Count("lookups") != count || len(seen.order) != count {
				t.Errorf("%d lookups for %d texts (%d kept), want one each", seen.work.Count("lookups"), count,
					len(seen.order))
			}
		})
	}
}

func TestASessionFileWithRecordsButNoHeaderIsReported(t *testing.T) {
	const entry = `{"type": "message", "id": "e1", "message": {"role": "user", "content": "hi"}}` + "\n"
	const export = "session-2026-01-01T00-00-00-000Z.jsonl"
	report := zipOf(t, [2]string{"report.json", `{}`}, [2]string{"diagnostics.json", `{}`},
		[2]string{"session.jsonl", entry})
	tests := []struct {
		name, rel string
		data      []byte
		want      []string
	}{
		{"a session file", ".pi/agent/sessions/--w--/2026_s1.jsonl", []byte(entry),
			[]string{"parser: .pi/agent/sessions/--w--/2026_s1.jsonl has no session header"}},
		{"an empty session file", ".pi/agent/sessions/--w--/2026_s2.jsonl", nil, nil},
		{"a JSONL export", export, []byte(entry), []string{"parser: " + export + " has no session header"}},
		{"a bug report's session", "pi-bug-report-1.zip", report,
			[]string{"parser: pi-bug-report-1.zip member session.jsonl has no session header"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(test.rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + test.rel, Rel: test.rel,
				Home: &input.Home{DiskPath: home, Original: "/h"}}
			var problems []string
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
				if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
					problems = append(problems, row.Text)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(problems, test.want) {
				t.Errorf("problem rows %q, want %q", problems, test.want)
			}
		})
	}
}
