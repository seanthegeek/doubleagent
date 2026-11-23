package parserfacts

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// problemPrefix opens the text of a problem row; a row text that starts
// with it is a problem message.
const problemPrefix = "parser: "

// target is one registered parser: its agent, the import path of its
// package and the name of its type there.
type target struct {
	Agent    string `json:"agent"`
	Package  string `json:"package"`
	TypeName string `json:"type"`
	// Home is the package of the function that constructs the parser,
	// which sets its settings: the type's own package unless a parser is
	// another's type with its own settings.
	Home string `json:"home"`
	// Secondary and the marks are the parser's own answers at run time.
	Secondary bool `json:"secondary"`
	Sync      bool `json:"sync"`
	Relay     bool `json:"relay"`
	Origin    bool `json:"origin"`
}

// fact is one key, row text or problem message a parser package passes to
// the kit, with the file and line of its first constant piece. Text is
// the whole template, each run-time piece written {}; Prefix is its
// constant start.
type fact struct {
	Package string   `json:"package"`
	Kind    factKind `json:"kind"`
	Text    string   `json:"text"`
	Prefix  string   `json:"prefix"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
}

// patternFact is one file pattern of a parser, Secondary when its
// Secondary method tests it, Resolved when every piece is constant.
type patternFact struct {
	Text      string `json:"text"`
	Regex     bool   `json:"regex"`
	Secondary bool   `json:"secondary"`
	Resolved  bool   `json:"resolved"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Package   string `json:"package"`
}

// parserFacts is a target with its file patterns and the agent packages it
// reads with (its own and those it imports).
type parserFacts struct {
	target
	Packages []string      `json:"packages"`
	Patterns []patternFact `json:"patterns"`
}

// report is the whole output.
type report struct {
	Parsers []parserFacts `json:"parsers"`
	Facts   []fact        `json:"facts"`
}

// settings say where the module is and which of its packages hold parsers.
type settings struct {
	root   string
	module string
	tags   []string
	// overlays maps an import path to a directory outside the module, for
	// a test's synthetic packages.
	overlays map[string]string
	// registry is the import path of the package whose constructors
	// function lists the parsers' constructors in registry order, "" when
	// each target's home is its own package.
	registry string
	// isAgent reports whether an import path is a parser's own package,
	// whose literals are the parsers' rather than the kit's.
	isAgent func(path string) bool
}

// analyze loads the targets' packages and returns their facts.
func analyze(config settings, targets []target) (report, error) {
	program := newProgram(config.root, config.module, config.tags)
	for _, each := range targets {
		var err error
		if dir, ok := config.overlays[each.Package]; ok {
			_, err = program.load(each.Package, dir)
		} else {
			_, err = program.Import(each.Package)
		}
		if err != nil {
			return report{}, err
		}
	}
	if err := setHomes(program, config, targets); err != nil {
		return report{}, err
	}
	packages := program.sorted()
	indexed := newIndex(program.info, packages)
	reader := &evaluator{index: indexed, visiting: map[types.Object]bool{}, artifactRel: artifactRel(program),
		isAgent: config.isAgent, packageOf: func(position token.Pos) string {
			return packageOf(program, program.files.Position(position).Filename)
		}}
	found := newSinks(reader, config.module, packages)
	reader.facts = map[*types.Var]bool{}
	for field := range found.fields {
		reader.facts[field] = true
	}
	found.derive(packages)
	result := report{Facts: collect(program, found, packages, config)}
	for _, each := range targets {
		facts, err := parserOf(program, reader, each, config)
		if err != nil {
			return report{}, err
		}
		result.Parsers = append(result.Parsers, facts)
	}
	return result, nil
}

// artifactRel returns the Rel field of input.Artifact, nil when the
// package was not loaded.
func artifactRel(program *program) *types.Var {
	found, ok := program.packages[program.module+"/internal/input"]
	if !ok {
		return nil
	}
	named, _ := found.checked.Scope().Lookup("Artifact").Type().(*types.Named)
	if named == nil {
		return nil
	}
	shape, _ := named.Underlying().(*types.Struct)
	for i := range shape.NumFields() {
		if shape.Field(i).Name() == "Rel" {
			return shape.Field(i)
		}
	}
	return nil
}

// collect returns the facts of the agent packages: each sink argument's
// templates whose first piece is a constant written in an agent package.
func collect(program *program, found *sinks, packages []*loaded, config settings) []fact {
	seen := map[fact]bool{}
	var facts []fact
	for _, each := range packages {
		if !config.isAgent(each.path) {
			continue
		}
		found.evaluator.home = each.path
		found.visitSinks(each.files, func(given argument) {
			for _, shape := range found.templates(given) {
				item, ok := factOf(program, given.kind, shape, config)
				if ok && !seen[item] {
					seen[item] = true
					facts = append(facts, item)
				}
			}
		})
	}
	return facts
}

