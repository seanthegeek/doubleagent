package parserfacts

import (
	"go/ast"
	"go/types"
	"strings"
)

// factKind is what a string the parsers pass to the kit is to the reader
// of the evidence: a record key, the text of a row, or a problem message.
type factKind string

const (
	keyFact     factKind = "key"
	textFact    factKind = "text"
	messageFact factKind = "message"
)

// sink is one parameter whose argument is a fact: its place, its kind,
// whether the argument is a slice whose elements are the facts, and for a
// text or message the rest the function appends after it (tail).
type sink struct {
	place int
	kind  factKind
	slice bool
	tail  string
}

// maxTails bounds the tails one parameter's sinks may have.
const maxTails = 8

// baseSinks returns the kit's methods and functions whose arguments are
// facts, by package path relative to the module, receiver type and name:
// the JSON, SQLite, TOML, protobuf and editor-state lookups whose string
// names a member, column, table, key or field of the evidence, a piece of
// a row's text, and the problem row. A function that passes its own
// parameter to one of these is a sink too (derive), so the kit's wrappers
// need no entry here.
func baseSinks() map[string][]sink {
	key := func(place int) []sink { return []sink{{place: place, kind: keyFact}} }
	keys := func(place int) []sink { return []sink{{place: place, kind: keyFact, slice: true}} }
	return map[string][]sink{
		"internal/jsonvalue.Value.Get": key(0), "internal/jsonvalue.Value.Lookup": key(0),
		"internal/jsonvalue.Value.Has": key(0),
		"internal/sqlitefile.Row.Get":  key(0), "internal/sqlitefile.Row.Has": key(0),
		"internal/sqlitefile.Row.Equals": key(0), "internal/sqlitefile.Row.Cmp": key(0),
		"internal/sqlitefile.Table.Has": key(0), "internal/sqlitefile.Table.Select": keys(0),
		"internal/sqlitefile.Query.Lookup": key(0), "internal/sqlitefile.Query.OrderBy": keys(0),
		"internal/sqlitefile.Lookup.Find": key(1), "internal/sqlitefile.Lookup.First": key(1),
		"internal/sqlitefile.Database.Table": key(0), "internal/sqlitefile.Database.HasTable": key(0),
		"internal/parserkit.Tables.Has": key(0), "internal/tomldoc.Table.Get": key(0),
		"internal/protowire.Message.Get": key(0), "internal/protowire.Message.Text": key(0),
		"internal/protowire.Message.Int": key(0), "internal/protowire.Message.Child": key(0),
		"internal/protowire.Message.List": key(0),
		"internal/vscodestate.ReadItem":   key(1), "internal/vscodestate.ReadItems": keys(1),
		"internal/parserkit.Base.ProblemRow":  {{place: 1, kind: messageFact}},
		"internal/timeline.TextBuilder.Write": {{place: 0, kind: textFact}},
	}
}

// baseFields returns the struct fields whose value is a fact wherever it
// is stored: a row's text and a problem's message.
func baseFields() map[string]factKind {
	return map[string]factKind{
		"internal/timeline.Row.Text":         textFact,
		"internal/parserkit.Problem.Message": messageFact,
	}
}

// sinks finds the facts of the checked packages: the base sinks above and
// every function whose parameter reaches one.
type sinks struct {
	evaluator *evaluator
	module    string
	base      map[string][]sink
	fields    map[*types.Var]factKind
	derived   map[*types.Func][]sink
	// tails holds the pieces of each derived sink's tail.
	tails map[derivedSink]template
}

// derivedSink is one sink of one function.
type derivedSink struct {
	function *types.Func
	sink     sink
}

// functionKey returns the name a function or method has in baseSinks,
// "" for one outside the module.
func functionKey(module string, function *types.Func) string {
	if function.Pkg() == nil || !strings.HasPrefix(function.Pkg().Path(), module+"/") {
		return ""
	}
	name := strings.TrimPrefix(function.Pkg().Path(), module+"/") + "."
	if receiver := function.Signature().Recv(); receiver != nil {
		kind := receiver.Type()
		if pointer, ok := kind.(*types.Pointer); ok {
			kind = pointer.Elem()
		}
		if named, ok := types.Unalias(kind).(*types.Named); ok {
			name += named.Obj().Name() + "."
		}
	}
	return name + function.Name()
}

// fieldKey returns the name a struct field has in baseFields: the package
// path relative to the module, the struct's type name and the field's.
func fieldKey(module string, field *types.Var, owner *types.Named) string {
	if owner == nil || field.Pkg() == nil {
		return ""
	}
	return strings.TrimPrefix(field.Pkg().Path(), module+"/") + "." + owner.Obj().Name() + "." + field.Name()
}

// newSinks returns the sinks of the checked packages, with the base field
// sinks resolved to their field objects.
func newSinks(evaluator *evaluator, module string, packages []*loaded) *sinks {
	found := &sinks{evaluator: evaluator, module: module, base: baseSinks(), fields: map[*types.Var]factKind{},
		derived: map[*types.Func][]sink{}, tails: map[derivedSink]template{}}
	fields := baseFields()
	for _, each := range packages {
		scope := each.checked.Scope()
		for _, name := range scope.Names() {
			named, ok := scope.Lookup(name).Type().(*types.Named)
			if !ok {
				continue
			}
			shape, ok := named.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for i := range shape.NumFields() {
				if kind, ok := fields[fieldKey(module, shape.Field(i), named)]; ok {
					found.fields[shape.Field(i)] = kind
				}
			}
		}
	}
	return found
}

