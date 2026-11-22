package kilocode

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestAMemoryManifestIsReadOncePerStateOfTheFile(t *testing.T) {
	const digests = 40
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := osfile.WriteFile(path, []byte(`{"canonical": "/srv/a"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := New()
	subject.Work = parser.CountingWork()
	var problems parser.Problems
	for i := range digests {
		if got := subject.manifestRoot(nil, path, &problems); got != "/srv/a" {
			t.Fatalf("digest %d: root %q, want /srv/a", i, got)
		}
	}
	if reads := subject.Work.Count("manifest.json reads"); reads != 1 {
		t.Errorf("%d digests read manifest.json %d times, want once", digests, reads)
	}
	if err := osfile.WriteFile(path, []byte(`[1, 2]`), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if got := subject.manifestRoot(nil, path, &problems); got != "" {
			t.Errorf("a manifest that is not an object gave %q", got)
		}
	}
	if reads := subject.Work.Count("manifest.json reads"); reads != 2 || len(problems) != 2 {
		t.Errorf("%d reads and problems %+v, want 2 reads and the problem once per lookup", reads, problems)
	}
}
