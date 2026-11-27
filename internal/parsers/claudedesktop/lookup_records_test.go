package claudedesktop

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// writeText writes text at rel under root, making its directories.
func writeText(t *testing.T, root, rel, text string) {
	t.Helper()
	writeEmpty(t, root, rel)
	if err := osfile.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAnOrgDirIsListedOnceForEveryShortNamedSessionDir(t *testing.T) {
	for _, n := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d short-named session dirs", n), func(t *testing.T) {
			home := t.TempDir()
			for i := range n {
				short := fmt.Sprintf("%08x", i)
				writeEmpty(t, home, cowork+"/"+short+"/audit.jsonl")
				writeText(t, home, cowork+"/local_"+short+"-1111-4222-8333-444444444444.json",
					`{"title": "`+short+`"}`)
			}
			subject := New()
			subject.Work = parserkit.CountingWork()
			for i := range n {
				short := fmt.Sprintf("%08x", i)
				record, found := subject.sessionRecord(&input.Artifact{Home: &input.Home{DiskPath: home}}, cowork+"/"+short)
				if !found || record.Get("title").Text() != short {
					t.Fatalf("session dir %s: got %v, %v", short, record, found)
				}
			}
			if listings := subject.Work.Count("org directory listings"); listings != 1 {
				t.Errorf("%d session dirs made %d listings of their org dir, want 1", n, listings)
			}
		})
	}
}

func TestAShortNameTakesTheFirstOfItsRecordsByName(t *testing.T) {
	first, second := "local_c0c0c0c0-0000-4000-8000-000000000000", "local_c0c0c0c0-ffff-4fff-8fff-ffffffffffff"
	next := "local_c0c0c0c1-1111-4222-8333-444444444444"
	tests := []struct {
		name           string
		records        map[string]string
		want, wantNext string
	}{
		{"two records of one short name", map[string]string{first: `{"title": "first"}`,
			second: `{"title": "second"}`}, "first", ""},
		{"the first is not an object", map[string]string{first: `[1]`, second: `{"title": "second"}`},
			"second", ""},
		{"a name of the prefix that is no session's", map[string]string{"local_c0c0c0c0-x": `{"title": "x"}`,
			second: `{"title": "second"}`}, "second", ""},
		{"each short name its own record", map[string]string{first: `{"title": "first"}`,
			next: `{"title": "next"}`}, "first", "next"},
		{"a record of the other short name only", map[string]string{next: `{"title": "next"}`}, "", "next"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeEmpty(t, home, cowork+"/c0c0c0c0/audit.jsonl")
			writeEmpty(t, home, cowork+"/c0c0c0c1/audit.jsonl")
			for name, text := range test.records {
				writeText(t, home, cowork+"/"+name+".json", text)
			}
			subject := New()
			for short, want := range map[string]string{"c0c0c0c0": test.want, "c0c0c0c1": test.wantNext} {
				record, _ := subject.sessionRecord(&input.Artifact{Home: &input.Home{DiskPath: home}}, cowork+"/"+short)
				if got := record.Get("title").Text(); got != want {
					t.Errorf("%s: got %q, want %q", short, got, want)
				}
			}
		})
	}
}
