package parsertest

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// RowsAt writes text as the file at rel, a path relative to the home, in a
// new home under the test's temporary directory, the home of user u on host
// h at /home/u, and returns the rows one parse of it by each gives, whether
// or not each wants the file.
func RowsAt(test testing.TB, each parserkit.Parser, rel, text string) []timeline.Row {
	test.Helper()
	home := test.TempDir()
	path := filepath.Join(home, rel)
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		test.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		test.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: each.Agent(),
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := each.Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		test.Fatal(err)
	}
	return rows
}
