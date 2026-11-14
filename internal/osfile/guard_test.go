package osfile

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pathFunctions returns the functions of os and path/filepath, by package
// name, that take a path and reach the filesystem. A Windows build must call
// them through this package, so every source and staged path gets the \\?\
// form.
func pathFunctions() map[string]map[string]bool {
	return map[string]map[string]bool{
		"os": setOf("Chdir", "Chmod", "Chown", "Chtimes", "Create", "CreateTemp", "DirFS", "Lchown",
			"Link", "Lstat", "Mkdir", "MkdirAll", "MkdirTemp", "Open", "OpenFile", "OpenInRoot",
			"OpenRoot", "ReadDir", "ReadFile", "Readlink", "Remove", "RemoveAll", "Rename", "Stat",
			"Symlink", "Truncate", "WriteFile"),
		"filepath": setOf("EvalSymlinks", "Glob", "Walk", "WalkDir"),
	}
}

// setOf returns the set of names.
func setOf(names ...string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set
}

func TestWindowsBuildCallsNoPathFunctionOfOsDirectly(t *testing.T) {
	windows := build.Default
	windows.GOOS, windows.GOARCH, windows.CgoEnabled = "windows", "amd64", false
	var found []string
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && filepath.Base(path) == "osfile":
			return filepath.SkipDir
		case entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		matches, err := windows.MatchFile(filepath.Dir(path), filepath.Base(path))
		if err != nil || !matches {
			return err
		}
		calls, err := directPathCalls(path)
		found = append(found, calls...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range found {
		t.Errorf("%s: call the osfile function instead, so the path gets its \\\\?\\ form", call)
	}
}

// directPathCalls returns "file:line os.Name" for each call in the Go file
// at path of a function in pathFunctions.
func directPathCalls(path string) ([]string, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	banned := pathFunctions()
	var calls []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && banned[pkg.Name][selector.Sel.Name] {
			calls = append(calls, files.Position(selector.Pos()).String()+" "+pkg.Name+"."+
				selector.Sel.Name)
		}
		return true
	})
	return calls, nil
}
