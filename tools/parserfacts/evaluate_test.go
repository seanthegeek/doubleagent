package parserfacts

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strings"
)

// pieceKind says what one piece of a string template stands for.
type pieceKind int

const (
	// constantPiece is text the code fixes.
	constantPiece pieceKind = iota
	// holePiece is text known only at run time.
	holePiece
	// parameterPiece is the value of a parameter of the function being
	// read, or with element set, one element of a slice parameter.
	parameterPiece
	// relPiece is the artifact's path relative to the home.
	relPiece
)

// piece is one piece of a template: its kind, its text when constant,
// the parameter it stands for, and where the constant is written.
type piece struct {
	kind      pieceKind
	text      string
	parameter *types.Var
	element   bool
	position  token.Pos
	// indirect says a constant piece was reached through a variable, a
	// field or a bound argument, not written in the expression read.
	indirect bool
}

// template is the shape of a string the code can build, piece by piece.
type template []piece

// limits bound the evaluator: how deep it follows calls and variables, and
// how many templates one expression may stand for.
const (
	maxDepth     = 16
	maxTemplates = 64
)

// evaluator gives the templates an expression of the checked packages can
// have. A parameter of the function being read is bound in an
// environment when the call that reached it is known; unbound, it is a
// parameter piece, so the caller decides what it holds.
type evaluator struct {
	index *index
	// artifactRel is the Rel field of input.Artifact, read as a relPiece.
	artifactRel *types.Var
	// home is the import path of the package being read, and packageOf
	// gives the package a position lies in.
	home      string
	packageOf func(position token.Pos) string
	// isAgent reports whether an import path is a parser's own package;
	// only the fields such a package declares are followed to their
	// stored values, the kit's being the evidence's.
	isAgent func(path string) bool
	// facts holds the fields whose value is a fact (a row's text), read
	// as a hole: what a row holds so far is the evidence's.
	facts map[*types.Var]bool
	// visiting holds the variables and functions on the evaluation path,
	// so a cycle (a variable assigned from itself) ends in a hole.
	visiting map[types.Object]bool
}

// environment binds parameters to the templates their arguments have.
type environment map[*types.Var][]template

// unknown returns the one template of an expression nothing fixes.
func unknown() []template {
	return []template{{{kind: holePiece}}}
}

// strings returns the templates an expression of type string can have.
func (e *evaluator) strings(expression ast.Expr, bound environment, depth int) []template {
	if depth > maxDepth {
		return unknown()
	}
	if value := e.index.info.Types[expression].Value; value != nil && value.Kind() == constant.String {
		return []template{{{kind: constantPiece, text: constant.StringVal(value), position: expression.Pos()}}}
	}
	switch typed := ast.Unparen(expression).(type) {
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return unknown()
		}
		return join(e.strings(typed.X, bound, depth+1), e.strings(typed.Y, bound, depth+1))
	case *ast.Ident:
		return e.variable(e.index.object(typed), bound, depth)
	case *ast.SelectorExpr:
		return e.selector(typed, bound, depth)
	case *ast.CallExpr:
		return e.call(typed, bound, depth)
	}
	return unknown()
}

// join returns every concatenation of a template of left with one of
// right, at most maxTemplates.
func join(left, right []template) []template {
	var joined []template
	for _, first := range left {
		for _, second := range right {
			if len(joined) == maxTemplates {
				return joined
			}
			joined = append(joined, append(append(template{}, first...), second...))
		}
	}
	return joined
}

// union appends the templates of more to found, at most maxTemplates.
func union(found, more []template) []template {
	for _, each := range more {
		if len(found) == maxTemplates {
			break
		}
		found = append(found, each)
	}
	return found
}

