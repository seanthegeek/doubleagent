package follow

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// keptNamesFile lists the exported names of the packages under internal that
// stay exported with no caller in another package's code, each with its
// reason.
const keptNamesFile = "testdata/exported-names-kept.txt"

// sourceFile is what the export check needs of one Go file: its directory
// relative to the module root in slash form, whether it is test code, its
// package name, and what exportedDeclarations finds in it.
type sourceFile struct {
	directory, packageName string
	test                   bool
	declared, types        []string
	reaches                map[string][]string
	parsed                 *ast.File
}

// exportUse says which kinds of file of another package name an exported
// name: a non-test file of a package that is not a test helper, or any other.
type exportUse struct {
	code, test bool
}

func TestEveryExportedNameHasACallerInAnotherPackage(t *testing.T) {
	breaches, err := unusedExports("../..")
	if err != nil {
		t.Fatal(err)
	}
	kept, err := readKeptNames(keptNamesFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range breaches {
		if _, listed := kept[name]; !listed {
			t.Errorf("%s is exported but no non-test file of another package names it: "+
				"unexport or remove it, or list it with its reason in %s", name, keptNamesFile)
		}
	}
	for name := range kept {
		if !slices.Contains(breaches, name) {
			t.Errorf("%s in %s no longer applies: it is used in another package, unexported or gone",
				name, keptNamesFile)
		}
	}
}

// unusedExports returns, sorted, each exported top-level name of a package
// under root's internal directory, as "<directory under internal>.<Name>",
// that neither passes by name (namedExports) nor, for a type, is reached
// from a name that passes (reachedTypes). Methods and struct fields are not
// checked: telling whose method a selector names takes type checking, and a
// method can be used only through an interface its type satisfies.
func unusedExports(root string) ([]string, error) {
	files, err := readSourceFiles(root)
	if err != nil {
		return nil, err
	}
	declared, passing, err := namedExports(files)
	if err != nil {
		return nil, err
	}
	reachedTypes(files, passing)
	var unused []string
	for _, key := range declared {
		if !passing[key] {
			unused = append(unused, key)
		}
	}
	slices.Sort(unused)
	return slices.Compact(unused), nil
}

// namedExports returns every exported top-level name of the packages under
// internal, as unusedExports spells it, and the set of those some file of
// another package names as <package>.<Name> in the way it needs: a non-test
// file of a package under internal or cmd that is not a test helper, or, for
// a name of a test helper (a package whose name ends in "test"), any file.
// Every file is read whatever its build constraints.
func namedExports(files []sourceFile) ([]string, map[string]bool, error) {
	names := map[string]string{}
	for _, file := range files {
		if !file.test {
			names[file.directory] = file.packageName
		}
	}
	uses := map[string]exportUse{}
	for _, file := range files {
		if err := recordUses(file, names, uses); err != nil {
			return nil, nil, err
		}
	}
	var declared []string
	passing := map[string]bool{}
	for _, file := range files {
		if file.test || !strings.HasPrefix(file.directory, "internal/") {
			continue
		}
		helper := strings.HasSuffix(file.packageName, "test")
		for _, name := range file.declared {
			key := strings.TrimPrefix(file.directory, "internal/") + "." + name
			declared = append(declared, key)
			if use := uses[key]; use.code || (helper && use.test) {
				passing[key] = true
			}
		}
	}
	return declared, passing, nil
}

// reachedTypes adds to passing, until nothing more is added, each exported
// type of a package that a passing name of the same package reaches, as
// exportedDeclarations gives the reaches: another package can hold a value
// of such a type without naming it.
func reachedTypes(files []sourceFile, passing map[string]bool) {
	types, reaches := map[string]bool{}, map[string][]string{}
	for _, file := range files {
		if file.test || !strings.HasPrefix(file.directory, "internal/") {
			continue
		}
		prefix := strings.TrimPrefix(file.directory, "internal/") + "."
		for _, name := range file.types {
			types[prefix+name] = true
		}
		for name, targets := range file.reaches {
			for _, target := range targets {
				reaches[prefix+name] = append(reaches[prefix+name], prefix+target)
			}
		}
	}
	for added := true; added; {
		added = false
		for name := range passing {
			for _, target := range reaches[name] {
				if types[target] && !passing[target] {
					passing[target], added = true, true
				}
			}
		}
	}
}

// recordUses adds to uses each selector of file that names an exported name
// of another package under internal through the name file imports it by. A
// selector whose left side is a declared variable or parameter is not a
// package's name and is passed over; a dot import is an error, since what it
// brings in is named with no package at all.
func recordUses(file sourceFile, names map[string]string, uses map[string]exportUse) error {
	imported := map[string]string{}
	for _, spec := range file.parsed.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return err
		}
		directory, inModule := strings.CutPrefix(importPath, modulePath)
		if !inModule || !strings.HasPrefix(directory, "internal/") || directory == file.directory {
			continue
		}
		local := names[directory]
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == "." {
			return errors.New(file.directory + ": dot import of " + importPath + "; name the package")
		}
		imported[local] = strings.TrimPrefix(directory, "internal/")
	}
	code := !file.test && !strings.HasSuffix(file.packageName, "test")
	ast.Inspect(file.parsed, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		owner, isName := selector.X.(*ast.Ident)
		if !isName || owner.Obj != nil || imported[owner.Name] == "" {
			return true
		}
		key := imported[owner.Name] + "." + selector.Sel.Name
		use := uses[key]
		use.code = use.code || code
		use.test = use.test || !code
		uses[key] = use
		return true
	})
	return nil
}

