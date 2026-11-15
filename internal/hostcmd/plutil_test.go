package hostcmd

import (
	"os"
	"path/filepath"
	"testing"
)

// plutilDocument wraps body in the header and closing lines plutil prints.
func plutilDocument(body string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" " +
		"\"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
		"<plist version=\"1.0\">\n" + body + "\n</plist>\n"
}

func TestExtractedStringReadsTheOneElementDocument(t *testing.T) {
	cases := []struct {
		name, output, value, reason string
	}{
		{"a string is its text", plutilDocument("<string>/opt/x</string>"), "/opt/x", ""},
		{"an empty string is an empty value", plutilDocument("<string></string>"), "", ""},
		{"a string spanning lines holds a newline",
			plutilDocument("<string>/opt/m\n3|HERMES_HOME|=/opt/injected</string>"), "",
			"holds a newline"},
		{"an encoded string holds an XML entity", plutilDocument("<string>/a&amp;b</string>"), "",
			"holds an XML entity"},
		{"a dictionary is not a string", plutilDocument("<dict>\n\t<key>x</key>\n</dict>"), "",
			"not a string"},
		{"an integer is not a string", plutilDocument("<integer>3</integer>"), "", "not a string"},
		{"a carriage return stays in the text", plutilDocument("<string>/a\rb</string>"), "/a\rb", ""},
		{"a one-line document is read by the same cuts", "<string>/x</string>\n", "/x", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, reason := extractedString(testCase.output)
			if value != testCase.value || reason != testCase.reason {
				t.Errorf("got %q, %q; want %q, %q", value, reason, testCase.value, testCase.reason)
			}
		})
	}
}

func TestPlutilRunsTheExpectedCommands(t *testing.T) {
	fake := &Fake{Outputs: map[string][]byte{"plutil": []byte(plutilDocument("<string>svc</string>"))}}
	plutil := NewPlutil(fake)
	if !plutil.Available() || !plutil.Lint("/p.plist") {
		t.Fatal("plutil should be available and lint should pass")
	}
	value, reason, found := plutil.ExtractString("/p.plist", "UserName")
	if !found || value != "svc" || reason != "" {
		t.Errorf("ExtractString = %q, %q, %v", value, reason, found)
	}
	want := []string{"plutil -lint /p.plist", "plutil -extract UserName xml1 -o - /p.plist"}
	if len(fake.Calls) != 2 || fake.Calls[0] != want[0] || fake.Calls[1] != want[1] {
		t.Errorf("calls = %q, want %q", fake.Calls, want)
	}
	missing := NewPlutil(&Fake{})
	if missing.Available() || missing.Lint("/p.plist") {
		t.Error("a host without plutil has none to run")
	}
	if _, _, found := missing.ExtractString("/p.plist", "UserName"); found {
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
		`<string>/real</string></dict></dict></plist>`
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
		{"EnvironmentVariables.C", "", "holds an XML entity", true},
		{"EnvironmentVariables.D", "/real", "", true},
		{"EnvironmentVariables.Z", "", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.keyPath, func(t *testing.T) {
			value, reason, found := plutil.ExtractString(path, testCase.keyPath)
			if value != testCase.value || reason != testCase.reason || found != testCase.found {
				t.Errorf("got %q, %q, %v", value, reason, found)
			}
		})
	}
	if plutil.Lint(filepath.Join(directory, "missing.plist")) {
		t.Error("lint passes a missing file")
	}
}
