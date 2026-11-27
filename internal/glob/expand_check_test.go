package glob

import (
	"github.com/seanthegeek/doubleagent/internal/follow"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// checkTree makes a base holding open/f, open/kept/f and fenced/f,
// fenced/inner/f and fenced/secret/User, and returns it canonical.
func checkTree(t *testing.T) string {
	t.Helper()
	base := syspath.FromNative(t.TempDir())
	for _, file := range []string{"open/f", "open/kept/f", "fenced/f", "fenced/inner/f",
		"fenced/secret/User/x"} {
		if err := os.MkdirAll(filepath.Dir(base+"/"+file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(base+"/"+file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func TestExpandLooksAtNothingTheCheckRefuses(t *testing.T) {
	base := checkTree(t)
	fenced := base + "/fenced"
	var asked []string
	check := func(path string) (follow.Refusal, bool) {
		asked = append(asked, path)
		return follow.Refusal{Link: fenced, Kind: follow.LeadsOut},
			path == fenced || strings.HasPrefix(path, fenced+"/")
	}
	cases := []struct {
		name, pattern string
		want          []string
	}{
		{"a refused directory is not listed", "fenced/*", nil},
		{"a literal name is not looked up in a refused directory", "fenced/f", nil},
		{"a literal tail is not looked up behind a refused directory", "fen*/inner/f", nil},
		{"a name under a refused directory is not listed or looked up", "fenced/*/User", nil},
		{"an allowed directory is still expanded", "open/*", []string{"open/f", "open/kept"}},
		{"an allowed literal tail is still found", "op*/kept/f", []string{"open/kept/f"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			asked = nil
			got, refused := Expand(base, testCase.pattern, check, nil)
			var relative []string
			for _, path := range got {
				relative = append(relative, strings.TrimPrefix(path, base+"/"))
			}
			if !slices.Equal(relative, testCase.want) {
				t.Errorf("matches %q, want %q", relative, testCase.want)
			}
			if wantRefused := testCase.want == nil; wantRefused != (len(refused) == 1) {
				t.Errorf("refusals %+v, want one only when the match was refused", refused)
			}
			for _, refusal := range refused {
				if refusal.Link != fenced || refusal.Pattern != testCase.pattern {
					t.Errorf("refusal %+v, want link %s for %q", refusal, fenced, testCase.pattern)
				}
			}
			if len(asked) == 0 {
				t.Error("the check was never asked")
			}
		})
	}
}

func TestExpandWithoutACheckReadsEverything(t *testing.T) {
	base := checkTree(t)
	got, refused := Expand(base, "fenced/*", nil, nil)
	if len(got) != 3 || len(refused) != 0 {
		t.Errorf("matches %q, refusals %+v; want the three entries and no refusal", got, refused)
	}
}
