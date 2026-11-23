package parserkit

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// plainDecodes are the reviewed calls of jsonvalue's decoders in the parser
// packages that bypass DecodeJSON, by file and count, each with the reason
// the value it reads holds no object, so no member name can repeat.
var plainDecodes = map[string]struct {
	count  int
	reason string
}{
	"antigravity/message.go":     {1, "the decimal text of a big integer the protobuf reader gave"},
	"codex/globalstate.go":       {1, "an object key read as a JSON array of two strings"},
	"cody/recover.go":            {1, "a chat title found by a pattern: a JSON string"},
	"continuedev/load.go":        {1, "a header found by a pattern: a JSON string"},
	"continuedev/permissions.go": {1, "a quoted YAML scalar: a JSON string"},
	"copilotcli/files.go":        {1, "a quoted YAML scalar: a JSON string"},
	"goose/standing.go":          {1, "a quoted YAML scalar: a JSON string"},
	"hermes/yaml.go":             {1, "a quoted YAML scalar: a JSON string"},
	"kiro/conversation.go":       {1, "a conversation id found by a pattern, quoted: a JSON string"},
}

// walkedDocuments are the reviewed jsonvalue.NewDocument calls in the parser
// packages, by file and count, each with where the document's TakeRepeats
// goes, since the kit cannot see a Document a parser walks itself.
var walkedDocuments = map[string]struct {
	count int
	goes  string
}{
	"agentzero/chat.go":         {1, "recoverLogs gives it to NoteRepeats"},
	"cody/recover.go":           {1, "the recovery's ArrayWalk notes it in the parse (ArrayWalk.Artifact)"},
	"continuedev/load.go":       {1, "LoadSession gives it to NoteRepeats"},
	"openhands/conversation.go": {1, "loadJSON merges it into the repeats its caller gives NoteRepeats"},
	"qwencode/session.go":       {1, "logEntries gives it to NoteRepeats"},
	"roocode/decisions.go":      {1, "decisionText gives it to NoteRepeats"},
}

// TestParsersWalkOnlyReviewedDocuments holds every parser package to the
// reviewed list of jsonvalue.NewDocument calls: a Document a parser walks
// itself must give its TakeRepeats to NoteRepeats, which only a reader of
// the code can check, so a new call fails until it is reviewed.
func TestParsersWalkOnlyReviewedDocuments(t *testing.T) {
	found := map[string]int{}
	for _, file := range parserSources(t) {
		if calls := jsonvalueCalls(t, file, "NewDocument"); calls > 0 {
			found[filepath.ToSlash(file)] = calls
		}
	}
	for _, file := range slices.Sorted(maps.Keys(found)) {
		if found[file] != walkedDocuments[file].count {
			t.Errorf("%s makes %d Documents, %d reviewed; review where each one's repeats go", file,
				found[file], walkedDocuments[file].count)
		}
	}
	for file := range walkedDocuments {
		if found[file] == 0 {
			t.Errorf("%s is reviewed but makes no Document; remove its entry", file)
		}
	}
}

// TestParsersDecodeEvidenceThroughTheKit holds every parser package to
// decoding the JSON of its evidence through the kit (DecodeJSON), so a
// member name repeated within an object is reported: a call of
// jsonvalue.Decode, DecodeString or DecodePrefix outside the reviewed list
// fails.
func TestParsersDecodeEvidenceThroughTheKit(t *testing.T) {
	found := map[string]int{}
	for _, file := range parserSources(t) {
		if calls := jsonvalueCalls(t, file, "Decode", "DecodeString", "DecodePrefix"); calls > 0 {
			found[filepath.ToSlash(file)] = calls
		}
	}
	for _, file := range slices.Sorted(maps.Keys(found)) {
		if found[file] != plainDecodes[file].count {
			t.Errorf("%s decodes JSON %d times without the kit, %d reviewed; decode evidence with DecodeJSON",
				file, found[file], plainDecodes[file].count)
		}
	}
	for file := range plainDecodes {
		if found[file] == 0 {
			t.Errorf("%s is reviewed but no longer decodes without the kit; remove its entry", file)
		}
	}
}

// TestParsersKeepTheRepeatsOfAStateDatabase holds every parser package to
// keeping the repeated member names vscodestate's readers return (for
// NoteRepeats): a call of vscodestate.ReadItem or ReadItems whose last
// result is not kept fails.
func TestParsersKeepTheRepeatsOfAStateDatabase(t *testing.T) {
	for _, file := range parserSources(t) {
		for _, line := range droppedStateRepeats(t, file) {
			t.Errorf("%s:%d drops the repeats a vscodestate reader returns; pass them to NoteRepeats",
				filepath.ToSlash(file), line)
		}
	}
}

// parsersDirectory holds one package per parser.
var parsersDirectory = filepath.Join("..", "parsers")

// parserSources returns the files of the parser packages, tests left out,
// relative to parsersDirectory, failing when fewer packages than the
// parsers are found.
func parserSources(t *testing.T) []string {
	t.Helper()
	var files []string
	packages := 0
	err := filepath.WalkDir(parsersDirectory, func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && filepath.Dir(file) == parsersDirectory:
			packages++
		case !entry.IsDir() && filepath.Dir(file) != parsersDirectory && strings.HasSuffix(file, ".go") &&
			!strings.HasSuffix(file, "_test.go"):
			relative, err := filepath.Rel(parsersDirectory, file)
			if err != nil {
				return err
			}
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if packages < 30 {
		t.Fatalf("looked at %d parser packages, want every one", packages)
	}
	return files
}

// droppedStateRepeats returns the lines of file where a vscodestate reader
// is called and its last result, the repeats, is not given a name.
func droppedStateRepeats(t *testing.T, file string) []int {
	t.Helper()
	files := token.NewFileSet()
	parsed, err := goparser.ParseFile(files, filepath.Join(parsersDirectory, file), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	kept := map[*ast.CallExpr]bool{}
	var lines []int
	ast.Inspect(parsed, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok && len(assign.Rhs) == 1 && len(assign.Lhs) == 3 {
			if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
				last, named := assign.Lhs[2].(*ast.Ident)
				kept[call] = !named || last.Name != "_"
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || !isStateReader(call) {
			return true
		}
		if !kept[call] {
			lines = append(lines, files.Position(call.Pos()).Line)
		}
		return true
	})
	return lines
}

// isStateReader reports whether call is of vscodestate.ReadItem or
// ReadItems.
func isStateReader(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name, ok := selector.X.(*ast.Ident)
	return ok && name.Name == "vscodestate" && (selector.Sel.Name == "ReadItem" || selector.Sel.Name == "ReadItems")
}

// jsonvalueCalls counts the uses in file of the named functions of
// jsonvalue.
func jsonvalueCalls(t *testing.T, file string, names ...string) int {
	t.Helper()
	parsed, err := goparser.ParseFile(token.NewFileSet(), filepath.Join(parsersDirectory, file), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if name, ok := selector.X.(*ast.Ident); ok && name.Name == "jsonvalue" &&
			slices.Contains(names, selector.Sel.Name) {
			calls++
		}
		return true
	})
	return calls
}
