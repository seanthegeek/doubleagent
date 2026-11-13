package relocation

import (
	"slices"
	"strings"
	"testing"
)

func TestBoundedNames(t *testing.T) {
	names := []string{"CODEX_HOME", "ENV", "CLAUDE_CONFIG_DIR"}
	cases := []struct {
		name, text string
		powerShell bool
		want       []LineName
	}{
		{"an export names its variable", "export CODEX_HOME=/opt/x\n", false,
			[]LineName{{1, "CODEX_HOME"}}},
		{"a longer name holding a variable is no hint", "export MY_CODEX_HOME_X=/opt/x\n", false, nil},
		{"another case is another variable in a shell file", "codex_home=/opt/x\n", false, nil},
		{"a PowerShell file matches in any case", "$env:codex_home = \"/opt/x\"\n", true,
			[]LineName{{1, "CODEX_HOME"}}},
		{"a PowerShell env: drive prefix gives no ENV hint", "$env:CODEX_HOME = 1\n", true,
			[]LineName{{1, "CODEX_HOME"}}},
		{"an env shebang line gives no ENV hint", "#!/usr/bin/env bash\nexport CODEX_HOME=/opt/x\n",
			false, []LineName{{2, "CODEX_HOME"}}},
		{"a later occurrence can be bounded", "XENV ENV\n", false, []LineName{{1, "ENV"}}},
		{"names come in table order per line", "CLAUDE_CONFIG_DIR CODEX_HOME\nENV\n", false,
			[]LineName{{1, "CODEX_HOME"}, {1, "CLAUDE_CONFIG_DIR"}, {2, "ENV"}}},
		{"a NUL byte is a space", "bplist00\x00\x01<key>CODEX_HOME</key>\n", false,
			[]LineName{{1, "CODEX_HOME"}}},
		{"a NUL byte bounds a name", "\x00ENV\x00\n", false, []LineName{{1, "ENV"}}},
		{"a carriage return bounds a name", "ENV\r\n", false, []LineName{{1, "ENV"}}},
		{"a last line without a line feed counts", "\n\nENV", false, []LineName{{3, "ENV"}}},
		{"a byte above ASCII bounds a name", "\xc3ENV\n", false, []LineName{{1, "ENV"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := BoundedNames(strings.NewReader(testCase.text), names, testCase.powerShell)
			if err != nil || !slices.Equal(got, testCase.want) {
				t.Errorf("got %v, %v; want %v", got, err, testCase.want)
			}
		})
	}
}

func TestKind(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/etc/environment", "environment"},
		{"/r/home/alice/backup/etc/environment", "environment"},
		{"/etc/environment.d/10-agents.conf", "environment.d"},
		{"/home/a/.config/environment.d/sub/x.conf", "environment.d"},
		{"/etc/environment.d/x.txt", ""},
		{"/Library/LaunchAgents/com.example.plist", "launchd"},
		{"/Library/LaunchDaemons/com.example.plist", "launchd"},
		{"/Library/LaunchDaemons/com.example.txt", ""},
		{"/home/a/.bashrc", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			if got := Kind(testCase.path); got != testCase.want {
				t.Errorf("Kind = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestIsPowerShell(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/a/profile.ps1", true}, {"/a/PROFILE.PS1", true}, {"/a/.Ps1", true},
		{"/a/profile.ps1x", false}, {"/a/profileps1", false}, {"ps1", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			if got := IsPowerShell(testCase.path); got != testCase.want {
				t.Errorf("IsPowerShell = %v, want %v", got, testCase.want)
			}
		})
	}
}
