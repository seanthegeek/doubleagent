package kilocode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// tasksDirectory is the pre-fork extension's tasks directory under a home.
const tasksDirectory = ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/"

// write writes text to path, making its directory.
func write(t testing.TB, path, text string) {
	t.Helper()
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// artifactAt returns the artifact of the file at rel under home.
func artifactAt(home, rel string) *input.Artifact {
	return &input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Home: &input.Home{DiskPath: home}}
}

// taskArtifact returns the artifact of a pre-fork task's history under home.
func taskArtifact(home, taskID string) *input.Artifact {
	return artifactAt(home, tasksDirectory+taskID+"/api_conversation_history.json")
}

// parsed returns the rows one parser gives the files at rels under home,
// in order, with thinking rows kept.
func parsed(t *testing.T, home string, rels ...string) []timeline.Row {
	t.Helper()
	subject := New()
	var rows []timeline.Row
	for _, rel := range rels {
		if err := subject.Parse(artifactAt(home, rel), parserkit.Options{IncludeThinking: true},
			func(row timeline.Row) { rows = append(rows, row) }); err != nil {
			t.Fatal(err)
		}
	}
	return rows
}

// texts returns the text of each row.
func texts(rows []timeline.Row) []string {
	var all []string
	for _, row := range rows {
		all = append(all, row.Text)
	}
	return all
}

func TestAPreForkTaskIsReadAsRooCodesTasksAre(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, tasksDirectory, "_index.json"), `{"entries": [{"id": "t", "ts": 1791021600000,
		"task": "fix tests", "workspace": "/srv/proj", "mode": "code", "tokensIn": 5}]}`)
	write(t, taskArtifact(home, "t").DiskPath, `[{"role": "user", "content": [{"type": "text", "text": "hi"},
		{"type": "text", "text": "there"}]}, {"role": "assistant", "ts": 1791021601000, "content": [
		{"type": "tool_use", "id": "u1", "name": "read_file", "input": {"path": "a.py"}}, {"type": "other"}]},
		{"type": "reasoning", "text": "think"}]`)
	rows := parsed(t, home, tasksDirectory+"_index.json", tasksDirectory+"t/api_conversation_history.json")
	want := []string{"task: fix tests | tokensIn=5 mode=code", "permission mode: code", "hi", "there",
		"a.py", "[other]", "think"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
	for _, row := range rows {
		if row.SessionID != "t" || row.ProjectPath != "/srv/proj" {
			t.Errorf("row %q has session %q and project %q, want t and /srv/proj", row.Text, row.SessionID,
				row.ProjectPath)
		}
	}
	if rows[2].TimestampUTC != "2026-10-03T10:00:00.000Z" || rows[4].TimestampUTC != "2026-10-03T10:00:01.000Z" {
		t.Errorf("times %q and %q, want the index's ts and the record's own", rows[2].TimestampUTC,
			rows[4].TimestampUTC)
	}
}

func TestAHistoryItemStandsForTheIndexEntry(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, tasksDirectory, "_index.json"), `{"entries": [{"id": "t", "workspace": "/index"}]}`)
	write(t, filepath.Join(home, tasksDirectory, "t/history_item.json"), `{"id": "t", "task": "x", "workspace": "/item"}`)
	write(t, taskArtifact(home, "t").DiskPath, `[{"role": "user", "content": "hi"}]`)
	rows := parsed(t, home, tasksDirectory+"_index.json", tasksDirectory+"t/history_item.json",
		tasksDirectory+"t/api_conversation_history.json")
	if got, want := texts(rows), []string{"task: x |", "hi"}; !slices.Equal(got, want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
	if rows[1].ProjectPath != "/item" {
		t.Errorf("project %q, want the item's", rows[1].ProjectPath)
	}
}

func TestAnIndexFileThatIsNotUsableIsReportedByItsOwnParse(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"an index without an entries array", "_index.json", `{"entries": 7}`, "parser: _index.json has no entries array"},
		{"an item that is an array", "t/history_item.json", `[1]`, "parser: history_item.json is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			write(t, filepath.Join(home, tasksDirectory, test.rel), test.text)
			write(t, taskArtifact(home, "t").DiskPath, `[{"role": "user", "content": "hi"}]`)
			got := texts(parsed(t, home, tasksDirectory+test.rel, tasksDirectory+"t/api_conversation_history.json"))
			if want := []string{test.want, "hi"}; !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}

func TestAPreForkTasksEnvironmentBlockIsASystemRow(t *testing.T) {
	home := t.TempDir()
	write(t, taskArtifact(home, "t").DiskPath, `[{"role": "user", "content": [{"type": "text", "text": "fix it"},
		{"type": "text", "text": " <environment_details>\ncwd /srv/proj\n</environment_details>"},
		{"type": "text", "text": "see <environment_details> below"}]}]`)
	var got []string
	for _, row := range parsed(t, home, tasksDirectory+"t/api_conversation_history.json") {
		got = append(got, row.TurnType+" "+row.Text)
	}
	want := []string{"user fix it", "system <environment_details>\ncwd /srv/proj\n</environment_details>",
		"user see <environment_details> below"}
	if !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q", got, want)
	}
}