// variable returns the templates of a variable: its binding, a parameter
// piece for an unbound parameter, else every value assigned to it or the
// elements of what it ranges over.
func (e *evaluator) variable(object types.Object, bound environment, depth int) []template {
	variable, ok := object.(*types.Var)
	if !ok || e.visiting[variable] {
		return unknown()
	}
	if values, ok := bound[variable]; ok {
		return indirect(values)
	}
	if _, ok := e.index.parameters[variable]; ok {
		return []template{{{kind: parameterPiece, parameter: variable}}}
	}
	e.visiting[variable] = true
	defer delete(e.visiting, variable)
	var found []template
	for _, value := range e.index.assigned[variable] {
		if !e.neverTrue(e.index.guards[value]) {
			found = union(found, e.strings(value, bound, depth+1))
		}
	}
	if over, ok := e.index.ranges[variable]; ok {
		if over.isKey {
			found = union(found, e.mapKeys(over.over, bound, depth+1))
		} else {
			found = union(found, e.elements(over.over, bound, depth+1))
		}
	}
	if len(found) == 0 {
		return unknown()
	}
	return indirect(found)
}

// indirect returns copies of the templates with each constant piece
// marked as reached through a variable, field or argument rather than
// written in the expression itself.
func indirect(found []template) []template {
	marked := make([]template, len(found))
	for i, each := range found {
		marked[i] = append(template{}, each...)
		for j := range marked[i] {
			if marked[i][j].kind == constantPiece {
				marked[i][j].indirect = true
			}
		}
	}
	return marked
}

// selector returns the templates of a selector: the artifact's relative
// path, a package-level variable, or every value stored in a field.
func (e *evaluator) selector(expression *ast.SelectorExpr, bound environment, depth int) []template {
	selection := e.index.info.Selections[expression]
	if selection == nil {
		return e.variable(e.index.info.Uses[expression.Sel], bound, depth)
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok || selection.Kind() != types.FieldVal {
		return unknown()
	}
	if field == e.artifactRel {
		return []template{{{kind: relPiece}}}
	}
	if e.facts[field] || field.Pkg() == nil || !e.isAgent(field.Pkg().Path()) {
		return unknown()
	}
	return e.stored(field, depth)
}

// stored returns the templates of every value stored in a field anywhere
// in the checked packages, each read without bindings, those under a
// condition that never holds left out. A field nothing is stored in, of a
// struct some composite literal builds, holds its zero value, "".
func (e *evaluator) stored(field *types.Var, depth int) []template {
	if e.visiting[field] {
		return unknown()
	}
	e.visiting[field] = true
	defer delete(e.visiting, field)
	var found []template
	for _, value := range e.homeValues(field) {
		if !e.neverTrue(e.index.guards[value]) {
			found = union(found, e.strings(value, nil, depth+1))
		}
	}
	if len(found) == 0 && e.built(field, false) && isString(field.Type()) {
		return []template{{{kind: constantPiece, indirect: true}}}
	}
	if len(found) == 0 {
		return unknown()
	}
	return indirect(found)
}

// homeValues returns the values stored in a field: those written in the
// home package when it writes any or builds the field's struct, else all
// of them. A field a shared package declares is set by each parser that
// uses it, and the parser being read sets its own.
func (e *evaluator) homeValues(field *types.Var) []ast.Expr {
	values := e.index.fields[field]
	var home []ast.Expr
	for _, value := range values {
		if e.packageOf(value.Pos()) == e.home {
			home = append(home, value)
		}
	}
	if len(home) > 0 || e.built(field, true) {
		return home
	}
	return values
}

// built reports whether a composite literal of the struct holding a field
// is written in the home package, or with anywhere set, in any package.
func (e *evaluator) built(field *types.Var, home bool) bool {
	for shape, literals := range e.index.structs {
		for i := range shape.NumFields() {
			if shape.Field(i) != field {
				continue
			}
			for _, literal := range literals {
				if !home || e.packageOf(literal.Pos()) == e.home {
					return true
				}
			}
		}
	}
	return false
}

// formatVerb matches one verb of a fmt format string.
var formatVerb = regexp.MustCompile(`%[-+# 0]*(?:\d+|\*)?(?:\.(?:\d+|\*)?)?(?:\[\d+\])?[a-zA-Z%]`)

// call returns the templates of a call's result: a conversion's operand, a
// format string with its %s and %v arguments, a regular expression's
// source, a quoted constant, or the returns of a function of the checked
// packages with its parameters bound to the call's arguments.
func (e *evaluator) call(call *ast.CallExpr, bound environment, depth int) []template {
	if kind := e.index.info.Types[call.Fun]; kind.IsType() && len(call.Args) == 1 {
		return e.strings(call.Args[0], bound, depth+1)
	}
	called := e.index.callee(call)
	if called == nil {
		return e.literalCall(call, bound, depth)
	}
	if called.Pkg() != nil {
		switch called.Pkg().Path() + "." + called.Name() {
		case "fmt.Sprintf":
			return e.format(call, bound, depth)
		case "regexp.MustCompile", "regexp.Compile":
			return e.strings(call.Args[0], bound, depth+1)
		case "regexp.QuoteMeta":
			return quoteMeta(e.strings(call.Args[0], bound, depth+1))
		}
	}
	declaration := e.index.bodies[called.Origin()]
	if declaration == nil || declaration.Body == nil {
		return unknown()
	}
	return e.returns(called.Origin(), declaration.Body, called.Signature(), call, bound, depth)
}

// literalCall returns the templates of a call of a variable holding a
// function literal, the literal's returns with its parameters bound.
func (e *evaluator) literalCall(call *ast.CallExpr, bound environment, depth int) []template {
	name, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return unknown()
	}
	variable, ok := e.index.object(name).(*types.Var)
	if !ok {
		return unknown()
	}
	var found []template
	for _, value := range e.index.assigned[variable] {
		literal, ok := ast.Unparen(value).(*ast.FuncLit)
		if !ok {
			continue
		}
		signature, _ := e.index.info.Types[literal].Type.(*types.Signature)
		if signature != nil {
			found = union(found, e.returns(variable, literal.Body, signature, call, bound, depth))
		}
	}
	if len(found) == 0 {
		return unknown()
	}
	return found
}

