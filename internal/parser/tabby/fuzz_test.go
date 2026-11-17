package tabby

import "testing"

func FuzzParseXML(fuzz *testing.F) {
	for _, seed := range []string{"<a><b value=\"v\"/></a>", "<a>", "<a x='1' x='2'/>", "<?xml version=\"1.0\"?><a/>"} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if root, err := parseXML(data); err == nil {
			_ = root.iter("")
		}
		_ = urlHostname(string(data))
	})
}
