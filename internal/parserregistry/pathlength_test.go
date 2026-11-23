package parserregistry

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// writeText writes text at path, making the directories above it.
func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// collectedBytes opens the collection at dir and parses it as a run does,
// returning the bytes of every string of its rows and of its problem lines.
func collectedBytes(t *testing.T, dir string) (rows, problems int) {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := catalog.NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := input.Open(t.Context(), dir, input.Options{Matcher: matcher})
	if err != nil {
		t.Fatal(err)
	}
	result, err := New().Collect(t.Context(), collection, parserkit.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range result.Primary {
		for _, field := range row.Strings() {
			rows += len(field)
		}
	}
	for _, line := range result.Problems {
		problems += len(line)
	}
	return rows, problems
}

// A settings file whose path in the manifest is 10,000 bytes, a path each
// of its rows would write in its source file and its text, is not read:
// the collection gives one problem line, whatever the rows the file holds.
func TestAFileWhosePathIsLongerThanTheLimitGivesOneProblemAndNoRows(t *testing.T) {
	const prefix = "/home/a/.config/Code/User/profiles/"
	path := prefix + strings.Repeat("p", 10_000-len(prefix)-len("/settings.json")) + "/settings.json"
	for _, members := range []int{100, 200} {
		t.Run(strconv.Itoa(members)+" settings", func(t *testing.T) {
			dir := t.TempDir()
			var entries []string
			for i := range members {
				entries = append(entries, `"tool`+strconv.Itoa(i)+`": true`)
			}
			writeText(t, filepath.Join(dir, "manifest.jsonl"), `{"user": "a", "home": "/home/a", "agent": "vscode", `+
				`"path": "`+path+`", "archive_path": "fs/settings.json", "type": "file", "status": "collected", `+
				`"size": 2}`+"\n")
			writeText(t, filepath.Join(dir, "fs", "settings.json"),
				`{"chat.tools.autoApprove": {`+strings.Join(entries, ", ")+`}}`)
			rows, problems := collectedBytes(t, dir)
			if want := len(path) + len(": a path longer than 4096 bytes, not read"); rows != 0 || problems != want {
				t.Errorf("rows of %d bytes and problems of %d, want none and %d", rows, problems, want)
			}
		})
	}
}
