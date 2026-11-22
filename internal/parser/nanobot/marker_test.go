package nanobot

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestAWorkspaceMarkerIsReadOncePerStateOfTheFile(t *testing.T) {
	const lookups = 50
	path := filepath.Join(t.TempDir(), ".workspace")
	if err := osfile.WriteFile(path, []byte(" /srv/a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := New()
	subject.Work = parser.CountingWork()
	var problems parser.Problems
	for i := range lookups {
		if got := subject.marker(path, &problems); got != "/srv/a" {
			t.Fatalf("lookup %d gave %q, want /srv/a", i, got)
		}
	}
	if reads := subject.Work.Count(".workspace reads"); reads != 1 {
		t.Errorf(".workspace read %d times for %d lookups, want once", reads, lookups)
	}
	if err := osfile.WriteFile(path, []byte("/srv/longer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := subject.marker(path, &problems); got != "/srv/longer" {
		t.Errorf("a changed marker gave %q, want its new text", got)
	}
	if got := subject.marker(filepath.Join(filepath.Dir(path), "absent"), &problems); got != "" {
		t.Errorf("an absent marker gave %q, want nothing", got)
	}
	if reads := subject.Work.Count(".workspace reads"); reads != 2 || len(problems) != 0 {
		t.Errorf("%d reads and problems %+v, want 2 and none", reads, problems)
	}
}