// readSourceFiles parses every Go file under root's internal and cmd
// directories, testdata directories excepted, keeping each file's
// declarations, imports and selectors with its local names resolved, so a
// selector on a variable can be told from one on a package.
func readSourceFiles(root string) ([]sourceFile, error) {
	var files []sourceFile
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(file string, entry fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case entry.IsDir() && entry.Name() == "testdata":
				return filepath.SkipDir
			case entry.IsDir() || !strings.HasSuffix(file, ".go"):
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, filepath.Dir(file))
			if err != nil {
				return err
			}
			declared, types, reaches := exportedDeclarations(parsed)
			files = append(files, sourceFile{
				directory:   filepath.ToSlash(relative),
				packageName: parsed.Name.Name,
				test:        strings.HasSuffix(file, "_test.go"),
				declared:    declared,
				types:       types,
				reaches:     reaches,
				parsed:      parsed,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// exportedDeclarations returns the exported top-level functions, types,
// variables and constants parsed declares, the exported types among them,
// and, by declared name, the type names a value of it can lead another
// package to: those of an exported function's or method's parameters and
// results (a method's under its receiver's type), of an exported variable's
// or constant's declared type, and of a type's underlying type.
func exportedDeclarations(parsed *ast.File) (names, types []string, reaches map[string][]string) {
	reaches = map[string][]string{}
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if !declaration.Name.IsExported() {
				continue
			}
			owner := declaration.Name.Name
			if declaration.Recv == nil {
				names = append(names, owner)
			} else {
				owner = receiverName(declaration.Recv.List[0].Type)
			}
			reaches[owner] = append(reaches[owner], typeNames(declaration.Type)...)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						names, types = append(names, spec.Name.Name), append(types, spec.Name.Name)
						reaches[spec.Name.Name] = append(reaches[spec.Name.Name], typeNames(spec.Type)...)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							names = append(names, name.Name)
							reaches[name.Name] = append(reaches[name.Name], typeNames(spec.Type)...)
						}
					}
				}
			}
		}
	}
	return names, types, reaches
}

// fieldTypes returns the typeNames of each field of fields, or, when
// members is set, of each exported or embedded one.
func fieldTypes(fields *ast.FieldList, members bool) []string {
	var found []string
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		if !members || len(field.Names) == 0 || slices.ContainsFunc(field.Names, (*ast.Ident).IsExported) {
			found = append(found, typeNames(field.Type)...)
		}
	}
	return found
}

// receiverName returns the name of a method receiver's type, without its
// pointer or its type parameters.
func receiverName(receiver ast.Expr) string {
	for {
		switch shape := receiver.(type) {
		case *ast.StarExpr:
			receiver = shape.X
		case *ast.IndexExpr:
			receiver = shape.X
		case *ast.IndexListExpr:
			receiver = shape.X
		case *ast.Ident:
			return shape.Name
		default:
			return ""
		}
	}
}

// typeNames returns the unqualified names in a type expression, through
// pointers, slices, maps, channels, function signatures and type arguments,
// taking of a struct only its exported and embedded fields and of an
// interface only its exported methods and embedded types. An array's length
// is passed over, so a constant gets no pass from a type.
func typeNames(expression ast.Node) []string {
	var found []string
	if expression == nil {
		return nil
	}
	ast.Inspect(expression, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Ident:
			found = append(found, node.Name)
		case *ast.SelectorExpr:
			return false
		case *ast.ArrayType:
			found = append(found, typeNames(node.Elt)...)
			return false
		case *ast.FuncType:
			found = append(found, fieldTypes(node.Params, false)...)
			found = append(found, fieldTypes(node.Results, false)...)
			return false
		case *ast.StructType:
			found = append(found, fieldTypes(node.Fields, true)...)
			return false
		case *ast.InterfaceType:
			found = append(found, fieldTypes(node.Methods, true)...)
			return false
		}
		return true
	})
	return found
}