// returns gives the templates of the first result of every return in a
// function body (not in the function literals inside it), its parameters
// bound to the call's arguments.
func (e *evaluator) returns(key types.Object, body *ast.BlockStmt, signature *types.Signature,
	call *ast.CallExpr, bound environment, depth int) []template {
	if e.visiting[key] {
		return unknown()
	}
	e.visiting[key] = true
	defer delete(e.visiting, key)
	inner := e.bind(signature, call, bound, depth)
	var found []template
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(typed.Results) > 0 {
				found = union(found, e.strings(typed.Results[0], inner, depth+1))
			}
		}
		return true
	})
	if len(found) == 0 {
		return unknown()
	}
	return found
}

// bind returns the environment of a call: each string parameter bound to
// the templates of its argument, read in the caller's environment.
func (e *evaluator) bind(signature *types.Signature, call *ast.CallExpr, bound environment,
	depth int) environment {
	inner := environment{}
	list := signature.Params()
	for i := range list.Len() {
		if i >= len(call.Args) || (signature.Variadic() && i == list.Len()-1) {
			break
		}
		if isString(list.At(i).Type()) {
			inner[list.At(i)] = e.strings(call.Args[i], bound, depth+1)
		}
	}
	return inner
}

// format returns the templates of a fmt.Sprintf call: the format's text
// with each %s and %v replaced by its argument's templates, any other verb
// a hole.
func (e *evaluator) format(call *ast.CallExpr, bound environment, depth int) []template {
	value := e.index.info.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.String {
		return unknown()
	}
	text := constant.StringVal(value)
	position := call.Args[0].Pos()
	found := []template{{}}
	argument := 1
	last := 0
	for _, place := range formatVerb.FindAllStringIndex(text, -1) {
		found = join(found, []template{{{kind: constantPiece, text: text[last:place[0]], position: position}}})
		verb := text[place[0]:place[1]]
		switch {
		case verb == "%%":
			found = join(found, []template{{{kind: constantPiece, text: "%", position: position}}})
			last = place[1]
			continue
		case (verb == "%s" || verb == "%v") && argument < len(call.Args):
			found = join(found, e.strings(call.Args[argument], bound, depth+1))
		default:
			found = join(found, unknown())
		}
		argument++
		last = place[1]
	}
	return join(found, []template{{{kind: constantPiece, text: text[last:], position: position}}})
}

