package tabby

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The fixed parts of a path match in any ASCII case, as the catalog matches
// them, and the backup's name in row text keeps the spelling the disk gave.
func TestAPathSpelledInAnotherCaseIsRead(t *testing.T) {
	const respelled = ".TABBY/EE/DB.Backup-1.SQLITE"
	data, err := osfile.ReadFile(filepath.Join("testdata", "narrow_tables.db"))
	if err != nil {
		t.Fatal(err)
	}
	want := parseFile(t, ".tabby/ee/db.backup-1.sqlite", string(data))
	for i := range want {
		want[i] = strings.ReplaceAll(want[i], "db.backup-1.sqlite", "DB.Backup-1.SQLITE")
	}
	got := parseFile(t, respelled, string(data))
	if !New().Wants(&input.Artifact{Rel: respelled}) || len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
