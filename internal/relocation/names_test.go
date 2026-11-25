package relocation

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestBoundedNames(t *testing.T) {
	names := []string{"CODEX_HOME", "ENV", "CLAUDE_CONFIG_DIR"}
	cases := []struct {
		name, text string
		powerShell bool
		want       []lineName
	}{
		{"an export names its variable", "export CODEX_HOME=/opt/x\n", false,
			[]lineName{{1, "CODEX_HOME"}}},
		{"a longer name holding a variable is no hint", "export MY_CODEX_HOME_X=/opt/x\n", false, nil},
		{"another case is another variable in a shell file", "codex_home=/opt/x\n", false, nil},
		{"a PowerShell file matches in any case", "$env:codex_home = \"/opt/x\"\n", true,
			[]lineName{{1, "CODEX_HOME"}}},
		{"a PowerShell env: drive prefix gives no ENV hint", "$env:CODEX_HOME = 1\n", true,
			[]lineName{{1, "CODEX_HOME"}}},
		{"an env shebang line gives no ENV hint", "#!/usr/bin/env bash\nexport CODEX_HOME=/opt/x\n",
			false, []lineName{{2, "CODEX_HOME"}}},
		{"a later occurrence can be bounded", "XENV ENV\n", false, []lineName{{1, "ENV"}}},
		{"names come in table order per line", "CLAUDE_CONFIG_DIR CODEX_HOME\nENV\n", false,
			[]lineName{{1, "CODEX_HOME"}, {1, "CLAUDE_CONFIG_DIR"}, {2, "ENV"}}},
		{"a NUL byte is a space", "bplist00\x00\x01<key>CODEX_HOME</key>\n", false,
			[]lineName{{1, "CODEX_HOME"}}},
		{"a NUL byte bounds a name", "\x00ENV\x00\n", false, []lineName{{1, "ENV"}}},
		{"a carriage return bounds a name", "ENV\r\n", false, []lineName{{1, "ENV"}}},
		{"a last line without a line feed counts", "\n\nENV", false, []lineName{{3, "ENV"}}},
		{"a byte above ASCII bounds a name", "\xc3ENV\n", false, []lineName{{1, "ENV"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := boundedNames(strings.NewReader(testCase.text), names, testCase.powerShell)
			if err != nil || !slices.Equal(got, testCase.want) {
				t.Errorf("got %v, %v; want %v", got, err, testCase.want)
			}
		})
	}
}

// encodeText returns text in UTF-16 (width 2) or UTF-32 (width 4) in order,
// after mark.
func encodeText(mark []byte, width int, order binary.AppendByteOrder, text string) []byte {
	encoded := append([]byte{}, mark...)
	if width == 2 {
		for _, unit := range utf16.Encode([]rune(text)) {
			encoded = order.AppendUint16(encoded, unit)
		}
		return encoded
	}
	for _, char := range text {
		encoded = order.AppendUint32(encoded, uint32(char))
	}
	return encoded
}

func TestDecodedBoundedNames(t *testing.T) {
	names := []string{"CODEX_HOME", "ENV"}
	profile := "$env:CODEX_HOME = 'C:\\x'\r\n# é ENV\r\n"
	both := []lineName{{1, "CODEX_HOME"}, {2, "ENV"}}
	cases := []struct {
		name       string
		text       []byte
		powerShell bool
		want       []lineName
	}{
		{"a UTF-16LE profile with CRLF lines names its variable on line 1",
			encodeText([]byte{0xFF, 0xFE}, 2, binary.LittleEndian, profile), true, both},
		{"a UTF-16BE file is decoded",
			encodeText([]byte{0xFE, 0xFF}, 2, binary.BigEndian, profile), true, both},
		{"a UTF-32LE file is decoded, not read as UTF-16LE",
			encodeText([]byte{0xFF, 0xFE, 0, 0}, 4, binary.LittleEndian, profile), true, both},
		{"a UTF-32BE file is decoded",
			encodeText([]byte{0, 0, 0xFE, 0xFF}, 4, binary.BigEndian, profile), true, both},
		{"a UTF-8 mark is kept and bounds the name after it",
			[]byte("\xEF\xBB\xBFENV=1\nCODEX_HOME=2\n"), false,
			[]lineName{{1, "ENV"}, {2, "CODEX_HOME"}}},
		{"UTF-16 without a mark is read as bytes, as grep finds no name in it",
			encodeText(nil, 2, binary.LittleEndian, "CODEX_HOME=1\n"), false, nil},
		{"a shell file in UTF-16 still matches in its own case only",
			encodeText([]byte{0xFF, 0xFE}, 2, binary.LittleEndian, "codex_home=1\nENV=1"), false,
			[]lineName{{2, "ENV"}}},
		{"a short last unit decodes to a replacement character",
			append(encodeText([]byte{0xFF, 0xFE}, 2, binary.LittleEndian, "ENV"), 'x'), false,
			[]lineName{{1, "ENV"}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodedBoundedNames(bytes.NewReader(test.text), names, test.powerShell)
			if err != nil || !slices.Equal(got, test.want) {
				t.Errorf("got %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestDecodeUnicodeText(t *testing.T) {
	cases := []struct {
		name string
		text []byte
		want string
	}{
		{"a UTF-16LE surrogate pair becomes one character",
			[]byte{0xFF, 0xFE, 0x3D, 0xD8, 0x00, 0xDE}, "\U0001F600"},
		{"a lone UTF-16 surrogate becomes U+FFFD", []byte{0xFE, 0xFF, 0xD8, 0x00}, "\uFFFD"},
		{"a UTF-32 unit past U+10FFFF becomes U+FFFD",
			[]byte{0, 0, 0xFE, 0xFF, 0x00, 0x11, 0x00, 0x00}, "\uFFFD"},
		{"a mark alone decodes to nothing", []byte{0xFF, 0xFE}, ""},
		{"text without a mark is unchanged", []byte("\xFFplain"), "\xFFplain"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := string(decodeUnicodeText(test.text)); got != test.want {
				t.Errorf("decoded %q, want %q", got, test.want)
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
			if got := isPowerShell(testCase.path); got != testCase.want {
				t.Errorf("isPowerShell = %v, want %v", got, testCase.want)
			}
		})
	}
}
