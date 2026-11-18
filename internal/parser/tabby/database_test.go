package tabby

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The databases under testdata are written by SQLite from the SQL in
// testdata/fixtures.py.

// parseDatabase copies the named testdata database into a new home as
// Tabby's database and returns the text of every row the parser gives.
func parseDatabase(t *testing.T, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, ".tabby/ee/db.sqlite", string(data))
}

func TestAnIntegrationsAPIBaseIsShownWithoutItsCredentials(t *testing.T) {
	want := []string{"source host: gitlab.example integration work kind=gitlab_self_hosted " +
		"api_base=https://redacted@gitlab.example/api (account not recorded; from integrations table, created_at)"}
	if got := parseDatabase(t, "integrations.db"); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOnlyAThreadOrPageWithoutItsRowIsMissing(t *testing.T) {
	var got []string
	for _, text := range parseDatabase(t, "narrow_tables.db") {
		if strings.HasPrefix(text, "thread start") || strings.HasPrefix(text, "page start") {
			got = append(got, text)
		}
	}
	want := []string{"thread start", "thread start | thread row missing", "page start | title Notes",
		"page start | page row missing"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
