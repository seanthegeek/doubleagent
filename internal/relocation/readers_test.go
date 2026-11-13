package relocation

import (
	"slices"
	"strings"
	"testing"
)

// testNames are the relocation variables the reader tests ask for.
func testNames() []string {
	return []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "HERMES_HOME", "LETTA_DIR", "AMP_HOME"}
}

// acceptCase is one row of the AcceptValue table.
type acceptCase struct {
	name, value, root, accepted, reason string
}

// acceptCases returns the AcceptValue table: the values of the smoke test's
// fixtures and the edges of each rule.
func acceptCases() []acceptCase {
	return []acceptCase{
		{"an absolute path gains the root", "/opt/x", "/r", "/r/opt/x", ""},
		{"a live path keeps no root", "/opt/x", "", "/opt/x", ""},
		{"a trailing slash is dropped", "/opt/d/", "/r", "/r/opt/d", ""},
		{"the root path is the image root", "//", "/r", "/r", ""},
		{"the root path is / live", "/", "", "/", ""},
		{"a dot segment is a path", "/opt/.", "", "/opt/.", ""},
		{"a newline", "/a\nb", "", "", "holds a newline"},
		{"an escape is a control character", "/opt/e\033x", "", "", "holds a control character"},
		{"a carriage return is a control character", "/a\rb", "", "", "holds a control character"},
		{"a form feed is a control character", "/ff\014", "", "", "holds a control character"},
		{"a delete is a control character", "/a\177", "", "", "holds a control character"},
		{"a variable reference", "$XDG_CONFIG_HOME/x", "", "", "references $XDG_CONFIG_HOME"},
		{"a braced reference", "${HOME}/x", "", "", "references $HOME"},
		{"a reference with a default", "${X:-y}", "", "", "references $X"},
		{"a dollar before a digit", "/opt/$1", "", "", "holds a $"},
		{"a doubled dollar", "/a$$HOME", "", "", "holds a $"},
		{"a doubled percent", "/a%%PATH%", "", "", "holds a %"},
		{"a Windows reference", "%APPDATA%\\x", "", "", "references %APPDATA%"},
		{"an unclosed percent", "/a%B", "", "", "holds a %"},
		{"a percent around a space", "/a%B C%", "", "", "holds a %"},
		{"a backquote", "/a`b", "", "", "holds a backquote"},
		{"a double quote", "/a\"b", "", "", "holds a quote"},
		{"a single quote", "/a'b", "", "", "holds a quote"},
		{"a .. segment", "/opt/../x", "", "", "holds a .. segment"},
		{"a final .. segment", "/opt/..", "", "", "holds a .. segment"},
		{"a relative path", "opt/rel", "", "", "relative"},
		{"a dot path", "./cc", "", "", "relative"},
		{"a file URI", "file:///opt/u", "", "", "uri"},
		{"a one-letter scheme is a drive", "C:\\x", "", "", "relative"},
		{"a colon first is a URI to the sh test", ":x", "", "", "uri"},
		{"a tilde", "~/cc", "", "", "not an absolute path"},
		{"a no-break space", "\302\240/nbsp", "", "", "not an absolute path"},
		{"an empty value", "", "", "", "not an absolute path"},
	}
}

func TestAcceptValue(t *testing.T) {
	for _, testCase := range acceptCases() {
		t.Run(testCase.name, func(t *testing.T) {
			accepted, reason := AcceptValue(testCase.value, testCase.root)
			if accepted != testCase.accepted || reason != testCase.reason {
				t.Errorf("AcceptValue(%q, %q) = %q, %q; want %q, %q", testCase.value,
					testCase.root, accepted, reason, testCase.accepted, testCase.reason)
			}
		})
	}
}

