package copilotdesktop

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// FuzzDataDatabase reads data.db from any bytes, starting from a database
// of every table the session rows join (testdata/data.db, built by
// testdata/gen_data.py): the parser gives rows or a problem
// row, never a panic, and every row's text is valid UTF-8.
func FuzzDataDatabase(fuzz *testing.F) {
	seed, err := osfile.ReadFile(filepath.Join("testdata", "data.db"))
	if err != nil {
		fuzz.Fatal(err)
	}
	fuzz.Add(seed)
	fuzz.Add([]byte("SQLite format 3\x00"))
	fuzz.Add([]byte{})
	subject := New()
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		home := t.TempDir()
		path := filepath.Join(home, ".copilot", "data.db")
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		artifact := &input.Artifact{DiskPath: path, Original: "/h/.copilot/data.db", Rel: ".copilot/data.db",
			Home: &input.Home{DiskPath: home}}
		err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
			if !utf8.ValidString(row.Text) {
				t.Errorf("row text %q is not valid UTF-8", row.Text)
			}
		})
		if err != nil {
			t.Error(err)
		}
	})
}
