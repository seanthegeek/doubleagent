package analyze

import (
	"flag"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestTheSecretsOptionsSetHowSecretsAreShown(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want secrets.Setting
	}{
		{"secrets are shown by default", []string{"in", "out"}, secrets.Setting{}},
		{"--no-secrets withholds them as REDACTED", []string{"--no-secrets", "in", "out"},
			secrets.Setting{Withhold: true, Placeholder: "REDACTED"}},
		{"--redaction-placeholder names another placeholder",
			[]string{"--no-secrets", "--redaction-placeholder", "[x]", "in", "out"},
			secrets.Setting{Withhold: true, Placeholder: "[x]"}},
		{"an empty placeholder is kept empty", []string{"--redaction-placeholder=", "--no-secrets", "in", "out"},
			secrets.Setting{Withhold: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timeline, _, err := ParseTimeline(test.args)
			if err != nil || timeline.Secrets != test.want {
				t.Errorf("timeline: %+v, %v; want %+v", timeline.Secrets, err, test.want)
			}
			detect, _, err := ParseDetect(test.args[:len(test.args)-1])
			if err != nil || detect.Secrets != test.want {
				t.Errorf("detect: %+v, %v; want %+v", detect.Secrets, err, test.want)
			}
		})
	}
}

func TestAPlaceholderWithoutNoSecretsIsAUsageError(t *testing.T) {
	work := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{"detect", []string{"detect", "--redaction-placeholder", "x", work}},
		{"timeline", []string{"timeline", "--redaction-placeholder", "x", work, filepath.Join(work, "out")}},
		{"an empty placeholder", []string{"detect", "--redaction-placeholder=", work}},
		{"--no-secrets turned off", []string{"detect", "--no-secrets=false", "--redaction-placeholder", "x", work}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(testEnvironment(), test.args...)
			want := "doubleagent " + test.args[0] + ": --redaction-placeholder needs --no-secrets\nUsage: "
			if code != ExitError || stdout != "" || !strings.HasPrefix(stderr, want) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 2 and %q", code, stdout, stderr, want)
			}
		})
	}
	if _, err := osfile.Lstat(filepath.Join(work, "out")); err == nil {
		t.Error("timeline made its output directory before refusing the line")
	}
}

// usageOption finds the option names an option line of a usage text
// starts with: "  -i, --ignore-case" names -i and --ignore-case.
var usageOption = regexp.MustCompile(`(?m)^  (--?[a-z][a-z-]*)(?:, (--[a-z][a-z-]*))?`)

func TestEachUsageNamesEveryOptionItsCommandTakes(t *testing.T) {
	for _, command := range Commands() {
		t.Run(command, func(t *testing.T) {
			var named []string
			for _, match := range usageOption.FindAllStringSubmatch(usageOf(command), -1) {
				for _, name := range match[1:] {
					if name != "" {
						named = append(named, strings.TrimLeft(name, "-"))
					}
				}
			}
			var registered []string
			FlagSet(command).VisitAll(func(option *flag.Flag) { registered = append(registered, option.Name) })
			slices.Sort(named)
			if !slices.Equal(named, registered) {
				t.Errorf("the usage names %q, the command registers %q", named, registered)
			}
		})
	}
}

// secretFiles is a collection holding two credentials the output shows: a
// repository whose remote URL carries a token, and a Hermes remote backend
// whose URL carries a password.
var secretFiles = []collectedFile{
	{"alice", "/srv/proj", "project", ".git/config",
		"[remote \"origin\"]\n\turl = https://x-access-token:ghs_REPO@github.com/o/r.git\n"},
	{"alice", "/srv/proj", "project", ".git/HEAD", "ref: refs/heads/main\n"},
	{"alice", "/home/alice", "hermes", ".config/Hermes/connections.json",
		`{"connections": [{"kind": "remote", "label": "work", "url": "https://alice:s3cret@hermes.example/"}]}`},
}

// secretOutputs runs detect, detect --json and timeline over secretFiles
// with the options given and returns what each printed or wrote, joined.
func secretOutputs(t *testing.T, options ...string) string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1")),
		member("projects", stringArray([]string{"/srv/proj"}))}, secretFiles)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	var outputs []string
	output := filepath.Join(work, "out")
	for _, args := range [][]string{{"detect"}, {"detect", "--json"}, {"timeline"}} {
		line := append(append(args, options...), collection)
		if args[0] == "timeline" {
			line = append(line, output)
		}
		code, stdout, stderr := runCommand(env, line...)
		if code != ExitFound || stderr != "" {
			t.Fatalf("%q exited %d: %s", line, code, stderr)
		}
		outputs = append(outputs, stdout)
	}
	for _, name := range []string{TimelineFile, SessionsFile, InventoryFile} {
		data, err := osfile.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, string(data))
	}
	return strings.Join(outputs, "\n")
}

func TestDetectAndTimelineShowSecretsUnlessWithheld(t *testing.T) {
	tests := []struct {
		name     string
		options  []string
		shown    []string
		withheld []string
	}{
		{"by default both credentials are shown", nil,
			[]string{"remotes: origin https://x-access-token:ghs_REPO@github.com/o/r.git",
				`"https://x-access-token:ghs_REPO@github.com/o/r.git"`,
				"remote backend: remote work https://alice:s3cret@hermes.example/"}, nil},
		{"--no-secrets replaces both with REDACTED", []string{"--no-secrets"},
			[]string{"remotes: origin https://REDACTED@github.com/o/r.git",
				`"https://REDACTED@github.com/o/r.git"`,
				"remote backend: remote work https://REDACTED@hermes.example/"},
			[]string{"ghs_REPO", "x-access-token", "s3cret", "alice:"}},
		{"another placeholder", []string{"--no-secrets", "--redaction-placeholder", "<gone>"},
			[]string{"remotes: origin https://<gone>@github.com/o/r.git",
				"remote backend: remote work https://<gone>@hermes.example/"},
			[]string{"ghs_REPO", "s3cret", "REDACTED"}},
		{"an empty placeholder", []string{"--no-secrets", "--redaction-placeholder", ""},
			[]string{"remotes: origin https://github.com/o/r.git",
				"remote backend: remote work https://hermes.example/"},
			[]string{"ghs_REPO", "s3cret", "REDACTED"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := secretOutputs(t, test.options...)
			for _, text := range test.shown {
				if !strings.Contains(got, text) {
					t.Errorf("the outputs do not hold %q:\n%s", text, got)
				}
			}
			for _, text := range test.withheld {
				if strings.Contains(got, text) {
					t.Errorf("the outputs hold %q:\n%s", text, got)
				}
			}
		})
	}
}

func TestTheSettingReachesEveryHome(t *testing.T) {
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("projects", stringArray([]string{"/srv/proj"}))}, secretFiles)
	found, err := matcher()
	if err != nil {
		t.Fatal(err)
	}
	setting := secrets.Setting{Withhold: true, Placeholder: "p"}
	opened, _, message := openInput(t.Context(), InputOptions{Input: collection, Secrets: setting}, found,
		testEnvironment())
	if opened == nil {
		t.Fatal(message)
	}
	defer func() { _ = opened.Cleanup() }()
	if len(opened.Artifacts) == 0 {
		t.Fatal("no artifacts")
	}
	for _, artifact := range opened.Artifacts {
		if !reflect.DeepEqual(artifact.Home.Secrets, setting) {
			t.Errorf("%s: home setting %+v", artifact.Original, artifact.Home.Secrets)
		}
	}
}