// readKeptNames returns the names the kept list at file holds, each mapped
// to its reason. A line is "<directory under internal>.<Name> <reason>"; a
// blank line or one starting with # is a comment. A line with no reason, or
// a name listed twice, is an error.
func readKeptNames(file string) (map[string]string, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	kept := map[string]string{}
	scanner := bufio.NewScanner(handle)
	for number := 1; scanner.Scan(); number++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, reason, _ := strings.Cut(line, " ")
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s:%d: %s has no reason", file, number, name)
		}
		if _, repeated := kept[name]; repeated {
			return nil, fmt.Errorf("%s:%d: %s is listed twice", file, number, name)
		}
		kept[name] = strings.TrimSpace(reason)
	}
	return kept, scanner.Err()
}

func TestUnusedExportsCountsOnlyAnotherPackagesCode(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"internal/library/library.go": "package library\n\nconst Aliased, Shadowed, TestOnly = 1, 2, 3\n\n" +
			"var WindowsOnly, Own int\n\nvar FromCommand Holder\n\n" +
			"func Use(List[Argument], chan Sent, func(Called)) (*Wrapper, map[Key]int) { return nil, nil }\n",
		"internal/library/types.go": "package library\n\ntype List[T any] []T\n\n" +
			"type (\n\tArgument int\n\tSent int\n\tCalled int\n\tKey int\n\tElement int\n\tEmbedded int\n" +
			"\tInner int\n\tHidden int\n\tGot int\n\tOrphan int\n\tUnreached int\n)\n\n" +
			"type Wrapper []Element\n\ntype Holder struct {\n\tEmbedded\n\tField Inner\n\thidden Hidden\n}\n\n" +
			"func (h *Holder) Get() []Got { return nil }\n\nfunc orphan() Orphan { return 0 }\n\n" +
			"func (h *Holder) unexported() Unreached { return 0 }\n\n" +
			"type Level int\n\nconst Typed, Untouched Level = 1, 2\n\nconst Count = 4\n\ntype Sized [Count]Level\n\n" +
			"var Lone Sized\n",
		"internal/helpers/helpers.go": "package helpertest\n\nfunc Fixture() {}\n\nfunc Unused() {}\n",
		"internal/user/user.go": "package user\n\nimport lib \"" + modulePath + "internal/library\"\n\n" +
			"var _ = lib.Aliased + lib.Use()\n\nfunc shadow(library struct{ Shadowed int }) int " +
			"{ return library.Shadowed }\n",
		"internal/user/user_windows.go": "//go:build windows\n\npackage user\n\nimport \"" + modulePath +
			"internal/library\"\n\nvar _ = library.WindowsOnly\n",
		"internal/user/user_test.go": "package user\n\nimport (\n\t\"" + modulePath + "internal/library\"\n\t\"" +
			modulePath + "internal/helpers\"\n)\n\nvar _, _ = library.TestOnly, helpertest.Fixture\n",
		"internal/library/library_test.go": "package library_test\n\nimport \"" + modulePath +
			"internal/library\"\n\nvar _ = library.Own\n",
		"cmd/tool/main.go": "package main\n\nimport \"" + modulePath + "internal/library\"\n\n" +
			"func main() { _, _ = library.FromCommand, library.Typed }\n",
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := unusedExports(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"helpers.Unused", "library.Count", "library.Hidden", "library.Lone",
		"library.Orphan", "library.Own", "library.Shadowed", "library.Sized", "library.TestOnly",
		"library.Unreached", "library.Untouched"}
	if !slices.Equal(got, want) {
		t.Errorf("found %q, want %q: an alias, a Windows file and cmd count, a parameter, "+
			"the package's own tests and another package's tests do not, a helper's name "+
			"passes on a test's use, and a type passes when a passing name reaches it", got, want)
	}
	dot := filepath.Join(root, "internal", "user", "dot.go")
	if err := os.WriteFile(dot, []byte("package user\n\nimport . \""+modulePath+"internal/library\"\n\n"+
		"var _ = Aliased\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := unusedExports(root); err == nil || !strings.Contains(err.Error(), "dot import") {
		t.Errorf("a dot import gave %v, want an error naming it", err)
	}
}
