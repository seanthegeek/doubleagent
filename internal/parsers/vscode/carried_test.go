package vscode

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// An auto-approve key of 100,000 bytes stands before each of its n
// members: each row holds it cut, so the output grows by 4,237 bytes per
// member, not by the key's length.
func TestALongAutoApproveKeyIsCutInEachOfItsMembersRows(t *testing.T) {
	const rel = ".config/Code/User/settings.json"
	key := "chat.tools.autoApprove" + strings.Repeat("k", 100_000)
	tests := []struct {
		members, size int
	}{
		{200, 847_407},
		{400, 1_694_807},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.members)+" members", func(t *testing.T) {
			var members []string
			for i := range test.members {
				members = append(members, `"tool`+strconv.Itoa(i)+`": true`)
			}
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			document := `{"` + key + `": {` + strings.Join(members, ", ") + `}}`
			if err := osfile.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel,
				Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
			var rows []timeline.Row
			size := 0
			if err := parsertest.ParseWithoutSecrets(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) {
				rows = append(rows, row)
				for _, field := range row.Strings() {
					size += len(field)
				}
			}); err != nil {
				t.Fatal(err)
			}
			problem := parserkit.ProblemPrefix + parserkit.CarriedProblem("settings key", 1)
			if size != test.size || len(rows) != test.members+1 || rows[len(rows)-1].Text != problem {
				t.Errorf("%d rows of %d bytes, last %q; want %d rows of %d bytes and %q", len(rows), size,
					rows[len(rows)-1].Text, test.members+1, test.size, problem)
			}
		})
	}
}