// quoteMeta returns the templates with each constant piece quoted as
// regexp.QuoteMeta quotes it.
func quoteMeta(found []template) []template {
	quoted := make([]template, len(found))
	for i, each := range found {
		for _, part := range each {
			if part.kind == constantPiece {
				part.text = regexp.QuoteMeta(part.text)
			}
			quoted[i] = append(quoted[i], part)
		}
	}
	return quoted
}

// isString reports whether a type's underlying type is string.
func isString(kind types.Type) bool {
	basic, ok := kind.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// elements returns the templates of the elements of a slice or array
// expression: a composite literal's, an append's, a variable's or field's
// values, or one element piece of a slice parameter.
func (e *evaluator) elements(expression ast.Expr, bound environment, depth int) []template {
	if depth > maxDepth {
		return unknown()
	}
	switch typed := ast.Unparen(expression).(type) {
	case *ast.CompositeLit:
		var found []template
		for _, element := range typed.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				element = pair.Value
			}
			if _, nested := element.(*ast.CompositeLit); !nested {
				found = union(found, e.strings(element, bound, depth+1))
			}
		}
		return found
	case *ast.Ident:
		return e.elementsOf(e.index.object(typed), bound, depth)
	case *ast.SelectorExpr:
		if selection := e.index.info.Selections[typed]; selection != nil {
			return e.elementsOf(selection.Obj(), bound, depth)
		}
		return e.elementsOf(e.index.info.Uses[typed.Sel], bound, depth)
	case *ast.CallExpr:
		if isBuiltinAppend(e.index, typed) {
			found := e.elements(typed.Args[0], bound, depth+1)
			for _, argument := range typed.Args[1:] {
				found = union(found, e.strings(argument, bound, depth+1))
			}
			return found
		}
		return e.returnedElements(typed, bound, depth)
	}
	return unknown()
}

// returnedElements returns the element templates of the slices a function
// of the checked packages returns, its parameters bound to the call's
// arguments.
func (e *evaluator) returnedElements(call *ast.CallExpr, bound environment, depth int) []template {
	called := e.index.callee(call)
	if called == nil || e.visiting[called.Origin()] {
		return unknown()
	}
	declaration := e.index.bodies[called.Origin()]
	if declaration == nil || declaration.Body == nil {
		return unknown()
	}
	e.visiting[called.Origin()] = true
	defer delete(e.visiting, called.Origin())
	inner := e.bind(called.Signature(), call, bound, depth)
	var found []template
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(typed.Results) > 0 {
				found = union(found, e.elements(typed.Results[0], inner, depth+1))
			}
		}
		return true
	})
	if len(found) == 0 {
		return unknown()
	}
	return found
}

// isBuiltinAppend reports whether a call calls the built-in append.
func isBuiltinAppend(lookup *index, call *ast.CallExpr) bool {
	name, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	_, builtin := lookup.info.Uses[name].(*types.Builtin)
	return builtin && name.Name == "append"
}

