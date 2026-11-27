package letta

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseTranscriptOf writes files under a new home, each at its name, and
// returns the rows the parser gives for the transcript at rel, parsed by
// parse (a function of package parsertest) in a run that shows secrets as
// setting says.
func parseTranscriptOf(t *testing.T, setting secrets.Setting, files map[string]string, rel string,
	parse func(testing.TB, parserkit.Parser, *input.Artifact, parserkit.Options, parserkit.Emit) error) []string {
	t.Helper()
	home := t.TempDir()
	for name, text := range files {
		path := filepath.Join(home, filepath.FromSlash(name))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Original: "/home/u/" + rel, Rel: rel, Agent: "letta", Manifest: jsonvalue.NewNull(),
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)}}
	var got []string
	err := parse(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestATranscriptsServerShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".letta/transcripts/agent-1/conv-1/transcript.jsonl"
	files := map[string]string{
		settingsRel: `{"agents": [{"agentId": "agent-1", "baseUrl": "https://tok:s3cret@letta.example/"}]}`,
		rel:         `{"captured_at": "2026-10-01T09:00:00Z"}` + "\n",
	}
	tests := []struct {
		name    string
		setting secrets.Setting
		server  string
	}{
		{"shown by default", secrets.Setting{}, "https://tok:s3cret@letta.example/"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@letta.example/"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@letta.example/"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://letta.example/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseTranscriptOf(t, test.setting, files, rel, parsertest.Parse)
			want := "session synced: server conversation conv-1 agent agent-1 server " + test.server + " "
			if len(got) == 0 || !strings.HasPrefix(got[0], want) {
				t.Errorf("got %q, want a first row starting %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
				t.Errorf("a withheld run shows the password: %q", got)
			}
		})
	}
}

// serverTranscripts parses each of n transcripts of an agent whose
// settings.json server is server, by parse under setting, checks that each
// synced row holds shown and no withheld password, and returns the bytes
// of the rows' text, the problems of the server's cut and the rows holding
// the server whole.
func serverTranscripts(t *testing.T, setting secrets.Setting, server string, n int,
	parse func(testing.TB, parserkit.Parser, *input.Artifact, parserkit.Options, parserkit.Emit) error,
	shown string) (int, int, int) {
	t.Helper()
	files := map[string]string{settingsRel: `{"agents": [{"agentId": "agent-1", "baseUrl": "` + server + `"}]}`}
	for i := range n {
		files[fmt.Sprintf(".letta/transcripts/agent-1/conv-%d/transcript.jsonl", i)] =
			`{"captured_at": "2026-10-01T09:00:00Z"}` + "\n"
	}
	size, problems, whole := 0, 0, 0
	for i := range n {
		rel := fmt.Sprintf(".letta/transcripts/agent-1/conv-%d/transcript.jsonl", i)
		for _, text := range parseTranscriptOf(t, setting, files, rel, parse) {
			size += len(text)
			if text == parserkit.ProblemPrefix+parserkit.CarriedProblem("server", 1) {
				problems++
			}
			if strings.Contains(text, server) {
				whole++
			}
			if strings.HasPrefix(text, "session synced:") && !strings.Contains(text, shown) {
				t.Errorf("the synced row does not hold %q: %.120q", shown, text)
			}
			if setting.Withhold && strings.Contains(text, "s3cret") {
				t.Errorf("a withheld run shows the password: %.120q", text)
			}
		}
	}
	return size, problems, whole
}

// A server's URL comes from settings.json and every transcript of the
// agent repeats it, so a value over the limit is cut in each transcript's
// synced row, with one problem for the transcript, and twice the
// transcripts cost twice the bytes, never the value's length again. The
// cut keeps a URL's credentials only where the kept part holds the whole
// URL, and they are then shown or withheld as the run says; a URL longer
// than the limit is cut after its scheme, so no credential of it is written.
func TestALongServerIsCutInEachTranscriptWithItsCredentialsStillFound(t *testing.T) {
	tail := strings.Repeat("a", 10_000)
	tests := []struct {
		name, server string
		shown        func(mode secretMode) string
		parse        func(testing.TB, parserkit.Parser, *input.Artifact, parserkit.Options, parserkit.Emit) error
	}{
		{"a URL followed by long text keeps the URL", "https://ci:s3cret@letta.example/ " + tail,
			func(mode secretMode) string { return " server " + mode.scheme + "letta.example/ aaa" }, parsertest.Parse},
		{"a URL longer than the limit is cut after its scheme", "https://ci:s3cret@letta.example/" + tail,
			func(secretMode) string { return " server https:// [cut from 10032 bytes]" }, parsertest.ParseWithoutSecrets},
	}
	for _, test := range tests {
		for _, mode := range secretModes() {
			t.Run(test.name+" "+mode.name, func(t *testing.T) {
				var sizes []int
				for _, n := range []int{20, 40} {
					size, problems, whole := serverTranscripts(t, mode.setting, test.server, n, test.parse,
						test.shown(mode))
					sizes = append(sizes, size)
					if problems != n || whole != 0 {
						t.Errorf("%d transcripts gave %d problems and %d whole servers, want %d and 0", n, problems,
							whole, n)
					}
				}
				if growth := sizes[1] - sizes[0]; growth > 20*(2*parserkit.CarriedLimit+1024) {
					t.Errorf("20 more transcripts cost %d bytes", growth)
				}
			})
		}
	}
}
