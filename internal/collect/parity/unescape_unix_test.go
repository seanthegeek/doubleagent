//go:build parity && unix

package parity

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/manifest"
)

func TestUnescapeManifestGivesBackTheBytesTheWriterEscaped(t *testing.T) {
	for _, name := range []string{"fs/home/a/lone\xff\xfe.jsonl", `fs/home/a/quote"and\back`,
		"fs/home/a/tab\tline\nbell\x07", "fs/home/a/café"} {
		if got := unescapeManifest(manifest.EscapeJSON(name)); got != name {
			t.Errorf("unescapeManifest(EscapeJSON(%q)) = %q", name, got)
		}
	}
}
