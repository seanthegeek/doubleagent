package opencode

import (
	"fmt"
	"path/filepath"
	"slices"
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
	collectedHome := &input.Home{DiskPath: home}
	for _, rel := range rels {
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
			Original: "/home/u/" + rel, Home: collectedHome}
		if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
			texts = append(texts, row.Text)
		}); err != nil {
			t.Fatal(err)
		}
	}
	return texts, subject.Work
}

// Message files whose ids are spellings of one id the disk opens its part
// directory by (another case, or an accent written as a combining mark)
// name one directory, so its parts are shown once, with the first file,
// however many spellings name it; ids that name two directories show
// both. A spelling the disk tells apart names no directory, and the case
// says so.
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
		{"spellings in accented capitals show the parts once",
			[]string{"msg_\u00e9t\u00e9", "msg_\u00c9T\u00c9", "msg_\u00c9t\u00e9", "MSG_\u00e9T\u00c9"},
			[]string{"msg_\u00e9t\u00e9"}, true, 3},
		{"spellings in Greek capitals show the parts once",
			[]string{"msg_\u03b1\u03b2\u03b3", "msg_\u0391\u0392\u0393", "msg_\u0391\u03b2\u0393"},
			[]string{"msg_\u03b1\u03b2\u03b3"}, true, 3},
		{"a spelling with combining accents shows the parts once", []string{"msg_\u00e9t\u00e9", "msg_e\u0301te\u0301"},
			[]string{"msg_\u00e9t\u00e9"}, true, 3},
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
			for _, messageID := range test.ids[1:] {
				if test.anyCase {
					requireAnyCase(t, home, spellingStorage+"/part/"+messageID)
				}
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

// Message files of the first layout whose session ids are case spellings
// of one id read its info file once on a disk that matches names in any
// case, and all take its directory; two sessions read both files.
func TestEverySpellingOfOneSessionIDReadsItsInfoFileOnce(t *testing.T) {
	const oldSessions = ".local/share/opencode/project/p/storage/session"
	tests := []struct {
		name     string
		sessions []string
		infos    []string
		anyCase  bool
		reads    int
	}{
		{"four spellings of one session id read its info file once", []string{"ses_a", "SES_A", "Ses_A", "sEs_a"},
			[]string{"ses_a"}, true, 1},
		{"two session ids read both info files", []string{"ses_a", "ses_b"}, []string{"ses_a", "ses_b"}, false, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			var rels []string
			for i, sessionID := range test.sessions {
				rel := fmt.Sprintf("%s/message/%s/m%d.json", oldSessions, sessionID, i)
				rels = append(rels, rel)
				files[rel] = fmt.Sprintf(`{"id": "m%d", "sessionID": "%s", "role": "user"}`, i, sessionID)
			}
			for _, sessionID := range test.infos {
				files[oldSessions+"/info/"+sessionID+".json"] = `{"directory": "/work/` + sessionID + `"}`
			}
			home := writeTree(t, files)
			if test.anyCase {
				requireAnyCase(t, home, oldSessions+"/info/"+test.sessions[1]+".json")
			}
			_, work := parseRels(t, home, rels)
			if reads := work.Count("quiet side reads"); reads != test.reads {
				t.Errorf("the info files were read %d times, want %d", reads, test.reads)
			}
		})
	}
}

// A parser value that outlives one collection, as a test's or a long
// run's may, meets the next collection's directories afresh: a set of the
// part directories shown lasts one home, so a directory of a new home is
// never taken for one shown before, even where the disk gives it the same
// number, which a deleted directory's number may be given again.
func TestANewHomeShowsThePartsOfAnIDAnEarlierHomeShowed(t *testing.T) {
	files := map[string]string{}
	rels := []string{spellingStorage + "/message/ses_x/m0.json", spellingStorage + "/message/ses_x/m1.json"}
	for _, rel := range rels {
		files[rel] = `{"id": "msg_abc", "sessionID": "ses_x", "role": "assistant"}`
	}
	files[spellingStorage+"/part/msg_abc/prt_0.json"] = `{"type": "text", "text": "part msg_abc"}`
	home := writeTree(t, files)
	subject := New()
	tests := []struct{ name, rel string }{
		{"the first home shows the parts", rels[0]},
		{"a second home over the same directory shows them again", rels[1]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(test.rel)), Rel: test.rel,
				Original: "/home/u/" + test.rel, Home: &input.Home{DiskPath: home}}
			var texts []string
			if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
				texts = append(texts, row.Text)
			}); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(texts, "part msg_abc") {
				t.Errorf("rows %q, want the part", texts)
			}
		})
	}
}
