package parser

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

// TestParsersDecodeEvidenceThroughTheKit holds every parser package to
// decoding the JSON of its evidence through the kit (DecodeJSON, or a
// Document whose repeats reach NoteRepeats), so a member name repeated
// within an object is reported: a call of jsonvalue.Decode, DecodeString
// or DecodePrefix outside the reviewed list fails.
func TestParsersDecodeEvidenceThroughTheKit(t *testing.T) {
	found := map[string]int{}
	packages := 0
	err := filepath.WalkDir(".", func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && filepath.Dir(file) == ".":
			packages++
			return nil
		case entry.IsDir() || filepath.Dir(file) == "." || !strings.HasSuffix(file, ".go") ||
			strings.HasSuffix(file, "_test.go"):
			return nil
		}
		if calls := plainDecodeCalls(t, file); calls > 0 {
			found[filepath.ToSlash(file)] = calls
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if packages < 30 {
		t.Fatalf("looked at %d parser packages, want every one", packages)
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

// plainDecodeCalls counts the calls in file of jsonvalue's Decode,
// DecodeString and DecodePrefix functions.
func plainDecodeCalls(t *testing.T, file string) int {
	t.Helper()
	parsed, err := goparser.ParseFile(token.NewFileSet(), file, nil, 0)
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
			slices.Contains([]string{"Decode", "DecodeString", "DecodePrefix"}, selector.Sel.Name) {
			calls++
		}
		return true
	})
	return calls
}
