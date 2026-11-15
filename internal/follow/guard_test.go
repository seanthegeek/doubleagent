package follow

import (
	"go/ast"
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

// followingFunctions returns, by import path, the functions that follow a
// link in the last component of the path they are given.
func followingFunctions() map[string][]string {
	return map[string][]string{
		"os": {"Stat", "Open", "OpenFile", "ReadDir", "ReadFile", "Chdir", "DirFS"},
		"github.com/seanthegeek/doubleagent/internal/osfile": {"Stat", "Open", "OpenFile", "ReadDir",
			"ReadFile"},
		"path/filepath": {"EvalSymlinks", "Glob", "Walk", "WalkDir"},
		"github.com/seanthegeek/doubleagent/internal/fsmeta": {"DirReadError"},
		"github.com/seanthegeek/doubleagent/internal/stage":  {"PhysicalDir"},
		"golang.org/x/sys/unix": {"Access", "Faccessat", "Stat", "Open", "Openat",
			"Chdir"},
		"syscall":   {"Stat", "Open", "Access", "Chdir"},
		"io/ioutil": {"ReadDir", "ReadFile"},
	}
}

// ownerPackages are the directories under internal whose every file may call
// a following function: this package, the path wrappers it is built on, the
// metadata layer whose functions it wraps, and the staging layer, which
// works on the output directory and resolves the root the run is given.
func ownerPackages() []string {
	return []string{"follow", "osfile", "fsmeta", "stage"}
}

// allowance is how many calls of one following function one file may make,
// and why none of them can follow a link out of the image root.
type allowance struct {
	count  int
	reason string
}

// allowedCalls are the other calls of a following function, keyed by file
// and function, with their number and reason: a further call in the same
// file fails too.
func allowedCalls() map[string]allowance {
	allowed := map[string]allowance{}
	for _, entry := range []struct {
		file, call string
		count      int
		reason     string
	}{
		{"internal/archive/archive.go", "osfile.ReadDir", 1,
			"lists the staging tree the run built"},
		{"internal/archive/archive.go", "osfile.Open", 2,
			"reads files of the staging tree the run built"},
		{"internal/collect/options.go", "osfile.Stat", 1,
			"tests -r, -o and -p as the operator gives them"},
		{"internal/collect/run_finish.go", "osfile.Open", 1,
			"copies the run's own output files"},
		{"internal/collect/run_finish.go", "osfile.OpenFile", 1,
			"creates the run's own output files"},
		{"internal/collect/run_setup.go", "osfile.OpenFile", 1,
			"creates the probe file in the output directory"},
		{"internal/collect/run_setup.go", "stage.PhysicalDir", 1,
			"resolves the staging directory"},
		{"internal/collect/run_unix.go", "unix.Faccessat", 1,
			"tests the output directory -o names"},
		{"internal/glob/expand.go", "osfile.Stat", 1,
			"asked the root check of the path first"},
		{"internal/glob/expand.go", "osfile.Open", 1,
			"asked the root check of the directory first"},
		{"internal/projects/files.go", "osfile.Open", 1,
			"opens a directory its listing typed one, below a checked start"},
		{"internal/relocation/walk.go", "osfile.Open", 1,
			"opens a directory its listing typed one, below a checked base"},
		{"internal/runlog/runlog.go", "osfile.OpenFile", 1,
			"creates the run's log file"},
		{"internal/users/users_windows.go", "osfile.ReadDir", 1,
			"lists the parent of a candidate the root check let through"},
		{"internal/walk/project.go", "stage.PhysicalDir", 2,
			"resolves the image root the operator gave"},
		{"internal/walk/tree.go", "osfile.Open", 2,
			"opens a directory its own lstat typed a directory"},
	} {
		allowed[entry.file+" "+entry.call] = allowance{entry.count, entry.reason}
	}
	return allowed
}

func TestNoFollowingCallOutsideThisPackage(t *testing.T) {
	found := map[string][]string{}
	err := filepath.WalkDir("../..", func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && (entry.Name() == "vendor" || strings.HasPrefix(entry.Name(), ".")) &&
			file != "../..":
			return filepath.SkipDir
		case entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go"):
			return nil
		}
		relative := strings.TrimPrefix(filepath.ToSlash(file), "../../")
		if owned(relative) {
			return nil
		}
		source, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		calls, err := followingCalls(relative, source)
		found[relative] = append(found[relative], calls...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	checkFoundCalls(t, found)
}

// owned reports whether file, relative to the repository, is in an owner
// package or outside the binary's tree.
func owned(file string) bool {
	if !strings.HasPrefix(file, "internal/") && !strings.HasPrefix(file, "cmd/") {
		return true
	}
	for _, owner := range ownerPackages() {
		if strings.HasPrefix(file, "internal/"+owner+"/") {
			return true
		}
	}
	return false
}

// checkFoundCalls fails on every call not allowed, on every allowed call
// that no longer exists, and when nothing was found at all, which would mean
// the scan read nothing.
func checkFoundCalls(t *testing.T, found map[string][]string) {
	t.Helper()
	allowed, seen := allowedCalls(), map[string]int{}
	total := 0
	for file, calls := range found {
		for _, call := range calls {
			total++
			_, name, _ := strings.Cut(call, " ")
			key := file + " " + name
			if _, ok := allowed[key]; ok {
				seen[key]++
				continue
			}
			t.Errorf("%s follows a link unchecked: call the follow function with the root check", call)
		}
	}
	for key, entry := range allowed {
		if seen[key] != entry.count {
			t.Errorf("%q makes %d calls, %d allowed (%s): route a new one through this package, "+
				"or update the entry for one removed", key, seen[key], entry.count, entry.reason)
		}
	}
	if total == 0 {
		t.Fatal("the scan found no call of a following function anywhere; it is not reading the tree")
	}
}

func TestGuardFindsEveryWayOfNamingAFollowingFunction(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{"a call through the package name is found", "import \"os\"\nvar _, _ = os.Stat(\"x\")",
			[]string{"os.Stat"}},
		{"a call through an alias is found",
			"import files \"github.com/seanthegeek/doubleagent/internal/osfile\"\n" +
				"var _, _ = files.Stat(\"x\")",
			[]string{"osfile.Stat"}},
		{"a reference without a call is found", "import \"path/filepath\"\nvar _ = filepath.EvalSymlinks",
			[]string{"filepath.EvalSymlinks"}},
		{"a dot import fails outright", "import . \"os\"\nvar _, _ = Stat(\"x\")",
			[]string{"dot import of os"}},
		{"a function that does not follow passes", "import \"os\"\nvar _, _ = os.Lstat(\"x\")", nil},
		{"a local value named like the package is not it",
			"import system \"os\"\nvar _ = system.Getenv\ntype files struct{}\n" +
				"func (files) Stat() {}\nvar os files\nvar _ = os.Stat", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			calls, err := followingCalls("guard.go", []byte("package guard\n"+test.source+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, call := range calls {
				_, name, _ := strings.Cut(call, " ")
				got = append(got, name)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("found %q, want %q", calls, test.want)
			}
		})
	}
}

// followingCalls returns "file:line package.Name" for each reference in the
// Go source of fileName to a function followingFunctions names, under
// whatever local name its package is imported, and "file:line dot import of
// package" for a dot import of one of those packages.
func followingCalls(fileName string, source []byte) ([]string, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, fileName, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	banned := followingFunctions()
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
	shadowed := declaredNames(parsed)
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || shadowed[ident.Name] {
			return true
		}
		importPath, ok := local[ident.Name]
		if ok && slices.Contains(banned[importPath], selector.Sel.Name) {
			calls = append(calls, files.Position(selector.Pos()).String()+" "+
				path.Base(importPath)+"."+selector.Sel.Name)
		}
		return true
	})
	return calls, nil
}

// declaredNames returns the names the file declares at package level, which
// hide an imported package of the same name in the guard's simple reading.
func declaredNames(parsed *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range value.Names {
					names[name.Name] = true
				}
			}
		}
	}
	return names
}
