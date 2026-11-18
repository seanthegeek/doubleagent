package antigravity

import "testing"

func TestURIPathReadsFileURIsAndKeepsOtherText(t *testing.T) {
	for _, test := range []struct{ uri, want string }{
		{"", ""},
		{"/plain/path", "/plain/path"},
		{"File:///x", "File:///x"},
		{"file:///home/u/proj", "/home/u/proj"},
		{"file:///c%3A/Users/u/p", "c:/Users/u/p"},
		{"file:///C:/Users/u/p", "C:/Users/u/p"},
		{"file://host/share/x", "/share/x"},
		{"file://localhost/home/u", "/home/u"},
		{"file:/home/u", "/home/u"},
		{"file:home/u", "home/u"},
		{"file:///a%20b/c%2Fd", "/a b/c/d"},
		{"file:///a?q=1#frag", "/a"},
		{"file:///a#f?q", "/a"},
		{"file:///%E2%82", "/\ufffd\ufffd"},
		{"file:///%E2%82%AC", "/\u20ac"},
		{"file:///%zz%4", "file:///%zz%4"},
		{"file:///a%", "file:///a%"},
		{"file:///caf\u00e9/%C3%A9", "/caf\u00e9/\u00e9"},
		{"file:///a\tb\nc\rd", "/abcd"},
		{"file:///a ", "/a "},
		{"file:", ""},
		{"file://", ""},
		{"file:///d:x", "d:x"},
		{"file:///1:/x", "/1:/x"},
		{"file:///%2Fc:/x", "//c:/x"},
		{"file://host", ""},
		{"file://host?x", ""},
		{"file:///\u00e9%C3", "/\u00e9\ufffd"},
		{"file:///%C3\u00e9", "/\ufffd\u00e9"},
		{"file:///%e2%82%ac", "/\u20ac"},
	} {
		t.Run(test.uri, func(t *testing.T) {
			if got := uriPath(test.uri); got != test.want {
				t.Errorf("uriPath(%q) = %q, want %q", test.uri, got, test.want)
			}
		})
	}
}
