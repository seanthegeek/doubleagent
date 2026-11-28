package twinny

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The files the workspace tests write, under VS Code's user directory.
const (
	codeUser        = ".config/Code/User/"
	codeGlobal      = codeUser + "globalStorage/state.vscdb"
	codeWorkspace   = codeUser + "workspaceStorage/abc123/state.vscdb"
	workspaceFolder = codeUser + "workspaceStorage/abc123/workspace.json"
	openID          = "11111111-2222-3333-4444-555555555555"
)

// stateDatabase returns a state.vscdb whose rjmacarthy.twinny item holds
// value.
func stateDatabase(t *testing.T, value string) []byte {
	t.Helper()
	data, err := sqlitetest.Build(sqlitetest.Table{Name: "ItemTable",
		SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)", Rows: [][]any{{itemKey, value}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeHome writes each file, by path relative to a new home, and returns
// the home.
func writeHome(t *testing.T, files map[string][]byte) *input.Home {
	t.Helper()
	home := t.TempDir()
	for rel, data := range files {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &input.Home{DiskPath: home, Repeats: &jsonvalue.Repeats{}}
}

// parseRows returns the rows subject gives for the file at rel in home.
func parseRows(t *testing.T, subject *Parser, home *input.Home, rel string) []timeline.Row {
	t.Helper()
	artifact := &input.Artifact{DiskPath: filepath.Join(home.DiskPath, filepath.FromSlash(rel)), Original: "/h/" + rel,
		Rel: rel, Home: home}
	var rows []timeline.Row
	if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// openConversationJSON returns an open conversation of the id holding the
// messages.
func openConversationJSON(identifier, messages string) string {
	return `{"id":"` + identifier + `","title":"New conversation","messages":` + messages + `}`
}

// rowLines returns each row as its session id, project path and text.
func rowLines(rows []timeline.Row) []string {
	var lines []string
	for _, row := range rows {
		lines = append(lines, row.SessionID+" | "+row.ProjectPath+" | "+row.Text)
	}
	return lines
}

func TestAWorkspaceOpenConversationIsShownWhenNoGlobalRowShowsItsID(t *testing.T) {
	const message = `[{"role":"user","content":"delete the audit logs"}]`
	shown := []string{openID + " | /srv/proj | conversation: New conversation | messages=1 | updatedAt unknown",
		openID + " | /srv/proj | delete the audit logs"}
	folder := []byte(`{"folder":"file:///srv/proj"}`)
	for _, test := range []struct {
		name   string
		global []byte
		open   string
		want   []string
	}{
		{"an id no saved conversation has", stateDatabase(t, `{"twinny.conversations":{}}`),
			openConversationJSON(openID, message), shown},
		{"an id saved globally", stateDatabase(t, `{"twinny.conversations":{"`+openID+`":{"id":"`+openID+
			`","messages":[]}}}`), openConversationJSON(openID, message), nil},
		{"an id saved globally in a list", stateDatabase(t, `{"twinny.conversations":[{"id":"`+openID+`"}]}`),
			openConversationJSON(openID, message), nil},
		{"the id of the global row's open conversation, which the global row shows",
			stateDatabase(t, `{"twinny.active-conversation":`+openConversationJSON(openID, message)+`}`),
			openConversationJSON(openID, message), nil},
		{"the id of the global row's open conversation, which has no messages",
			stateDatabase(t, `{"twinny.active-conversation":`+openConversationJSON(openID, `[]`)+`}`),
			openConversationJSON(openID, message), shown},
		{"no messages", stateDatabase(t, `{}`), openConversationJSON(openID, `[]`), nil},
		{"no messages member", stateDatabase(t, `{}`), `{"id":"` + openID + `"}`, nil},
		{"an open conversation that is not an object", stateDatabase(t, `{}`), `"` + openID + `"`, nil},
		{"no global database", nil, openConversationJSON(openID, message), shown},
		{"a global database that is not SQLite", []byte("not a database"), openConversationJSON(openID, message), shown},
		{"a global value that is not JSON", stateDatabase(t, `{"twinny.conversations":`),
			openConversationJSON(openID, message), shown},
		{"a global value that repeats a member name", stateDatabase(t, `{"twinny.conversations":{},`+
			`"twinny.conversations":{}}`), openConversationJSON(openID, message), shown},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string][]byte{workspaceFolder: folder,
				codeWorkspace: stateDatabase(t, `{"twinny.active-conversation":`+test.open+`}`)}
			if test.global != nil {
				files[codeGlobal] = test.global
			}
			home := writeHome(t, files)
			if got := rowLines(parseRows(t, New(), home, codeWorkspace)); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
			if home.Repeats.Count() != 0 {
				t.Errorf("the workspace parse noted %d repeats of the global database", home.Repeats.Count())
			}
		})
	}
}

func TestTheGlobalRowsOpenConversationIsShownOnceBesideAWorkspaceCopy(t *testing.T) {
	const message = `[{"role":"user","content":"hi"}]`
	home := writeHome(t, map[string][]byte{
		codeGlobal:    stateDatabase(t, `{"twinny.active-conversation":`+openConversationJSON(openID, message)+`}`),
		codeWorkspace: stateDatabase(t, `{"twinny.active-conversation":`+openConversationJSON(openID, message)+`}`)})
	subject := New()
	rows := append(parseRows(t, subject, home, codeGlobal), parseRows(t, subject, home, codeWorkspace)...)
	headings := 0
	for _, row := range rows {
		if strings.HasPrefix(row.Text, "conversation: ") {
			headings++
			if !strings.HasSuffix(row.SourceFile, codeGlobal) {
				t.Errorf("heading %q from %s, want the global database", row.Text, row.SourceFile)
			}
		}
	}
	if headings != 1 {
		t.Errorf("%d conversation headings in %q, want 1", headings, rowLines(rows))
	}
}

func TestAWorkspaceOpenConversationTakesTheFolderOfItsWorkspaceJSON(t *testing.T) {
	for _, test := range []struct {
		name      string
		workspace []byte
		project   string
		problems  []string
	}{
		{"a folder", []byte(`{"folder":"file:///srv/proj"}`), "/srv/proj", nil},
		{"no workspace.json", nil, "", nil},
		{"a workspace.json with no folder", []byte(`{"workspace":"file:///srv/w.code-workspace"}`), "", nil},
		{"a workspace.json that is not an object", []byte(`[]`), "",
			[]string{"parser: workspace.json is not a JSON object"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string][]byte{codeWorkspace: stateDatabase(t, `{"twinny.active-conversation":`+
				openConversationJSON(openID, `[{"role":"user","content":"hi"}]`)+`}`)}
			if test.workspace != nil {
				files[workspaceFolder] = test.workspace
			}
			rows := parseRows(t, New(), writeHome(t, files), codeWorkspace)
			var problems []string
			for _, row := range rows {
				if row.ProjectPath != test.project {
					t.Errorf("row %q has project %q, want %q", row.Text, row.ProjectPath, test.project)
				}
				if strings.HasPrefix(row.Text, parserkit.ProblemPrefix) {
					problems = append(problems, row.Text)
				}
			}
			if len(rows) < 2 || !slices.Equal(problems, test.problems) {
				t.Errorf("rows %q with problems %q, want the conversation and %q", rowLines(rows), problems,
					test.problems)
			}
		})
	}
}

