package parserkit

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestReadWorkspaceGivesTheFolderAndWorkspaceFileOrOneProblem(t *testing.T) {
	tests := []struct {
		name, content, folder, workspace string
		problems                         []string
	}{
		{"a folder", `{"folder":"file:///srv/proj"}`, "/srv/proj", "", nil},
		{"a workspace file", `{"workspace":"file:///srv/w.code-workspace"}`, "", "/srv/w.code-workspace", nil},
		{"neither", `{"other":1}`, "", "", nil},
		{"a folder that is a number keeps its text", `{"folder":3}`, "3", "", nil},
		{"not JSON", `{"folder":`, "", "", []string{"workspace.json not read (invalid JSON at line 1, column 11: unexpected end of JSON input)"}},
		{"not an object", `["file:///srv/proj"]`, "", "", []string{"workspace.json is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := writeFile(t, "workspace.json", []byte(test.content))
			var problems Problems
			folder, workspace := ReadWorkspace(&input.Artifact{DiskPath: file}, file, &problems)
			var messages []string
			for _, problem := range problems {
				messages = append(messages, problem.Message)
			}
			if folder != test.folder || workspace != test.workspace || !slices.Equal(messages, test.problems) {
				t.Errorf("got %q, %q, %q; want %q, %q, %q", folder, workspace, messages, test.folder, test.workspace,
					test.problems)
			}
		})
	}
}

func TestReadWorkspaceIsSilentForAFileThatIsAbsentOrNotRegular(t *testing.T) {
	directory := t.TempDir()
	for _, test := range []struct{ name, file string }{
		{"absent", filepath.Join(directory, "workspace.json")},
		{"a directory", directory},
	} {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			folder, workspace := ReadWorkspace(&input.Artifact{DiskPath: test.file}, test.file, &problems)
			if folder != "" || workspace != "" || len(problems) != 0 {
				t.Errorf("got %q, %q, %v; want nothing", folder, workspace, problems)
			}
		})
	}
}

func TestReadWorkspaceNotesARepeatedMemberInTheParseOfTheArtifact(t *testing.T) {
	file := writeFile(t, "workspace.json", []byte(`{"folder":"file:///a","folder":"file:///b"}`))
	repeats := &jsonvalue.Repeats{}
	artifact := &input.Artifact{DiskPath: file, Home: &input.Home{DiskPath: filepath.Dir(file), Repeats: repeats}}
	var problems Problems
	if folder, _ := ReadWorkspace(artifact, file, &problems); folder != "/b" || repeats.Count() != 1 {
		t.Errorf("folder %q with %d repeats, want /b and 1", folder, repeats.Count())
	}
}
