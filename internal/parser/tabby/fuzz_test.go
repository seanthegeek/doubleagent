package tabby

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
)

func FuzzParseXML(fuzz *testing.F) {
	for _, seed := range []string{"<a><b value=\"v\"/></a>", "<a>", "<a x='1' x='2'/>", "<?xml version=\"1.0\"?><a/>"} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if root, err := parseXML(data); err == nil {
			if got, want := historyCommands(root, parser.Work{}), commandsByWalk(root); !slices.Equal(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		}
		_ = urlHostname(string(data))
	})
}