// smokeEnvironment is the /etc/environment the smoke test's image holds.
func smokeEnvironment() string {
	return strings.Join([]string{"CODEX_HOME=/opt/x", `CODEX_HOME="/opt/q"`,
		"CODEX_HOME='/opt/single'", "CODEX_HOME=/opt/e\033x", "CODEX_HOME=/a%%PATH%",
		"CODEX_HOME=/a\rb", "CODEX_HOME=/opt/a#b", `CODEX_HOME="/opt/x\"`, `CODEX_HOME=/opt/h\`,
		`LANG=C\`, "CODEX_HOME=/opt/joined", `CLAUDE_CONFIG_DIR=/opt/t\ `, "",
		"HERMES_HOME=/opt/after", "CODEX_HOME=/opt/crlf\r", "CODEX_HOME=/opt/n\x00ul",
		"CODEX_HOME=/opt/afternul"}, "\n") + "\n"
}

func TestResolveEnvironmentReadsTheSmokeFile(t *testing.T) {
	want := []Assignment{
		{1, "CODEX_HOME", "/opt/x", ""}, {2, "CODEX_HOME", "/opt/q", ""},
		{3, "CODEX_HOME", "", "quoted"}, {4, "CODEX_HOME", "/opt/e\033x", ""},
		{5, "CODEX_HOME", "/a%%PATH%", ""}, {6, "CODEX_HOME", "/a\rb", ""},
		{7, "CODEX_HOME", "", "holds a #"}, {8, "CODEX_HOME", `/opt/x\`, ""},
		{9, "CODEX_HOME", "", "continued"}, {11, "CODEX_HOME", "", "continued"},
		{12, "CLAUDE_CONFIG_DIR", "", "continued"}, {14, "HERMES_HOME", "", "continued"},
		{15, "CODEX_HOME", "/opt/crlf\r", ""}, {16, "CODEX_HOME", "", "after a NUL byte"},
		{17, "CODEX_HOME", "", "after a NUL byte"},
	}
	got, err := ResolveEnvironment(strings.NewReader(smokeEnvironment()), testNames())
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("got %#v, %v\nwant %#v", got, err, want)
	}
}

func TestResolveEnvironmentRules(t *testing.T) {
	cases := []struct {
		name, text string
		want       []Assignment
	}{
		{"a comment and an export prefix assign nothing",
			"# CODEX_HOME=/a\nexport CODEX_HOME=/b\n", nil},
		{"leading blanks are skipped", " \tCODEX_HOME=/a", []Assignment{{1, "CODEX_HOME", "/a", ""}}},
		{"a hash on a joined line ends no join", "A=#\\\nCODEX_HOME=/a\n",
			[]Assignment{{2, "CODEX_HOME", "/a", ""}}},
		{"a NUL on a comment line ends the file", "#\x00\nCODEX_HOME=/a\n",
			[]Assignment{{2, "CODEX_HOME", "", "after a NUL byte"}}},
		{"a start-of-heading byte is no NUL", "CODEX_HOME=/a\x01\n",
			[]Assignment{{1, "CODEX_HOME", "/a\x02", ""}}},
		{"one quote is a quote", `CODEX_HOME="`, []Assignment{{1, "CODEX_HOME", "", "quoted"}}},
		{"no name", "=/a\n", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveEnvironment(strings.NewReader(testCase.text), testNames())
			if err != nil || !slices.Equal(got, testCase.want) {
				t.Errorf("got %#v, %v; want %#v", got, err, testCase.want)
			}
		})
	}
}

// smokeValuesConf is the environment.d/values.conf the smoke image holds.
func smokeValuesConf() string {
	return strings.Join([]string{"CODEX_HOME='/opt/s'", "CODEX_HOME=$XDG_CONFIG_HOME/x",
		"CODEX_HOME=opt/rel", "CODEX_HOME=file:///opt/u", "CODEX_HOME=/opt/../x",
		`CODEX_HOME=/opt/\`, "cont/", "# CODEX_HOME=/opt/comment", "CODEX_HOME=/opt/$1",
		"CODEX_HOME=/a$$HOME", "CODEX_HOME=/a${X", "CODEX_HOME=\r/lead", `CODEX_HOME=/a\\`,
		"HERMES_HOME=/b", `CLAUDE_CONFIG_DIR="/c\\`, `d"`, `CODEX_HOME\`, "=/keyjoin",
		`# comment\`, "CODEX_HOME=/aftercomment", "CODEX_HOME=/g1\x01", "CLAUDE_CONFIG_DIR=/g2",
		`CODEX_HOME=/opt/store\ `, "CODEX_HOME=/opt/plain   ", `CODEX_HOME=/opt/even\\ `},
		"\n") + "\n"
}

