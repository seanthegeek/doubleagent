package parserfacts

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
)

// program holds every package of the module the analysis type-checked,
// with one types.Info shared by all of them, so a node of any package
// finds its types and objects in the same maps.
type program struct {
	files      *token.FileSet
	info       *types.Info
	root       string
	module     string
	context    build.Context
	standard   types.ImporterFrom
	packages   map[string]*loaded
	inProgress map[string]bool
}

// loaded is one package of the module or of a test directory: its import
// path, its parsed files and its checked package.
type loaded struct {
	path    string
	dir     string
	files   []*ast.File
	checked *types.Package
}

// newProgram returns an empty program for the module whose root directory
// is root and whose module path is module. Packages outside the module
// (the standard library and the vendored modules) are checked from source
// by the standard importer; the module's own are parsed here, so their
// syntax and the objects in info are the ones every lookup uses.
func newProgram(root, module string, tags []string) *program {
	files := token.NewFileSet()
	context := build.Default
	context.BuildTags = append(slices.Clone(context.BuildTags), tags...)
	context.CgoEnabled = false
	standard, _ := importer.ForCompiler(files, "source", nil).(types.ImporterFrom)
	return &program{
		files: files, root: root, module: module, context: context, standard: standard,
		info: &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{},
			Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{},
		},
		packages: map[string]*loaded{}, inProgress: map[string]bool{},
	}
}

// Import returns the package at path, for a package with no directory of
// its own to import from.
func (p *program) Import(path string) (*types.Package, error) {
	return p.ImportFrom(path, p.root, 0)
}

// ImportFrom returns the package at path imported from the directory dir:
// a package of the module is parsed and checked here, any other is the
// standard importer's.
func (p *program) ImportFrom(path, dir string, mode types.ImportMode) (*types.Package, error) {
	if path == p.module || strings.HasPrefix(path, p.module+"/") {
		relative := strings.TrimPrefix(strings.TrimPrefix(path, p.module), "/")
		found, err := p.load(path, filepath.Join(p.root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, err
		}
		return found.checked, nil
	}
	return p.standard.ImportFrom(path, dir, mode)
}

// load parses and checks the package in dir under the import path path,
// once, with the files the build context selects (test files left out).
func (p *program) load(path, dir string) (*loaded, error) {
	if found, ok := p.packages[path]; ok {
		return found, nil
	}
	if p.inProgress[path] {
		return nil, errors.New("import cycle through " + path)
	}
	p.inProgress[path] = true
	defer delete(p.inProgress, path)
	listing, err := p.context.ImportDir(dir, 0)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	found := &loaded{path: path, dir: dir}
	for _, name := range listing.GoFiles {
		file, err := parser.ParseFile(p.files, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		found.files = append(found.files, file)
	}
	config := types.Config{Importer: p}
	found.checked, err = config.Check(path, p.files, found.files, p.info)
	if err != nil {
		return nil, fmt.Errorf("check %s: %w", path, err)
	}
	p.packages[path] = found
	return found, nil
}

// sorted returns the loaded packages in import path order.
func (p *program) sorted() []*loaded {
	paths := make([]string, 0, len(p.packages))
	for path := range p.packages {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	all := make([]*loaded, 0, len(paths))
	for _, path := range paths {
		all = append(all, p.packages[path])
	}
	return all
}
