package evidencepath

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFromFileURIReadsEachFormEditorsWrite(t *testing.T) {
	tests := []struct {
		name, uri, want string
		ok              bool
	}{
		{"a POSIX path", "file:///home/u/x", "/home/u/x", true},
		{"a percent escape is decoded", "file:///home/a%20b/%C3%A9", "/home/a b/é", true},
		{"a drive path loses its leading slash", "file:///C:/Users/x", "C:/Users/x", true},
		{"an escaped drive colon is decoded", "file:///c%3A/Users/x", "c:/Users/x", true},
		{"a bare drive keeps its colon", "file:///d:", "d:", true},
		{"the host is dropped", "file://server/share/x", "/share/x", true},
		{"localhost is dropped", "file://LocalHost/C:/x", "C:/x", true},
		{"a drive written as the host is a drive", "file://C:/Users/x", "C:/Users/x", true},
		{"a drive alone written as the host is a drive", "file://d:", "d:", true},
		{"a one-letter host with a port is not a drive", "file://c:80/x", "/x", true},
		{"an escaped NUL is refused", "file:///a%00b", "", false},
		{"an escaped NUL in the opaque form is refused", "file:a%00b", "", false},
		{"a control character is refused", "file:///a\x01b", "", false},
		{"an invalid escape in the opaque form is refused", "file:%zz", "", false},
		{"the opaque form is read", "file:C:/a%20b", "C:/a b", true},
		{"the query and fragment are dropped", "file:///a?q=1#f", "/a", true},
		{"an escaped question mark stays in the path", "file:///a%3Fb", "/a?b", true},
		{"tabs and line breaks are removed", "file:///a\tb\r\nc", "/abc", true},
		{"an invalid byte becomes U+FFFD", "file:///a%FF%FEb", "/a\uFFFD\uFFFDb", true},
		{"a plus sign is not a space", "file:///a+b", "/a+b", true},
		{"no path gives the empty path", "file:", "", true},
		{"another scheme is refused", "https://h/x", "", false},
		{"an upper-case scheme is refused", "FILE:///x", "", false},
		{"plain text is refused", "/home/u/x", "", false},
		{"an invalid escape is refused", "file:///a%zz", "", false},
		{"a malformed host is refused", "file://[x/y", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := FromFileURI(test.uri)
			if got != test.want || ok != test.ok {
				t.Errorf("FromFileURI(%q) = %q, %v; want %q, %v", test.uri, got, ok, test.want, test.ok)
			}
		})
	}
}

func FuzzFromFileURIGivesValidText(fuzz *testing.F) {
	for _, seed := range []string{"file:///c%3A/x", "file://h/a%FF", "file:C:/a", "file:///a%zz", "file:",
		"file://C:/x", "file://localhost/a", "file:///a%00"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, uri string) {
		path, ok := FromFileURI(uri)
		if ok && !utf8.ValidString(path) {
			t.Errorf("FromFileURI(%q) = %q, which is not valid UTF-8", uri, path)
		}
		if ok && strings.IndexByte(path, 0) >= 0 {
			t.Errorf("FromFileURI(%q) = %q, which holds a NUL", uri, path)
		}
	})
}
