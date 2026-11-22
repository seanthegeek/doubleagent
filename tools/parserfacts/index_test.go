package parserfacts

import (
	"go/ast"
	"go/token"
	"go/types"
)

// index holds what the evaluator looks up about the checked packages: the
// body of each function, the function each parameter belongs to, every
// value assigned to each variable and struct field, and what each range
// variable ranges over.
type index struct {
	info *types.Info
	// bodies maps a function to its declaration or literal.
	bodies map[*types.Func]*ast.FuncDecl
	// parameters maps a parameter to its place among its function's
	// parameters and whether it is variadic.
	parameters map[*types.Var]parameter
	// assigned maps a variable to the expressions assigned to it.
	assigned map[*types.Var][]ast.Expr
	// fields maps a struct field to the expressions stored in it, by
	// assignment or in a composite literal.
	fields map[*types.Var][]ast.Expr
	// ranges maps a range variable to what it ranges over.
	ranges map[*types.Var]rangeOver
	// literals maps a function literal's type to the literal.
	literals map[*types.Signature]*ast.FuncLit
	// functions maps a declared function's signature to the function.
	functions map[*types.Signature]*types.Func
	// guards maps a value assigned or stored inside the body of an if
	// statement to that statement's condition, the innermost one.
	guards map[ast.Node]ast.Expr
	// structs maps a struct type to the positions of its composite
	// literals.
	structs map[*types.Struct][]ast.Expr
}

// parameter is one parameter's place in its function's signature.
type parameter struct {
	signature *types.Signature
	place     int
	variadic  bool
}

// rangeOver is the expression a range variable ranges over, and whether
// the variable is the key (of a map) rather than the value.
type rangeOver struct {
	over  ast.Expr
	isKey bool
}

// newIndex indexes every file of the loaded packages.
func newIndex(info *types.Info, packages []*loaded) *index {
	found := &index{info: info, bodies: map[*types.Func]*ast.FuncDecl{},
		parameters: map[*types.Var]parameter{}, assigned: map[*types.Var][]ast.Expr{},
		fields: map[*types.Var][]ast.Expr{}, ranges: map[*types.Var]rangeOver{},
		literals: map[*types.Signature]*ast.FuncLit{}, functions: map[*types.Signature]*types.Func{},
		guards: map[ast.Node]ast.Expr{}, structs: map[*types.Struct][]ast.Expr{}}
	for _, each := range packages {
		for _, file := range each.files {
			ast.Inspect(file, found.visit)
		}
	}
	return found
}

// visit indexes one node.
func (x *index) visit(node ast.Node) bool {
	switch typed := node.(type) {
	case *ast.FuncDecl:
		if function, ok := x.info.Defs[typed.Name].(*types.Func); ok {
			x.bodies[function] = typed
			x.functions[function.Signature()] = function
			x.addParameters(function.Signature())
		}
	case *ast.FuncLit:
		if signature, ok := x.info.Types[typed].Type.(*types.Signature); ok {
			x.literals[signature] = typed
			x.addParameters(signature)
		}
	case *ast.AssignStmt:
		// x += y appends to x's value; it is no value of x.
		if typed.Tok == token.ASSIGN || typed.Tok == token.DEFINE {
			x.addAssignment(typed.Lhs, typed.Rhs)
		}
	case *ast.ValueSpec:
		names := make([]ast.Expr, len(typed.Names))
		for i, name := range typed.Names {
			names[i] = name
		}
		x.addAssignment(names, typed.Values)
	case *ast.RangeStmt:
		x.addRange(typed)
	case *ast.CompositeLit:
		x.addComposite(typed)
	case *ast.IfStmt:
		ast.Inspect(typed.Body, func(inner ast.Node) bool {
			if inner != nil {
				x.guards[inner] = typed.Cond
			}
			return true
		})
	}
	return true
}

// addParameters records the place of each parameter of a signature.
func (x *index) addParameters(signature *types.Signature) {
	list := signature.Params()
	for i := range list.Len() {
		x.parameters[list.At(i)] = parameter{signature: signature, place: i,
			variadic: signature.Variadic() && i == list.Len()-1}
	}
}

// addAssignment records each left side's value when the sides pair one to
// one; a multi-value call assigns nothing the evaluator can read.
func (x *index) addAssignment(left, right []ast.Expr) {
	if len(left) != len(right) {
		return
	}
	for i, target := range left {
		switch typed := ast.Unparen(target).(type) {
		case *ast.Ident:
			object := x.info.Defs[typed]
			if object == nil {
				object = x.info.Uses[typed]
			}
			if variable, ok := object.(*types.Var); ok {
				x.assigned[variable] = append(x.assigned[variable], right[i])
			}
		case *ast.SelectorExpr:
			if selection := x.info.Selections[typed]; selection != nil && selection.Kind() == types.FieldVal {
				field, _ := selection.Obj().(*types.Var)
				x.fields[field] = append(x.fields[field], right[i])
			}
		}
	}
}

// addRange records the range variables of a range statement.
func (x *index) addRange(statement *ast.RangeStmt) {
	for place, target := range []ast.Expr{statement.Key, statement.Value} {
		name, ok := target.(*ast.Ident)
		if !ok {
			continue
		}
		object := x.info.Defs[name]
		if object == nil {
			object = x.info.Uses[name]
		}
		if variable, ok := object.(*types.Var); ok {
			x.ranges[variable] = rangeOver{over: statement.X, isKey: place == 0}
		}
	}
}

// addComposite records the fields a struct composite literal sets, keyed
// or by place.
func (x *index) addComposite(literal *ast.CompositeLit) {
	shape, ok := types.Unalias(x.info.Types[literal].Type).Underlying().(*types.Struct)
	if !ok {
		return
	}
	x.structs[shape] = append(x.structs[shape], literal)
	for place, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := pair.Key.(*ast.Ident); ok {
				if field, ok := x.info.Uses[key].(*types.Var); ok {
					x.fields[field] = append(x.fields[field], pair.Value)
				}
			}
		} else if place < shape.NumFields() {
			field := shape.Field(place)
			x.fields[field] = append(x.fields[field], element)
		}
	}
}

// object returns the object an identifier defines or uses.
func (x *index) object(name *ast.Ident) types.Object {
	if object := x.info.Uses[name]; object != nil {
		return object
	}
	return x.info.Defs[name]
}

// callee returns the function or method a call calls statically, nil for
// a call through a function value, an interface or a conversion.
func (x *index) callee(call *ast.CallExpr) *types.Func {
	switch function := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		called, _ := x.info.Uses[function].(*types.Func)
		return called
	case *ast.SelectorExpr:
		if selection := x.info.Selections[function]; selection != nil {
			if selection.Kind() == types.MethodVal {
				called, _ := selection.Obj().(*types.Func)
				if _, isInterface := selection.Recv().Underlying().(*types.Interface); isInterface {
					return nil
				}
				return called
			}
			return nil
		}
		called, _ := x.info.Uses[function.Sel].(*types.Func)
		return called
	}
	return nil
}

// functionOf returns the declared function whose signature this is, nil
// for a function literal's.
func (x *index) functionOf(signature *types.Signature) *types.Func {
	return x.functions[signature]
}
