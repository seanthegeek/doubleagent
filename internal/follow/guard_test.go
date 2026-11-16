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

func TestNoFollowingCallOutsideThisPackage(t *testing.T) {
	found, err := scanTree("../..")
	if err != nil {
		t.Fatal(err)
	}
	checkFoundCalls(t, found)
}

// scanTree returns, by file relative to root, what followingCalls finds in
// every Go file under root that is not a test, not vendored and not in an
// owner package, in every directory of the module.
func scanTree(root string) (map[string][]string, error) {
	found := map[string][]string{}
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && (entry.Name() == "vendor" || strings.HasPrefix(entry.Name(), ".")) &&
			file != root:
			return filepath.SkipDir
		case entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go"):
			return nil
		}
		relative := strings.TrimPrefix(filepath.ToSlash(file), filepath.ToSlash(root)+"/")
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
	return found, err
}

func TestTheScanCoversEveryDirectoryButTheOwnersOwn(t *testing.T) {
	root := t.TempDir()
	source := "package x\nimport \"os\"\nvar _, _ = os.Stat(\"x\")\n"
	for _, file := range []string{"pkgx/s.go", "internal/stage/sub/s.go", "internal/stage/s.go",
		"vendor/v/s.go", "pkgx/s_test.go"} {
		full := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	var scanned []string
	for file, calls := range found {
		if len(calls) > 0 {
			scanned = append(scanned, file)
		}
	}
	slices.Sort(scanned)
	if want := []string{"internal/stage/sub/s.go", "pkgx/s.go"}; !slices.Equal(scanned, want) {
		t.Errorf("files with findings %q, want %q: a file outside internal and a subpackage "+
			"of an owner are scanned, the owner's own files, vendor and tests are not", scanned, want)
	}
}

// owned reports whether file, relative to the repository, is in one of the
// owner packages: its directory is exactly an owner's, so a subpackage of an
// owner is scanned like any other.
func owned(file string) bool {
	return slices.Contains(ownerPackages(), path.Dir(file))
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
			t.Errorf("%s is not reviewed: route a path call through package follow, or list "+
				"the name or import in guard_lists_test.go with the reason it is sound", call)
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

func TestGuardFindsEveryNameNotKnownToBeSafe(t *testing.T) {
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
		{"a listed safe name passes", "import \"path/filepath\"\nvar _ = filepath.Join(\"a\", \"b\")",
			nil},
		{"an unlisted name fails, harmless or not", "import \"os\"\nvar _ = os.Getpid()",
			[]string{"os.Getpid"}},
		{"an import nobody reviewed fails", "import \"debug/elf\"\nvar _, _ = elf.Open(\"x\")",
			[]string{"import of debug/elf"}},
		{"a call of os/exec fails", "import \"os/exec\"\nvar _ = exec.Command(\"sh\")",
			[]string{"exec.Command"}},
		{"a removal is counted, not safe",
			"import \"github.com/seanthegeek/doubleagent/internal/osfile\"\n" +
				"var _ = osfile.Remove(\"x\")",
			[]string{"osfile.Remove"}},
		{"an lstat needs no count",
			"import \"github.com/seanthegeek/doubleagent/internal/osfile\"\n" +
				"var _, _ = osfile.Lstat(\"x\")",
			nil},
		{"windows.Open fails",
			"import \"golang.org/x/sys/windows\"\nvar _, _ = windows.Open(\"x\", 0, 0)",
			[]string{"windows.Open"}},
		{"windows.Chmod fails", "import \"golang.org/x/sys/windows\"\nvar _ = windows.Chmod(\"x\", 0)",
			[]string{"windows.Chmod"}},
		{"unix.Clonefile fails",
			"import \"golang.org/x/sys/unix\"\nvar _ = unix.Clonefile(\"a\", \"b\", 0)",
			[]string{"unix.Clonefile"}},
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

// modulePath is the import path every package of this module starts with.
const modulePath = "github.com/seanthegeek/doubleagent/"

// followingCalls returns "file:line package.Name" for each reference in the
// Go source of fileName to an exported name of a guarded package that no
// list allows, under whatever local name its package is imported, "file:line
// import of P" for an import reviewedImports does not list, and "file:line
// dot import of P" for a dot import of a guarded package.
func followingCalls(fileName string, source []byte) ([]string, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, fileName, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	local, calls := guardedImports(files, parsed)
	allowed := map[string][]string{}
	for _, list := range []map[string][]string{safeNames(), noFollowNames()} {
		for importPath, names := range list {
			allowed[importPath] = append(allowed[importPath], names...)
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
		if ok && !slices.Contains(allowed[importPath], selector.Sel.Name) {
			calls = append(calls, files.Position(selector.Pos()).String()+" "+
				path.Base(importPath)+"."+selector.Sel.Name)
		}
		return true
	})
	return calls, nil
}

// guardedImports returns the local names of the file's guarded imports, by
// import path, and an entry for each import that is not reviewed and each
// dot import of a guarded package.
func guardedImports(files *token.FileSet, parsed *ast.File) (map[string]string, []string) {
	reviewed := reviewedImports()
	local, calls := map[string]string{}, []string(nil)
	for _, spec := range parsed.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		where := files.Position(spec.Pos()).String()
		module := strings.HasPrefix(importPath, modulePath)
		guarded := reviewed[importPath] == "guarded" ||
			slices.Contains(guardedModulePackages(), importPath)
		switch {
		case err != nil:
		case !module && reviewed[importPath] == "":
			calls = append(calls, where+" import of "+importPath)
		case !guarded:
		case spec.Name == nil:
			local[path.Base(importPath)] = importPath
		case spec.Name.Name == ".":
			calls = append(calls, where+" dot import of "+importPath)
		case spec.Name.Name != "_":
			local[spec.Name.Name] = importPath
		}
	}
	return local, calls
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
