//go:build parity && unix

package parity

import (
	"slices"
	"strings"
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

func TestTheGuardFindsRowsNamingNoMember(t *testing.T) {
	rows := []string{
		`{"path":"/a","archive_path":"fs/home/a/x","status":"collected"}`,
		`{"path":"/b","archive_path":"fs/home/a/Y","status":"collected"}`,
		`{"path":"/c","archive_path":"","status":"error_copy"}`,
		`{"path":"/d","archive_path":"fs/home/a/q\"uote","status":"collected"}`,
	}
	run := collectorRun{files: map[string]string{".manifest.jsonl": strings.Join(rows, "\n")},
		members: map[string]string{"./fs/home/a/x": "h", "./fs/home/a/y": "h",
			"./fs/home/a/q\"uote": "h"}}
	if got := archivePathsNamingNoMember(run); !slices.Equal(got, []string{"fs/home/a/Y"}) {
		t.Errorf("archivePathsNamingNoMember = %q; want only fs/home/a/Y, whose member is "+
			"spelled otherwise, and no row without an archive path", got)
	}
}
