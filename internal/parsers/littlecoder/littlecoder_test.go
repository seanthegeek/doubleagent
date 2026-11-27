package littlecoder

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every file kind are checked by the golden output of the
// analysis commands (internal/analyze), over inputs that hold every parsed
// agent; these tests cover the patterns that recover a cut file and the
// routing by path.

func TestRecoveredKeysTakeJSONWhitespaceOnly(t *testing.T) {
	subject := New()
	patterns := []struct {
		name     string
		pattern  *regexp.Regexp
		template string
	}{
		{"sessions", subject.sessionsKey, `"sessions"%s:%s[`},
		{"messages", subject.messagesKey, `"messages"%s:%s[`},
		{"session_id", subject.sessionID, `"session_id"%s:%s"s1"`},
		{"saved_at", subject.savedAt, `"saved_at"%s:%s"t"`},
	}
	for _, test := range patterns {
		t.Run(test.name, func(t *testing.T) {
			if text := fmt.Sprintf(test.template, " \r\n", "\t"); !test.pattern.MatchString(text) {
				t.Errorf("%q does not match", text)
			}
			for _, space := range []string{" ", " ", "\u0085"} {
				if text := fmt.Sprintf(test.template, space, ""); test.pattern.MatchString(text) {
					t.Errorf("%q matches", text)
				}
			}
		})
	}
}

func TestWantsLittleCodersOwnFilesAndMarksTheCopiesSecondary(t *testing.T) {
	tests := []struct {
		rel             string
		want, secondary bool
	}{
		{".pi/agent/little-coder-prompt-history.json", true, false},
		{".little-coder/checkpoints/2026-10-01T11-00-00-000Z_0b9e.jsonl/src_a.ts", true, false},
		{".little-coder/checkpoints/2026-10-01T11-00-00-000Z_0b9e.jsonl/sub/src_a.ts", false, false},
		{".little-coder/checkpoints/s1/snapshots.json", true, false},
		{".little-coder/sessions/daily/2026-01-01/s1.json", true, false},
		{".little-coder/sessions/history.json", true, true},
		{".little-coder/sessions/mr_sessions/session_latest.json", true, true},
		{".little-coder/input_history.txt", true, true},
		{".little-coder/config.json", true, false},
		{".little-coder/sessions/notes.txt", false, false},
		{".pi/agent/sessions/--p--/a_b.jsonl", false, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.want {
				t.Errorf("wants %v, want %v", got, test.want)
			}
			if got := subject.Secondary(artifact); got != test.secondary {
				t.Errorf("secondary %v, want %v", got, test.secondary)
			}
		})
	}
}

func TestImageCountCountsTheItemsOfTheImagesField(t *testing.T) {
	tests := []struct {
		json  string
		count int
		ok    bool
	}{
		{`["a.png","b.png"]`, 2, true},
		{`[]`, 0, true},
		{`{"a":1}`, 1, true},
		{`"ab€"`, 3, true},
		{`null`, 0, false},
		{`3`, 0, false},
		{`true`, 0, false},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if count, ok := imageCount(value); count != test.count || ok != test.ok {
				t.Errorf("got %d, %v; want %d, %v", count, ok, test.count, test.ok)
			}
		})
	}
}

// writeFiles writes each file of files, by path relative to home, empty.
func writeFiles(t *testing.T, home string, rels []string) {
	t.Helper()
	for _, rel := range rels {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestACheckpointDirectoryListsItsFilesOnceOrTwice(t *testing.T) {
	const directory = ".little-coder/checkpoints/2026-01-01T00-00-00-000Z_s1.jsonl/"
	for _, test := range []struct {
		name     string
		reversed bool
	}{{"first file first", false}, {"first file last", true}} {
		t.Run(test.name, func(t *testing.T) {
			var rels []string
			for i := range 200 {
				rels = append(rels, directory+fmt.Sprintf("file%03d.py", i))
			}
			home := t.TempDir()
			writeFiles(t, home, rels)
			if test.reversed {
				slices.Reverse(rels)
			}
			subject := New()
			subject.Work = parserkit.CountingWork()
			rows := 0
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
					Rel: rel, Home: &input.Home{DiskPath: home, Original: "/h"}}
				if err := subject.Parse(artifact, parserkit.Options{}, func(timeline.Row) { rows++ }); err != nil {
					t.Fatal(err)
				}
			}
			if rows != 1 {
				t.Errorf("%d rows, want one for the directory", rows)
			}
			if listings := subject.Work.Count("checkpoint listings"); listings == 0 || listings > 2 {
				t.Errorf("the directory was listed %d times, want at most twice", listings)
			}
		})
	}
}

func TestAnAbsenceMarkerIsToldInAnyASCIICase(t *testing.T) {
	directory := t.TempDir()
	writeFiles(t, directory, []string{"a.py.ABSENT", "b.py"})
	if _, listed := describeFiles(regularFiles(directory)); !slices.Equal(listed,
		[]string{"a.py (did not exist)", "b.py"}) {
		t.Errorf("listed %q", listed)
	}
}

// checkpointRows writes the checkpoint file at rel, modified at the epoch,
// under a new home and returns its rows with source_file cleared.
func checkpointRows(t *testing.T, rel string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	writeFiles(t, home, []string{rel})
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.Chtimes(path, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel,
		Home: &input.Home{DiskPath: home, Original: "/h"}}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		row.SourceFile = ""
		rows = append(rows, row)
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

// The fixed parts of a checkpoint's path are matched in any ASCII case,
// and the session id cut from its directory keeps the disk's spelling.
func TestACheckpointSpelledInAnotherCaseGivesTheSameRows(t *testing.T) {
	const set = "/2026-01-01T00-00-00-000Z_Sess_A.jsonl/a.py"
	want := checkpointRows(t, ".little-coder/checkpoints"+set)
	got := checkpointRows(t, ".Little-Coder/CHECKPOINTS"+set)
	if len(want) != 1 || want[0].SessionID != "A" || !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAnArrayEntryOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, rel, text string
		want            []string
	}{
		{"a prompt that is not a string", historyRel, `["first", 5, "", {"a": 1}]`, []string{"first",
			"parser: unparseable record 2 (not a JSON string): 5",
			`parser: unparseable record 4 (not a JSON string): {"a":1}`}},
		{"a snapshot that is not an object", ".little-coder/checkpoints/s1/snapshots.json", `[[1]]`,
			[]string{"parser: unparseable record 1 (not a JSON object): [1]"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(test.rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + test.rel, Rel: test.rel, Home: &input.Home{DiskPath: home}}
			var texts []string
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(texts, test.want) {
				t.Errorf("rows %q, want %q", texts, test.want)
			}
		})
	}
}