// elementsOf returns the element templates of a slice variable or field:
// an element piece for a parameter, else the elements of its values.
func (e *evaluator) elementsOf(object types.Object, bound environment, depth int) []template {
	variable, ok := object.(*types.Var)
	if !ok || e.visiting[variable] {
		return unknown()
	}
	if _, ok := e.index.parameters[variable]; ok && bound[variable] == nil {
		return []template{{{kind: parameterPiece, parameter: variable, element: true}}}
	}
	e.visiting[variable] = true
	defer delete(e.visiting, variable)
	values := e.index.assigned[variable]
	if variable.IsField() {
		values = e.index.fields[variable]
	}
	var found []template
	for _, value := range values {
		found = union(found, e.elements(value, bound, depth+1))
	}
	if len(found) == 0 {
		return unknown()
	}
	return found
}

// mapKeys returns the templates of the keys of a map composite literal,
// or of the map literals assigned to a variable.
func (e *evaluator) mapKeys(expression ast.Expr, bound environment, depth int) []template {
	switch typed := ast.Unparen(expression).(type) {
	case *ast.CompositeLit:
		var found []template
		for _, element := range typed.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				found = union(found, e.strings(pair.Key, bound, depth+1))
			}
		}
		return found
	case *ast.Ident:
		variable, ok := e.index.object(typed).(*types.Var)
		if !ok || depth > maxDepth {
			return unknown()
		}
		var found []template
		for _, value := range e.index.assigned[variable] {
			found = union(found, e.mapKeys(value, bound, depth+1))
		}
		return found
	}
	return unknown()
}

// render returns a template as text, each piece that is not constant
// written {}.
func render(found template) string {
	var text strings.Builder
	for _, part := range found {
		if part.kind == constantPiece {
			text.WriteString(part.text)
		} else {
			text.WriteString("{}")
		}
	}
	return text.String()
}

// renderWritten returns a template as text, each piece that is not a
// constant written in the expression read (a variable's or argument's
// value, or a run-time value) written {}.
func renderWritten(found template) string {
	var text strings.Builder
	for _, part := range found {
		if part.kind == constantPiece && !part.indirect {
			text.WriteString(part.text)
		} else if text.Len() == 0 || !strings.HasSuffix(text.String(), "{}") {
			text.WriteString("{}")
		}
	}
	return text.String()
}

// neverTrue reports whether a condition is false for the parser being
// read: a boolean field its home package builds the struct without, or
// sets only to false, or a string compared unequal to "" that is always
// "". Any other condition may hold.
func (e *evaluator) neverTrue(condition ast.Expr) bool {
	switch typed := ast.Unparen(condition).(type) {
	case *ast.SelectorExpr:
		selection := e.index.info.Selections[typed]
		if selection == nil || selection.Kind() != types.FieldVal {
			return false
		}
		field, _ := selection.Obj().(*types.Var)
		return e.unsetInHome(field)
	case *ast.BinaryExpr:
		if typed.Op != token.NEQ {
			return false
		}
		for _, sides := range [][2]ast.Expr{{typed.X, typed.Y}, {typed.Y, typed.X}} {
			if value := e.index.info.Types[sides[1]].Value; value != nil && value.Kind() == constant.String &&
				constant.StringVal(value) == "" {
				return e.alwaysEmpty(sides[0])
			}
		}
	}
	return false
}

// unsetInHome reports whether the home package builds a field's struct
// and never sets the boolean field to anything but the constant false.
func (e *evaluator) unsetInHome(field *types.Var) bool {
	if !e.built(field, true) {
		return false
	}
	for _, value := range e.index.fields[field] {
		if e.packageOf(value.Pos()) != e.home {
			continue
		}
		known := e.index.info.Types[value].Value
		if known == nil || known.Kind() != constant.Bool || constant.BoolVal(known) {
			return false
		}
	}
	return true
}

// alwaysEmpty reports whether every template of a string is the constant
// empty string.
func (e *evaluator) alwaysEmpty(expression ast.Expr) bool {
	for _, shape := range e.strings(expression, nil, 0) {
		if render(shape) != "" {
			return false
		}
	}
	return true
}
