package osfile

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// pathFunctions returns the functions of os, path/filepath and io/ioutil, by
// import path, that take a path and reach the filesystem. A Windows build
// must call them through this package, so every source and staged path gets
// the \\?\ form.
func pathFunctions() map[string]map[string]bool {
	return map[string]map[string]bool{
		"os": setOf("Chdir", "Chmod", "Chown", "Chtimes", "CopyFS", "Create", "CreateTemp",
			"DirFS", "Lchown", "Link", "Lstat", "Mkdir", "MkdirAll", "MkdirTemp", "Open",
			"OpenFile", "OpenInRoot", "OpenRoot", "ReadDir", "ReadFile", "Readlink", "Remove",
			"RemoveAll", "Rename", "Stat", "Symlink", "Truncate", "WriteFile"),
		"path/filepath": setOf("EvalSymlinks", "Glob", "Walk", "WalkDir"),
		"io/ioutil":     setOf("ReadDir", "ReadFile", "TempDir", "TempFile", "WriteFile"),
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
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		calls, err := directPathCalls(path, source)
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

func TestGuardFindsEveryWayOfNamingAPathFunction(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{"a call through the package name is found",
			"import \"os\"\nvar _, _ = os.Lstat(\"x\")", []string{"os.Lstat"}},
		{"a call through an alias is found",
			"import system \"os\"\nvar _, _ = system.Lstat(\"x\")", []string{"os.Lstat"}},
		{"an aliased filepath is found",
			"import walker \"path/filepath\"\nvar _ = walker.WalkDir", []string{"path/filepath.WalkDir"}},
		{"a dot import fails outright",
			"import . \"os\"\nvar _, _ = Lstat(\"x\")", []string{"dot import of os"}},
		{"an ioutil call is found",
			"import \"io/ioutil\"\nvar _, _ = ioutil.ReadFile(\"x\")", []string{"io/ioutil.ReadFile"}},
		{"os.CopyFS is found",
			"import \"os\"\nvar _ = os.CopyFS(\"x\", nil)", []string{"os.CopyFS"}},
		{"a function of os that takes no path passes",
			"import \"os\"\nvar _ = os.Getenv(\"HOME\")", nil},
		{"a local value named os is not the package",
			"import system \"os\"\nvar _ = system.Getenv\ntype files struct{}\n" +
				"func (files) Lstat() {}\nvar os files\nvar _ = os.Lstat", nil},
		{"a blank import passes", "import _ \"os\"", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			calls, err := directPathCalls("guard.go", []byte("package guard\n"+test.source+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, call := range calls {
				_, found, _ := strings.Cut(call, " ")
				got = append(got, found)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("found %q, want %q", calls, test.want)
			}
		})
	}
}

// directPathCalls returns "file:line package.Name" for each reference in the
// Go source of the file fileName to a function in pathFunctions, whatever
// local name its package is imported under, and "file:line dot import of
// package" for a dot import of one of those packages, whose functions could
// then be called by their bare names.
func directPathCalls(fileName string, source []byte) ([]string, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, fileName, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	banned := pathFunctions()
	local, calls := map[string]string{}, []string(nil)
	for _, spec := range parsed.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || banned[importPath] == nil {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		switch name {
		case ".":
			calls = append(calls, files.Position(spec.Pos()).String()+" dot import of "+importPath)
		case "_":
		default:
			local[name] = importPath
		}
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && banned[local[pkg.Name]][selector.Sel.Name] {
			calls = append(calls, files.Position(selector.Pos()).String()+" "+local[pkg.Name]+"."+
				selector.Sel.Name)
		}
		return true
	})
	return calls, nil
}
