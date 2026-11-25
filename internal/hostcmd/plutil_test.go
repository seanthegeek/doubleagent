package hostcmd

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// plutilRunner is a Runner standing in for plutil, answering each run by its
// first argument (`-type`, `-extract`); a run it has no answer for fails, as
// plutil does for a missing key.
type plutilRunner struct {
	answers map[string]string
	calls   []string
}

// Output returns the answer for the run's first argument, or an error.
func (runner *plutilRunner) Output(_ context.Context, name string, args ...string) ([]byte,
	error,
) {
	runner.calls = append(runner.calls, args[0])
	answer, ok := runner.answers[args[0]]
	if !ok {
		return nil, &missingError{name}
	}
	return []byte(answer), nil
}

// Available reports that plutil is there.
func (runner *plutilRunner) Available(string) bool { return true }

func TestExtractStringReadsTheTypeThenTheRawText(t *testing.T) {
	cases := []struct {
		name, kind, raw, value, reason string
		found                          bool
		calls                          []string
	}{
		{"a string is its text", "string\n", "/opt/x\n", "/opt/x", "", true,
			[]string{"-type", "-extract"}},
		{"an ampersand and a less-than sign are kept", "string\n", "/a&b<c\n", "/a&b<c", "", true,
			[]string{"-type", "-extract"}},
		{"an empty string is an empty value", "string\n", "\n", "", "", true,
			[]string{"-type", "-extract"}},
		{"a string spanning lines holds a newline", "string\n", "/opt/m\nx\n", "", "holds a newline",
			true, []string{"-type", "-extract"}},
		{"a string ending in a newline holds one", "string\n", "/opt/m\n\n", "", "holds a newline",
			true, []string{"-type", "-extract"}},
		{"a carriage return stays in the text", "string\n", "/a\rb\n", "/a\rb", "", true,
			[]string{"-type", "-extract"}},
		{"an integer is not a string and is not extracted", "integer\n", "5\n", "", "not a string",
			true, []string{"-type"}},
		{"a dictionary is not a string", "dictionary\n", "\n", "", "not a string", true,
			[]string{"-type"}},
		{"a missing key is not found", "", "", "", "", false, []string{"-type"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &plutilRunner{answers: map[string]string{}}
			if testCase.kind != "" {
				runner.answers["-type"] = testCase.kind
				runner.answers["-extract"] = testCase.raw
			}
			value, reason, found := NewPlutil(runner).ExtractString(context.Background(),
				"/p.plist", "UserName")
			if value != testCase.value || reason != testCase.reason || found != testCase.found {
				t.Errorf("got %q, %q, %v; want %q, %q, %v", value, reason, found, testCase.value,
					testCase.reason, testCase.found)
			}
			if !slices.Equal(runner.calls, testCase.calls) {
				t.Errorf("runs %q, want %q", runner.calls, testCase.calls)
			}
		})
	}
}

func TestPlutilRunsTheExpectedCommands(t *testing.T) {
	fake := &Fake{Outputs: map[string][]byte{"plutil": []byte("string\n")}}
	plutil := NewPlutil(fake)
	if !plutil.Available() || !plutil.Lint(context.Background(), "/p.plist") {
		t.Fatal("plutil should be available and lint should pass")
	}
	plutil.ExtractString(context.Background(), "/p.plist", "UserName")
	want := []string{"plutil -lint /p.plist", "plutil -type UserName /p.plist",
		"plutil -extract UserName raw -o - /p.plist"}
	if !slices.Equal(fake.Calls, want) {
		t.Errorf("calls = %q, want %q", fake.Calls, want)
	}
	missing := NewPlutil(&Fake{})
	if missing.Available() || missing.Lint(context.Background(), "/p.plist") {
		t.Error("a host without plutil has none to run")
	}
	if _, _, found := missing.ExtractString(context.Background(), "/p.plist", "UserName"); found {
		t.Error("no value is found without plutil")
	}
}

func TestPlutilOnThisHost(t *testing.T) {
	plutil := NewPlutil(System())
	if !plutil.Available() {
		t.Skip("plutil is not on this host (it ships with macOS only)")
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "agents.plist")
	text := `<plist version="1.0"><dict><key>UserName</key><string>root</string>` +
		`<key>UserName</key><string>svc</string><key>EnvironmentVariables</key><dict>` +
		`<key>A</key><string>/opt/<![CDATA[x]]>y</string><key>B</key><dict/>` +
		`<key>C</key><string>/a&amp;b</string><key>D</key><!-- a > <string>/f</string> -->` +
		`<string>/real</string><key>E</key><string>/a&lt;b</string>` +
		`<key>F</key><string>/opt/m` + "\n" + `x</string><key>G</key><integer>5</integer>` +
		`<key>H</key><string>/opt/t` + "\n" + `</string><key>I</key><string></string>` +
		`</dict></dict></plist>`
	if err := os.WriteFile(path, []byte(text+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		keyPath, value, reason string
		found                  bool
	}{
		{"UserName", "svc", "", true},
		{"EnvironmentVariables.A", "/opt/xy", "", true},
		{"EnvironmentVariables.B", "", "not a string", true},
		{"EnvironmentVariables.C", "/a&b", "", true},
		{"EnvironmentVariables.D", "/real", "", true},
		{"EnvironmentVariables.E", "/a<b", "", true},
		{"EnvironmentVariables.F", "", "holds a newline", true},
		{"EnvironmentVariables.G", "", "not a string", true},
		{"EnvironmentVariables.H", "", "holds a newline", true},
		{"EnvironmentVariables.I", "", "", true},
		{"EnvironmentVariables.Z", "", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.keyPath, func(t *testing.T) {
			value, reason, found := plutil.ExtractString(context.Background(), path, testCase.keyPath)
			if value != testCase.value || reason != testCase.reason || found != testCase.found {
				t.Errorf("got %q, %q, %v", value, reason, found)
			}
		})
	}
	if plutil.Lint(context.Background(), filepath.Join(directory, "missing.plist")) {
		t.Error("lint passes a missing file")
	}
}