// of returns the sinks of a called function: its base sinks, else the
// ones derived for it.
func (s *sinks) sinksOf(function *types.Func) []sink {
	if function == nil {
		return nil
	}
	if base, ok := s.base[functionKey(s.module, function)]; ok {
		return base
	}
	return s.derived[function.Origin()]
}

// argument is one argument a sink receives: the expressions holding its
// facts, whether each holds a slice of them, and the tail the sink
// appends.
type argument struct {
	kind        factKind
	expressions []ast.Expr
	slice       bool
	tail        template
}

// arguments returns the arguments a call passes to the sinks of its
// callee. A variadic sink's arguments are each one fact, or with an
// ellipsis one slice of them.
func (s *sinks) arguments(call *ast.CallExpr) []argument {
	called := s.evaluator.index.callee(call)
	var found []argument
	for _, each := range s.sinksOf(called) {
		if each.place >= len(call.Args) {
			continue
		}
		variadic := called.Signature().Variadic() && each.place == called.Signature().Params().Len()-1
		tail := s.tails[derivedSink{called.Origin(), each}]
		switch {
		case variadic && call.Ellipsis.IsValid():
			found = append(found, argument{each.kind, call.Args[each.place:], true, tail})
		case variadic:
			found = append(found, argument{each.kind, call.Args[each.place:], false, tail})
		default:
			found = append(found, argument{each.kind, call.Args[each.place : each.place+1], each.slice, tail})
		}
	}
	return found
}

// stores returns the arguments a node stores in a fact field: an
// assignment to the field or a composite literal setting it.
func (s *sinks) stores(node ast.Node) []argument {
	var found []argument
	switch typed := node.(type) {
	case *ast.AssignStmt:
		if len(typed.Lhs) != len(typed.Rhs) || typed.Tok.String() == "+=" {
			return nil
		}
		for i, target := range typed.Lhs {
			selector, ok := ast.Unparen(target).(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if selection := s.evaluator.index.info.Selections[selector]; selection != nil {
				field, _ := selection.Obj().(*types.Var)
				if kind, ok := s.fields[field]; ok {
					found = append(found, argument{kind, typed.Rhs[i : i+1], false, nil})
				}
			}
		}
	case *ast.CompositeLit:
		for _, element := range typed.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if name, ok := pair.Key.(*ast.Ident); ok {
				field, _ := s.evaluator.index.info.Uses[name].(*types.Var)
				if kind, ok := s.fields[field]; ok {
					found = append(found, argument{kind, []ast.Expr{pair.Value}, false, nil})
				}
			}
		}
	}
	return found
}

// templates returns the templates of one sink argument: each expression's
// templates, or a slice's element templates.
func (s *sinks) templates(each argument) []template {
	var found []template
	for _, expression := range each.expressions {
		if each.slice {
			found = union(found, s.evaluator.elements(expression, nil, 0))
		} else {
			found = union(found, s.evaluator.strings(expression, nil, 0))
		}
	}
	if len(each.tail) > 0 {
		return join(found, []template{each.tail})
	}
	return found
}

// visitSinks calls visit with every sink argument in the files.
func (s *sinks) visitSinks(files []*ast.File, visit func(argument)) {
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				for _, each := range s.arguments(call) {
					visit(each)
				}
			}
			for _, each := range s.stores(node) {
				visit(each)
			}
			return true
		})
	}
}

// derive marks, until nothing changes, each function parameter whose
// value reaches a sink: as the whole of a key, an element of a key slice,
// or the start of a text or message.
func (s *sinks) derive(packages []*loaded) {
	for changed := true; changed; {
		changed = false
		for _, each := range packages {
			s.visitSinks(each.files, func(found argument) {
				for _, shape := range s.templates(found) {
					if s.markParameter(found.kind, shape) {
						changed = true
					}
				}
			})
		}
	}
}

// markParameter records the sink a template's parameter piece makes of
// its function, and reports whether it is new.
func (s *sinks) markParameter(kind factKind, shape template) bool {
	if len(shape) == 0 || shape[0].kind != parameterPiece {
		return false
	}
	if kind == keyFact && len(shape) != 1 {
		return false
	}
	if kind != keyFact && shape[0].element {
		return false
	}
	place, ok := s.evaluator.index.parameters[shape[0].parameter]
	if !ok {
		return false
	}
	function := s.evaluator.index.functionOf(place.signature)
	if function == nil {
		return false
	}
	marked := sink{place: place.place, kind: kind, slice: shape[0].element}
	if kind != keyFact {
		marked.tail = render(shape[1:])
	}
	count := 0
	for _, known := range s.derived[function] {
		if known == marked {
			return false
		}
		if known.place == marked.place && known.kind == marked.kind {
			count++
		}
	}
	if count >= maxTails {
		return false
	}
	s.derived[function] = append(s.derived[function], marked)
	s.tails[derivedSink{function, marked}] = append(template{}, shape[1:]...)
	return true
}
