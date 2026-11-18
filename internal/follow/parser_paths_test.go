package follow

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// pathBuilders are the path/filepath names that make one path from another.
// A parser package uses none of them outside its tests: it takes every path
// it opens from the artifact or from the parser kit, which looks at each
// directory on the way down and refuses a link, so a name read from the
// evidence can never lead a parser out of the collection. The kit itself, in
// internal/parser, is where these calls are made.
func pathBuilders() []string {
	return []string{"Abs", "Clean", "Dir", "EvalSymlinks", "Glob", "Join", "Rel", "Walk", "WalkDir"}
}

func TestNoParserPackageBuildsAPath(t *testing.T) {
	const kit = "../parser"
	packages := 0
	err := filepath.WalkDir(kit, func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && filepath.Dir(file) == kit:
			packages++
			return nil
		case entry.IsDir() || filepath.Dir(file) == kit || !strings.HasSuffix(file, ".go") ||
			strings.HasSuffix(file, "_test.go"):
			return nil
		}
		for _, call := range pathBuilderCalls(t, file) {
			t.Errorf("%s builds a path with %s; take the path from the parser kit", filepath.ToSlash(file), call)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if packages < 30 {
		t.Fatalf("looked at %d parser packages under %s, want every one", packages, kit)
	}
}

// pathBuilderCalls returns each use in file of a pathBuilders name of
// path/filepath, under whatever name the file imports the package.
func pathBuilderCalls(t *testing.T, file string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	local := ""
	for _, spec := range parsed.Imports {
		if spec.Path.Value != `"path/filepath"` {
			continue
		}
		local = "filepath"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	var calls []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if owner, isName := selector.X.(*ast.Ident); isName && local != "" && owner.Name == local &&
			slices.Contains(pathBuilders(), selector.Sel.Name) {
			calls = append(calls, "filepath."+selector.Sel.Name)
		}
		return true
	})
	return calls
}

func TestPathBuilderCallsFindsAnAliasedImport(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sample.go")
	source := "package sample\n\nimport paths \"path/filepath\"\n\n" +
		"func build(name string) string { return paths.Join(paths.Base(name), \"x\") }\n"
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pathBuilderCalls(t, file); !slices.Equal(got, []string{"filepath.Join"}) {
		t.Errorf("found %q, want the one Join and not the Base", got)
	}
}