// factOf returns the fact a template is, when it starts with a constant
// an agent package writes: a key only when it is that one constant, a
// text or message by that constant start, a text opening "parser: " a
// message. A message with a constant the kit writes is the kit's wording
// (a member name followed by the kit's "is not a JSON array"), so it is
// left to the key it names. A start with no letter is punctuation.
func factOf(program *program, kind factKind, shape template, config settings) (fact, bool) {
	if len(shape) == 0 || shape[0].kind != constantPiece || !shape[0].position.IsValid() {
		return fact{}, false
	}
	if kind == keyFact && len(shape) != 1 {
		return fact{}, false
	}
	position := program.files.Position(shape[0].position)
	owner := packageOf(program, position.Filename)
	if owner == "" || !config.isAgent(owner) {
		return fact{}, false
	}
	text, prefix := render(shape), shape[0].text
	if kind == textFact && strings.HasPrefix(text, problemPrefix) {
		kind, text, prefix = messageFact, text[len(problemPrefix):], strings.TrimPrefix(prefix, problemPrefix)
	}
	if kind == messageFact {
		for _, part := range shape {
			if part.kind == constantPiece && !config.isAgent(packageOf(program, program.files.Position(part.position).Filename)) {
				return fact{}, false
			}
		}
		if written := strings.TrimPrefix(renderWritten(shape), problemPrefix); strings.ContainsFunc(written, unicode.IsLetter) {
			text = written
		}
	}
	if !strings.ContainsFunc(prefix, unicode.IsLetter) {
		return fact{}, false
	}
	return fact{Package: owner, Kind: kind, Text: text, Prefix: prefix,
		File: relative(program.root, position.Filename), Line: position.Line}, true
}

// packageOf returns the import path of the loaded package holding a file.
func packageOf(program *program, filename string) string {
	dir := filepath.Dir(filename)
	for path, each := range program.packages {
		if each.dir == dir {
			return path
		}
	}
	return ""
}

// relative returns a file's path relative to the root, in slash form, or
// the path itself when it lies outside.
func relative(root, filename string) string {
	if path, err := filepath.Rel(root, filename); err == nil && !strings.HasPrefix(path, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(filename)
}

// parserOf returns a target's file patterns and its agent packages.
func parserOf(program *program, reader *evaluator, each target, config settings) (parserFacts, error) {
	found := parserFacts{target: each, Packages: agentImports(program, each.Home, config)}
	reader.home = each.Home
	own := program.packages[each.Package]
	if own == nil {
		return found, errors.New("package not loaded: " + each.Package)
	}
	object := own.checked.Scope().Lookup(each.TypeName)
	if object == nil {
		return found, errors.New("no type " + each.TypeName + " in " + each.Package)
	}
	methods := types.NewMethodSet(types.NewPointer(object.Type()))
	kit := config.module + "/internal/parserkit"
	for _, name := range []string{"Kinds", "Secondary"} {
		selection := methods.Lookup(own.checked, name)
		if selection == nil {
			continue
		}
		method, _ := selection.Obj().(*types.Func)
		declaration := reader.index.bodies[method.Origin()]
		if declaration == nil || declaration.Body == nil {
			continue
		}
		walker := &patternWalker{evaluator: reader, kit: kit, seen: map[*types.Func]bool{}}
		if name == "Kinds" {
			walker.kinds(declaration.Body, 0)
		} else {
			walker.body(declaration.Body, environment{}, 0, true)
		}
		found.Patterns = append(found.Patterns, patternFacts(program, walker.found, name == "Secondary")...)
	}
	return found, nil
}

// patternFacts returns the patterns as output records, each once.
func patternFacts(program *program, found []pattern, secondary bool) []patternFact {
	var facts []patternFact
	for _, each := range found {
		position := program.files.Position(each.position)
		item := patternFact{Text: each.text, Regex: each.regex, Secondary: secondary, Resolved: each.resolved,
			File: relative(program.root, position.Filename), Line: position.Line,
			Package: packageOf(program, position.Filename)}
		if !slices.Contains(facts, item) {
			facts = append(facts, item)
		}
	}
	return facts
}

// agentImports returns a package and every agent package it imports, at
// any depth, sorted.
func agentImports(program *program, path string, config settings) []string {
	seen := map[string]bool{}
	var visit func(each *types.Package)
	visit = func(each *types.Package) {
		if seen[each.Path()] || !config.isAgent(each.Path()) {
			return
		}
		seen[each.Path()] = true
		for _, imported := range each.Imports() {
			visit(imported)
		}
	}
	if own := program.packages[path]; own != nil {
		visit(own.checked)
	}
	found := make([]string, 0, len(seen))
	for each := range seen {
		found = append(found, each)
	}
	slices.Sort(found)
	return found
}

// setHomes sets each target's home: the package of the New function its
// constructor in the registry calls, else its own package. The registry's
// constructors are function literals in the order of the targets.
func setHomes(program *program, config settings, targets []target) error {
	for i := range targets {
		targets[i].Home = targets[i].Package
	}
	if config.registry == "" {
		return nil
	}
	registry, err := program.load(config.registry, filepath.Join(config.root,
		filepath.FromSlash(strings.TrimPrefix(config.registry, config.module+"/"))))
	if err != nil {
		return err
	}
	var homes []string
	for _, file := range registry.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "constructors" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if literal, ok := node.(*ast.FuncLit); ok {
					homes = append(homes, constructedIn(program, literal))
					return false
				}
				return true
			})
		}
	}
	if len(homes) != len(targets) {
		return fmt.Errorf("%s lists %d constructors for %d parsers", config.registry, len(homes), len(targets))
	}
	for i, home := range homes {
		if home != "" {
			targets[i].Home = home
		}
	}
	return nil
}

// constructedIn returns the package of the first function a constructor
// literal calls, "" when it calls none.
func constructedIn(program *program, literal *ast.FuncLit) string {
	home := ""
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || home != "" {
			return home == ""
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			if called, ok := program.info.Uses[selector.Sel].(*types.Func); ok && called.Pkg() != nil {
				home = called.Pkg().Path()
			}
		}
		return home == ""
	})
	return home
}