func TestEachEditorsWorkspaceLooksOnlyAtItsOwnGlobalRow(t *testing.T) {
	const cursorUser = ".config/Cursor/User/"
	open := stateDatabase(t, `{"twinny.active-conversation":`+
		openConversationJSON(openID, `[{"role":"user","content":"hi"}]`)+`}`)
	home := writeHome(t, map[string][]byte{
		codeGlobal:                               stateDatabase(t, `{"twinny.conversations":{"`+openID+`":{"id":"`+openID+`"}}}`),
		codeWorkspace:                            open,
		cursorUser + "globalStorage/state.vscdb": stateDatabase(t, `{"twinny.conversations":{}}`),
		cursorUser + "workspaceStorage/abc123/state.vscdb": open})
	subject := New()
	if rows := parseRows(t, subject, home, codeWorkspace); len(rows) != 0 {
		t.Errorf("VS Code's workspace gives %q, want nothing: VS Code's global row saved the id", rowLines(rows))
	}
	if rows := parseRows(t, subject, home, cursorUser+"workspaceStorage/abc123/state.vscdb"); len(rows) != 2 {
		t.Errorf("Cursor's workspace gives %q, want the conversation: Cursor's global row did not save it",
			rowLines(rows))
	}
}

func TestManyWorkspacesReadTheGlobalDatabaseOnce(t *testing.T) {
	const workspaces = 20
	files := map[string][]byte{codeGlobal: stateDatabase(t, `{"twinny.conversations":{}}`)}
	for i := range workspaces {
		files[fmt.Sprintf("%sworkspaceStorage/w%d/state.vscdb", codeUser, i)] = stateDatabase(t,
			`{"twinny.active-conversation":`+openConversationJSON(fmt.Sprint(i), `[{"role":"user","content":"hi"}]`)+`}`)
	}
	files[codeUser+"workspaceStorage/none/state.vscdb"] = stateDatabase(t, `{}`)
	home := writeHome(t, files)
	subject := New()
	subject.Work = parserkit.CountingWork()
	if rows := parseRows(t, subject, home, codeUser+"workspaceStorage/none/state.vscdb"); len(rows) != 0 ||
		subject.Work.Count("global state reads") != 0 {
		t.Errorf("a workspace with no open conversation gave %q and read the global database %d times, want neither",
			rowLines(rows), subject.Work.Count("global state reads"))
	}
	for i := range workspaces {
		if rows := parseRows(t, subject, home, fmt.Sprintf("%sworkspaceStorage/w%d/state.vscdb", codeUser, i)); len(rows) != 2 {
			t.Fatalf("workspace %d gives %q, want its conversation", i, rowLines(rows))
		}
	}
	if reads := subject.Work.Count("global state reads"); reads != 1 {
		t.Errorf("%d workspaces read the global database %d times, want once", workspaces, reads)
	}
}

func TestAWorkspaceRowGivesItsOpenConversationAndItsReviewRepository(t *testing.T) {
	home := writeHome(t, map[string][]byte{codeWorkspace: stateDatabase(t, `{"twinny.active-conversation":`+
		openConversationJSON(openID, `[{"role":"user","content":"hi"}]`)+
		`,"twinny-workspace-context-reviewOwner":"acme","twinny-workspace-context-reviewRepo":"widgets"}`)})
	got := rowLines(parseRows(t, New(), home, codeWorkspace))
	if len(got) != 3 || !strings.Contains(got[2], "source host repository: https://github.com/acme/widgets type=review") {
		t.Errorf("rows %q, want the conversation's two and the review repository", got)
	}
}
