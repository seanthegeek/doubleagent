package regtext

import "testing"

func TestDecode(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"an embedded NUL stays and only the terminating one goes",
			[]byte("C\x00:\x00\\\x00a\x00\x00\x00b\x00\x00\x00"), "C:\\a\x00b"},
		{"data without a terminating NUL keeps every code unit",
			[]byte("a\x00b\x00"), "ab"},
		{"only one of two trailing NULs is removed",
			[]byte("a\x00\x00\x00\x00\x00"), "a\x00"},
		{"an odd last byte is a code unit with a zero high byte",
			[]byte("a\x00b"), "ab"},
		{"empty data is empty text", nil, ""},
		{"a lone terminating NUL is empty text", []byte{0, 0}, ""},
		{"a code unit above ASCII decodes to its UTF-8 bytes",
			[]byte{0xe9, 0x00, 0x00, 0x00}, "\u00e9"},
		{"a surrogate pair decodes to one character",
			[]byte{0x3d, 0xd8, 0x00, 0xde, 0x00, 0x00}, "\U0001f600"},
		{"a lone surrogate becomes the replacement character",
			[]byte{0x3d, 0xd8}, "\ufffd"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := Decode(test.data); got != test.want {
				t.Errorf("Decode(%v) = %q, want %q", test.data, got, test.want)
			}
		})
	}
}

func TestName(t *testing.T) {
	cases := []struct {
		name  string
		units []uint16
		want  string
	}{
		{"an embedded NUL and what follows it stay", []uint16{'A', 0, 'x'}, "A\x00x"},
		{"a trailing NUL stays, since the name is counted", []uint16{'A', 0}, "A\x00"},
		{"a name of one NUL is that NUL", []uint16{0}, "\x00"},
		{"no units is the empty name", nil, ""},
		{"a surrogate pair decodes to one character", []uint16{0xd83d, 0xde00}, "\U0001f600"},
		{"a lone surrogate becomes the replacement character", []uint16{0xd83d}, "\ufffd"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := Name(test.units); got != test.want {
				t.Errorf("Name(%v) = %q, want %q", test.units, got, test.want)
			}
		})
	}
}
