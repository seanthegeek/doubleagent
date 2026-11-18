package cursor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// usagesOf returns usages that each hold one list of scopes.
func usagesOf(t *testing.T, scopeLists ...string) []jsonvalue.Value {
	t.Helper()
	var usages []jsonvalue.Value
	for _, scopes := range scopeLists {
		usage, err := jsonvalue.DecodeString(`{"scopes":` + scopes + `}`)
		if err != nil {
			t.Fatal(err)
		}
		usages = append(usages, usage)
	}
	return usages
}

func TestGithubTextListsEachScopeOnceInTheOrderFirstMet(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   string
	}{
		{"no usage", nil, "-"},
		{"repeats across usages", []string{`["repo","user"]`, `["user","gist","repo"]`}, "repo,user,gist"},
		{"a number and its text are one scope", []string{`[1,"1",true]`}, "1,true"},
		{"scopes that are not a list", []string{`"repo"`}, "-"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := githubText("me", "k", usagesOf(t, test.scopes...), parser.Work{})
			if want := "source host: github account me scopes=" + test.want + " (from state.vscdb key k)"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestGithubTextLooksUpEachScopeOnce(t *testing.T) {
	const size = 4000
	scopes := make([]string, size)
	for i := range scopes {
		scopes[i] = `"s` + strconv.Itoa(i) + `"`
	}
	work := parser.CountingWork()
	got := githubText("me", "k", usagesOf(t, "["+strings.Join(scopes, ",")+"]"), work)
	if lookups := work.Count("scope lookups"); lookups != size || !strings.Contains(got, ",s3999 ") {
		t.Errorf("%d lookups for %d distinct scopes, want one each; text ends %q", lookups, size, got[len(got)-40:])
	}
}