func TestResolveEnvironmentDReadsTheSmokeFile(t *testing.T) {
	want := []Assignment{
		{1, "CODEX_HOME", "/opt/s", ""}, {2, "CODEX_HOME", "", "references $XDG_CONFIG_HOME"},
		{3, "CODEX_HOME", "opt/rel", ""}, {4, "CODEX_HOME", "file:///opt/u", ""},
		{5, "CODEX_HOME", "/opt/../x", ""}, {6, "CODEX_HOME", "/opt/cont/", ""},
		{9, "CODEX_HOME", "", "holds a $"}, {10, "CODEX_HOME", "", "holds a $"},
		{11, "CODEX_HOME", "", "holds a $"}, {12, "CODEX_HOME", "/lead", ""},
		{13, "CODEX_HOME", `/a\`, ""}, {14, "HERMES_HOME", "/b", ""},
		{15, "CLAUDE_CONFIG_DIR", "", "holds a newline"}, {20, "CODEX_HOME", "/aftercomment", ""},
		{21, "CODEX_HOME", "/g1\x02", ""}, {22, "CLAUDE_CONFIG_DIR", "/g2", ""},
		{23, "CODEX_HOME", "/opt/store ", ""}, {24, "CODEX_HOME", "/opt/plain", ""},
		{25, "CODEX_HOME", `/opt/even\`, ""},
	}
	got, err := ResolveEnvironmentD(strings.NewReader(smokeValuesConf()), testNames(), "")
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("got %#v, %v\nwant %#v", got, err, want)
	}
}

// readerCase is one row of a value reader table.
type readerCase struct {
	name, text, home string
	want             []Assignment
}

// environmentDCases returns the environment.d reader's rules, the smoke
// test's other environment.d files among them.
func environmentDCases() []readerCase {
	quoted := "CODEX_HOME=\"/opt/a\nCLAUDE_CONFIG_DIR=/opt/forged\n\"\n" +
		"HERMES_HOME=\"/opt/x\\\"\nAMP_HOME=/opt/h\n"
	return []readerCase{
		{"$HOME is the home of a home's own file", "CODEX_HOME=$HOME/x\n", "/home/alice",
			[]Assignment{{1, "CODEX_HOME", "/home/alice/x", ""}}},
		{"${HOME} outside a home is a reference", "CODEX_HOME=${HOME}/x\n", "",
			[]Assignment{{1, "CODEX_HOME", "", "references $HOME"}}},
		{"${HOME:-x} is a reference", "CODEX_HOME=${HOME:-/x}\n", "/h",
			[]Assignment{{1, "CODEX_HOME", "", "references $HOME"}}},
		{"a CRLF continuation joins", "CODEX_HOME=/opt/cr\\\r\nlf/\r\n", "",
			[]Assignment{{1, "CODEX_HOME", "/opt/crlf/", ""}}},
		{"a quoted value spanning lines holds a newline and hides what it spans",
			quoted, "", []Assignment{{1, "CODEX_HOME", "", "holds a newline"},
				{4, "HERMES_HOME", "", "quoted"}}},
		{"an escaped closing quote at the end of the file is quoted", `CODEX_HOME="/opt/x\"`, "",
			[]Assignment{{1, "CODEX_HOME", "", "quoted"}}},
		{"a lone carriage return splits no line", "CODEX_HOME=/a\rCLAUDE_CONFIG_DIR=/c\n", "",
			[]Assignment{{1, "CODEX_HOME", "/a\rCLAUDE_CONFIG_DIR=/c", ""}}},
		{"a NUL byte ends the file", "HERMES_HOME=/n\x00ul\nCODEX_HOME=/opt/afternul\n", "",
			[]Assignment{{1, "HERMES_HOME", "", "after a NUL byte"},
				{2, "CODEX_HOME", "", "after a NUL byte"}}},
		{"a byte order mark is part of the name", "\357\273\277CODEX_HOME=/bom\n", "", nil},
		{"a semicolon line is a comment", ";CODEX_HOME=/a\n", "", nil},
		{"blanks around the name are trimmed", "  CODEX_HOME \t= /a\n", "",
			[]Assignment{{1, "CODEX_HOME", "/a", ""}}},
		{"a joined single-quoted value is quoted", "CODEX_HOME='/a\\\nb'\n", "",
			[]Assignment{{1, "CODEX_HOME", "", "quoted"}}},
		{"a stray quote is quoted", "CODEX_HOME=/a\"b\"\n", "",
			[]Assignment{{1, "CODEX_HOME", "", "quoted"}}},
		{"double quotes keep a backslash before a plain character", `CODEX_HOME="/a\b"`, "",
			[]Assignment{{1, "CODEX_HOME", `/a\b`, ""}}},
	}
}

func TestResolveEnvironmentDRules(t *testing.T) {
	for _, testCase := range environmentDCases() {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveEnvironmentD(strings.NewReader(testCase.text), testNames(),
				testCase.home)
			if err != nil || !slices.Equal(got, testCase.want) {
				t.Errorf("got %#v, %v; want %#v", got, err, testCase.want)
			}
		})
	}
}

func TestReadFieldDropsALoneTrailingBar(t *testing.T) {
	cases := []struct{ value, want string }{
		{"/a|", "/a"}, {"/a||", "/a||"}, {"/a|b|", "/a|b|"}, {"/a|b", "/a|b"}, {"|", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.value, func(t *testing.T) {
			if got := readField(testCase.value); got != testCase.want {
				t.Errorf("readField(%q) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}
