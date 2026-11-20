package opencode

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// spellingStorage is the legacy tree's storage directory below the home.
const spellingStorage = ".local/share/opencode/storage"

// requireAnyCase skips the test unless the disk under home opens the file
// at rel for spelled, a case spelling of rel, as macOS and Windows do.
func requireAnyCase(t *testing.T, home, spelled string) {
	t.Helper()
	if _, err := osfile.Lstat(filepath.Join(home, filepath.FromSlash(spelled))); err != nil {
		t.Skip("this disk tells names apart by case, so no two spellings open one file")
	}
}

// parseRels parses each file of rels under home with one parser and
// returns the rows' texts and the parser's work.
func parseRels(t *testing.T, home string, rels []string) ([]string, parser.Work) {
	t.Helper()
	subject := New()
	subject.Work = parser.CountingWork()
	var texts []string
	for _, rel := range rels {
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
			Original: "/home/u/" + rel, Home: &input.Home{DiskPath: home}}
		if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
			texts = append(texts, row.Text)
		}); err != nil {
			t.Fatal(err)
		}
	}
	return texts, subject.Work
}

// Message files whose ids are case spellings of one id name one part
// directory on a disk that matches names in any case, so its parts are
// shown once, with the first file, however many spellings name it; ids
// that name two directories show both.
func TestEverySpellingOfOneMessageIDShowsItsPartsOnce(t *testing.T) {
	tests := []struct {
		name      string
		ids       []string
		parts     []string
		anyCase   bool
		wantParts int
	}{
		{"four spellings of one id show its parts once", []string{"msg_abc", "MSG_ABC", "Msg_Abc", "msg_ABC"},
			[]string{"msg_abc"}, true, 3},
		{"two ids show the parts of both", []string{"msg_abc", "msg_def"}, []string{"msg_abc", "msg_def"}, false, 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			var rels []string
			for i, messageID := range test.ids {
				rel := fmt.Sprintf("%s/message/ses_x/m%d.json", spellingStorage, i)
				rels = append(rels, rel)
				files[rel] = `{"id": "` + messageID + `", "sessionID": "ses_x", "role": "assistant"}`
			}
			for _, messageID := range test.parts {
				for j := range 3 {
					files[fmt.Sprintf("%s/part/%s/prt_%d.json", spellingStorage, messageID, j)] =
						`{"type": "text", "text": "part ` + messageID + `"}`
				}
			}
			home := writeTree(t, files)
			if test.anyCase {
				requireAnyCase(t, home, spellingStorage+"/part/"+test.ids[1])
			}
			texts, _ := parseRels(t, home, rels)
			parts, repeats := 0, 0
			for _, text := range texts {
				switch {
				case strings.HasPrefix(text, "part "):
					parts++
				case strings.Contains(text, "repeats the id of an earlier message file"):
					repeats++
				}
			}
			if parts != test.wantParts || repeats != len(test.ids)-len(test.parts) {
				t.Errorf("%d part rows and %d repeat rows, want %d and %d: %q", parts, repeats, test.wantParts,
					len(test.ids)-len(test.parts), texts)
			}
		})
	}
}

// Sessions naming one project file by case spellings of its id read it
// once on a disk that matches names in any case; two project files are
// both read.
func TestEverySpellingOfOneProjectIDReadsItsFileOnce(t *testing.T) {
	tests := []struct {
		name     string
		projects []string
		files    []string
		anyCase  bool
		reads    int
	}{
		{"four spellings of one project id read its file once", []string{"prj", "PRJ", "Prj", "pRj"},
			[]string{"prj"}, true, 1},
		{"two project ids read both files", []string{"prj", "other"}, []string{"prj", "other"}, false, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			var rels []string
			for i, project := range test.projects {
				rel := fmt.Sprintf("%s/session/%s/ses_%d.json", spellingStorage, project, i)
				rels = append(rels, rel)
				files[rel] = fmt.Sprintf(`{"id": "ses_%d", "projectID": "%s"}`, i, project)
			}
			for _, project := range test.files {
				files[spellingStorage+"/project/"+project+".json"] = `{"worktree": "/work/` + project + `"}`
			}
			home := writeTree(t, files)
			if test.anyCase {
				requireAnyCase(t, home, spellingStorage+"/project/"+test.projects[1]+".json")
			}
			_, work := parseRels(t, home, rels)
			if reads := work.Count("project file reads"); reads != test.reads {
				t.Errorf("the project files were read %d times, want %d", reads, test.reads)
			}
		})
	}
}
